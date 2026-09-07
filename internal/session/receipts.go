package session

import (
	"context"
	"errors"
	"time"

	"streaming-agent/internal/live"
	"streaming-agent/internal/timeline"
)

type playbackReceipt struct {
	ID       live.SegmentID
	End      time.Duration
	Fallback bool
}

func (r *Runtime) nextForOutputLocked() (*live.Segment, error) {
	for i := range r.session.Timeline.Segments {
		segment := &r.session.Timeline.Segments[i]
		if segment.Start != r.queuedUntil {
			continue
		}
		if segment.Status == live.SegmentPlaying {
			return segment, nil
		}
		if err := segment.Transition(live.SegmentPlaying); err != nil {
			return nil, err
		}
		return segment, nil
	}
	return nil, errors.New("no committed segment at output cursor")
}

// A pipe write is not a playback receipt. Advance only when the output reports
// that its media timestamp has crossed the end of a queued segment.
func (r *Runtime) acknowledgePlayback(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.session == nil {
		return
	}
	position := r.outputBase + r.output.Stats().OutTime
	for len(r.receipts) > 0 && r.receipts[0].End <= position {
		receipt := r.receipts[0]
		before := *r.session
		before.Timeline.Segments = append([]live.Segment(nil), r.session.Timeline.Segments...)
		if err := timeline.FinishPlaying(&r.session.Timeline, receipt.ID); err != nil {
			r.lastGenErr = err.Error()
			return
		}
		if !receipt.Fallback {
			segment, _ := r.session.Timeline.Segment(receipt.ID)
			if segment.Asset != nil {
				delta := segment.Asset.Observed
				delta.Events = append([]live.WorldEvent(nil), delta.Events...)
				for i := range delta.Events {
					delta.Events[i].At = receipt.End
				}
				r.session.World.Apply(delta, 40)
			}
			// This records media delivery, not an unverified claim that the model
			// performed every action in the prompt.
			r.session.World.Apply(live.WorldDelta{Events: []live.WorldEvent{{At: receipt.End, Kind: "segment_played", Summary: string(receipt.ID)}}}, 40)
		}
		if err := r.store.UpdateSession(ctx, r.session); err != nil {
			*r.session = before
			r.lastGenErr = err.Error()
			return
		}
		if receipt.Fallback {
			r.fallbackFor += receipt.End - before.Timeline.Playhead
		}
		r.receipts = r.receipts[1:]
	}
}
