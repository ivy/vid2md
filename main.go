package main

import (
	"context"
	"embed"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/subosito/gotenv"
)

// Cue represents a single subtitle/caption entry with timing.
type Cue struct {
	Start time.Duration
	End   time.Duration
	Text  string
}

// Frame represents an extracted video frame on disk.
type Frame struct {
	Timestamp time.Duration
	Path      string
}

// DescribedFrame holds a vision-model description for a frame.
type DescribedFrame struct {
	Timestamp   time.Duration
	Description string
}

// TimelineEntry is a unified transcript or visual event.
type TimelineEntry struct {
	Timestamp time.Duration
	Kind      string // "transcript" or "visual"
	Text      string
}

//go:embed prompts/*.md.tmpl
var promptFS embed.FS

var version = "dev"

var logLevel = new(slog.LevelVar)

func main() {
	_ = gotenv.Load()

	pflag.CommandLine.StringP("output", "o", "", "output file path (default: stdout)")
	pflag.CommandLine.String("s3-bucket-uri", "", "S3 bucket URI (s3://bucket/prefix)")
	pflag.CommandLine.String("aws-region", "us-east-1", "AWS region")
	pflag.CommandLine.String("language-code", "en-US", "transcription language")
	pflag.CommandLine.String("model", "gpt-5.2", "OpenAI model for final assembly")
	pflag.CommandLine.String("vision-model", "gpt-5.2", "OpenAI model for frame description")
	pflag.CommandLine.String("openai-base-url", "", "OpenAI API base URL override")
	pflag.CommandLine.Float64("scene-threshold", 0.3, "scene detection threshold")
	pflag.CommandLine.Bool("no-cache", false, "disable caching, use ephemeral temp dir")
	pflag.CommandLine.BoolP("verbose", "v", false, "enable verbose (INFO) logging")
	pflag.CommandLine.Bool("debug", false, "enable debug logging")
	pflag.CommandLine.Bool("version", false, "print version and exit")

	pflag.CommandLine.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: vid2md [flags] <video-file>\n\n")
		fmt.Fprintf(os.Stderr, "Converts a video into a structured Markdown document by\n")
		fmt.Fprintf(os.Stderr, "extracting audio, transcribing, capturing key frames,\n")
		fmt.Fprintf(os.Stderr, "and assembling everything with an LLM.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		pflag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  vid2md lecture.mp4\n")
		fmt.Fprintf(os.Stderr, "  vid2md -o notes.md --model gpt-5.2 talk.mp4\n")
		fmt.Fprintf(os.Stderr, "  vid2md --s3-bucket-uri s3://my-bucket/audio --verbose demo.mp4\n")
	}

	pflag.Parse()

	viper.SetConfigName("vid2md")
	viper.SetConfigType("toml")
	viper.AddConfigPath("$HOME/.config")
	_ = viper.ReadInConfig()
	_ = viper.BindPFlags(pflag.CommandLine)

	_ = viper.BindEnv("s3-bucket-uri", "AWS_S3_BUCKET_URI")
	_ = viper.BindEnv("aws-region", "AWS_REGION")
	_ = viper.BindEnv("model", "VID2MD_MODEL")
	_ = viper.BindEnv("openai-base-url", "OPENAI_BASE_URL")

	if viper.GetBool("version") {
		fmt.Println(version)
		os.Exit(0)
	}

	// Logging setup.
	switch {
	case viper.GetBool("debug"):
		logLevel.Set(slog.LevelDebug)
	case viper.GetBool("verbose"):
		logLevel.Set(slog.LevelInfo)
	default:
		logLevel.Set(slog.LevelWarn)
	}

	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		},
	})
	slog.SetDefault(slog.New(handler))

	args := pflag.Args()
	if len(args) != 1 {
		pflag.Usage()
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := run(ctx, args[0]); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, videoPath string) error {
	dir, cleanup, err := resolveDir(videoPath, viper.GetBool("no-cache"))
	if err != nil {
		return err
	}
	defer cleanup()

	slog.Info("cache dir", "path", dir)

	// Extract audio and detect/extract frames concurrently (with cache checks).
	type audioResult struct {
		path string
		err  error
	}
	type framesResult struct {
		frames []Frame
		err    error
	}

	audioCh := make(chan audioResult, 1)
	framesCh := make(chan framesResult, 1)

	go func() {
		audioPath := filepath.Join(dir, "audio.ogg")
		if _, err := os.Stat(audioPath); err == nil {
			slog.Info("audio cached", "path", audioPath)
			audioCh <- audioResult{audioPath, nil}
			return
		}
		slog.Info("extracting audio")
		path, err := extractAudio(ctx, videoPath, dir)
		audioCh <- audioResult{path, err}
	}()

	go func() {
		if frames, err := loadFrames(dir); err == nil {
			slog.Info("frames cached", "count", len(frames))
			framesCh <- framesResult{frames, nil}
			return
		}
		slog.Info("detecting scenes and extracting frames")
		frames, err := detectAndExtractFrames(ctx, videoPath, dir, viper.GetFloat64("scene-threshold"))
		if err != nil {
			framesCh <- framesResult{nil, err}
			return
		}
		if err := saveFrameTimestamps(dir, frames); err != nil {
			framesCh <- framesResult{nil, fmt.Errorf("save frame timestamps: %w", err)}
			return
		}
		framesCh <- framesResult{frames, nil}
	}()

	audioRes := <-audioCh
	if audioRes.err != nil {
		return fmt.Errorf("extract audio: %w", audioRes.err)
	}

	framesRes := <-framesCh
	if framesRes.err != nil {
		return fmt.Errorf("detect and extract frames: %w", framesRes.err)
	}

	// Transcribe (cached if transcript.vtt exists).
	vttPath := filepath.Join(dir, "transcript.vtt")
	var vttContent string
	if data, err := os.ReadFile(vttPath); err == nil {
		slog.Info("transcript cached", "path", vttPath)
		vttContent = string(data)
	} else {
		slog.Info("uploading audio to S3")
		s3URI, err := uploadToS3(ctx, audioRes.path, viper.GetString("s3-bucket-uri"), viper.GetString("aws-region"))
		if err != nil {
			return fmt.Errorf("upload to S3: %w", err)
		}

		slog.Info("transcribing audio")
		vttContent, err = transcribeAudio(ctx, s3URI, viper.GetString("s3-bucket-uri"), viper.GetString("aws-region"), viper.GetString("language-code"))
		if err != nil {
			return fmt.Errorf("transcribe audio: %w", err)
		}

		_ = os.WriteFile(vttPath, []byte(vttContent), 0o644)
		slog.Info("transcript saved", "path", vttPath)
	}

	slog.Info("parsing WebVTT")
	cues, err := parseWebVTT(vttContent)
	if err != nil {
		return fmt.Errorf("parse WebVTT: %w", err)
	}

	// Describe frames (individually cached via sidecar .txt files).
	slog.Info("describing frames")
	describedFrames, err := describeFrames(ctx, framesRes.frames, viper.GetString("vision-model"), viper.GetString("openai-base-url"))
	if err != nil {
		return fmt.Errorf("describe frames: %w", err)
	}

	// Interleave + assemble always re-run.
	slog.Info("interleaving timeline")
	timeline := interleave(cues, describedFrames)

	slog.Info("assembling markdown")
	markdown, err := assembleMarkdown(ctx, timeline, viper.GetString("model"), viper.GetString("openai-base-url"))
	if err != nil {
		return fmt.Errorf("assemble markdown: %w", err)
	}

	_ = os.WriteFile(filepath.Join(dir, "output.md"), []byte(markdown), 0o644)

	var w io.Writer
	if output := viper.GetString("output"); output != "" {
		f, err := os.Create(output)
		if err != nil {
			return fmt.Errorf("create output file: %w", err)
		}
		defer f.Close()
		w = f
	} else {
		w = os.Stdout
	}

	fmt.Fprint(w, markdown)
	return nil
}
