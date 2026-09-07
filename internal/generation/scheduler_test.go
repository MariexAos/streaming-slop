package generation

import (
	"testing"
	"time"

	"streaming-agent/internal/live"
)

func TestSchedulerModes(t *testing.T) {
	tests := []struct {
		ready time.Duration
		want  SchedulerMode
	}{
		{91 * time.Second, ModePause},
		{90 * time.Second, ModeNormal},
		{30 * time.Second, ModeNormal},
		{29 * time.Second, ModeHigh},
		{15 * time.Second, ModeHigh},
		{14 * time.Second, ModeCritical},
		{5 * time.Second, ModeCritical},
		{4 * time.Second, ModeFallback},
	}
	for _, test := range tests {
		if got := Mode(test.ready); got != test.want {
			t.Errorf("Mode(%s) = %s, want %s", test.ready, got, test.want)
		}
	}
}

func TestTargetConcurrencyUsesLastTwentyP95(t *testing.T) {
	scheduler := NewScheduler(DefaultSchedulerConfig(20))
	latencies := []time.Duration{100 * time.Second}
	for range 20 {
		latencies = append(latencies, 10*time.Second)
	}
	if got := scheduler.TargetConcurrency(latencies); got != 3 {
		t.Fatalf("target concurrency = %d, want 3", got)
	}
}

func TestSelectNearestSegmentsAndHonorsAttemptLimit(t *testing.T) {
	config := DefaultSchedulerConfig(3)
	scheduler := NewScheduler(config)
	segments := []live.Segment{
		segment("third", 10*time.Second, live.SegmentPlanned),
		segment("first", 0, live.SegmentPlanned),
		segment("second", 5*time.Second, live.SegmentGenerating),
	}
	attempts := []Attempt{
		{ID: "old-1", SegmentID: "second", Number: 1, Status: AttemptFailed},
		{ID: "old-2", SegmentID: "second", Number: 2, Status: AttemptFailed},
	}
	selected := scheduler.Select(0, 10*time.Second, segments, attempts, nil)
	if len(selected) != 2 {
		t.Fatalf("selected %d candidates, want 2", len(selected))
	}
	if selected[0].SegmentID != "first" || selected[1].SegmentID != "third" {
		t.Fatalf("unexpected order: %#v", selected)
	}
}

func segment(id live.SegmentID, start time.Duration, status live.SegmentStatus) live.Segment {
	return live.Segment{ID: id, Start: start, End: start + 5*time.Second, Status: status}
}
