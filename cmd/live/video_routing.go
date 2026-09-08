package main

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"streaming-agent/internal/generation"
	"streaming-agent/internal/generation/fal"
	video "streaming-agent/internal/generation/minimax"
	"streaming-agent/internal/models"
)

func (c modelServicesController) BuildGeneration(ctx context.Context, request generation.Request) (generation.Request, error) {
	selected, err := c.next(ctx)
	if err != nil {
		return request, err
	}
	request.Provider = selected.Video.Provider
	request.Model = selected.Video.Model
	request.Spec.Resolution = selected.Video.Resolution
	client, err := c.GeneratorFor(ctx, request)
	if err != nil {
		return request, err
	}
	builder, ok := client.(generation.RequestBuilder)
	if !ok {
		return request, fmt.Errorf("selected provider cannot quote requests")
	}
	return builder.BuildRequest(request)
}
func (c modelServicesController) GeneratorFor(ctx context.Context, request generation.Request) (generation.Generator, error) {
	selected := models.Selection{Provider: request.Provider, Model: request.Model, Resolution: request.Spec.Resolution}
	if selected.Model == "" && selected.Provider == "minimax" {
		selected.Model = video.DefaultModel
	}
	if selected.Resolution == "" {
		selected.Resolution = "768P"
	}
	if err := models.ValidateVideo(selected); err != nil {
		return nil, err
	}
	cfg := c.cfg
	if err := loadProviderSecrets(ctx, c.store, &cfg); err != nil {
		return nil, err
	}
	directory := filepath.Join(cfg.DataDir, selected.Provider)
	var client generation.Generator
	var err error
	if selected.Provider == "fal" {
		client, err = fal.New(fal.Config{APIKey: cfg.FalAPIKey, Model: selected.Model, Resolution: selected.Resolution, BaseURL: cfg.FalBaseURL, DataDir: directory})
	} else {
		if cfg.MiniMaxAPIKey == "" {
			return nil, fmt.Errorf("请先保存 MiniMax 视频凭据")
		}
		duration := request.Duration
		if duration <= 0 {
			duration = cfg.SegmentDuration
		}
		client, err = video.New(video.Config{APIKey: cfg.MiniMaxAPIKey, BaseURL: cfg.MiniMaxBaseURL, DataDir: directory, Settings: video.Settings{Model: selected.Model, Resolution: selected.Resolution, Duration: int(duration / time.Second), Ratio: "16:9"}})
	}
	if err != nil {
		return nil, err
	}
	return generation.Budgeted{Generator: client, Budget: c.store.BudgetFor(request.BudgetID), AllowUnsettledCost: selected.Provider == "fal"}, nil
}
