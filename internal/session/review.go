package session

import (
	"context"
	"fmt"
	"streaming-agent/internal/generation"
	"streaming-agent/internal/live"
)

type reviewStore interface {
	SaveReview(context.Context, live.AttemptID, generation.Review) error
}

func (r *Runtime) inspect(ctx context.Context, id live.AttemptID, path string) (live.WorldDelta, error) {
	if r.config.Inspector == nil {
		return live.WorldDelta{}, nil
	}
	frames, err := r.config.Frames.Frames(ctx, path)
	if err != nil {
		return live.WorldDelta{}, err
	}
	r.mu.Lock()
	request := r.attempt(id).Request
	r.mu.Unlock()
	review, err := r.config.Inspector.Review(ctx, request, frames)
	if err != nil {
		return live.WorldDelta{}, err
	}
	if s, ok := r.store.(reviewStore); ok {
		if err = s.SaveReview(ctx, id, review); err != nil {
			return live.WorldDelta{}, err
		}
	}
	if !review.Accepted {
		return live.WorldDelta{}, fmt.Errorf("visual review rejected: %s", review.Reason)
	}
	// Only directly visible events may affect memory, never model-authored identity or goals.
	return live.WorldDelta{Events: review.Observed.Events}, nil
}
