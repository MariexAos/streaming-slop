package fal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"streaming-agent/internal/generation"
)

const (
	DefaultModel   = "minimax/h3-max/reference-to-video"
	defaultBaseURL = "https://queue.fal.run"
	maxBodyBytes   = 1 << 20
)

type Config struct {
	APIKey         string
	Model          string
	BaseURL        string
	DataDir        string
	HTTPClient     *http.Client
	RequestTimeout time.Duration
}

type Client struct {
	apiKey         string
	model          string
	baseURL        string
	dataDir        string
	httpClient     *http.Client
	requestTimeout time.Duration
}

type Spec struct {
	Prompt             string
	ReferenceImageURLs []string
	ReferenceVideoURLs []string
	ReferenceAudioURLs []string
}

func New(config Config) (*Client, error) {
	if strings.TrimSpace(config.APIKey) == "" {
		return nil, fmt.Errorf("fal API key is required")
	}
	if config.Model == "" {
		config.Model = DefaultModel
	}
	if config.BaseURL == "" {
		config.BaseURL = defaultBaseURL
	}
	if config.DataDir == "" {
		config.DataDir = "."
	}
	if config.RequestTimeout <= 0 {
		config.RequestTimeout = 30 * time.Second
	}
	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{Timeout: config.RequestTimeout}
	}
	return &Client{
		apiKey:         config.APIKey,
		model:          strings.Trim(config.Model, "/"),
		baseURL:        strings.TrimRight(config.BaseURL, "/"),
		dataDir:        config.DataDir,
		httpClient:     config.HTTPClient,
		requestTimeout: config.RequestTimeout,
	}, nil
}

func (c *Client) Submit(ctx context.Context, generationRequest generation.Request) (generation.Job, error) {
	request, err := c.compile(generationRequest)
	if err != nil {
		return generation.Job{}, err
	}
	var response submitResponse
	if err := c.doJSON(ctx, http.MethodPost, c.modelURL(), request, &response, http.StatusOK, http.StatusCreated); err != nil {
		return generation.Job{}, fmt.Errorf("submit fal request: %w", err)
	}
	if response.RequestID == "" {
		return generation.Job{}, fmt.Errorf("submit fal request: response has no request_id")
	}
	return generation.Job{ID: response.RequestID, Status: generation.JobQueued}, nil
}

func (c *Client) Status(ctx context.Context, requestID string) (generation.Job, error) {
	var response statusResponse
	endpoint := c.requestURL(requestID) + "/status"
	if err := c.doJSON(ctx, http.MethodGet, endpoint, nil, &response, http.StatusOK); err != nil {
		return generation.Job{}, fmt.Errorf("get fal status: %w", err)
	}
	job := generation.Job{
		ID:           response.RequestID,
		ErrorCode:    response.ErrorType,
		ErrorMessage: response.Error,
	}
	switch response.Status {
	case "IN_QUEUE":
		job.Status = generation.JobQueued
	case "IN_PROGRESS":
		job.Status = generation.JobRunning
	case "COMPLETED":
		if response.Error != "" || response.ErrorType != "" {
			job.Status = generation.JobFailed
		} else {
			job.Status = generation.JobCompleted
		}
	default:
		return generation.Job{}, fmt.Errorf("get fal status: unknown state %q", response.Status)
	}
	return job, nil
}

// Result retrieves the completed response and immediately persists its media.
func (c *Client) Result(ctx context.Context, requestID string) (generation.Result, error) {
	var response resultResponse
	if err := c.doJSON(ctx, http.MethodGet, c.requestURL(requestID), nil, &response, http.StatusOK); err != nil {
		return generation.Result{}, fmt.Errorf("get fal result: %w", err)
	}
	if response.Video.URL == "" {
		return generation.Result{}, fmt.Errorf("get fal result: response has no video URL")
	}
	path, _, err := c.download(ctx, requestID, response.Video)
	if err != nil {
		return generation.Result{}, err
	}
	return generation.Result{JobID: requestID, AssetURL: path}, nil
}

func (c *Client) Cancel(ctx context.Context, requestID string) error {
	var response cancelResponse
	err := c.doJSON(ctx, http.MethodPut, c.requestURL(requestID)+"/cancel", nil, &response, http.StatusAccepted)
	if err != nil {
		return fmt.Errorf("cancel fal request: %w", err)
	}
	if response.Status != "CANCELLATION_REQUESTED" {
		return fmt.Errorf("cancel fal request: unexpected status %q", response.Status)
	}
	return nil
}

