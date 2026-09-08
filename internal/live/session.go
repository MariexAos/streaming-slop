package live

import (
	"errors"
	"fmt"
	"time"
)

type SessionStatus string

const (
	SessionStarting   SessionStatus = "STARTING"
	SessionBuffering  SessionStatus = "BUFFERING"
	SessionRunning    SessionStatus = "RUNNING"
	SessionStopping   SessionStatus = "STOPPING"
	SessionStopped    SessionStatus = "STOPPED"
	SessionRecovering SessionStatus = "RECOVERING"
	SessionFailed     SessionStatus = "FAILED"
)

type StreamState struct {
	Status        string `json:"status"`
	LastError     string `json:"lastError,omitempty"`
	GapTotal      int64  `json:"gapTotal"`
	DroppedFrames int64  `json:"droppedFrames"`
}

type ModelSelection struct {
	Provider   string `json:"provider"`
	Model      string `json:"model"`
	Resolution string `json:"resolution,omitempty"`
}
type ModelSettings struct {
	Video ModelSelection `json:"video"`
	Text  ModelSelection `json:"text"`
}

type LiveSession struct {
	BudgetLimitMicros int64             `json:"budgetLimitMicros,omitempty"`
	Models            *ModelSettings    `json:"models,omitempty"`
	Profile           *CharacterProfile `json:"profile,omitempty"`
	ID                SessionID         `json:"id"`
	Status            SessionStatus     `json:"status"`
	World             WorldState        `json:"world"`
	Timeline          Timeline          `json:"timeline"`
	Stream            StreamState       `json:"stream"`
	Version           int64             `json:"version"`
	CreatedAt         time.Time         `json:"createdAt"`
	UpdatedAt         time.Time         `json:"updatedAt"`
}

func NewSession(id SessionID, world WorldState, timeline Timeline, now time.Time) (LiveSession, error) {
	if id == "" {
		return LiveSession{}, errors.New("session id is required")
	}
	if err := timeline.Validate(); err != nil {
		return LiveSession{}, fmt.Errorf("validate timeline: %w", err)
	}
	return LiveSession{
		ID: id, Status: SessionStarting, World: world, Timeline: timeline,
		CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (s *LiveSession) Transition(to SessionStatus, now time.Time) error {
	allowed := map[SessionStatus]map[SessionStatus]bool{
		SessionStarting:   {SessionBuffering: true, SessionStopping: true, SessionRecovering: true, SessionFailed: true},
		SessionBuffering:  {SessionRunning: true, SessionStopping: true, SessionRecovering: true, SessionFailed: true},
		SessionRunning:    {SessionStopping: true, SessionRecovering: true, SessionFailed: true},
		SessionStopping:   {SessionStopped: true, SessionRecovering: true, SessionFailed: true},
		SessionRecovering: {SessionBuffering: true, SessionStopping: true, SessionFailed: true},
	}
	if !allowed[s.Status][to] {
		return fmt.Errorf("session transition %s -> %s is not allowed", s.Status, to)
	}
	s.Status = to
	s.UpdatedAt = now
	return nil
}
