package session

import (
	"context"
	"os"
	"streaming-agent/internal/live"
)

func (r *Runtime) speech(ctx context.Context, id live.AttemptID, path string) (string, error) {
	if r.config.Speech == nil {
		return "", nil
	}
	r.mu.Lock()
	request := r.attempt(id).Request
	r.mu.Unlock()
	voice := request.World.Character.Attributes["voiceId"]
	if voice == "" || request.Direction.Dialogue == "" {
		return "", nil
	}
	data, err := r.config.Speech.Synthesize(ctx, request.Direction.Dialogue, voice, request.Duration)
	if err != nil {
		return "", err
	}
	audio := path + ".speech.mp3"
	if err = os.WriteFile(audio, data, 0o640); err != nil {
		return "", err
	}
	return audio, nil
}
