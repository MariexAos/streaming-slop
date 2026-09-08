package main

import (
	"context"
	"errors"
	"net/http"
	"streaming-agent/internal/character"
	"streaming-agent/internal/config"
	"streaming-agent/internal/live"
	httpapi "streaming-agent/internal/server"
	"streaming-agent/internal/server/webui"
	"streaming-agent/internal/store"
)

func configureCharacter(ctx context.Context, s *store.Store, cfg config.Config) (character.Catalog, error) {
	catalog := character.Catalog{Store: s, Root: cfg.DataDir}
	_, err := s.Character(ctx, "")
	if err == nil {
		return catalog, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return catalog, err
	}
	first, err := webui.ReadAsset("anchorframes/chat-live-start.png")
	if err != nil {
		return catalog, err
	}
	last, err := webui.ReadAsset("anchorframes/chat-live-end.png")
	if err != nil {
		return catalog, err
	}
	_, err = catalog.Publish(ctx, live.CharacterProfile{CharacterID: "host", Name: "聊天主播", Description: cfg.Character,
		Scene:  live.SceneState{Location: "普通房间，木质衣柜、黑色显示器，背景自然虚化", Time: "日间", Lighting: "自然室内光"},
		Camera: live.CameraState{Shot: "medium close-up", Angle: "fixed slightly low desktop webcam"},
	}, first, last)
	return catalog, err
}

func managementHandler(catalog character.Catalog, store *store.Store, handler, unresolved http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/api/v1/characters", httpapi.CharacterHandler(catalog))
	mux.Handle("/api/v1/characters/", httpapi.CharacterHandler(catalog))
	mux.Handle("/api/v1/budget", budgetHandler(store))
	mux.Handle("/api/v1/attempts/", unresolved)
	mux.Handle("/", handler)
	return mux
}
