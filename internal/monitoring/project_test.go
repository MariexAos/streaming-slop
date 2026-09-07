package monitoring

import (
	"testing"
	"time"

	"streaming-agent/internal/live"
)

func TestProjectEmpty(t *testing.T) {
	now := time.Unix(100, 0).UTC()
	snapshot := Project(ProjectionInput{Now: now, ReadyTarget: 45 * time.Second, SubmittedTarget: 90 * time.Second})
	if snapshot.Session.Status != "stopped" || !snapshot.Controls.CanStart {
		t.Fatalf("empty snapshot = %#v", snapshot)
	}
}

func TestControlsWhileRunning(t *testing.T) {
	got := controls(live.SessionRunning, false)
	if !got.CanStop || !got.CanEnableFallback || got.CanStart {
		t.Fatalf("controls = %#v", got)
	}
}
