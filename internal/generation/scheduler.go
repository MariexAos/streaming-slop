package generation

import (
	"math"
	"sort"
	"time"

	"streaming-agent/internal/live"
)

type SchedulerMode string

const (
	ModeNormal   SchedulerMode = "NORMAL"
	ModeHigh     SchedulerMode = "HIGH"
	ModeCritical SchedulerMode = "CRITICAL"
	ModeFallback SchedulerMode = "FALLBACK"
	ModePause    SchedulerMode = "PAUSE"
)

const (
	HighThreshold     = 30 * time.Second
	CriticalThreshold = 15 * time.Second
	FallbackThreshold = 5 * time.Second
	PauseThreshold    = 90 * time.Second
)

type SchedulerConfig struct {
	SegmentDuration       time.Duration
	InitialConcurrency    int
	MaxConcurrency        int
	MaxSubmissionsPerTick int
	SafetyFactor          float64
}

func DefaultSchedulerConfig(maxConcurrency int) SchedulerConfig {
	return SchedulerConfig{
		SegmentDuration: 5 * time.Second, InitialConcurrency: 3,
		MaxConcurrency: maxConcurrency, MaxSubmissionsPerTick: 4, SafetyFactor: 1.5,
	}
}

type Scheduler struct {
	config SchedulerConfig
}

func NewScheduler(config SchedulerConfig) Scheduler {
	return Scheduler{config: config}
}

func Mode(readyAhead time.Duration) SchedulerMode {
	switch {
	case readyAhead > PauseThreshold:
		return ModePause
	case readyAhead >= HighThreshold:
		return ModeNormal
	case readyAhead >= CriticalThreshold:
		return ModeHigh
	case readyAhead >= FallbackThreshold:
		return ModeCritical
	default:
		return ModeFallback
	}
}

func (s Scheduler) TargetConcurrency(successLatencies []time.Duration) int {
	maxConcurrency := max(1, s.config.MaxConcurrency)
	if len(successLatencies) == 0 {
		return clamp(s.config.InitialConcurrency, 1, maxConcurrency)
	}
	latencies := successLatencies
	if len(latencies) > 20 {
		latencies = latencies[len(latencies)-20:]
	}
	ordered := append([]time.Duration(nil), latencies...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	index := int(math.Ceil(float64(len(ordered))*0.95)) - 1
	p95 := ordered[index]
	segmentDuration := s.config.SegmentDuration
	if segmentDuration <= 0 {
		segmentDuration = 5 * time.Second
	}
	safetyFactor := s.config.SafetyFactor
	if safetyFactor <= 0 {
		safetyFactor = 1.5
	}
	target := int(math.Ceil(float64(p95) / float64(segmentDuration) * safetyFactor))
	return clamp(target, 1, maxConcurrency)
}

type Candidate struct {
	SegmentID     live.SegmentID
	AttemptNumber int
}

func (s Scheduler) Select(
	playhead, readyAhead time.Duration,
	segments []live.Segment,
	attempts []Attempt,
	successLatencies []time.Duration,
) []Candidate {
	if Mode(readyAhead) == ModePause {
		return nil
	}
	attemptCount := make(map[live.SegmentID]int)
	active := make(map[live.SegmentID]bool)
	inFlight := 0
	for _, attempt := range attempts {
		if attempt.Number > attemptCount[attempt.SegmentID] {
			attemptCount[attempt.SegmentID] = attempt.Number
		}
		if !attempt.Terminal() {
			active[attempt.SegmentID] = true
			inFlight++
		}
	}
	slots := s.TargetConcurrency(successLatencies) - inFlight
	if s.config.MaxSubmissionsPerTick > 0 && slots > s.config.MaxSubmissionsPerTick {
		slots = s.config.MaxSubmissionsPerTick
	}
	if slots <= 0 {
		return nil
	}

	ordered := append([]live.Segment(nil), segments...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Start < ordered[j].Start })
	candidates := make([]Candidate, 0, slots)
	for _, segment := range ordered {
		if len(candidates) == slots {
			break
		}
		if segment.End <= playhead || active[segment.ID] || attemptCount[segment.ID] >= MaxAttemptsPerSegment {
			continue
		}
		if segment.Status != live.SegmentPlanned && segment.Status != live.SegmentGenerating {
			continue
		}
		candidates = append(candidates, Candidate{SegmentID: segment.ID, AttemptNumber: attemptCount[segment.ID] + 1})
	}
	return candidates
}

func clamp(value, minimum, maximum int) int {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}
