package main

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("parseSceneTimestamps", func() {
	It("parses pts_time values from ffmpeg showinfo output", func() {
		stderr := `[Parsed_showinfo_1 @ 0x55f] n:   0 pts:      0 pts_time:0       pos:...
[Parsed_showinfo_1 @ 0x55f] n:   1 pts:  90000 pts_time:3.5     pos:...
[Parsed_showinfo_1 @ 0x55f] n:   2 pts: 270000 pts_time:10.75   pos:...`
		timestamps := parseSceneTimestamps(stderr)
		Expect(timestamps).To(HaveLen(3))
		Expect(timestamps[0]).To(Equal(0 * time.Second))
		Expect(timestamps[1]).To(Equal(3*time.Second + 500*time.Millisecond))
		Expect(timestamps[2]).To(Equal(10*time.Second + 750*time.Millisecond))
	})

	It("returns empty slice for no matches", func() {
		Expect(parseSceneTimestamps("no matches here")).To(BeEmpty())
	})
})

var _ = Describe("formatSeekTimestamp", func() {
	It("formats zero duration", func() {
		Expect(formatSeekTimestamp(0)).To(Equal("00:00:00.000"))
	})

	It("formats hours, minutes, seconds, milliseconds", func() {
		d := 1*time.Hour + 2*time.Minute + 3*time.Second + 456*time.Millisecond
		Expect(formatSeekTimestamp(d)).To(Equal("01:02:03.456"))
	})

	It("formats seconds and milliseconds only", func() {
		d := 10*time.Second + 750*time.Millisecond
		Expect(formatSeekTimestamp(d)).To(Equal("00:00:10.750"))
	})
})
