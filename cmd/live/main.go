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

	"streaming-agent/internal/audience"
	"streaming-agent/internal/audience/bilibili"
	"streaming-agent/internal/config"
	"streaming-agent/internal/generation"
	"streaming-agent/internal/generation/minimax"
	"streaming-agent/internal/inference/qwen"
	"streaming-agent/internal/live"
	"streaming-agent/internal/monitoring"
	"streaming-agent/internal/pricing"
	httpapi "streaming-agent/internal/server"
	"streaming-agent/internal/server/webui"
	"streaming-agent/internal/session"
	postgresstore "streaming-agent/internal/store"
	"streaming-agent/internal/streaming/ffmpeg"
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
	if err := loadProviderSecrets(rootCtx, store, &cfg); err != nil {
		return err
	}

	if err := store.ConfigureSessionBudgets(rootCtx, 10_000_000); err != nil {
		return err
	}
	catalog, err := configureCharacter(rootCtx, store, cfg)
	if err != nil {
		return err
	}
	generatorClient, err := configureMiniMax(cfg)
	if err != nil {
		return err
	}
	producer, generationController, err := configureGenerator(cfg, generatorClient, store)
	if err != nil {
		return err
	}
	producer = generation.Budgeted{Generator: producer, Budget: store}
	services := modelServicesController{store: store, cfg: cfg}
	startReconciliation(rootCtx, store, services.resolveAttempt)
	output, preparer, err := configureMedia(cfg)
	if err != nil {
		return err
	}

	m3, generationController := configureInference(cfg.MiniMaxAPIKey, store, generationController)

	registry := prometheus.NewRegistry()
	metrics := monitoring.NewMetrics(registry)
	hub := monitoring.NewHub(monitoring.EmptySnapshot(time.Now().UTC(), cfg.ReadyTarget, cfg.SubmittedTarget, cfg.CommitHorizon))
	audienceWindow, bilibiliManager, err := configureAudience(cfg)
	if err != nil {
		return err
	}
	defer bilibiliManager.Close()
	readiness := readinessReader{services: &services, catalog: catalog, store: store, generator: generationController, cfg: cfg, inference: m3}
	runtime := session.NewRuntime(session.RuntimeConfig{
		BuildGeneration: services.BuildGeneration, GeneratorFor: services.GeneratorFor, ResolveServices: services.Resolve, CheckStart: readiness.check, Speech: m3, Continuous: false, Frames: preparer, Catalog: catalog, Audience: audienceWindow, Observer: m3,
		StorySeed: cfg.StorySeed, Character: cfg.Character, AnchorFrames: anchorFrames(),
		SegmentDuration: cfg.SegmentDuration, CommitHorizon: cfg.CommitHorizon,
		ReadyTarget: cfg.ReadyTarget, SubmittedTarget: cfg.SubmittedTarget,
		PlannedHorizon: cfg.PlannedHorizon, SchedulerInterval: cfg.SchedulerInterval,
		PollInterval: cfg.GenerationPoll, FallbackExitReady: cfg.FallbackExitReady,
		DataDir: filepath.Clean(cfg.DataDir), Provider: cfg.GenerationProvider,
	}, store, m3, producer, preparer, output, configureScheduler(cfg), hub, metrics, slog.Default())
	if err := runtime.Recover(rootCtx); err != nil {
		return err
	}

	var handler http.Handler = httpapi.New(
		runtime, hub, promhttp.HandlerFor(registry, promhttp.HandlerOpts{}), webui.NewHandler(),
		generationController, bilibiliConfigController{manager: bilibiliManager, audience: audienceWindow, runtime: runtime},
		qwenConfigController{client: configureQwen(cfg), store: store},
	)
	handler = httpapi.ModelServicesHandler(modelServicesController{store: store, cfg: cfg}, handler)
	return serveGuests(rootCtx, readiness.handler(managementHandler(catalog, store, handler, unresolvedHandler(store, runtime, services.resolveAttempt))), hub, bilibiliConfigController{manager: bilibiliManager, audience: audienceWindow, runtime: runtime})
}

