package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"streaming-agent/internal/models"
)

type ProviderCredential struct {
	Provider   string `json:"provider"`
	Configured bool   `json:"configured"`
}
type ModelServices struct {
	Active      *models.Settings     `json:"active"`
	Saved       models.Settings      `json:"saved"`
	Options     []models.Option      `json:"options"`
	Credentials []ProviderCredential `json:"credentials"`
}
type CredentialUpdate struct {
	Provider string `json:"provider"`
	APIKey   string `json:"apiKey"`
}
type ServicesController interface {
	Services(context.Context) (ModelServices, error)
	SaveServices(context.Context, models.Settings) (ModelServices, error)
	SaveCredential(context.Context, CredentialUpdate) (ModelServices, error)
}

func ModelServicesHandler(c ServicesController, next http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/config/services", func(w http.ResponseWriter, r *http.Request) {
		value, err := c.Services(r.Context())
		if err != nil {
			writeError(w, 503, "services_unavailable", "模型服务配置暂不可用")
			return
		}
		writeJSON(w, 200, value)
	})
	mux.HandleFunc("PUT /api/v1/config/services", func(w http.ResponseWriter, r *http.Request) {
		var settings models.Settings
		if !decodeServiceBody(w, r, &settings) {
			return
		}
		value, err := c.SaveServices(r.Context(), settings)
		if err != nil {
			writeError(w, 400, "invalid_services", err.Error())
			return
		}
		writeJSON(w, 200, value)
	})
	mux.HandleFunc("PUT /api/v1/config/services/credentials", func(w http.ResponseWriter, r *http.Request) {
		var update CredentialUpdate
		if !decodeServiceBody(w, r, &update) {
			return
		}
		value, err := c.SaveCredential(r.Context(), update)
		if err != nil {
			writeError(w, 400, "invalid_credential", "凭据保存失败，请检查供应商和输入")
			return
		}
		writeJSON(w, 200, value)
	})
	mux.Handle("/", next)
	return mux
}
func decodeServiceBody(w http.ResponseWriter, r *http.Request, value any) bool {
	defer func() { _ = r.Body.Close() }()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		writeError(w, 400, "invalid_request", "配置格式不正确")
		return false
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		writeError(w, 400, "invalid_request", "请求只能包含一个配置对象")
		return false
	}
	return true
}