func (c *Client) download(ctx context.Context, requestID string, file fileResponse) (string, int64, error) {
	requestCtx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, file.URL, nil)
	if err != nil {
		return "", 0, fmt.Errorf("create fal asset download: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("download fal asset: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("download fal asset: returned %s", resp.Status)
	}

	if err := os.MkdirAll(c.dataDir, 0o755); err != nil {
		return "", 0, fmt.Errorf("create fal data directory: %w", err)
	}
	extension := filepath.Ext(file.FileName)
	if extension == "" {
		if parsed, parseErr := url.Parse(file.URL); parseErr == nil {
			extension = filepath.Ext(parsed.Path)
		}
	}
	if extension == "" {
		extension = ".mp4"
	}
	path := filepath.Join(c.dataDir, safeName(requestID)+extension)
	temporary, err := os.CreateTemp(c.dataDir, ".fal-download-*")
	if err != nil {
		return "", 0, fmt.Errorf("create fal asset file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()

	size, copyErr := io.Copy(temporary, resp.Body)
	closeErr := temporary.Close()
	if copyErr != nil {
		return "", 0, fmt.Errorf("save fal asset: %w", copyErr)
	}
	if closeErr != nil {
		return "", 0, fmt.Errorf("close fal asset: %w", closeErr)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return "", 0, fmt.Errorf("commit fal asset: %w", err)
	}
	return path, size, nil
}

func (c *Client) doJSON(ctx context.Context, method, endpoint string, input, output any, successCodes ...int) error {
	var body io.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
		body = bytes.NewReader(encoded)
	}
	requestCtx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, method, endpoint, body)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Key "+c.apiKey)
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	accepted := false
	for _, code := range successCodes {
		accepted = accepted || resp.StatusCode == code
	}
	if !accepted {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
		return fmt.Errorf("returned %s: %s", resp.Status, strings.TrimSpace(string(message)))
	}
	decoder := json.NewDecoder(io.LimitReader(resp.Body, maxBodyBytes))
	if err := decoder.Decode(output); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func (c *Client) modelURL() string {
	return c.baseURL + "/" + c.model
}

func (c *Client) requestURL(requestID string) string {
	parts := strings.Split(c.model, "/")
	return c.baseURL + "/" + strings.Join(parts[:2], "/") + "/requests/" + url.PathEscape(requestID)
}

func safeName(value string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, value)
}

type submitRequest struct {
	ImageURL            string   `json:"image_url,omitempty"`
	EndImageURL         string   `json:"end_image_url,omitempty"`
	Prompt              string   `json:"prompt"`
	Duration            int      `json:"duration"`
	Resolution          string   `json:"resolution"`
	AspectRatio         string   `json:"aspect_ratio,omitempty"`
	PromptExpansionMode string   `json:"prompt_expansion_mode"`
	EnableSafetyChecker bool     `json:"enable_safety_checker"`
	ReferenceImageURLs  []string `json:"reference_image_urls,omitempty"`
	ReferenceVideoURLs  []string `json:"reference_video_urls,omitempty"`
	ReferenceAudioURLs  []string `json:"reference_audio_urls,omitempty"`
}

type submitResponse struct {
	RequestID     string `json:"request_id"`
	ResponseURL   string `json:"response_url"`
	StatusURL     string `json:"status_url"`
	CancelURL     string `json:"cancel_url"`
	QueuePosition int    `json:"queue_position"`
}

type statusResponse struct {
	Status        string `json:"status"`
	RequestID     string `json:"request_id"`
	QueuePosition int    `json:"queue_position"`
	Error         string `json:"error"`
	ErrorType     string `json:"error_type"`
	Metrics       struct {
		InferenceTime float64 `json:"inference_time"`
	} `json:"metrics"`
}

type resultResponse struct {
	Video          fileResponse `json:"video"`
	ExpandedPrompt string       `json:"expanded_prompt"`
	Seed           int64        `json:"seed"`
}

type fileResponse struct {
	URL         string `json:"url"`
	ContentType string `json:"content_type"`
	FileName    string `json:"file_name"`
	FileSize    int64  `json:"file_size"`
}

type cancelResponse struct {
	Status string `json:"status"`
}

var _ generation.Generator = (*Client)(nil)
