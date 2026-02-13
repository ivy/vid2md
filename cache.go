package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

func videoHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil))[:12], nil
}

func resolveDir(videoPath string, noCache bool) (string, func(), error) {
	if noCache {
		dir, err := os.MkdirTemp("", "vid2md-*")
		if err != nil {
			return "", nil, fmt.Errorf("create temp dir: %w", err)
		}
		if err := os.MkdirAll(filepath.Join(dir, "frames"), 0o755); err != nil {
			os.RemoveAll(dir)
			return "", nil, err
		}
		return dir, func() { os.RemoveAll(dir) }, nil
	}

	slog.Info("hashing video")
	hash, err := videoHash(videoPath)
	if err != nil {
		return "", nil, fmt.Errorf("hash video: %w", err)
	}
	slog.Info("video hash", "hash", hash)

	base, err := os.UserCacheDir()
	if err != nil {
		return "", nil, fmt.Errorf("user cache dir: %w", err)
	}

	dir := filepath.Join(base, "vid2md", hash)
	if err := os.MkdirAll(filepath.Join(dir, "frames"), 0o755); err != nil {
		return "", nil, err
	}

	return dir, func() {}, nil
}

func saveFrameTimestamps(dir string, frames []Frame) error {
	seconds := make([]float64, len(frames))
	for i, f := range frames {
		seconds[i] = f.Timestamp.Seconds()
	}
	data, err := json.Marshal(seconds)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "frames", "timestamps.json"), data, 0o644)
}

func loadFrames(dir string) ([]Frame, error) {
	data, err := os.ReadFile(filepath.Join(dir, "frames", "timestamps.json"))
	if err != nil {
		return nil, err
	}
	var seconds []float64
	if err := json.Unmarshal(data, &seconds); err != nil {
		return nil, err
	}
	frames := make([]Frame, len(seconds))
	for i, s := range seconds {
		frames[i] = Frame{
			Timestamp: time.Duration(s * float64(time.Second)),
			Path:      filepath.Join(dir, "frames", fmt.Sprintf("frame_%03d.png", i)),
		}
	}
	return frames, nil
}
