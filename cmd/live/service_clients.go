package main

import (
	"context"
	"fmt"
	"math"
	"time"

	"streaming-agent/internal/generation"
	inference "streaming-agent/internal/inference/minimax"
	"streaming-agent/internal/live"
	"streaming-agent/internal/models"
	"streaming-agent/internal/pricing"
	"streaming-agent/internal/server"
	"streaming-agent/internal/session"
)

func (c modelServicesController) next(ctx context.Context) (models.Settings, error) {
	selected, found, err := c.store.ModelSettings(ctx)
	if err != nil {
		return selected, err
	}
	if !found {
		selected = activeModels(c.cfg)
	}
	return selected, models.Validate(selected)
}
func (c modelServicesController) Resolve(ctx context.Context, current *live.LiveSession) (session.Services, error) {
	selected := current.Models
	limit := current.BudgetLimitMicros
	if selected == nil {
		next, err := c.next(ctx)
		if err != nil {
			return session.Services{}, err
		}
		selected = &next
		limit, err = c.store.NextBudget(ctx)
		if err != nil {
			return session.Services{}, err
		}
		if err := c.checkBudget(*selected, limit); err != nil {
			return session.Services{}, err
		}
	}
	if err := models.Validate(*selected); err != nil {
		return session.Services{}, err
	}
	cfg := c.cfg
	if err := loadProviderSecrets(ctx, c.store, &cfg); err != nil {
		return session.Services{}, err
	}
	if cfg.MiniMaxAPIKey == "" {
		return session.Services{}, fmt.Errorf("请先保存 MiniMax 文本推理凭据")
	}
	budgetID := ""
	if limit > 0 {
		budgetID = string(current.ID)
	}
	producer, err := c.GeneratorFor(ctx, generation.Request{BudgetID: budgetID, Provider: selected.Video.Provider, Model: selected.Video.Model, Spec: generation.GenerationSpec{Resolution: selected.Video.Resolution}, Duration: cfg.SegmentDuration})
	if err != nil {
		return session.Services{}, err
	}
	m3 := inference.New(cfg.MiniMaxAPIKey, c.store.BudgetFor(budgetID))
	return session.Services{BudgetLimitMicros: limit, Models: selected, Generator: producer, Director: m3, Speech: m3, Observer: m3}, nil
}
func (c modelServicesController) checkBudget(selected models.Settings, limit int64) error {
	q, err := pricing.Lookup(selected.Video.Provider, selected.Video.Model, selected.Video.Resolution, time.Now())
	if err != nil {
		return err
	}
	seconds := math.Ceil(c.cfg.ReadyTarget.Seconds()/c.cfg.SegmentDuration.Seconds()) * c.cfg.SegmentDuration.Seconds()
	if limit < generation.Micros(q.Reserve(seconds)) {
		return fmt.Errorf("所选视频模型的启动缓冲预算不足")
	}
	return nil
}

func (c readinessReader) startConfiguration(ctx context.Context) (server.GenerationConfig, bool, error) {
	if c.services == nil {
		g, err := c.generator.GenerationConfig()
		return g, g.APIKeyConfigured && c.inference.CredentialSaved(), err
	}
	selected, err := c.services.next(ctx)
	if err != nil {
		return server.GenerationConfig{}, false, err
	}
	cfg := c.services.cfg
	if err := loadProviderSecrets(ctx, c.services.store, &cfg); err != nil {
		return server.GenerationConfig{}, false, err
	}
	q, err := pricing.Lookup(selected.Video.Provider, selected.Video.Model, selected.Video.Resolution, time.Now())
	if err != nil {
		return server.GenerationConfig{}, false, err
	}
	key := cfg.MiniMaxAPIKey
	if selected.Video.Provider == "fal" {
		key = cfg.FalAPIKey
	}
	price := q.Reserve(1)
	return server.GenerationConfig{Provider: selected.Video.Provider, Model: selected.Video.Model, Resolution: selected.Video.Resolution, UnitPriceCNYPerSecond: &price}, key != "" && cfg.MiniMaxAPIKey != "", nil
}
func (c modelServicesController) resolveAttempt(ctx context.Context, a generation.Attempt) (generation.Generator, error) {
	request := a.Request
	request.Provider = a.Provider
	return c.GeneratorFor(ctx, request)
}
