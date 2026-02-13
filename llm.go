package main

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"text/template"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

const developerPrompt = `You are a technical writer. Given an interleaved timeline of transcript and visual descriptions from a video, produce a well-structured Markdown document that captures the content. Use headings, paragraphs, and formatting to create a readable article. Preserve key details from both the spoken content and visual elements. Do not include timestamps.`

func assembleMarkdown(ctx context.Context, timeline []TimelineEntry, model, baseURL string) (string, error) {
	opts := []option.RequestOption{}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	client := openai.NewClient(opts...)

	tmpl, err := template.ParseFS(promptFS, "prompts/assembly.md.tmpl")
	if err != nil {
		return "", fmt.Errorf("parse assembly template: %w", err)
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, map[string]any{"Timeline": timeline})
	if err != nil {
		return "", fmt.Errorf("execute assembly template: %w", err)
	}

	slog.Debug("assembly prompt", "length", buf.Len())

	resp, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: openai.ChatModel(model),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.DeveloperMessage(developerPrompt),
			openai.UserMessage(buf.String()),
		},
	})
	if err != nil {
		return "", fmt.Errorf("openai chat completion: %w", err)
	}

	return resp.Choices[0].Message.Content, nil
}
