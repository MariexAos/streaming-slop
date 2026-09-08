package session

import (
	"context"
	"errors"
	"testing"
)

func TestStartReadinessBudgetAndCredentials(t *testing.T) {
	price := 0.5
	r := EvaluateStart("host", "Host", true, 1, &price, 45)
	if r.Ready || r.MinimumMicros != 22_500_000 {
		t.Fatalf("unexpected budget result: %+v", r)
	}
	r = EvaluateStart("host", "Host", true, 23_000_000, &price, 45)
	if !r.Ready {
		t.Fatal(r.Blockers)
	}
	r = EvaluateStart("", "", false, 23_000_000, nil, 45)
	if r.Ready || len(r.Blockers) != 3 {
		t.Fatalf("missing checks: %+v", r)
	}
}
func TestStartRejectsBeforeCreatingSession(t *testing.T) {
	blocked := errors.New("budget unavailable")
	r := &Runtime{config: RuntimeConfig{CheckStart: func(context.Context) error { return blocked }}}
	if err := r.Start(context.Background()); !errors.Is(err, blocked) {
		t.Fatalf("got %v", err)
	}
	if r.session != nil {
		t.Fatal("blocked start created session")
	}
}
