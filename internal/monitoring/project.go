package monitoring

import (
	"math"
	"sort"
	"strings"
	"time"

	"streaming-agent/internal/generation"
	"streaming-agent/internal/live"
	"streaming-agent/internal/streaming"
	"streaming-agent/internal/timeline"
)

type ProjectionInput struct {
	Now                 time.Time
	Session             *live.LiveSession
	Attempts            []generation.Attempt
	TargetConcurrency   int
	ReadyTarget         time.Duration
	SubmittedTarget     time.Duration
	FallbackActive      bool
	FallbackForced      bool
	FallbackReason      string
	FallbackSince       *time.Time
	FallbackDuration    time.Duration
	StreamStats         streaming.Stats
	StreamStatus        string
	StreamGapTotal      uint64
	GenerationLastError string
	SegmentDuration     time.Duration
}

func Project(input ProjectionInput) Snapshot {
	if input.Now.IsZero() {
		input.Now = time.Now().UTC()
	}
	if input.Session == nil {
		return EmptySnapshot(input.Now, input.ReadyTarget, input.SubmittedTarget, 30*time.Second)
	}

	current := input.Session
	readyAhead := timeline.ReadyAhead(current.Timeline)
	submittedAhead := timeline.SubmittedAhead(current.Timeline)
	mode := generation.Mode(readyAhead)
	latencies, failures, succeeded, inFlight, cost, knownCost := summarizeAttempts(input.Attempts)
	p50, p95 := quantiles(latencies)
	failureRate := 0.0
	if len(input.Attempts) > 0 {
		failureRate = float64(failures) / float64(len(input.Attempts))
	}

	id := string(current.ID)
	started := current.CreatedAt
	snapshot := Snapshot{
		ObservedAt: input.Now,
		Session: Session{
			ID: &id, Status: strings.ToLower(string(current.Status)), StartedAt: &started,
			UptimeSeconds: max(0, input.Now.Sub(started).Seconds()),
		},
		Buffer: Buffer{
			PlayheadSeconds:      current.Timeline.Playhead.Seconds(),
			CommitHorizonSeconds: current.Timeline.CommitHorizon.Seconds(),
			ReadySeconds:         readyAhead.Seconds(), SubmittedSeconds: submittedAhead.Seconds(),
			ReadyTargetSeconds: input.ReadyTarget.Seconds(), SubmittedTargetSeconds: input.SubmittedTarget.Seconds(),
		},
		Timeline: projectSegments(current.Timeline),
		Generation: Generation{
			Mode: strings.ToLower(string(mode)), InFlight: inFlight,
			TargetConcurrency: input.TargetConcurrency,
			LatencyP50Seconds: p50, LatencyP95Seconds: p95,
			FailuresTotal: uint64(failures), FailureRate: failureRate,
			SucceededTotal: uint64(succeeded),
		},
		Stream: Stream{
			Status:             strings.ToLower(input.StreamStatus),
			DroppedFramesTotal: input.StreamStats.DroppedFrames,
			GapTotal:           input.StreamGapTotal,
		},
		Fallback: Fallback{
			Active: input.FallbackActive, Forced: input.FallbackForced,
			Since: input.FallbackSince, SecondsTotal: input.FallbackDuration.Seconds(),
		},
	}
	if snapshot.Stream.Status == "" {
		snapshot.Stream.Status = "stopped"
	}
	if current.Stream.LastError != "" {
		snapshot.Session.LastError = stringPointer(current.Stream.LastError)
	}
	if input.StreamStats.BitrateKbps > 0 {
		snapshot.Stream.BitrateKbps = floatPointer(input.StreamStats.BitrateKbps)
	}
	if input.StreamStats.LastError != "" {
		snapshot.Stream.LastError = stringPointer(input.StreamStats.LastError)
	}
	if input.GenerationLastError != "" {
		snapshot.Generation.LastError = stringPointer(input.GenerationLastError)
	}
	if input.FallbackReason != "" {
		snapshot.Fallback.Reason = stringPointer(input.FallbackReason)
	}
	if knownCost {
		snapshot.Generation.CostCNY = floatPointer(cost)
		if input.SegmentDuration <= 0 {
			input.SegmentDuration = 5 * time.Second
		}
		generatedHours := (time.Duration(succeeded) * input.SegmentDuration).Hours()
		if generatedHours > 0 {
			snapshot.Generation.CostPerLiveHourCNY = floatPointer(cost / generatedHours)
		}
	}
	snapshot.Controls = controls(current.Status, input.FallbackForced)
	return snapshot
}

func projectSegments(t live.Timeline) []Segment {
	start := 0
	for i, segment := range t.Segments {
		if segment.End > t.Playhead {
			start = max(0, i-3)
			break
		}
	}
	end := min(len(t.Segments), start+24)
	segments := make([]Segment, 0, end-start)
	for _, item := range t.Segments[start:end] {
		source := string(live.AssetSourceGenerated)
		if item.Asset != nil && item.Asset.Source == live.AssetSourceFallback {
			source = string(live.AssetSourceFallback)
		}
		segments = append(segments, Segment{
			ID: string(item.ID), Sequence: item.Sequence,
			StartSeconds: item.Start.Seconds(), EndSeconds: item.End.Seconds(),
			Status: strings.ToLower(string(item.Status)), Source: source,
		})
	}
	return segments
}

func summarizeAttempts(attempts []generation.Attempt) ([]time.Duration, int, int, int, float64, bool) {
	var latencies []time.Duration
	failures, succeeded, inFlight := 0, 0, 0
	cost, knownCost := 0.0, false
	for _, attempt := range attempts {
		switch attempt.Status {
		case generation.AttemptSucceeded:
			succeeded++
			if attempt.Latency > 0 {
				latencies = append(latencies, attempt.Latency)
			}
		case generation.AttemptFailed:
			failures++
		default:
			if !attempt.Terminal() {
				inFlight++
			}
		}
		if attempt.CostCNY != nil {
			knownCost = true
			cost += *attempt.CostCNY
		}
	}
	return latencies, failures, succeeded, inFlight, cost, knownCost
}

func quantiles(values []time.Duration) (*float64, *float64) {
	if len(values) == 0 {
		return nil, nil
	}
	ordered := append([]time.Duration(nil), values...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	at := func(q float64) *float64 {
		index := int(math.Ceil(float64(len(ordered))*q)) - 1
		value := ordered[max(0, index)].Seconds()
		return &value
	}
	return at(0.5), at(0.95)
}

func controls(status live.SessionStatus, forced bool) Controls {
	running := status == live.SessionStarting || status == live.SessionBuffering || status == live.SessionRunning || status == live.SessionRecovering
	return Controls{
		CanStart:           status == live.SessionStopped || status == live.SessionFailed,
		CanStop:            running,
		CanEnableFallback:  running && !forced,
		CanDisableFallback: running && forced,
	}
}

func stringPointer(value string) *string  { return &value }
func floatPointer(value float64) *float64 { return &value }
