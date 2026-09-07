package timeline

import (
	"errors"
	"fmt"
	"time"

	"streaming-agent/internal/live"
)

func ReadyAhead(t live.Timeline) time.Duration {
	return contiguousAhead(t, func(segment live.Segment) bool {
		if segment.Asset == nil || !segment.Asset.Playable() {
			return false
		}
		switch segment.Status {
		case live.SegmentReady, live.SegmentCommitted, live.SegmentPlaying:
			return true
		default:
			return false
		}
	})
}

func SubmittedAhead(t live.Timeline) time.Duration {
	return contiguousAhead(t, func(segment live.Segment) bool {
		switch segment.Status {
		case live.SegmentGenerating:
			return true
		case live.SegmentReady, live.SegmentCommitted, live.SegmentPlaying:
			return segment.Asset != nil && segment.Asset.Playable()
		default:
			return false
		}
	})
}

func contiguousAhead(t live.Timeline, covered func(live.Segment) bool) time.Duration {
	cursor := t.Playhead
	for _, segment := range t.Segments {
		if segment.End <= cursor {
			continue
		}
		if segment.Start > cursor || !covered(segment) {
			break
		}
		cursor = segment.End
	}
	return cursor - t.Playhead
}

func CommitReady(t *live.Timeline) ([]live.SegmentID, error) {
	if err := t.Validate(); err != nil {
		return nil, fmt.Errorf("validate timeline: %w", err)
	}
	deadline := t.Playhead + t.CommitHorizon
	cursor := t.Playhead
	var committed []live.SegmentID
	for i := range t.Segments {
		segment := &t.Segments[i]
		if segment.End <= cursor {
			continue
		}
		if segment.Start > cursor || segment.Start >= deadline {
			break
		}
		switch segment.Status {
		case live.SegmentReady:
			if err := segment.Transition(live.SegmentCommitted); err != nil {
				return nil, fmt.Errorf("commit segment %s: %w", segment.ID, err)
			}
			committed = append(committed, segment.ID)
		case live.SegmentCommitted, live.SegmentPlaying, live.SegmentPlayed:
		default:
			return committed, nil
		}
		cursor = segment.End
	}
	return committed, nil
}

func StartNext(t *live.Timeline) (*live.Segment, error) {
	for i := range t.Segments {
		segment := &t.Segments[i]
		if segment.End <= t.Playhead {
			continue
		}
		if segment.Start != t.Playhead {
			return nil, errors.New("next segment does not begin at the playhead")
		}
		if segment.Status == live.SegmentPlaying {
			return segment, nil
		}
		if err := segment.Transition(live.SegmentPlaying); err != nil {
			return nil, fmt.Errorf("start segment %s: %w", segment.ID, err)
		}
		return segment, nil
	}
	return nil, errors.New("no segment available at the playhead")
}

func FinishPlaying(t *live.Timeline, id live.SegmentID) error {
	segment, ok := t.Segment(id)
	if !ok {
		return fmt.Errorf("segment %s not found", id)
	}
	if segment.Start != t.Playhead {
		return errors.New("segments must finish in timeline order")
	}
	if err := segment.Transition(live.SegmentPlayed); err != nil {
		return fmt.Errorf("finish segment %s: %w", id, err)
	}
	if err := t.AdvancePlayhead(segment.End); err != nil {
		return fmt.Errorf("advance playhead: %w", err)
	}
	return nil
}

func Replan(t *live.Timeline, id live.SegmentID, direction live.Direction) error {
	segment, ok := t.Segment(id)
	if !ok {
		return fmt.Errorf("segment %s not found", id)
	}
	return segment.Replan(direction, t.Playhead+t.CommitHorizon)
}
