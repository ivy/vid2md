package main

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("parseVTTTimestamp", func() {
	It("parses HH:MM:SS.mmm format", func() {
		d, err := parseVTTTimestamp("00:01:23.456")
		Expect(err).NotTo(HaveOccurred())
		Expect(d).To(Equal(1*time.Minute + 23*time.Second + 456*time.Millisecond))
	})

	It("parses MM:SS.mmm format", func() {
		d, err := parseVTTTimestamp("01:23.456")
		Expect(err).NotTo(HaveOccurred())
		Expect(d).To(Equal(1*time.Minute + 23*time.Second + 456*time.Millisecond))
	})

	It("parses zero timestamp", func() {
		d, err := parseVTTTimestamp("00:00:00.000")
		Expect(err).NotTo(HaveOccurred())
		Expect(d).To(BeZero())
	})

	It("handles hours", func() {
		d, err := parseVTTTimestamp("02:30:00.000")
		Expect(err).NotTo(HaveOccurred())
		Expect(d).To(Equal(2*time.Hour + 30*time.Minute))
	})

	It("returns error for invalid format", func() {
		_, err := parseVTTTimestamp("invalid")
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("parseWebVTT", func() {
	It("parses a simple WebVTT file", func() {
		content := "WEBVTT\n\n00:00:00.000 --> 00:00:03.580\nWelcome everyone to the quarterly\nbusiness review for Q4.\n\n00:00:03.580 --> 00:00:07.210\nLet's start with the revenue numbers.\n"
		cues, err := parseWebVTT(content)
		Expect(err).NotTo(HaveOccurred())
		Expect(cues).To(HaveLen(2))
		Expect(cues[0].Start).To(BeZero())
		Expect(cues[0].End).To(Equal(3*time.Second + 580*time.Millisecond))
		Expect(cues[0].Text).To(Equal("Welcome everyone to the quarterly business review for Q4."))
		Expect(cues[1].Start).To(Equal(3*time.Second + 580*time.Millisecond))
		Expect(cues[1].Text).To(Equal("Let's start with the revenue numbers."))
	})

	It("handles BOM", func() {
		content := "\uFEFFWEBVTT\n\n00:00:00.000 --> 00:00:01.000\nHello\n"
		cues, err := parseWebVTT(content)
		Expect(err).NotTo(HaveOccurred())
		Expect(cues).To(HaveLen(1))
	})

	It("skips cue identifiers", func() {
		content := "WEBVTT\n\n1\n00:00:00.000 --> 00:00:01.000\nFirst\n\n2\n00:00:01.000 --> 00:00:02.000\nSecond\n"
		cues, err := parseWebVTT(content)
		Expect(err).NotTo(HaveOccurred())
		Expect(cues).To(HaveLen(2))
		Expect(cues[0].Text).To(Equal("First"))
		Expect(cues[1].Text).To(Equal("Second"))
	})

	It("handles file not ending with newline", func() {
		content := "WEBVTT\n\n00:00:00.000 --> 00:00:01.000\nHello"
		cues, err := parseWebVTT(content)
		Expect(err).NotTo(HaveOccurred())
		Expect(cues).To(HaveLen(1))
		Expect(cues[0].Text).To(Equal("Hello"))
	})

	It("returns error for missing WEBVTT header", func() {
		_, err := parseWebVTT("not a webvtt file")
		Expect(err).To(HaveOccurred())
	})

	It("returns empty cues for header only", func() {
		cues, err := parseWebVTT("WEBVTT\n\n")
		Expect(err).NotTo(HaveOccurred())
		Expect(cues).To(BeEmpty())
	})
})
