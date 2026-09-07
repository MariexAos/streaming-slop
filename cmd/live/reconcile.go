package main

import (
	"context"
	"log/slog"
	"streaming-agent/internal/generation"
	"streaming-agent/internal/store"
	"time"
)

func startReconciliation(ctx context.Context, s *store.Store, client generation.Generator, provider string) {
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := generation.ReconcileClosed(ctx, s, client, provider); err != nil {
					slog.Error("reconcile stopped tasks", "error", err)
				}
			}
		}
	}()
}
