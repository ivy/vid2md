package main

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"time"
)

func extractAudio(ctx context.Context, videoPath, tmpDir string) (string, error) {
	outPath := filepath.Join(tmpDir, "audio.ogg")
	cmd := exec.CommandContext(ctx, "ffmpeg", "-i", videoPath, "-vn", "-c:a", "libopus", outPath)
	stderr, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("extractAudio: %w: %s", err, stderr)
	}
	return outPath, nil
}

func detectAndExtractFrames(ctx context.Context, videoPath, tmpDir string, threshold float64) ([]Frame, error) {
	// Pass 1 — scene detection
	vf := fmt.Sprintf("select='gt(scene,%s)',showinfo", strconv.FormatFloat(threshold, 'f', -1, 64))
	cmd := exec.CommandContext(ctx, "ffmpeg", "-i", videoPath, "-vf", vf, "-f", "null", "-")
	stderr, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("scene detection: %w: %s", err, stderr)
	}

	timestamps := parseSceneTimestamps(string(stderr))

	// Pass 2 — frame extraction
	var frames []Frame
	for i, ts := range timestamps {
		outPath := filepath.Join(tmpDir, "frames", fmt.Sprintf("frame_%03d.png", i))
		slog.Debug("extracting frame", "timestamp", ts, "path", outPath)

		cmd := exec.CommandContext(ctx, "ffmpeg", "-ss", formatSeekTimestamp(ts), "-i", videoPath, "-frames:v", "1", "-y", outPath)
		stderr, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("extract frame at %s: %w: %s", ts, err, stderr)
		}
		frames = append(frames, Frame{Timestamp: ts, Path: outPath})
	}

	sort.Slice(frames, func(i, j int) bool {
		return frames[i].Timestamp < frames[j].Timestamp
	})

	return frames, nil
}

var sceneTimestampRe = regexp.MustCompile(`pts_time:(\d+[\.\d]*)`)

func parseSceneTimestamps(stderr string) []time.Duration {
	matches := sceneTimestampRe.FindAllStringSubmatch(stderr, -1)
	if len(matches) == 0 {
		return nil
	}

	durations := make([]time.Duration, 0, len(matches))
	for _, m := range matches {
		seconds, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			continue
		}
		durations = append(durations, time.Duration(float64(time.Second)*seconds))
	}

	sort.Slice(durations, func(i, j int) bool {
		return durations[i] < durations[j]
	})

	return durations
}

func formatSeekTimestamp(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	ms := int(d.Milliseconds()) % 1000
	return fmt.Sprintf("%02d:%02d:%02d.%03d", h, m, s, ms)
}
