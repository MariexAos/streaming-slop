package monitoring

import "time"

type Snapshot struct {
	Revision   uint64     `json:"revision"`
	ObservedAt time.Time  `json:"observedAt"`
	Session    Session    `json:"session"`
	Buffer     Buffer     `json:"buffer"`
	Timeline   []Segment  `json:"timeline"`
	Generation Generation `json:"generation"`
	Stream     Stream     `json:"stream"`
	Fallback   Fallback   `json:"fallback"`
	Controls   Controls   `json:"controls"`
}

type Session struct {
	ID            *string    `json:"id"`
	Status        string     `json:"status"`
	StartedAt     *time.Time `json:"startedAt"`
	UptimeSeconds float64    `json:"uptimeSeconds"`
	LastError     *string    `json:"lastError"`
}

type Buffer struct {
	PlayheadSeconds        float64 `json:"playheadSeconds"`
	CommitHorizonSeconds   float64 `json:"commitHorizonSeconds"`
	ReadySeconds           float64 `json:"readySeconds"`
	SubmittedSeconds       float64 `json:"submittedSeconds"`
	ReadyTargetSeconds     float64 `json:"readyTargetSeconds"`
	SubmittedTargetSeconds float64 `json:"submittedTargetSeconds"`
}

type Segment struct {
	ID           string  `json:"id"`
	Sequence     int     `json:"sequence"`
	StartSeconds float64 `json:"startSeconds"`
	EndSeconds   float64 `json:"endSeconds"`
	Status       string  `json:"status"`
	Source       string  `json:"source"`
}

type Generation struct {
	Mode               string   `json:"mode"`
	InFlight           int      `json:"inFlight"`
	TargetConcurrency  int      `json:"targetConcurrency"`
	LatencyP50Seconds  *float64 `json:"latencyP50Seconds"`
	LatencyP95Seconds  *float64 `json:"latencyP95Seconds"`
	FailuresTotal      uint64   `json:"failuresTotal"`
	FailureRate        float64  `json:"failureRate"`
	SucceededTotal     uint64   `json:"succeededTotal"`
	CostCNY            *float64 `json:"costCny"`
	CostPerLiveHourCNY *float64 `json:"costPerLiveHourCny"`
	LastError          *string  `json:"lastError"`
}

type Stream struct {
	Status             string   `json:"status"`
	BitrateKbps        *float64 `json:"bitrateKbps"`
	DroppedFramesTotal uint64   `json:"droppedFramesTotal"`
	GapTotal           uint64   `json:"gapTotal"`
	LastError          *string  `json:"lastError"`
}

type Fallback struct {
	Active       bool       `json:"active"`
	Forced       bool       `json:"forced"`
	Reason       *string    `json:"reason"`
	Since        *time.Time `json:"since"`
	SecondsTotal float64    `json:"secondsTotal"`
}

type Controls struct {
	CanStart           bool `json:"canStart"`
	CanStop            bool `json:"canStop"`
	CanEnableFallback  bool `json:"canEnableFallback"`
	CanDisableFallback bool `json:"canDisableFallback"`
}

func EmptySnapshot(now time.Time, readyTarget, submittedTarget, commitHorizon time.Duration) Snapshot {
	return Snapshot{
		ObservedAt: now,
		Session:    Session{Status: "stopped"},
		Buffer: Buffer{
			CommitHorizonSeconds:   commitHorizon.Seconds(),
			ReadyTargetSeconds:     readyTarget.Seconds(),
			SubmittedTargetSeconds: submittedTarget.Seconds(),
		},
		Generation: Generation{Mode: "normal"},
		Stream:     Stream{Status: "stopped"},
		Timeline:   []Segment{},
		Controls:   Controls{CanStart: true},
	}
}
