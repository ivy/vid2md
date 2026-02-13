package main

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("interleave", func() {
	It("interleaves transcript cues and visual frames by timestamp", func() {
		cues := []Cue{
			{Start: 0, End: 2 * time.Second, Text: "Hello"},
			{Start: 5 * time.Second, End: 7 * time.Second, Text: "World"},
		}
		frames := []DescribedFrame{
			{Timestamp: 3 * time.Second, Description: "Title slide"},
			{Timestamp: 6 * time.Second, Description: "Code editor"},
		}
		result := interleave(cues, frames)
		Expect(result).To(HaveLen(4))
		Expect(result[0]).To(Equal(TimelineEntry{Timestamp: 0, Kind: "transcript", Text: "Hello"}))
		Expect(result[1]).To(Equal(TimelineEntry{Timestamp: 3 * time.Second, Kind: "visual", Text: "Title slide"}))
		Expect(result[2]).To(Equal(TimelineEntry{Timestamp: 5 * time.Second, Kind: "transcript", Text: "World"}))
		Expect(result[3]).To(Equal(TimelineEntry{Timestamp: 6 * time.Second, Kind: "visual", Text: "Code editor"}))
	})

	It("handles empty cues", func() {
		frames := []DescribedFrame{
			{Timestamp: 1 * time.Second, Description: "Frame 1"},
		}
		result := interleave(nil, frames)
		Expect(result).To(HaveLen(1))
		Expect(result[0].Kind).To(Equal("visual"))
	})

	It("handles empty frames", func() {
		cues := []Cue{
			{Start: 1 * time.Second, End: 2 * time.Second, Text: "Only cue"},
		}
		result := interleave(cues, nil)
		Expect(result).To(HaveLen(1))
		Expect(result[0].Kind).To(Equal("transcript"))
	})

	It("handles both empty", func() {
		result := interleave(nil, nil)
		Expect(result).To(BeEmpty())
	})

	It("preserves order for same timestamp (stable sort)", func() {
		cues := []Cue{
			{Start: 5 * time.Second, End: 6 * time.Second, Text: "Speech"},
		}
		frames := []DescribedFrame{
			{Timestamp: 5 * time.Second, Description: "Visual"},
		}
		result := interleave(cues, frames)
		Expect(result).To(HaveLen(2))
		// Transcript comes first because cues are added to slice before frames
		Expect(result[0].Kind).To(Equal("transcript"))
		Expect(result[1].Kind).To(Equal("visual"))
	})
})
