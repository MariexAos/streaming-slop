package fal

import (
	"fmt"
	"strings"

	"streaming-agent/internal/live"
)

// PromptInput contains provider-neutral facts used to build a MiniMax prompt.
type PromptInput struct {
	Direction       live.Direction
	ReferenceImages []string
	ReferenceVideos []string
	ReferenceAudio  []string
}

// Compiler keeps MiniMax prompt wording out of live and generation packages.
type Compiler struct{}

func (Compiler) Compile(input PromptInput) (Spec, error) {
	if err := input.Direction.Validate(); err != nil {
		return Spec{}, fmt.Errorf("compile prompt: %w", err)
	}
	parts := []string{
		"Action: " + input.Direction.Action,
		"Emotion: " + input.Direction.Emotion,
		"Camera: " + strings.Join([]string{input.Direction.Camera.Shot, input.Direction.Camera.Movement, input.Direction.Camera.Angle}, ", "),
		"Continuity: " + input.Direction.Continuity.Notes,
	}
	if len(input.Direction.Continuity.Anchors) > 0 {
		parts = append(parts, "Continuity anchors: "+strings.Join(input.Direction.Continuity.Anchors, ", "))
	}
	if input.Direction.Dialogue != "" {
		parts = append(parts, "Dialogue: "+input.Direction.Dialogue)
	}
	return Spec{
		Prompt:             strings.Join(parts, "\n"),
		ReferenceImageURLs: append([]string(nil), input.ReferenceImages...),
		ReferenceVideoURLs: append([]string(nil), input.ReferenceVideos...),
		ReferenceAudioURLs: append([]string(nil), input.ReferenceAudio...),
	}, nil
}
