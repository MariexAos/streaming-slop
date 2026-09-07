//go:build integration

package store

import (
	"context"
	"os"
	"streaming-agent/internal/character"
	"streaming-agent/internal/live"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCharacterVersionsPinSessionAndDetectCorruption(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../../web/public/anchorframes/chat-live-start.png")
	if err != nil {
		t.Fatal(err)
	}
	c := character.Catalog{Store: s, Root: t.TempDir()}
	p, err := c.Publish(ctx, live.CharacterProfile{CharacterID: uuid.NewString(), Name: "host", Description: "white cardigan"}, data, data)
	if err != nil {
		t.Fatal(err)
	}
	old := p
	p.Description = "blue cardigan"
	next, err := c.Publish(ctx, p, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if old.ID == next.ID {
		t.Fatal("version not changed")
	}
	if err = s.SelectCharacter(ctx, next.ID); err != nil {
		t.Fatal(err)
	}
	restored, anchors, err := c.Resolve(ctx, old.ID)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Description != old.Description || len(anchors) != 2 {
		t.Fatal("old version changed")
	}
	timeline, _ := live.NewTimeline(5 * time.Second)
	session, _ := live.NewSession(live.SessionID(uuid.NewString()), live.WorldState{}, timeline, time.Now())
	session.Profile = &old
	if err = s.CreateSession(ctx, &session); err != nil {
		t.Fatal(err)
	}
	var got *live.LiveSession
	row := s.pool.QueryRow(ctx, `SELECT id::text,status,playhead_ms,world_state,runtime_config,stream_state,version,created_at,updated_at FROM live_sessions WHERE id=$1`, session.ID)
	if got, err = scanSession(row); err != nil {
		t.Fatal(err)
	}
	if got.Profile.ID != old.ID {
		t.Fatal("session followed current character")
	}
	if err = os.WriteFile(c.Root+"/"+old.FirstFrame.Path, []byte("corrupted"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err = c.Resolve(ctx, old.ID); err == nil {
		t.Fatal("corrupt image accepted")
	}
}
