package main

import (
	"cmp"
	"slices"
)

func interleave(cues []Cue, frames []DescribedFrame) []TimelineEntry {
	entries := make([]TimelineEntry, 0, len(cues)+len(frames))

	for _, c := range cues {
		entries = append(entries, TimelineEntry{
			Timestamp: c.Start,
			Kind:      "transcript",
			Text:      c.Text,
		})
	}

	for _, f := range frames {
		entries = append(entries, TimelineEntry{
			Timestamp: f.Timestamp,
			Kind:      "visual",
			Text:      f.Description,
		})
	}

	slices.SortStableFunc(entries, func(a, b TimelineEntry) int {
		return cmp.Compare(int64(a.Timestamp), int64(b.Timestamp))
	})

	return entries
}
