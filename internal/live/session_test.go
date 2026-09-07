package live

import (
	"testing"
	"time"
)

func TestSessionTransitions(t *testing.T) {
	timeline, _ := NewTimeline(30 * time.Second)
	session, err := NewSession("session-1", WorldState{}, timeline, time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	for _, status := range []SessionStatus{SessionBuffering, SessionRunning, SessionStopping, SessionStopped} {
		if err := session.Transition(status, time.Unix(2, 0)); err != nil {
			t.Fatalf("transition to %s: %v", status, err)
		}
	}
	if err := session.Transition(SessionRunning, time.Unix(3, 0)); err == nil {
		t.Fatal("expected stopped session to be terminal")
	}
}

func TestSessionRecoveryTransition(t *testing.T) {
	timeline, _ := NewTimeline(30 * time.Second)
	session, _ := NewSession("session-1", WorldState{}, timeline, time.Unix(1, 0))
	if err := session.Transition(SessionRecovering, time.Unix(2, 0)); err != nil {
		t.Fatal(err)
	}
	if err := session.Transition(SessionBuffering, time.Unix(3, 0)); err != nil {
		t.Fatal(err)
	}
}
