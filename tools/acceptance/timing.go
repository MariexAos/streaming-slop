package main

import (
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

type httpMeasurement struct {
	Stage     string    `json:"stage"`
	StartedAt time.Time `json:"startedAt"`
	Seconds   float64   `json:"seconds"`
	Status    int       `json:"status"`
}

// Timings include reading the response body, without recording URLs or credentials.
type measuredTransport struct {
	mu           sync.Mutex
	measurements []httpMeasurement
}

func (m *measuredTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	stage := "download"
	if request.Method == http.MethodPost {
		stage = "submit"
	} else if strings.HasPrefix(request.URL.Path, "/v2/query/") {
		stage = "query"
	}
	started := time.Now()
	response, err := http.DefaultTransport.RoundTrip(request)
	if err != nil {
		return response, err
	}
	response.Body = &measuredBody{ReadCloser: response.Body, finish: func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.measurements = append(m.measurements, httpMeasurement{Stage: stage, StartedAt: started, Seconds: time.Since(started).Seconds(), Status: response.StatusCode})
	}}
	return response, nil
}

func (m *measuredTransport) save(out string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return saveJSON(out, "http-timing.json", m.measurements)
}

type measuredBody struct {
	io.ReadCloser
	finish func()
}

func (b *measuredBody) Close() error {
	err := b.ReadCloser.Close()
	b.finish()
	return err
}
