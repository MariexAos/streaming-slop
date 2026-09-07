package webui

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
)

func TestEmbeddedFrontend(t *testing.T) {
	handler := NewHandler()
	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/", nil))
	if page.Code != http.StatusOK {
		t.Fatalf("index: status %d", page.Code)
	}
	asset := regexp.MustCompile(`src="(/assets/[^" ]+\.js)"`).FindStringSubmatch(page.Body.String())
	if len(asset) != 2 {
		t.Fatal("index does not reference a built JavaScript asset")
	}
	script := httptest.NewRecorder()
	handler.ServeHTTP(script, httptest.NewRequest(http.MethodGet, asset[1], nil))
	if script.Code != http.StatusOK || script.Body.Len() == 0 {
		t.Fatalf("embedded script: status %d, size %d", script.Code, script.Body.Len())
	}
	api := httptest.NewRecorder()
	handler.ServeHTTP(api, httptest.NewRequest(http.MethodGet, "/api/v1/ops/snapshot", nil))
	if api.Code != http.StatusNotFound {
		t.Fatalf("frontend swallowed API route: status %d", api.Code)
	}
}
