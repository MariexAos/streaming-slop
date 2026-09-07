package session

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/google/uuid"

	"streaming-agent/internal/director"
	"streaming-agent/internal/generation"
	"streaming-agent/internal/live"
)

func (r *Runtime) plan(ctx context.Context, limit int) {
	for range limit {
		if !r.planNext(ctx) {
			return
		}
	}
}

func (r *Runtime) planNext(ctx context.Context) bool {
	r.mu.Lock()
	if r.session == nil || r.stopWanted {
		r.mu.Unlock()
		return false
	}
	end := time.Duration(0)
	sequence := 0
	var previous *live.Direction
	if segments := r.session.Timeline.Segments; len(segments) > 0 {
		last := segments[len(segments)-1]
		end = last.End
		sequence = last.Sequence + 1
		copy := last.Direction
		previous = &copy
	}
	if end-r.session.Timeline.Playhead >= r.config.PlannedHorizon {
		r.mu.Unlock()
		return false
	}
	world := r.session.World
	r.mu.Unlock()

	directionValue, err := r.director.Direct(ctx, director.Input{
		Duration: r.config.SegmentDuration,
		World:    world, StorySeed: r.config.StorySeed, Previous: previous, Position: end,
		Audience: r.directorAudience(time.Now().UTC()),
	})
	if err != nil {
		r.mu.Lock()
		r.directFails[sequence]++
		failures := r.directFails[sequence]
		r.lastGenErr = err.Error()
		r.mu.Unlock()
		if failures < 2 {
			return false
		}
		directionValue = director.IdleDirection()
	}
	segmentValue, err := live.NewSegment(
		live.SegmentID(uuid.NewString()), sequence, end, end+r.config.SegmentDuration, directionValue,
	)
	if err != nil {
		r.fail(ctx, err)
		return false
	}
	r.mu.Lock()
	if r.session == nil || len(r.session.Timeline.Segments) != sequence {
		r.mu.Unlock()
		return false
	}
	if err := r.session.Timeline.Append(segmentValue); err != nil {
		r.mu.Unlock()
		r.fail(ctx, err)
		return false
	}
	sessionID := r.session.ID
	delete(r.directFails, sequence)
	r.mu.Unlock()
	if err := r.store.UpsertSegment(ctx, sessionID, segmentValue); err != nil {
		r.fail(ctx, fmt.Errorf("save planned segment: %w", err))
		return false
	}
	return true
}

func generationSpec(_ int, anchors map[string]string) generation.GenerationSpec {
	base := generation.GenerationSpec{Resolution: "768P", Duration: 5 * time.Second, Mode: generation.ModeTextToVideo, Ratio: "16:9"}
	if anchor, ok := anchors["chat-live-start"]; ok {
		base.Mode = generation.ModeFirstFrameToVideo
		base.Ratio = "adaptive"
		base.FirstFrame = &generation.AssetRef{ID: "chat-live-start", URL: anchor}
		base.AnchorVersion = fmt.Sprintf("%x", sha256.Sum256([]byte(anchor)))
	}
	return base
}

func directedSpec(direction live.Direction, anchors map[string]string) generation.GenerationSpec {
	spec := generationSpec(0, anchors)
	var refs []generation.AssetRef
	for _, id := range direction.Continuity.Anchors {
		if value := anchors[id]; value != "" {
			refs = append(refs, generation.AssetRef{ID: id, URL: value})
		}
		if len(refs) == 2 {
			break
		}
	}
	if len(refs) > 0 {
		spec.FirstFrame = &refs[0]
		spec.Mode = generation.ModeFirstFrameToVideo
		spec.Ratio = "adaptive"
		spec.AnchorVersion = fmt.Sprintf("%x", sha256.Sum256([]byte(refs[0].URL)))
	}
	if len(refs) == 2 {
		spec.LastFrame = &refs[1]
		spec.Mode = generation.ModeFirstLastFrameToVideo
		spec.AnchorVersion = fmt.Sprintf("%x", sha256.Sum256([]byte(refs[0].URL+refs[1].URL)))
	}
	return spec
}
