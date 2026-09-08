package main

import (
	"context"
	"log/slog"
	"streaming-agent/internal/generation"
	"streaming-agent/internal/store"
	"time"
)

func startReconciliation(ctx context.Context, s *store.Store, resolve func(context.Context, generation.Attempt) (generation.Generator, error)) {
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := generation.ReconcileClosedWith(ctx, s, resolve); err != nil {
					slog.Error("reconcile stopped tasks", "error", err)
				}
			}
		}
	}()
}
