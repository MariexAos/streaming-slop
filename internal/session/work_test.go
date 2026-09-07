package session

import (
	"context"
	"testing"
)

func TestSlowPlanningDoesNotBlockPollingOrDuplicateWork(t *testing.T) {
	w := newWorkGroup()
	blocked := make(chan struct{})
	started := make(chan struct{})
	w.start(context.Background(), "planning", func(context.Context) { close(started); <-blocked })
	<-started
	w.start(context.Background(), "planning", func(context.Context) { t.Error("duplicate planning task") })
	w.start(context.Background(), "poll", func(context.Context) {})
	if name := <-w.done; name != "poll" {
		t.Fatalf("completion=%s", name)
	}
	close(blocked)
	w.wg.Wait()
}
