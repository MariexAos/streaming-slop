package monitoring

import (
	"testing"
	"time"
)

func TestHubPublishesMonotonicRevision(t *testing.T) {
	hub := NewHub(EmptySnapshot(time.Unix(0, 0), 45*time.Second, 90*time.Second, 30*time.Second))
	events, unsubscribe := hub.Subscribe()
	defer unsubscribe()
	<-events

	snapshot := hub.Current()
	snapshot.Session.Status = "running"
	published := hub.Publish(snapshot)
	if published.Revision != 1 {
		t.Fatalf("Revision = %d, want 1", published.Revision)
	}
	if event := <-events; event.Session.Status != "running" {
		t.Fatalf("status = %q", event.Session.Status)
	}
}
