package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"

	"streaming-agent/internal/character"
	"streaming-agent/internal/config"
	minimaxInference "streaming-agent/internal/inference/minimax"
	httpapi "streaming-agent/internal/server"
	"streaming-agent/internal/session"
	"streaming-agent/internal/store"
)

type readinessReader struct {
	services  *modelServicesController
	catalog   character.Catalog
	store     *store.Store
	generator httpapi.ConfigController
	cfg       config.Config
	inference *minimaxInference.Client
}

func (c readinessReader) read(ctx context.Context) (session.StartReadiness, error) {
	g, credentialReady, err := c.startConfiguration(ctx)
	if err != nil {
		return session.StartReadiness{}, err
	}
	limit, err := c.store.NextBudget(ctx)
	if err != nil {
		return session.StartReadiness{}, err
	}
	p, _, characterErr := c.catalog.Resolve(ctx, "")
	if characterErr != nil {
		p.ID = ""
	}
	seconds := math.Ceil(c.cfg.ReadyTarget.Seconds()/c.cfg.SegmentDuration.Seconds()) * c.cfg.SegmentDuration.Seconds()
	r := session.EvaluateStart(p.ID, p.Name, credentialReady, limit, g.UnitPriceCNYPerSecond, seconds)
	r.Target = "外部推流目标"
	target, err := url.Parse(c.cfg.RTMPURL)
	if err == nil && (target.Hostname() == "127.0.0.1" || target.Hostname() == "localhost") {
		r.Target = "本地测试"
	}
	return r, nil
}
func (c readinessReader) check(ctx context.Context) error {
	r, err := c.read(ctx)
	if err != nil {
		return fmt.Errorf("检查开播条件: %w", err)
	}
	if !r.Ready {
		return fmt.Errorf("%w: %s", session.ErrInvalidState, strings.Join(r.Blockers, "；"))
	}
	return nil
}
func (c readinessReader) handler(next http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/ops/readiness", func(w http.ResponseWriter, r *http.Request) {
		value, err := c.read(r.Context())
		if err != nil {
			http.Error(w, "无法检查开播条件", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(value)
	})
	target, err := url.Parse(c.cfg.RTMPURL)
	if err == nil && (target.Hostname() == "localhost" || target.Hostname() == "127.0.0.1") {
		upstream := &url.URL{Scheme: "http", Host: "127.0.0.1:8888", Path: strings.TrimSuffix(target.Path, "/")}
		mux.Handle("/live-preview/", http.StripPrefix("/live-preview", localPreviewProxy(upstream)))
	}
	mux.Handle("/", next)
	return mux
}

type liveCredentials struct {
	httpapi.ConfigController
	inference *minimaxInference.Client
}

func (c liveCredentials) UpdateGenerationConfig(ctx context.Context, update httpapi.GenerationConfigUpdate) (httpapi.GenerationConfig, error) {
	result, err := c.ConfigController.UpdateGenerationConfig(ctx, update)
	if err == nil && update.APIKey != "" {
		c.inference.UpdateAPIKey(update.APIKey)
	}
	return result, err
}

func configureInference(key string, s *store.Store, controller httpapi.ConfigController) (*minimaxInference.Client, httpapi.ConfigController) {
	client := minimaxInference.New(key, s)
	return client, liveCredentials{ConfigController: controller, inference: client}
}
