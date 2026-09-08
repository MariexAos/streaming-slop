package session

import (
	"errors"
	"time"

	"streaming-agent/internal/audience"
	"streaming-agent/internal/live"
)

var ErrHistoryNotFound = errors.New("session history not found")

type HistorySummary struct {
	BudgetLimitMicros int64     `json:"budgetLimitMicros,omitempty"`
	ReservedMicros    int64     `json:"reservedMicros,omitempty"`
	ID                string    `json:"id"`
	Status            string    `json:"status"`
	StartedAt         time.Time `json:"startedAt"`
	EndedAt           time.Time `json:"endedAt"`
	SegmentTotal      int       `json:"segmentTotal"`
	ReadyTotal        int       `json:"readyTotal"`
	CostCNY           float64   `json:"costCny"`
}

type HistorySegment struct {
	ID           string         `json:"id"`
	Sequence     int            `json:"sequence"`
	StartSeconds float64        `json:"startSeconds"`
	EndSeconds   float64        `json:"endSeconds"`
	Status       string         `json:"status"`
	Direction    live.Direction `json:"direction"`
	Playable     bool           `json:"playable"`
}

type ObserverRun struct {
	Revision    uint64               `json:"revision"`
	Messages    []string             `json:"messages"`
	Observation audience.Observation `json:"observation"`
	Error       *string              `json:"error"`
	CreatedAt   time.Time            `json:"createdAt"`
}

type History struct {
	Session      HistorySummary   `json:"session"`
	Segments     []HistorySegment `json:"segments"`
	ObserverRuns []ObserverRun    `json:"observerRuns"`
}
