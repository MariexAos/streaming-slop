package main

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"streaming-agent/internal/audience"
	"streaming-agent/internal/director"
	inference "streaming-agent/internal/inference/minimax"
	"streaming-agent/internal/live"
	"streaming-agent/internal/store"
	"streaming-agent/internal/streaming"
	"streaming-agent/internal/streaming/ffmpeg"
)

func testSpeech(ctx context.Context, s *store.Store, key, source, out string) error {
	c := inference.New(key, s)
	audio, err := c.Synthesize(ctx, "收到，大家下午好。", "female-chengshu", 5*time.Second)
	if err != nil {
		return err
	}
	path := filepath.Join(out, "speech.mp3")
	if err = os.WriteFile(path, audio, 0o640); err != nil {
		return err
	}
	segment, err := ffmpeg.NewPreparer(ffmpeg.PreparerConfig{}).Prepare(ctx, streaming.PrepareRequest{SourcePath: source, AudioPath: path, DestinationPath: filepath.Join(out, "speech.ts"), Duration: 5 * time.Second})
	if err != nil {
		return err
	}
	return saveJSON(out, "speech.json", segment)
}
func testAudience(ctx context.Context, s *store.Store, key, out string, p live.CharacterProfile) error {
	c := inference.New(key, s)
	messages := []string{"看一下弹幕，轻轻点头，不用说话"}
	observation, err := c.Observe(ctx, audience.Input{Messages: messages})
	if err != nil {
		return err
	}
	if err = saveJSON(out, "observation.json", observation); err != nil {
		return err
	}
	direction, err := c.Direct(ctx, director.Input{Duration: 5 * time.Second, World: live.WorldState{Character: live.CharacterState{Name: p.Name, Attributes: map[string]string{"appearance": p.Description}}, Scene: p.Scene, Camera: p.Camera}, Audience: &director.AudienceInput{Messages: messages, Summary: observation.Summary, Mood: observation.Mood}})
	if err != nil {
		return err
	}
	return saveJSON(out, "direction.json", direction)
}
