package live

import (
	"errors"
	"strings"
)

type CameraDirection struct {
	Shot     string `json:"shot"`
	Movement string `json:"movement"`
	Angle    string `json:"angle"`
}

type ContinuityConstraint struct {
	Notes   string   `json:"notes"`
	Anchors []string `json:"anchors,omitempty"`
}

type Direction struct {
	Action     string               `json:"action"`
	Dialogue   string               `json:"dialogue"`
	Emotion    string               `json:"emotion"`
	Camera     CameraDirection      `json:"camera"`
	Continuity ContinuityConstraint `json:"continuity"`
}

func (d Direction) Validate() error {
	if strings.TrimSpace(d.Action) == "" {
		return errors.New("direction action is required")
	}
	if strings.TrimSpace(d.Emotion) == "" {
		return errors.New("direction emotion is required")
	}
	if strings.TrimSpace(d.Camera.Shot) == "" {
		return errors.New("direction camera shot is required")
	}
	return nil
}
