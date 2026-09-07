package server

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"streaming-agent/internal/character"
	"streaming-agent/internal/live"
	"strings"
)

func CharacterHandler(c character.Catalog) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/characters", func(w http.ResponseWriter, r *http.Request) {
		values, err := c.Store.Characters(r.Context())
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, 200, values)
	})
	mux.HandleFunc("GET /api/v1/characters/current", func(w http.ResponseWriter, r *http.Request) {
		value, err := c.Store.Character(r.Context(), "")
		if err != nil {
			http.Error(w, err.Error(), 404)
			return
		}
		writeJSON(w, 200, value)
	})
	mux.HandleFunc("PUT /api/v1/characters/current", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			VersionID string `json:"versionId"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input); err != nil {
			http.Error(w, "invalid input", 400)
			return
		}
		if _, _, err := c.Resolve(r.Context(), input.VersionID); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if err := c.Store.SelectCharacter(r.Context(), input.VersionID); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		w.WriteHeader(204)
	})
	mux.HandleFunc("POST /api/v1/characters", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 30<<20)
		var input struct {
			Profile live.CharacterProfile `json:"profile"`
			First   []byte                `json:"first"`
			Last    []byte                `json:"last"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, "invalid character or images", 400)
			return
		}
		p, err := c.Publish(r.Context(), input.Profile, input.First, input.Last)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		writeJSON(w, 201, p)
	})
	mux.HandleFunc("GET /api/v1/characters/{id}/{frame}", func(w http.ResponseWriter, r *http.Request) {
		_, anchors, err := c.Resolve(r.Context(), r.PathValue("id"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		value := anchors["chat-live-"+r.PathValue("frame")]
		parts := strings.SplitN(value, ",", 2)
		if len(parts) != 2 {
			http.NotFound(w, r)
			return
		}
		data, err := base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			http.Error(w, "invalid media", 500)
			return
		}
		w.Header().Set("Content-Type", strings.TrimSuffix(strings.TrimPrefix(parts[0], "data:"), ";base64"))
		_, _ = w.Write(data)
	})
	return mux
}
