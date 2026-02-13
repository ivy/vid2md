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

func assembleMarkdown(ctx context.Context, timeline []TimelineEntry, model, baseURL string) (string, error) {
	opts := []option.RequestOption{}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	client := openai.NewClient(opts...)

	systemPrompt, err := promptFS.ReadFile("prompts/assembly-system.md.tmpl")
	if err != nil {
		return "", fmt.Errorf("read assembly system prompt: %w", err)
	}

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
			openai.DeveloperMessage(string(systemPrompt)),
			openai.UserMessage(buf.String()),
		},
	})
	if err != nil {
		return "", fmt.Errorf("openai chat completion: %w", err)
	}

	return resp.Choices[0].Message.Content, nil
}
