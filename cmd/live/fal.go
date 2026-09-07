package main

import (
	"context"
	"errors"
	"time"

	"streaming-agent/internal/config"
	"streaming-agent/internal/generation"
	"streaming-agent/internal/generation/fal"
	"streaming-agent/internal/generation/minimax"
	"streaming-agent/internal/server"
	"streaming-agent/internal/store"
)

type falConfigController struct{ config config.Config }

func (c falConfigController) GenerationConfig() (server.GenerationConfig, error) {
	return server.GenerationConfig{Provider: "fal", BaseURL: c.config.FalBaseURL, Model: c.config.FalModel, Resolution: "768P", DurationSeconds: int(c.config.SegmentDuration / time.Second), Ratio: "adaptive", APIKeyConfigured: c.config.FalAPIKey != ""}, nil
}

func (c falConfigController) UpdateGenerationConfig(context.Context, server.GenerationConfigUpdate) (server.GenerationConfig, error) {
	return server.GenerationConfig{}, errors.New("configure fal through FAL_KEY and FAL_MODEL, then restart after stopping the session")
}

func configureGenerator(cfg config.Config, client *minimax.Client, storage *store.Store) (generation.Generator, server.ConfigController, error) {
	if cfg.GenerationProvider == "fal" {
		producer, err := fal.New(fal.Config{APIKey: cfg.FalAPIKey, Model: cfg.FalModel, BaseURL: cfg.FalBaseURL, DataDir: cfg.DataDir})
		return producer, falConfigController{config: cfg}, err
	}
	return client, generationConfigController{client: client, store: storage}, nil
}
