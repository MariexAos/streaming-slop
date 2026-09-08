package session

import (
	"context"
	"fmt"
	"streaming-agent/internal/audience"
	"streaming-agent/internal/director"
	"streaming-agent/internal/generation"
	"streaming-agent/internal/live"
	"streaming-agent/internal/streaming"
)

// Services are constructed once at session start, then retained for its tasks.
type Services struct {
	BudgetLimitMicros int64
	Models            *live.ModelSettings
	Generator         generation.Generator
	Director          director.Director
	Speech            streaming.Speech
	Observer          audience.Observer
}

func (r *Runtime) selectServices(ctx context.Context, current *live.LiveSession) error {
	if r.config.ResolveServices == nil {
		return nil
	}
	services, err := r.config.ResolveServices(ctx, current)
	if err != nil {
		return err
	}
	current.Models = services.Models
	current.BudgetLimitMicros = services.BudgetLimitMicros
	r.config.Provider = services.Models.Video.Provider
	r.generator = services.Generator
	r.director = services.Director
	r.config.Speech = services.Speech
	r.config.Observer = services.Observer
	return nil
}

func (r *Runtime) generatorFor(ctx context.Context, request generation.Request) (generation.Generator, error) {
	if r.config.GeneratorFor != nil {
		return r.config.GeneratorFor(ctx, request)
	}
	return r.generator, nil
}
func attemptRequest(a generation.Attempt) generation.Request {
	request := a.Request
	request.Provider = a.Provider
	return request
}
func (r *Runtime) requestForAttempt(id live.AttemptID) (generation.Request, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	a := r.attempt(id)
	if a == nil {
		return generation.Request{}, fmt.Errorf("attempt %s not found", id)
	}
	return attemptRequest(*a), nil
}
