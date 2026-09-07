package server

import (
	"context"
	"encoding/json"
	"net/http"
	"streaming-agent/internal/live"
	"streaming-agent/internal/session"
)

type reconciliationController interface {
	Unresolved() []session.UnresolvedAttempt
	AttachJob(context.Context, live.AttemptID, string) error
}

func registerReconciliation(mux *http.ServeMux, controller Controller) {
	c, ok := controller.(reconciliationController)
	if !ok {
		return
	}
	mux.HandleFunc("GET /api/v1/attempts/unresolved", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, http.StatusOK, c.Unresolved()) })
	mux.HandleFunc("PUT /api/v1/attempts/{id}/job", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			JobID string `json:"jobId"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input); err != nil {
			http.Error(w, "invalid input", 400)
			return
		}
		if err := c.AttachJob(r.Context(), live.AttemptID(r.PathValue("id")), input.JobID); err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
