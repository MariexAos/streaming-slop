package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"streaming-agent/internal/config"
	"streaming-agent/internal/models"
	"streaming-agent/internal/server"
	"streaming-agent/internal/store"
)

type modelServicesController struct {
	store *store.Store
	cfg   config.Config
}

func activeModels(cfg config.Config) models.Settings {
	video := models.Selection{Provider: "minimax", Model: cfg.MiniMaxModel, Resolution: cfg.MiniMaxResolution}
	if cfg.GenerationProvider == "fal" {
		video = models.Selection{Provider: "fal", Model: cfg.FalModel, Resolution: cfg.FalResolution}
	}
	return models.Settings{Video: video, Text: models.Selection{Provider: "minimax", Model: "MiniMax-M3"}}
}
func (c modelServicesController) Services(ctx context.Context) (server.ModelServices, error) {
	active := activeModels(c.cfg)
	saved, found, err := c.store.ModelSettings(ctx)
	if err != nil {
		return server.ModelServices{}, err
	}
	if !found {
		saved = active
	}
	options, err := models.Options(time.Now())
	if err != nil {
		return server.ModelServices{}, err
	}
	secrets, err := c.store.ProviderSecrets(ctx)
	if err != nil {
		return server.ModelServices{}, err
	}

	credentials := []server.ProviderCredential{
		{Provider: "minimax", Configured: secrets.MiniMaxAPIKey != "" || c.cfg.MiniMaxAPIKey != ""},
		{Provider: "fal", Configured: secrets.FalAPIKey != "" || c.cfg.FalAPIKey != ""},
	}
	current, err := c.store.ActiveSession(ctx)
	if err != nil {
		return server.ModelServices{}, err
	}
	var selected *models.Settings
	if current != nil {
		selected = current.Models
	}
	return server.ModelServices{Active: selected, Saved: saved, Options: options, Credentials: credentials}, nil
}
func (c modelServicesController) SaveServices(ctx context.Context, settings models.Settings) (server.ModelServices, error) {
	if err := models.Validate(settings); err != nil {
		return server.ModelServices{}, err
	}
	if err := c.store.SaveModelSettings(ctx, settings); err != nil {
		return server.ModelServices{}, fmt.Errorf("保存模型配置失败")
	}
	return c.Services(ctx)
}
func (c modelServicesController) SaveCredential(ctx context.Context, update server.CredentialUpdate) (server.ModelServices, error) {
	name := store.MiniMaxAPIKey
	if update.Provider == "fal" {
		name = store.FalAPIKey
	} else if update.Provider != "minimax" {
		return server.ModelServices{}, fmt.Errorf("不支持该供应商")
	}
	if strings.TrimSpace(update.APIKey) == "" {
		return server.ModelServices{}, fmt.Errorf("请输入凭据")
	}
	if err := c.store.SaveProviderSecret(ctx, name, update.APIKey); err != nil {
		return server.ModelServices{}, err
	}
	return c.Services(ctx)
}
