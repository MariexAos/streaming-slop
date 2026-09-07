package flow

import (
	"errors"
	"fmt"
	"time"

	"streaming-agent/internal/generation"
)

type FlowKind string

const (
	FlowKindPrimary  FlowKind = "primary"
	FlowKindModifier FlowKind = "modifier"
)

type NodeDefinition struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Label string `json:"label"`
}

type EdgeDefinition struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type FlowDefinition struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Version     string           `json:"version"`
	Enabled     bool             `json:"enabled"`
	Kind        FlowKind         `json:"kind"`
	Description string           `json:"description"`
	Nodes       []NodeDefinition `json:"nodes"`
	Edges       []EdgeDefinition `json:"edges"`
}

type RunStatus string

const (
	RunIdle      RunStatus = "idle"
	RunRunning   RunStatus = "running"
	RunCompleted RunStatus = "completed"
	RunFailed    RunStatus = "failed"
)

type NodeRunStatus string

const (
	NodePending   NodeRunStatus = "pending"
	NodeRunning   NodeRunStatus = "running"
	NodeCompleted NodeRunStatus = "completed"
	NodeFailed    NodeRunStatus = "failed"
	NodeSkipped   NodeRunStatus = "skipped"
)

type NodeRun struct {
	ID         string        `json:"id"`
	Status     NodeRunStatus `json:"status"`
	DurationMS *int64        `json:"durationMs"`
	Summary    string        `json:"summary"`
	CostCNY    *float64      `json:"costCny"`
}

type DirectionAxis struct {
	Name    string  `json:"name"`
	Current float64 `json:"current"`
	Target  float64 `json:"target"`
}

type DirectionPlan struct {
	CurrentLabel       string          `json:"currentLabel"`
	TargetLabel        string          `json:"targetLabel"`
	Summary            string          `json:"summary"`
	HorizonSeconds     int             `json:"horizonSeconds"`
	EffectiveInSeconds float64         `json:"effectiveInSeconds"`
	Axes               []DirectionAxis `json:"axes"`
}

type AudienceIntent struct {
	Label   string  `json:"label"`
	Support float64 `json:"support"`
}

type AudienceState struct {
	WindowSeconds int              `json:"windowSeconds"`
	MessageCount  int              `json:"messageCount"`
	Summary       string           `json:"summary"`
	Intents       []AudienceIntent `json:"intents"`
}

type BeatStatus string

const (
	BeatPlanned    BeatStatus = "planned"
	BeatSubmitted  BeatStatus = "submitted"
	BeatGenerating BeatStatus = "generating"
	BeatReady      BeatStatus = "ready"
	BeatLocked     BeatStatus = "locked"
)

type Beat struct {
	Index        int                       `json:"index"`
	StartSeconds int                       `json:"startSeconds"`
	EndSeconds   int                       `json:"endSeconds"`
	Intent       string                    `json:"intent"`
	Mode         generation.GenerationMode `json:"mode"`
	AnchorFrame  *string                   `json:"anchorFrame"`
	Status       BeatStatus                `json:"status"`
}

type FlowRun struct {
	FlowID    string        `json:"flowId"`
	Status    RunStatus     `json:"status"`
	StartedAt *time.Time    `json:"startedAt"`
	Direction DirectionPlan `json:"direction"`
	Audience  AudienceState `json:"audience"`
	Nodes     []NodeRun     `json:"nodes"`
	Beats     []Beat        `json:"beats"`
}

func (p DirectionPlan) Validate(beats []Beat) error {
	if p.HorizonSeconds != 60 {
		return errors.New("direction horizon must be 60 seconds")
	}
	if len(beats) != 12 {
		return errors.New("direction plan must contain 12 beats")
	}
	for i, beat := range beats {
		if beat.Index != i || beat.StartSeconds != i*5 || beat.EndSeconds != (i+1)*5 {
			return fmt.Errorf("beat %d must cover its five-second slot", i)
		}
	}
	return nil
}
