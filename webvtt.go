package main

import (
	"bufio"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var timestampLineRe = regexp.MustCompile(`(\d{2,}:\d{2}:\d{2}\.\d{3})\s+-->\s+(\d{2,}:\d{2}:\d{2}\.\d{3})`)

func parseWebVTT(content string) ([]Cue, error) {
	scanner := bufio.NewScanner(strings.NewReader(content))

	// Validate WEBVTT header (first line, strip optional BOM).
	if !scanner.Scan() {
		return nil, fmt.Errorf("empty WebVTT input")
	}
	firstLine := strings.TrimPrefix(scanner.Text(), "\uFEFF")
	if !strings.HasPrefix(firstLine, "WEBVTT") {
		return nil, fmt.Errorf("missing WEBVTT header")
	}

	// Skip remaining header lines until first blank line.
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == "" {
			break
		}
	}

	var cues []Cue
	var textLines []string
	var curStart, curEnd time.Duration
	inCue := false

	for scanner.Scan() {
		line := scanner.Text()

		if strings.TrimSpace(line) == "" {
			// Blank line ends the current cue.
			if inCue && len(textLines) > 0 {
				cues = append(cues, Cue{
					Start: curStart,
					End:   curEnd,
					Text:  strings.Join(textLines, " "),
				})
				textLines = nil
				inCue = false
			}
			continue
		}

		if m := timestampLineRe.FindStringSubmatch(line); m != nil {
			start, err := parseVTTTimestamp(m[1])
			if err != nil {
				return nil, fmt.Errorf("parsing start timestamp: %w", err)
			}
			end, err := parseVTTTimestamp(m[2])
			if err != nil {
				return nil, fmt.Errorf("parsing end timestamp: %w", err)
			}
			curStart = start
			curEnd = end
			inCue = true
			continue
		}

		if inCue {
			textLines = append(textLines, line)
		}
		// Otherwise it's a cue identifier or other non-timestamp line before
		// the timestamp — skip it.
	}

	// Handle final cue not terminated by a blank line.
	if inCue && len(textLines) > 0 {
		cues = append(cues, Cue{
			Start: curStart,
			End:   curEnd,
			Text:  strings.Join(textLines, " "),
		})
	}

	return cues, nil
}

func parseVTTTimestamp(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	parts := strings.Split(s, ":")
	var h, m int
	var secPart string
	var err error

	switch len(parts) {
	case 3: // HH:MM:SS.mmm
		h, err = strconv.Atoi(parts[0])
		if err != nil {
			return 0, fmt.Errorf("invalid hours: %w", err)
		}
		m, err = strconv.Atoi(parts[1])
		if err != nil {
			return 0, fmt.Errorf("invalid minutes: %w", err)
		}
		secPart = parts[2]
	case 2: // MM:SS.mmm
		m, err = strconv.Atoi(parts[0])
		if err != nil {
			return 0, fmt.Errorf("invalid minutes: %w", err)
		}
		secPart = parts[1]
	default:
		return 0, fmt.Errorf("invalid timestamp format: %s", s)
	}

	secParts := strings.Split(secPart, ".")
	if len(secParts) != 2 {
		return 0, fmt.Errorf("invalid seconds format: %s", secPart)
	}

	sec, err := strconv.Atoi(secParts[0])
	if err != nil {
		return 0, fmt.Errorf("invalid seconds: %w", err)
	}
	ms, err := strconv.Atoi(secParts[1])
	if err != nil {
		return 0, fmt.Errorf("invalid milliseconds: %w", err)
	}

	return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(sec)*time.Second + time.Duration(ms)*time.Millisecond, nil
}
