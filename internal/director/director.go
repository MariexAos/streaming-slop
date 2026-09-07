package director

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"streaming-agent/internal/live"
)

// Director decides what should happen in the next timeline segment.
type Director interface {
	Direct(context.Context, Input) (live.Direction, error)
}

// Input contains only the continuity facts needed for one direction decision.
type Input struct {
	World     live.WorldState `json:"world"`
	StorySeed string          `json:"storySeed"`
	Previous  *live.Direction `json:"previous,omitempty"`
	Audience  *AudienceInput  `json:"audience,omitempty"`
	Position  time.Duration   `json:"-"`
}

type AudienceInput struct {
	WindowSeconds int      `json:"windowSeconds"`
	Messages      []string `json:"messages"`
	Summary       string   `json:"summary,omitempty"`
	Mood          string   `json:"mood,omitempty"`
	Intents       []string `json:"intents,omitempty"`
}

func (in Input) MarshalJSON() ([]byte, error) {
	type inputJSON struct {
		World      live.WorldState `json:"world"`
		StorySeed  string          `json:"storySeed"`
		Previous   *live.Direction `json:"previous,omitempty"`
		Audience   *AudienceInput  `json:"audience,omitempty"`
		PositionMS int64           `json:"positionMs"`
	}
	return json.Marshal(inputJSON{
		World:      in.World,
		StorySeed:  in.StorySeed,
		Previous:   in.Previous,
		Audience:   in.Audience,
		PositionMS: in.Position.Milliseconds(),
	})
}

type directionJSON struct {
	Action     string                    `json:"action"`
	Dialogue   string                    `json:"dialogue"`
	Emotion    string                    `json:"emotion"`
	Camera     live.CameraDirection      `json:"camera"`
	Continuity live.ContinuityConstraint `json:"continuity"`
}

// DecodeDirection strictly decodes the provider-independent Direction shape.
func DecodeDirection(data []byte) (live.Direction, error) {
	var wire directionJSON
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return live.Direction{}, fmt.Errorf("decode direction: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return live.Direction{}, fmt.Errorf("decode direction: trailing JSON value")
	}
	if strings.TrimSpace(wire.Action) == "" || strings.TrimSpace(wire.Emotion) == "" || strings.TrimSpace(wire.Camera.Shot) == "" {
		return live.Direction{}, fmt.Errorf("decode direction: action, emotion, and camera.shot are required")
	}

	direction := live.Direction{
		Action:     wire.Action,
		Dialogue:   wire.Dialogue,
		Emotion:    wire.Emotion,
		Camera:     wire.Camera,
		Continuity: wire.Continuity,
	}
	if err := direction.Validate(); err != nil {
		return live.Direction{}, fmt.Errorf("validate direction: %w", err)
	}
	return direction, nil
}

// IdleDirection returns the deterministic safe direction used after bounded
// Director retries are exhausted.
func IdleDirection() live.Direction {
	return live.Direction{
		Action:   "The host remains naturally present on the live camera and waits for the next message.",
		Dialogue: "",
		Emotion:  "calm and attentive",
		Camera: live.CameraDirection{
			Shot:     "loose medium close-up",
			Movement: "locked",
			Angle:    "slightly low and tilted",
		},
		Continuity: live.ContinuityConstraint{
			Notes:   "Preserve the current character, wardrobe, scene, lighting, and screen direction.",
			Anchors: []string{},
		},
	}
}