func serve(rootCtx context.Context, handler, guest http.Handler, runtime *session.Runtime) error {
	server := &http.Server{
		Addr: config.HTTPAddr, Handler: handler,
		ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second,
	}

	guestServer := &http.Server{Addr: "0.0.0.0:8082", Handler: guest, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	defer func() { _ = guestServer.Close() }()
	defer func() { _ = server.Close() }()
	serverErrors := make(chan error, 2)
	go func() {
		slog.Info("guest livestream listening", "address", guestServer.Addr)
		serverErrors <- guestServer.ListenAndServe()
	}()
	go func() {
		slog.Info("operations console listening", "address", config.HTTPAddr)
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case <-rootCtx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = runtime.Stop(shutdownCtx)
		_ = guestServer.Shutdown(shutdownCtx)
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
	runtime  *session.Runtime
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
	view := c.runtime.View()
	if view.Session == nil || (view.Session.Status != live.SessionRunning && view.Session.Status != live.SessionBuffering) {
		return errors.New("直播未运行，无法提交互动")
	}
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
	q, err := pricing.Lookup("minimax", settings.Model, settings.Resolution, time.Now())
	if err != nil {
		return httpapi.GenerationConfig{}, err
	}
	price := q.Reserve(1)
	return httpapi.GenerationConfig{
		Pricing:  &q,
		Provider: "MiniMax", BaseURL: c.client.BaseURL(), Model: settings.Model,
		Resolution: settings.Resolution, DurationSeconds: settings.Duration,
		Ratio: settings.Ratio, APIKeyConfigured: c.client.APIKeyConfigured(),
		UnitPriceCNYPerSecond: &price,
	}, nil
}

func loadProviderSecrets(rootCtx context.Context, store *postgresstore.Store, cfg *config.Config) error {
	secrets, err := store.ProviderSecrets(rootCtx)
	if err != nil {
		return err
	}
	if cfg.QwenAPIKey == "" {
		cfg.QwenAPIKey = secrets.QwenAPIKey
	}
	if secrets.MiniMaxAPIKey != "" {
		cfg.MiniMaxAPIKey = secrets.MiniMaxAPIKey
	}
	if secrets.FalAPIKey != "" {
		cfg.FalAPIKey = secrets.FalAPIKey
	}

	return nil
}

func configureMedia(cfg config.Config) (*ffmpeg.Output, *ffmpeg.Preparer, error) {
	output, err := ffmpeg.NewOutput(ffmpeg.OutputConfig{FFmpegPath: cfg.FFmpegPath, RTMPURL: cfg.RTMPURL})
	preparer := ffmpeg.NewPreparer(ffmpeg.PreparerConfig{FFmpegPath: cfg.FFmpegPath, FFprobePath: cfg.FFprobePath, Duration: cfg.SegmentDuration})
	return output, preparer, err
}

func configureAudience(cfg config.Config) (*audience.Window, *bilibili.Manager, error) {
	window := audience.NewWindow(20 * time.Second)
	manager := bilibili.NewManager(window)
	if cfg.BilibiliRoomID > 0 {
		if err := manager.Update(cfg.BilibiliRoomID, cfg.BilibiliCookie); err != nil {
			manager.Close()
			return nil, nil, err
		}
	}
	return window, manager, nil
}

func configureMiniMax(cfg config.Config) (*minimax.Client, error) {
	return minimax.New(minimax.Config{
		APIKey: cfg.MiniMaxAPIKey, BaseURL: cfg.MiniMaxBaseURL, DataDir: cfg.DataDir,
		Settings: minimax.Settings{Model: cfg.MiniMaxModel, Resolution: cfg.MiniMaxResolution, Duration: int(cfg.SegmentDuration / time.Second), Ratio: cfg.MiniMaxRatio},
	})
}

func configureScheduler(cfg config.Config) generation.Scheduler {
	c := generation.DefaultSchedulerConfig(cfg.GenerationMaxConcurrency)
	c.InitialConcurrency = cfg.InitialConcurrency
	c.SegmentDuration = cfg.SegmentDuration
	return generation.NewScheduler(c)
}

func configureQwen(cfg config.Config) *qwen.Client {
	return qwen.New(qwen.Config{APIKey: cfg.QwenAPIKey, BaseURL: cfg.QwenBaseURL, ObserverModel: cfg.QwenObserverModel, DirectorModel: cfg.QwenDirectorModel})
}
