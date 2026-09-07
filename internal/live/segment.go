package live

import (
	"errors"
	"fmt"
	"time"
)

type SegmentStatus string

const (
	SegmentPlanned    SegmentStatus = "PLANNED"
	SegmentGenerating SegmentStatus = "GENERATING"
	SegmentReady      SegmentStatus = "READY"
	SegmentCommitted  SegmentStatus = "COMMITTED"
	SegmentPlaying    SegmentStatus = "PLAYING"
	SegmentPlayed     SegmentStatus = "PLAYED"
)

type Segment struct {
	PlanRevision int64         `json:"planRevision"`
	ID           SegmentID     `json:"id"`
	Sequence     int           `json:"sequence"`
	Start        time.Duration `json:"start"`
	End          time.Duration `json:"end"`
	Direction    Direction     `json:"direction"`
	Asset        *VideoAsset   `json:"asset,omitempty"`
	Status       SegmentStatus `json:"status"`
	Version      int64         `json:"version"`
}

func NewSegment(id SegmentID, sequence int, start, end time.Duration, direction Direction) (Segment, error) {
	segment := Segment{ID: id, Sequence: sequence, Start: start, End: end, Direction: direction, Status: SegmentPlanned}
	if err := segment.Validate(); err != nil {
		return Segment{}, err
	}
	return segment, nil
}

func (s Segment) Validate() error {
	if s.ID == "" {
		return errors.New("segment id is required")
	}
	if s.Sequence < 0 {
		return errors.New("segment sequence must not be negative")
	}
	if s.Start < 0 || s.End <= s.Start {
		return errors.New("segment must have a positive duration")
	}
	if err := s.Direction.Validate(); err != nil {
		return fmt.Errorf("validate direction: %w", err)
	}
	if s.Status == SegmentReady || s.Status == SegmentCommitted || s.Status == SegmentPlaying || s.Status == SegmentPlayed {
		if s.Asset == nil || !s.Asset.Playable() {
			return errors.New("playable segment requires a verified asset")
		}
	}
	return nil
}

func (s *Segment) Transition(to SegmentStatus) error {
	allowed := map[SegmentStatus]SegmentStatus{
		SegmentPlanned:    SegmentGenerating,
		SegmentGenerating: SegmentReady,
		SegmentReady:      SegmentCommitted,
		SegmentCommitted:  SegmentPlaying,
		SegmentPlaying:    SegmentPlayed,
	}
	if allowed[s.Status] != to {
		return fmt.Errorf("segment transition %s -> %s is not allowed", s.Status, to)
	}
	if to == SegmentReady && (s.Asset == nil || !s.Asset.Playable()) {
		return errors.New("cannot mark segment ready without a verified asset")
	}
	s.Status = to
	return nil
}

func (s *Segment) MarkReady(asset VideoAsset) error {
	if !asset.Playable() {
		return errors.New("asset is not playable")
	}
	previous := s.Asset
	s.Asset = &asset
	if err := s.Transition(SegmentReady); err != nil {
		s.Asset = previous
		return err
	}
	return nil
}

func (s *Segment) Replan(direction Direction, freezeAt time.Duration) error {
	if s.Start < freezeAt {
		return errors.New("segment is inside the commit horizon")
	}
	if s.Status != SegmentPlanned && s.Status != SegmentGenerating && s.Status != SegmentReady {
		return fmt.Errorf("segment in %s cannot be replanned", s.Status)
	}
	if err := direction.Validate(); err != nil {
		return fmt.Errorf("validate replacement direction: %w", err)
	}
	s.PlanRevision++
	s.Direction = direction
	s.Asset = nil
	s.Status = SegmentPlanned
	return nil
}
