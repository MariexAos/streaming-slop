// Command acceptance exercises real provider calls or a local RTMP round trip.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"streaming-agent/internal/character"
	"streaming-agent/internal/generation"
	"streaming-agent/internal/live"
	"streaming-agent/internal/store"
)

func main() {
	mode := flag.String("mode", "inspect", "inspect, generate, compare480 or rtmp")
	source := flag.String("source", "data/acceptance/blurred-frames/video.mp4", "source clip")
	destination := flag.String("output", "data/acceptance/continuous", "artifact directory")
	duration := flag.Duration("duration", 30*time.Second, "RTMP receiving duration")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	if err := run(ctx, *mode, *source, *destination, *duration); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(ctx context.Context, mode, source, out string, duration time.Duration) error {
	if err := os.MkdirAll(out, 0o750); err != nil {
		return err
	}
	if mode == "rtmp" {
		return testRTMP(ctx, source, out, duration)
	}
	s, err := store.Open(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	defer s.Close()
	if err = s.Migrate(ctx); err != nil {
		return err
	}
	if err = s.ConfigureBudget(ctx, 10_000_000); err != nil {
		return err
	}
	secrets, err := s.ProviderSecrets(ctx)
	if err != nil {
		return err
	}
	catalog, p, err := loadCharacter(ctx, s)
	if err != nil {
		return err
	}
	_, anchors, err := catalog.Resolve(ctx, p.ID)
	if err != nil {
		return err
	}
	if mode == "speech" {
		return testSpeech(ctx, s, secrets.MiniMaxAPIKey, source, out)
	}
	if mode == "audience" {
		return testAudience(ctx, s, secrets.MiniMaxAPIKey, out, p)
	}
	if mode == "setup" {
		return saveJSON(out, "character.json", p)
	}
	if mode == "compare480" {
		return compare480(ctx, s, secrets.MiniMaxAPIKey, anchors, source, out)
	}
	return testProvider(ctx, s, secrets.MiniMaxAPIKey, p, anchors, mode, source, out)
}
func loadCharacter(ctx context.Context, s *store.Store) (character.Catalog, live.CharacterProfile, error) {
	c := character.Catalog{Root: "data", Store: s}
	p, err := s.Character(ctx, "")
	if err == nil {
		return c, p, nil
	}
	if !errorsIsNotFound(err) {
		return c, p, err
	}
	first, err := os.ReadFile("web/public/anchorframes/chat-live-start.png")
	if err != nil {
		return c, p, err
	}
	last, err := os.ReadFile("web/public/anchorframes/chat-live-end.png")
	if err != nil {
		return c, p, err
	}
	p, err = c.Publish(ctx, live.CharacterProfile{CharacterID: "host", Name: "聊天主播", Description: "长深色头发、米白色针织开衫，普通桌面直播视角、自然肤质",
		Scene: live.SceneState{Location: "木质衣柜与黑色显示器，背景自然虚化", Time: "日间", Lighting: "自然室内光"}, Camera: live.CameraState{Shot: "medium close-up", Angle: "fixed slightly low webcam"}}, first, last)
	return c, p, err
}
func saveJSON(out, name string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(out, name), data, 0o640)
}
func quote(r generation.Request) float64 { return *r.UnitPriceCNY * r.Duration.Seconds() }
