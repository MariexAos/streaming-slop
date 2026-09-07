package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"streaming-agent/internal/adapter/in/bilibili"
	"streaming-agent/internal/adapter/in/httpapi"
	"streaming-agent/internal/adapter/in/webui"
	"streaming-agent/internal/adapter/out/ffmpeg"
	"streaming-agent/internal/adapter/out/minimax"
	"streaming-agent/internal/adapter/out/qwen"
	postgresstore "streaming-agent/internal/adapter/out/storage/postgres"
	"streaming-agent/internal/audience"
	"streaming-agent/internal/generation"
	"streaming-agent/internal/monitoring"
	"streaming-agent/internal/platform/config"
	"streaming-agent/internal/session"
)

func main() {
	if err := run(); err != nil {
		slog.Error("live service stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if !httpapi.IsLoopbackAddress(config.HTTPAddr) {
		return fmt.Errorf("refusing non-loopback listen address")
	}
	if err := os.MkdirAll(cfg.DataDir, 0o750); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}

	rootCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	store, err := postgresstore.Open(rootCtx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer store.Close()
	if err := store.Migrate(rootCtx); err != nil {
		return err
	}
	secrets, err := store.ProviderSecrets(rootCtx)
	if err != nil {
		return err
	}
	if cfg.QwenAPIKey == "" {
		cfg.QwenAPIKey = secrets.QwenAPIKey
	}
	if cfg.MiniMaxAPIKey == "" {
		cfg.MiniMaxAPIKey = secrets.MiniMaxAPIKey
	}

	qwenClient := qwen.New(qwen.Config{
		APIKey: cfg.QwenAPIKey, BaseURL: cfg.QwenBaseURL,
		ObserverModel: cfg.QwenObserverModel, DirectorModel: cfg.QwenDirectorModel,
	})
	generatorClient, err := minimax.New(minimax.Config{
		APIKey: cfg.MiniMaxAPIKey, BaseURL: cfg.MiniMaxBaseURL, DataDir: cfg.DataDir,
		Settings: minimax.DefaultSettings(),
	})
	if err != nil {
		return err
	}
	output, err := ffmpeg.NewOutput(ffmpeg.OutputConfig{FFmpegPath: cfg.FFmpegPath, RTMPURL: cfg.RTMPURL})
	if err != nil {
		return err
	}
	preparer := ffmpeg.NewPreparer(ffmpeg.PreparerConfig{FFmpegPath: cfg.FFmpegPath, FFprobePath: cfg.FFprobePath})
	schedulerConfig := generation.DefaultSchedulerConfig(cfg.GenerationMaxConcurrency)
	schedulerConfig.InitialConcurrency = cfg.InitialConcurrency
	schedulerConfig.SegmentDuration = cfg.SegmentDuration
	scheduler := generation.NewScheduler(schedulerConfig)

	registry := prometheus.NewRegistry()
	metrics := monitoring.NewMetrics(registry)
	hub := monitoring.NewHub(monitoring.EmptySnapshot(time.Now().UTC(), cfg.ReadyTarget, cfg.SubmittedTarget, cfg.CommitHorizon))
	audienceWindow := audience.NewWindow(20 * time.Second)
	bilibiliManager := bilibili.NewManager(audienceWindow)
	defer bilibiliManager.Close()
	if cfg.BilibiliRoomID > 0 {
		if err := bilibiliManager.Update(cfg.BilibiliRoomID, cfg.BilibiliCookie); err != nil {
			return err
		}
	}
	runtime := session.NewRuntime(session.RuntimeConfig{
		Audience: audienceWindow, Observer: qwenClient,
		StorySeed: cfg.StorySeed, Character: cfg.Character, AnchorFrames: anchorFrames(),
		SegmentDuration: cfg.SegmentDuration, CommitHorizon: cfg.CommitHorizon,
		ReadyTarget: cfg.ReadyTarget, SubmittedTarget: cfg.SubmittedTarget,
		PlannedHorizon: cfg.PlannedHorizon, SchedulerInterval: cfg.SchedulerInterval,
		PollInterval: cfg.GenerationPoll, FallbackExitReady: cfg.FallbackExitReady,
		DataDir: filepath.Clean(cfg.DataDir), Provider: "minimax",
	}, store, qwenClient, generatorClient, preparer, output, scheduler, hub, metrics, slog.Default())
	if err := runtime.Recover(rootCtx); err != nil {
		return err
	}

	handler := httpapi.New(
		runtime, hub, promhttp.HandlerFor(registry, promhttp.HandlerOpts{}), webui.NewHandler(),
		generationConfigController{client: generatorClient, store: store}, bilibiliConfigController{manager: bilibiliManager, audience: audienceWindow},
		qwenConfigController{client: qwenClient, store: store},
	)
	server := &http.Server{
		Addr: config.HTTPAddr, Handler: handler,
		ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second,
	}
	serverErrors := make(chan error, 1)
	go func() {
		slog.Info("operations console listening", "address", config.HTTPAddr)
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case <-rootCtx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = runtime.Stop(shutdownCtx)
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown HTTP server: %w", err)
		}
		return nil
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve HTTP: %w", err)
	}
}

