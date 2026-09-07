package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"streaming-agent/internal/generation"
	"streaming-agent/internal/live"
	"streaming-agent/internal/session"
	"streaming-agent/internal/store"
	"time"
)

func unresolvedHandler(s *store.Store, runtime *session.Runtime, client generation.Generator, provider string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/attempts/unresolved", func(w http.ResponseWriter, r *http.Request) {
		pending, err := s.UnresolvedAttempts(r.Context())
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		result := []session.UnresolvedAttempt{}
		for _, p := range pending {
			a := p.Attempt
			result = append(result, session.UnresolvedAttempt{ID: a.ID, Provider: a.Provider, JobID: a.ProviderJobID, Status: a.Status})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	})
	mux.HandleFunc("PUT /api/v1/attempts/{id}/job", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			JobID string `json:"jobId"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input); err != nil {
			http.Error(w, "invalid input", 400)
			return
		}
		err := attachUnresolved(r.Context(), s, runtime, client, provider, live.AttemptID(r.PathValue("id")), input.JobID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	return mux
}
func attachUnresolved(ctx context.Context, s *store.Store, runtime *session.Runtime, c generation.Generator, provider string, id live.AttemptID, jobID string) error {
	if jobID == "" {
		return errors.New("task id required")
	}
	for _, a := range runtime.Unresolved() {
		if a.ID == id {
			return runtime.AttachJob(ctx, id, jobID)
		}
	}
	pending, err := s.UnresolvedAttempts(ctx)
	if err != nil {
		return err
	}
	for _, p := range pending {
		a := p.Attempt
		if a.ID != id {
			continue
		}
		if a.Provider != provider || a.Status != generation.AttemptPendingSubmit || a.ProviderJobID != "" {
			return errors.New("attempt not eligible for binding")
		}
		if _, err = c.Status(ctx, jobID); err != nil {
			return err
		}
		a.ProviderJobID = jobID
		if err = a.Transition(generation.AttemptSubmitted, time.Now().UTC()); err != nil {
			return err
		}
		return s.UpdateAttempt(ctx, p.SessionID, &a)
	}
	return errors.New("attempt not found")
}
