package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

func describeFrames(ctx context.Context, frames []Frame, model, baseURL string) ([]DescribedFrame, error) {
	opts := []option.RequestOption{}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	client := openai.NewClient(opts...)

	var described []DescribedFrame
	for i, frame := range frames {
		txtPath := strings.TrimSuffix(frame.Path, ".png") + ".txt"

		if data, err := os.ReadFile(txtPath); err == nil {
			slog.Info("frame description cached", "index", i, "total", len(frames), "timestamp", frame.Timestamp)
			described = append(described, DescribedFrame{
				Timestamp:   frame.Timestamp,
				Description: string(data),
			})
			continue
		}

		slog.Info("describing frame", "index", i, "total", len(frames), "timestamp", frame.Timestamp)

		desc, err := describeFrame(ctx, client, frame, model)
		if err != nil {
			return nil, fmt.Errorf("describe frame %d (%s): %w", i, frame.Timestamp, err)
		}

		if err := os.WriteFile(txtPath, []byte(desc), 0o644); err != nil {
			slog.Warn("failed to cache frame description", "path", txtPath, "error", err)
		}

		described = append(described, DescribedFrame{
			Timestamp:   frame.Timestamp,
			Description: desc,
		})
	}
	return described, nil
}

func describeFrame(ctx context.Context, client openai.Client, frame Frame, model string) (string, error) {
	data, err := os.ReadFile(frame.Path)
	if err != nil {
		return "", fmt.Errorf("read frame image: %w", err)
	}

	b64 := base64.StdEncoding.EncodeToString(data)
	dataURI := fmt.Sprintf("data:image/png;base64,%s", b64)

	systemPrompt, err := promptFS.ReadFile("prompts/vision.md.tmpl")
	if err != nil {
		return "", fmt.Errorf("read vision prompt: %w", err)
	}

	params := openai.ChatCompletionNewParams{
		Model: openai.ChatModel(model),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.DeveloperMessage(string(systemPrompt)),
			openai.UserMessage([]openai.ChatCompletionContentPartUnionParam{
				openai.TextContentPart("Describe this video frame."),
				openai.ImageContentPart(openai.ChatCompletionContentPartImageImageURLParam{
					URL: dataURI,
				}),
			}),
		},
	}

	var resp *openai.ChatCompletion
	for attempt := range 5 {
		resp, err = client.Chat.Completions.New(ctx, params)
		if err == nil {
			break
		}
		backoff := time.Duration(1<<attempt) * time.Second
		slog.Warn("openai request failed, retrying", "attempt", attempt+1, "backoff", backoff, "error", err)
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(backoff):
		}
	}
	if err != nil {
		return "", fmt.Errorf("openai chat completion: %w", err)
	}

	slog.Debug("frame described", "timestamp", frame.Timestamp)
	return resp.Choices[0].Message.Content, nil
}