func anchorFrames() map[string]string {
	names := []string{"chat-live-start", "chat-live-end"}
	frames := make(map[string]string, len(names))
	for _, name := range names {
		data, err := webui.ReadAsset("anchorframes/" + name + ".png")
		if err == nil {
			frames[name] = "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)
		}
	}
	return frames
}

type generationConfigController struct {
	client *minimax.Client
	store  *postgresstore.Store
}

type qwenConfigController struct {
	client *qwen.Client
	store  *postgresstore.Store
}

func (c qwenConfigController) QwenConfig() httpapi.QwenConfig {
	settings := c.client.Settings()
	return httpapi.QwenConfig{
		BaseURL: settings.BaseURL, ObserverModel: settings.ObserverModel,
		DirectorModel: settings.DirectorModel, APIKeyConfigured: settings.APIKeyConfigured,
	}
}

func (c qwenConfigController) UpdateQwenConfig(ctx context.Context, update httpapi.QwenConfigUpdate) (httpapi.QwenConfig, error) {
	if err := c.client.Update(update.BaseURL, update.APIKey); err != nil {
		return httpapi.QwenConfig{}, err
	}
	if update.APIKey != "" {
		if err := c.store.SaveProviderSecret(ctx, postgresstore.QwenAPIKey, update.APIKey); err != nil {
			return httpapi.QwenConfig{}, err
		}
	}
	return c.QwenConfig(), nil
}

type bilibiliConfigController struct {
	manager  *bilibili.Manager
	audience *audience.Window
}

func (c bilibiliConfigController) BilibiliConfig() httpapi.BilibiliConfig {
	return projectBilibiliConfig(c.manager.Config())
}

func (c bilibiliConfigController) UpdateBilibiliConfig(_ context.Context, update httpapi.BilibiliConfigUpdate) (httpapi.BilibiliConfig, error) {
	if err := c.manager.Update(update.RoomID, update.Cookie); err != nil {
		return httpapi.BilibiliConfig{}, err
	}
	return projectBilibiliConfig(c.manager.Config()), nil
}

func (c bilibiliConfigController) InjectMockDanmaku(_ context.Context, message httpapi.MockDanmaku) error {
	text := strings.TrimSpace(message.Text)
	if text == "" {
		return errors.New("mock danmaku text is required")
	}
	if len([]rune(text)) > 200 {
		return errors.New("mock danmaku text is too long")
	}
	c.audience.Add(audience.Message{
		Username: strings.TrimSpace(message.Username), Text: text, At: time.Now().UTC(),
	})
	return nil
}

func projectBilibiliConfig(config bilibili.Config) httpapi.BilibiliConfig {
	var lastError *string
	if config.LastError != "" {
		value := config.LastError
		lastError = &value
	}
	return httpapi.BilibiliConfig{
		RoomID: config.RoomID, CookieConfigured: config.CookieConfigured,
		Status: string(config.Status), LastError: lastError,
	}
}

func (c generationConfigController) GenerationConfig() (httpapi.GenerationConfig, error) {
	return c.project(c.client.Settings())
}

func (c generationConfigController) UpdateGenerationConfig(ctx context.Context, update httpapi.GenerationConfigUpdate) (httpapi.GenerationConfig, error) {
	settings := minimax.Settings{
		Model: update.Model, Resolution: update.Resolution,
		Duration: update.DurationSeconds, Ratio: update.Ratio,
	}
	if err := settings.Validate(); err != nil {
		return httpapi.GenerationConfig{}, err
	}
	if update.APIKey != "" {
		if err := c.client.UpdateAPIKey(update.APIKey); err != nil {
			return httpapi.GenerationConfig{}, err
		}
		if err := c.store.SaveProviderSecret(ctx, postgresstore.MiniMaxAPIKey, update.APIKey); err != nil {
			return httpapi.GenerationConfig{}, err
		}
	}
	if err := c.client.UpdateSettings(settings); err != nil {
		return httpapi.GenerationConfig{}, err
	}
	return c.project(settings)
}

func (c generationConfigController) project(settings minimax.Settings) (httpapi.GenerationConfig, error) {
	price, err := minimax.UnitPriceCNY(settings.Model, settings.Resolution)
	if err != nil {
		return httpapi.GenerationConfig{}, err
	}
	return httpapi.GenerationConfig{
		Provider: "MiniMax", BaseURL: c.client.BaseURL(), Model: settings.Model,
		Resolution: settings.Resolution, DurationSeconds: settings.Duration,
		Ratio: settings.Ratio, APIKeyConfigured: c.client.APIKeyConfigured(),
		UnitPriceCNYPerSecond: price,
	}, nil
}
