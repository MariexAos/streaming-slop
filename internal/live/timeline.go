package live

import (
	"errors"
	"fmt"
	"time"
)

type Timeline struct {
	Playhead      time.Duration `json:"playhead"`
	CommitHorizon time.Duration `json:"commitHorizon"`
	Segments      []Segment     `json:"segments"`
}

func NewTimeline(commitHorizon time.Duration) (Timeline, error) {
	if commitHorizon <= 0 {
		return Timeline{}, errors.New("commit horizon must be positive")
	}
	return Timeline{CommitHorizon: commitHorizon}, nil
}

func (t *Timeline) Append(segment Segment) error {
	if err := segment.Validate(); err != nil {
		return fmt.Errorf("append segment: %w", err)
	}
	if len(t.Segments) == 0 {
		if segment.Sequence != 0 || segment.Start != 0 {
			return errors.New("timeline must start with sequence 0 at time 0")
		}
	} else {
		last := t.Segments[len(t.Segments)-1]
		if segment.Sequence != last.Sequence+1 {
			return errors.New("segment sequence must be contiguous")
		}
		if segment.Start != last.End {
			return errors.New("segment time range must be contiguous")
		}
	}
	t.Segments = append(t.Segments, segment)
	return nil
}

func (t Timeline) Validate() error {
	if t.Playhead < 0 {
		return errors.New("playhead must not be negative")
	}
	if t.CommitHorizon <= 0 {
		return errors.New("commit horizon must be positive")
	}
	for i := range t.Segments {
		if err := t.Segments[i].Validate(); err != nil {
			return fmt.Errorf("segment %d: %w", i, err)
		}
		if i == 0 {
			if t.Segments[i].Sequence != 0 || t.Segments[i].Start != 0 {
				return errors.New("timeline must start with sequence 0 at time 0")
			}
			continue
		}
		previous := t.Segments[i-1]
		if t.Segments[i].Sequence != previous.Sequence+1 || t.Segments[i].Start != previous.End {
			return errors.New("timeline segments must be contiguous")
		}
	}
	if len(t.Segments) > 0 && t.Playhead > t.Segments[len(t.Segments)-1].End {
		return errors.New("playhead is beyond the timeline")
	}
	return nil
}

func (t *Timeline) Segment(id SegmentID) (*Segment, bool) {
	for i := range t.Segments {
		if t.Segments[i].ID == id {
			return &t.Segments[i], true
		}
	}
	return nil, false
}

func (t *Timeline) AdvancePlayhead(to time.Duration) error {
	if to < t.Playhead {
		return errors.New("playhead cannot move backwards")
	}
	if to == t.Playhead {
		return nil
	}
	cursor := t.Playhead
	for i := range t.Segments {
		segment := t.Segments[i]
		if segment.End <= cursor {
			continue
		}
		if segment.Start > cursor {
			return errors.New("playhead cannot cross a timeline gap")
		}
		if segment.Status != SegmentPlaying && segment.Status != SegmentPlayed {
			return fmt.Errorf("playhead cannot cross segment %s in status %s", segment.ID, segment.Status)
		}
		if segment.End >= to {
			t.Playhead = to
			return nil
		}
		cursor = segment.End
	}
	return errors.New("playhead cannot move beyond the timeline")
}
