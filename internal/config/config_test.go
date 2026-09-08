package config

import "testing"

func TestLoadRequiresRuntimeEnvironment(t *testing.T) {
	for _, name := range []string{"DATABASE_URL", "ANTHROPIC_API_KEY", "MINIMAX_API_KEY", "RTMP_URL", "STORY_SEED"} {
		t.Setenv(name, "")
	}

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want missing environment error")
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://runtime")
	t.Setenv("ANTHROPIC_API_KEY", "anthropic")
	t.Setenv("MINIMAX_API_KEY", "")
	t.Setenv("MINIMAX_MODEL", "")
	t.Setenv("MINIMAX_RESOLUTION", "")
	t.Setenv("RTMP_URL", "rtmp://example/live/key")
	t.Setenv("STORY_SEED", "quiet room")
	t.Setenv("REFERENCE_IMAGE_URLS", "https://one, https://two")
	t.Setenv("BILIBILI_ROOM_ID", "")
	t.Setenv("BILIBILI_COOKIE", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AnthropicModel != "claude-haiku-4-5-20251001" {
		t.Fatalf("AnthropicModel = %q", cfg.AnthropicModel)
	}
	if cfg.MiniMaxModel != "MiniMax-H3-Max" || cfg.MiniMaxResolution != "480P" {
		t.Fatalf("MiniMax defaults = %q/%q", cfg.MiniMaxModel, cfg.MiniMaxResolution)
	}
	if len(cfg.ReferenceImageURLs) != 2 {
		t.Fatalf("ReferenceImageURLs = %v", cfg.ReferenceImageURLs)
	}
	if cfg.MiniMaxAPIKey != "" {
		t.Fatalf("MiniMaxAPIKey = %q", cfg.MiniMaxAPIKey)
	}
	if cfg.BilibiliRoomID != 0 || cfg.BilibiliCookie != "" {
		t.Fatalf("Bilibili defaults = %d/%q", cfg.BilibiliRoomID, cfg.BilibiliCookie)
	}
}
