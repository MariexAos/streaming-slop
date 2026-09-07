package generation

import (
	"context"
	"errors"
	"strings"
	"time"

	"streaming-agent/internal/live"
)

type GenerationMode string

const (
	ModeTextToVideo           GenerationMode = "text_to_video"
	ModeFirstFrameToVideo     GenerationMode = "first_frame_to_video"
	ModeFirstLastFrameToVideo GenerationMode = "first_last_frame_to_video"
)

type AssetRef struct {
	ID  string `json:"id,omitempty"`
	URL string `json:"url"`
}

type GenerationSpec struct {
	Mode       GenerationMode `json:"mode"`
	Prompt     string         `json:"prompt"`
	FirstFrame *AssetRef      `json:"firstFrame,omitempty"`
	LastFrame  *AssetRef      `json:"lastFrame,omitempty"`
	Resolution string         `json:"resolution"`
	Duration   time.Duration  `json:"duration"`
	Ratio      string         `json:"ratio"`
}

func (s GenerationSpec) Validate() error {
	if strings.TrimSpace(s.Prompt) == "" {
		return errors.New("generation prompt is required")
	}
	if s.Resolution != "768P" {
		return errors.New("online generation resolution must be 768P")
	}
	if s.Duration != 5*time.Second {
		return errors.New("online generation duration must be 5 seconds")
	}
	validRef := func(ref *AssetRef) bool {
		return ref != nil && strings.TrimSpace(ref.URL) != ""
	}
	switch s.Mode {
	case ModeTextToVideo:
		if s.FirstFrame != nil || s.LastFrame != nil || s.Ratio != "16:9" {
			return errors.New("text-to-video requires ratio 16:9 and no frames")
		}
	case ModeFirstFrameToVideo:
		if !validRef(s.FirstFrame) || s.LastFrame != nil || s.Ratio != "adaptive" {
			return errors.New("first-frame video requires one first frame and adaptive ratio")
		}
	case ModeFirstLastFrameToVideo:
		if !validRef(s.FirstFrame) || !validRef(s.LastFrame) || s.Ratio != "adaptive" {
			return errors.New("first-last-frame video requires both frames and adaptive ratio")
		}
	default:
		return errors.New("unsupported online generation mode")
	}
	return nil
}

type JobStatus string

const (
	JobQueued    JobStatus = "QUEUED"
	JobRunning   JobStatus = "RUNNING"
	JobCompleted JobStatus = "COMPLETED"
	JobFailed    JobStatus = "FAILED"
	JobCancelled JobStatus = "CANCELLED"
)

type Request struct {
	AttemptID      live.AttemptID
	SegmentID      live.SegmentID
	IdempotencyKey string
	Duration       time.Duration
	Direction      live.Direction
	World          live.WorldState
	Spec           GenerationSpec
	References     []string
}

type Job struct {
	ID           string
	Status       JobStatus
	ErrorCode    string
	ErrorMessage string
}

type Result struct {
	JobID    string
	AssetURL string
	CostCNY  *float64
}

type Generator interface {
	Submit(context.Context, Request) (Job, error)
	Status(context.Context, string) (Job, error)
	Result(context.Context, string) (Result, error)
	Cancel(context.Context, string) error
}
