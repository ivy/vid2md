package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/transcribe"
	"github.com/aws/aws-sdk-go-v2/service/transcribe/types"
)

func uploadToS3(ctx context.Context, localPath, bucketURI, region string) (string, error) {
	trimmed := strings.TrimPrefix(bucketURI, "s3://")
	parts := strings.SplitN(trimmed, "/", 2)
	bucket := parts[0]
	var prefix string
	if len(parts) > 1 {
		prefix = parts[1]
	}

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return "", fmt.Errorf("load AWS config: %w", err)
	}

	f, err := os.Open(localPath)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", localPath, err)
	}
	defer f.Close()

	key := filepath.Base(localPath)
	if prefix != "" {
		key = prefix + "/" + key
	}

	client := s3.NewFromConfig(cfg)
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   f,
	})
	if err != nil {
		return "", fmt.Errorf("PutObject: %w", err)
	}

	s3URI := fmt.Sprintf("s3://%s/%s", bucket, key)
	slog.Debug("uploaded to S3", "uri", s3URI)
	return s3URI, nil
}

func transcribeAudio(ctx context.Context, s3AudioURI, bucketURI, region, langCode string) (string, error) {
	trimmed := strings.TrimPrefix(bucketURI, "s3://")
	parts := strings.SplitN(trimmed, "/", 2)
	bucket := parts[0]
	var prefix string
	if len(parts) > 1 {
		prefix = parts[1]
	}

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return "", fmt.Errorf("load AWS config: %w", err)
	}

	client := transcribe.NewFromConfig(cfg)
	jobName := fmt.Sprintf("vid2md-%d", time.Now().UnixNano())

	_, err = client.StartTranscriptionJob(ctx, &transcribe.StartTranscriptionJobInput{
		TranscriptionJobName: aws.String(jobName),
		Media: &types.Media{
			MediaFileUri: aws.String(s3AudioURI),
		},
		LanguageCode: types.LanguageCode(langCode),
		MediaFormat: types.MediaFormatOgg,
		Subtitles: &types.Subtitles{
			Formats: []types.SubtitleFormat{types.SubtitleFormatVtt},
		},
		OutputBucketName: aws.String(bucket),
		OutputKey:        aws.String(prefix + "/"),
	})
	if err != nil {
		return "", fmt.Errorf("StartTranscriptionJob: %w", err)
	}

	waiter := transcribe.NewTranscriptionJobCompletedWaiter(client)
	result, err := waiter.WaitForOutput(ctx, &transcribe.GetTranscriptionJobInput{
		TranscriptionJobName: aws.String(jobName),
	}, 30*time.Minute)
	if err != nil {
		return "", fmt.Errorf("wait for transcription: %w", err)
	}

	if result.TranscriptionJob.TranscriptionJobStatus == types.TranscriptionJobStatusFailed {
		reason := aws.ToString(result.TranscriptionJob.FailureReason)
		return "", fmt.Errorf("transcription failed: %s", reason)
	}

	// Download VTT directly from S3 — presigned URIs from Transcribe
	// can fail with Access Denied in cross-region or policy-restricted setups.
	vttKey := prefix + "/" + jobName + ".vtt"
	slog.Debug("downloading VTT from S3", "bucket", bucket, "key", vttKey)

	s3Client := s3.NewFromConfig(cfg)
	obj, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(vttKey),
	})
	if err != nil {
		return "", fmt.Errorf("GetObject VTT: %w", err)
	}
	defer obj.Body.Close()

	body, err := io.ReadAll(obj.Body)
	if err != nil {
		return "", fmt.Errorf("read VTT body: %w", err)
	}

	return string(body), nil
}

