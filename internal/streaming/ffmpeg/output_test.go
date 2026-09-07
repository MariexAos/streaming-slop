package ffmpeg

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestProgressStats(t *testing.T) {
	output := &Output{}
	output.readProgress(strings.NewReader("bitrate=2048.5kbits/s\ndrop_frames=3\nout_time_us=2500000\n"))
	stats := output.Stats()
	if stats.BitrateKbps != 2048.5 || stats.DroppedFrames != 3 || stats.OutTime != 2500*time.Millisecond {
		t.Fatalf("stats = %+v", stats)
	}
}

func TestRestartIsBoundedToOne(t *testing.T) {
	output, err := NewOutput(OutputConfig{FFmpegPath: "/usr/bin/false", RTMPURL: "rtmp://example.invalid/live/key"})
	if err != nil {
		t.Fatal(err)
	}
	if err := output.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := output.Restart(context.Background()); err != nil {
		t.Fatalf("first restart: %v", err)
	}
	if err := output.Restart(context.Background()); err == nil {
		t.Fatal("expected restart limit error")
	}
	if output.Stats().Restarts != 1 {
		t.Fatalf("restarts = %d", output.Stats().Restarts)
	}
}
