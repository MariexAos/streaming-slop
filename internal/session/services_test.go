package session

import (
	"context"
	"testing"
	"time"

	"streaming-agent/internal/generation"
	"streaming-agent/internal/live"
)

type routedGenerator struct {
	fakeGenerator
	submitted, polled, cancelled int
}

func (g *routedGenerator) Submit(context.Context, generation.Request) (generation.Job, error) {
	g.submitted++
	return generation.Job{ID: "same-id", Status: generation.JobQueued}, nil
}
func (g *routedGenerator) Status(context.Context, string) (generation.Job, error) {
	g.polled++
	return generation.Job{ID: "same-id", Status: generation.JobQueued}, nil
}
func (g *routedGenerator) Cancel(context.Context, string) error { g.cancelled++; return nil }

func TestProviderSwitchKeepsInflightRoutingAndJobNamespaces(t *testing.T) {
	ctx := context.Background()
	mini, fal := &routedGenerator{}, &routedGenerator{}
	selected := "minimax"
	r := &Runtime{session: &live.LiveSession{ID: "session"}, store: &memoryStore{}, config: RuntimeConfig{
		BuildGeneration: func(_ context.Context, request generation.Request) (generation.Request, error) {
			request.Provider = selected
			return request, nil
		},
		GeneratorFor: func(_ context.Context, request generation.Request) (generation.Generator, error) {
			if request.Provider == "minimax" {
				return mini, nil
			}
			return fal, nil
		},
	}}
	for _, provider := range []string{"minimax", "fal"} {
		selected = provider
		request, err := r.buildRequest(ctx, generation.Request{})
		if err != nil {
			t.Fatal(err)
		}
		a, err := generation.NewAttempt(live.AttemptID(provider), "segment", 1, request.Provider, provider, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		a.Request = request
		r.attempts = append(r.attempts, a)
		r.submitAttempt(ctx, r.session.ID, a, request)
	}
	r.poll(ctx)
	r.cancelFallbackAttempts(ctx, r.attempts)
	if mini.submitted != 1 || fal.submitted != 1 || mini.polled != 1 || fal.polled != 1 || mini.cancelled != 1 || fal.cancelled != 1 {
		t.Fatalf("wrong provider routing: minimax=%+v fal=%+v", mini, fal)
	}
	// An unknown submission may use the same job ID as the other provider.
	r.attempts[1].ProviderJobID = ""
	r.attempts[1].Status = generation.AttemptPendingSubmit
	if err := r.AttachJob(ctx, "fal", "same-id"); err != nil {
		t.Fatal(err)
	}
	if mini.polled != 1 || fal.polled != 2 {
		t.Fatal("attachment verified with wrong provider")
	}
}
