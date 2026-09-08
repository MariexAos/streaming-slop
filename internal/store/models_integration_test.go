//go:build integration

package store

import (
	"context"
	"os"
	"testing"
	"time"

	"streaming-agent/internal/live"
	"streaming-agent/internal/models"

	"github.com/google/uuid"
)

func TestModelSettingsAndIndependentCredentialsPersist(t *testing.T) {
	ctx := context.Background()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("TEST_DATABASE_URL is not set")
	}
	s, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := s.pool.Exec(ctx, "DELETE FROM model_settings"); err != nil {
			t.Error(err)
		}
		if _, err := s.pool.Exec(ctx, "DELETE FROM provider_secrets WHERE name IN ('minimax_api_key', 'fal_api_key')"); err != nil {
			t.Error(err)
		}
	})
	initial := models.Settings{Video: models.Selection{Provider: "minimax", Model: "MiniMax-H3-Max", Resolution: "480P"}, Text: models.Selection{Provider: "minimax", Model: "MiniMax-M3"}}
	timeline, err := live.NewTimeline(30 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	session, err := live.NewSession(live.SessionID(uuid.NewString()), live.WorldState{}, timeline, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	session.Models = &initial
	if err := s.CreateSession(ctx, &session); err != nil {
		t.Fatal(err)
	}
	selected := initial
	selected.Video = models.Selection{Provider: "fal", Model: "minimax/h3-max-turbo/image-to-video", Resolution: "480P"}
	if err := s.SaveModelSettings(ctx, selected); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{MiniMaxAPIKey: "test-minimax", FalAPIKey: "test-fal"} {
		if err := s.SaveProviderSecret(ctx, name, value); err != nil {
			t.Fatal(err)
		}
	}
	reloaded, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer reloaded.Close()
	settings, found, err := reloaded.ModelSettings(ctx)
	if err != nil || !found || settings != selected {
		t.Fatalf("settings did not persist: %+v, %v", settings, err)
	}
	secrets, err := reloaded.ProviderSecrets(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if secrets.MiniMaxAPIKey != "test-minimax" || secrets.FalAPIKey != "test-fal" {
		t.Fatal("provider credentials were mixed")
	}
	restored, err := reloaded.ActiveSession(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if restored == nil || restored.ID != session.ID || restored.Models == nil || *restored.Models != initial {
		t.Fatal("initial session models were overwritten by a route change")
	}
}
