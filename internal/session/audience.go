package session

import (
	"context"
	"time"

	"streaming-agent/internal/audience"
	"streaming-agent/internal/director"
	"streaming-agent/internal/live"
)

func (r *Runtime) directorAudience(now time.Time) *director.AudienceInput {
	if r.config.Audience == nil {
		return nil
	}
	snapshot := r.config.Audience.Snapshot(now)
	if snapshot.MessageCount == 0 {
		return nil
	}
	r.mu.Lock()
	observation := r.lastObservation
	r.mu.Unlock()
	return observedAudience(snapshot, observation)
}

func (r *Runtime) replanAudience(ctx context.Context, limit int) {
	if r.config.Audience == nil {
		return
	}
	snapshot := r.config.Audience.Snapshot(time.Now().UTC())
	r.mu.Lock()
	if snapshot.Revision == r.audienceRevision || r.session == nil || r.stopWanted {
		r.mu.Unlock()
		return
	}
	if snapshot.MessageCount == 0 {
		r.audienceRevision = snapshot.Revision
		r.mu.Unlock()
		return
	}
	candidates := plannedCandidates(r.session.Timeline.Segments, len(r.session.Timeline.Segments))
	freezeAt := r.session.Timeline.Playhead + r.session.Timeline.CommitHorizon
	candidates = filterMutable(candidates, freezeAt)
	if !r.config.Continuous && len(candidates) > limit {
		candidates = candidates[:limit]
	}
	sessionID := r.session.ID
	world := r.session.World
	r.mu.Unlock()

	observation, ok := r.observeAudience(ctx, sessionID, snapshot)
	if !ok {
		return
	}
	audienceInput := observedAudience(snapshot, observation)
	replacements := make([]live.Segment, 0, len(candidates))
	for _, candidate := range candidates {
		directionValue, err := r.director.Direct(ctx, director.Input{
			Duration: r.config.SegmentDuration,
			World:    world, StorySeed: r.config.StorySeed, Previous: candidate.previous,
			Position: candidate.segment.Start, Audience: audienceInput,
		})
		if err != nil {
			r.mu.Lock()
			r.lastGenErr = err.Error()
			r.audienceRevision = snapshot.Revision
			r.mu.Unlock()
			return
		}
		next := candidate.segment
		if err := next.Replan(directionValue, freezeAt); err != nil {
			r.recordGenerationError(err)
			return
		}
		replacements = append(replacements, next)
	}
	r.applyReplan(ctx, sessionID, snapshot.Revision, replacements)
}

func (r *Runtime) applyReplan(ctx context.Context, sessionID live.SessionID, revision uint64, replacements []live.Segment) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.session == nil || r.session.ID != sessionID || r.stopWanted {
		return
	}
	for _, next := range replacements {
		current, ok := r.session.Timeline.Segment(next.ID)
		if !ok || current.PlanRevision+1 != next.PlanRevision || current.Start < r.session.Timeline.Playhead+r.session.Timeline.CommitHorizon {
			return
		}
		if current.Status == live.SegmentCommitted || current.Status == live.SegmentPlaying || current.Status == live.SegmentPlayed {
			return
		}
	}
	if err := r.store.ReplanSegments(ctx, sessionID, replacements); err != nil {
		r.lastGenErr = err.Error()
		return
	}
	for _, next := range replacements {
		current, _ := r.session.Timeline.Segment(next.ID)
		*current = next
	}
	r.audienceRevision = revision
}

func observedAudience(snapshot audience.Snapshot, observation audience.Observation) *director.AudienceInput {
	intents := make([]string, 0, len(observation.Intents))
	for _, intent := range observation.Intents {
		intents = append(intents, intent.Label)
	}
	return &director.AudienceInput{
		WindowSeconds: snapshot.WindowSeconds, Messages: snapshot.Messages,
		Summary: observation.Summary, Mood: observation.Mood, Intents: intents,
	}
}

type audienceCandidate struct {
	segment  live.Segment
	previous *live.Direction
}

func plannedCandidates(segments []live.Segment, limit int) []audienceCandidate {
	candidates := make([]audienceCandidate, 0, limit)
	for i, segment := range segments {
		if segment.Status != live.SegmentPlanned && segment.Status != live.SegmentGenerating && segment.Status != live.SegmentReady {
			continue
		}
		var previous *live.Direction
		if i > 0 {
			value := segments[i-1].Direction
			previous = &value
		}
		candidates = append(candidates, audienceCandidate{segment: segment, previous: previous})
		if len(candidates) == limit {
			break
		}
	}
	return candidates
}

func (r *Runtime) observeAudience(ctx context.Context, sessionID live.SessionID, snapshot audience.Snapshot) (audience.Observation, bool) {
	observation := audience.Observation{}
	if r.config.Observer != nil {
		value, err := r.config.Observer.Observe(ctx, audience.Input{Messages: snapshot.Messages})
		if err != nil {
			r.mu.Lock()
			r.lastGenErr = err.Error()
			observation = r.lastObservation
			r.mu.Unlock()
			if recordErr := r.store.RecordObserverRun(ctx, sessionID, snapshot.Revision, snapshot.Messages, observation, err.Error()); recordErr != nil {
				r.recordGenerationError(recordErr)
			}
			if observation.Summary == "" {
				r.mu.Lock()
				r.audienceRevision = snapshot.Revision
				r.mu.Unlock()
				return observation, false
			}
		} else {
			if err := r.store.RecordObserverRun(ctx, sessionID, snapshot.Revision, snapshot.Messages, value, ""); err != nil {
				r.recordGenerationError(err)
			}
			r.mu.Lock()
			r.lastObservation = value
			r.mu.Unlock()
			observation = value
		}
	}
	return observation, true
}

func filterMutable(candidates []audienceCandidate, freezeAt time.Duration) []audienceCandidate {
	result := make([]audienceCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.segment.Start >= freezeAt {
			result = append(result, candidate)
		}
	}
	return result
}
