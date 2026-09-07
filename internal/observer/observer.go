package observer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

type Input struct {
	Messages []string
}

type Intent struct {
	Label   string  `json:"label"`
	Support float64 `json:"support"`
}

type Observation struct {
	Summary string   `json:"summary"`
	Mood    string   `json:"mood"`
	Intents []Intent `json:"intents"`
}

type Observer interface {
	Observe(context.Context, Input) (Observation, error)
}

func Decode(data []byte) (Observation, error) {
	var value Observation
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return Observation{}, fmt.Errorf("decode observation: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return Observation{}, errors.New("decode observation: trailing JSON value")
	}
	if strings.TrimSpace(value.Summary) == "" || strings.TrimSpace(value.Mood) == "" {
		return Observation{}, errors.New("decode observation: summary and mood are required")
	}
	for _, intent := range value.Intents {
		if strings.TrimSpace(intent.Label) == "" || intent.Support < 0 || intent.Support > 1 {
			return Observation{}, errors.New("decode observation: invalid intent")
		}
	}
	return value, nil
}
