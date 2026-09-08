package main

import (
	"encoding/json"
	"io"
	"net/http"

	"streaming-agent/internal/store"
)

func budgetHandler(s *store.Store) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/budget", func(w http.ResponseWriter, r *http.Request) {
		state, err := s.CurrentSpending(r.Context())
		if err != nil {
			http.Error(w, "无法读取本场预算", 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(state)
	})
	mux.HandleFunc("PUT /api/v1/budget", func(w http.ResponseWriter, r *http.Request) {
		defer func() { _ = r.Body.Close() }()
		var input struct {
			LimitMicros int64 `json:"limitMicros"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			http.Error(w, "预算格式不正确", 400)
			return
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			http.Error(w, "预算格式不正确", 400)
			return
		}
		if err := s.SaveNextBudget(r.Context(), input.LimitMicros); err != nil {
			http.Error(w, "预算保存失败，请输入大于零的金额", 400)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	return mux
}
