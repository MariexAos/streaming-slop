package minimax

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
	"sync"
	"time"

	"streaming-agent/internal/generation"
)

const (
	DefaultBaseURL = "https://api.minimaxi.com"
	DefaultModel   = "MiniMax-H3-Max"
	maxBodyBytes   = 1 << 20
)

type Settings struct {
	Model      string
	Resolution string
	Duration   int
	Ratio      string
}

func DefaultSettings() Settings {
	return Settings{Model: DefaultModel, Resolution: "768P", Duration: 5, Ratio: "16:9"}
}

func (s Settings) Validate() error {
	if s.Model != DefaultModel {
		return fmt.Errorf("online generation model must be %s", DefaultModel)
	}
	if s.Duration < 5 || s.Duration > 15 {
		return fmt.Errorf("duration must be between 5 and 15 seconds")
	}
	if s.Resolution != "768P" {
		return fmt.Errorf("online generation resolution must be 768P")
	}
	if s.Ratio != "16:9" {
		return fmt.Errorf("online text-to-video ratio must be 16:9")
	}
	return nil
}

func UnitPriceCNY(model, resolution string) (float64, error) {
	if model != DefaultModel || resolution != "768P" {
		return 0, fmt.Errorf("no price for %s at %s", model, resolution)
	}
	return 0.50, nil
}

type Config struct {
	APIKey         string
	BaseURL        string
	DataDir        string
	Settings       Settings
	HTTPClient     *http.Client
	RequestTimeout time.Duration
}

type Client struct {
	apiKey         string
	baseURL        string
	dataDir        string
	httpClient     *http.Client
	requestTimeout time.Duration

	mu       sync.RWMutex
	settings Settings
}

func New(config Config) (*Client, error) {
	if config.BaseURL == "" {
		config.BaseURL = DefaultBaseURL
	}
	if config.DataDir == "" {
		config.DataDir = "."
	}
	if config.Settings.Model == "" {
		config.Settings = DefaultSettings()
	}
	if err := config.Settings.Validate(); err != nil {
		return nil, err
	}
	if config.RequestTimeout <= 0 {
		config.RequestTimeout = 30 * time.Second
	}
	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{Timeout: config.RequestTimeout}
	}
	return &Client{
		apiKey: strings.TrimSpace(config.APIKey), baseURL: strings.TrimRight(config.BaseURL, "/"),
		dataDir: config.DataDir, httpClient: config.HTTPClient, requestTimeout: config.RequestTimeout,
		settings: config.Settings,
	}, nil
}

func (c *Client) Settings() Settings {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.settings
}

func (c *Client) UpdateSettings(settings Settings) error {
	if err := settings.Validate(); err != nil {
		return err
	}
	c.mu.Lock()
	c.settings = settings
	c.mu.Unlock()
	return nil
}

func (c *Client) UpdateAPIKey(apiKey string) error {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return fmt.Errorf("MiniMax API key is required")
	}
	c.mu.Lock()
	c.apiKey = apiKey
	c.mu.Unlock()
	return nil
}

func (c *Client) APIKeyConfigured() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.apiKey != ""
}

func (c *Client) currentAPIKey() (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.apiKey == "" {
		return "", fmt.Errorf("MiniMax API key is not configured")
	}
	return c.apiKey, nil
}

func (c *Client) BaseURL() string { return c.baseURL }

func (c *Client) BuildRequest(request generation.Request) (generation.Request, error) {
	settings := c.Settings()
	if request.Model != "" {
		settings.Model = request.Model
	}
	if _, err := c.currentAPIKey(); err != nil {
		return request, err
	}
	if settings.Model != DefaultModel {
		return request, fmt.Errorf("unsupported MiniMax model %q", settings.Model)
	}
	if len(request.References) > 0 {
		return generation.Request{}, fmt.Errorf("submit MiniMax request: multimodal references are not supported online")
	}
	spec := request.Spec
	if spec.Mode == "" {
		prompt, err := compilePrompt(request)
		if err != nil {
			return generation.Request{}, err
		}
		spec = generation.GenerationSpec{
			Mode: generation.ModeTextToVideo, Prompt: prompt,
			Resolution: settings.Resolution, Duration: time.Duration(settings.Duration) * time.Second,
			Ratio: settings.Ratio,
		}
	} else if strings.TrimSpace(spec.Prompt) == "" {
		prompt, err := compilePrompt(request)
		if err != nil {
			return generation.Request{}, err
		}
		spec.Prompt = prompt
	}
	if err := spec.Validate(); err != nil {
		return generation.Request{}, fmt.Errorf("submit MiniMax request: %w", err)
	}
	request.Spec = spec
	request.Model = settings.Model
	price, err := UnitPriceCNY(request.Model, spec.Resolution)
	if err != nil {
		return request, err
	}
	request.UnitPriceCNY = &price
	return request, nil
}

func (c *Client) Submit(ctx context.Context, input generation.Request) (generation.Job, error) {
	request, err := c.BuildRequest(input)
	if err != nil {
		return generation.Job{}, err
	}
	spec := request.Spec
	content := contentFor(spec)
	var response createResponse
	err = c.doJSON(ctx, http.MethodPost, c.baseURL+"/v2/video_generation", createRequest{
		Model: request.Model, Content: content, Resolution: spec.Resolution,
		Duration: int(spec.Duration / time.Second), Ratio: spec.Ratio,
	}, &response, http.StatusOK)
	if err != nil {
		return generation.Job{}, fmt.Errorf("submit MiniMax request: %w", err)
	}
	if response.TaskID == "" {
		return generation.Job{}, fmt.Errorf("submit MiniMax request: response has no task_id")
	}
	return generation.Job{ID: response.TaskID, Status: generation.JobQueued}, nil
}

func contentFor(spec generation.GenerationSpec) []contentItem {
	content := []contentItem{{Type: "text", Text: spec.Prompt}}
	if spec.FirstFrame != nil {
		content = append(content, contentItem{
			Type: "image_url", ImageURL: &mediaURL{URL: spec.FirstFrame.URL}, Role: "first_frame",
		})
	}
	if spec.LastFrame != nil {
		content = append(content, contentItem{
			Type: "image_url", ImageURL: &mediaURL{URL: spec.LastFrame.URL}, Role: "last_frame",
		})
	}
	return content
}

func (c *Client) Status(ctx context.Context, taskID string) (generation.Job, error) {
	response, err := c.query(ctx, taskID)
	if err != nil {
		return generation.Job{}, err
	}
	job := generation.Job{ID: taskID}
	switch response.Task.Status {
	case "queued":
		job.Status = generation.JobQueued
	case "running":
		job.Status = generation.JobRunning
	case "succeeded":
		job.Status = generation.JobCompleted
	case "failed", "expired":
		job.Status = generation.JobFailed
		job.ErrorCode = response.Task.Status
		job.ErrorMessage = response.Task.Error.Message
	case "cancelled":
		job.Status = generation.JobCancelled
	default:
		return generation.Job{}, fmt.Errorf("query MiniMax task: unknown state %q", response.Task.Status)
	}
	return job, nil
}

func (c *Client) Result(ctx context.Context, taskID string) (generation.Result, error) {
	response, err := c.query(ctx, taskID)
	if err != nil {
		return generation.Result{}, err
	}
	if response.Task.Status != "succeeded" || response.Task.Content.URL == "" {
		return generation.Result{}, fmt.Errorf("get MiniMax result: task is not complete")
	}
	path, err := c.download(ctx, taskID, response.Task.Content.URL)
	if err != nil {
		return generation.Result{}, err
	}
	seconds := response.Task.Usage.OutputSeconds
	if seconds == 0 {
		seconds = response.Task.Duration
	}
	price, err := UnitPriceCNY(response.Task.Model, response.Task.Resolution)
	if err != nil {
		return generation.Result{}, err
	}
	cost := float64(seconds) * price
	return generation.Result{JobID: taskID, AssetURL: path, CostCNY: &cost}, nil
}

func (c *Client) Cancel(ctx context.Context, taskID string) error {
	apiKey, err := c.currentAPIKey()
	if err != nil {
		return err
	}
	endpoint := c.baseURL + "/v2/video_generation/" + url.PathEscape(taskID)
	requestCtx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return fmt.Errorf("cancel MiniMax task: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("cancel MiniMax task: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("cancel MiniMax task: returned %s", resp.Status)
	}
	return nil
}

func (c *Client) query(ctx context.Context, taskID string) (queryResponse, error) {
	var response queryResponse
	endpoint := c.baseURL + "/v2/query/video_generation/" + url.PathEscape(taskID)
	if err := c.doJSON(ctx, http.MethodGet, endpoint, nil, &response, http.StatusOK); err != nil {
		return queryResponse{}, fmt.Errorf("query MiniMax task: %w", err)
	}
	return response, nil
}

func (c *Client) download(ctx context.Context, taskID, assetURL string) (string, error) {
	requestCtx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, assetURL, nil)
	if err != nil {
		return "", fmt.Errorf("create MiniMax download: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("download MiniMax asset: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download MiniMax asset: returned %s", resp.Status)
	}
	if err := os.MkdirAll(c.dataDir, 0o755); err != nil {
		return "", fmt.Errorf("create MiniMax data directory: %w", err)
	}
	path := filepath.Join(c.dataDir, safeName(taskID)+".mp4")
	temporary, err := os.CreateTemp(c.dataDir, ".minimax-download-*")
	if err != nil {
		return "", fmt.Errorf("create MiniMax asset file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if _, err := io.Copy(temporary, resp.Body); err != nil {
		_ = temporary.Close()
		return "", fmt.Errorf("save MiniMax asset: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return "", fmt.Errorf("close MiniMax asset: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return "", fmt.Errorf("commit MiniMax asset: %w", err)
	}
	return path, nil
}

func (c *Client) doJSON(ctx context.Context, method, endpoint string, input, output any, successCodes ...int) error {
	apiKey, err := c.currentAPIKey()
	if err != nil {
		return err
	}
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
	req.Header.Set("Authorization", "Bearer "+apiKey)
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	for _, code := range successCodes {
		if resp.StatusCode == code {
			return json.NewDecoder(io.LimitReader(resp.Body, maxBodyBytes)).Decode(output)
		}
	}
	message, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
	return fmt.Errorf("returned %s: %s", resp.Status, strings.TrimSpace(string(message)))
}

func compilePrompt(request generation.Request) (string, error) {
	if err := request.Direction.Validate(); err != nil {
		return "", fmt.Errorf("compile MiniMax prompt: %w", err)
	}
	parts := []string{
		"Persistent character: " + request.World.Character.Name,
		"Appearance: " + request.World.Character.Attributes["appearance"],
		"Livestream baseline: " + request.World.Scene.Location,
		"Baseline lighting: " + request.World.Scene.Lighting,
		"Visual style: photorealistic consumer-camera livestream, fixed slightly low viewpoint, imperfect framing, mild softness and uneven exposure, stable face and wardrobe, no text, no logos, no promotional polish.",
		"Interaction rule: do not invent a permanent theme or restrict the host to sitting and talking; carry out the requested observable action while preserving continuity.",
		"Action: " + request.Direction.Action,
		"Emotion: " + request.Direction.Emotion,
		"Camera: " + strings.Join([]string{request.Direction.Camera.Shot, request.Direction.Camera.Movement, request.Direction.Camera.Angle}, ", "),
		"Continuity: " + request.Direction.Continuity.Notes,
	}
	if request.Direction.Dialogue != "" {
		parts = append(parts, "Dialogue: "+request.Direction.Dialogue)
	}
	return strings.Join(parts, "\n"), nil
}

type createRequest struct {
	Model      string        `json:"model"`
	Content    []contentItem `json:"content"`
	Resolution string        `json:"resolution"`
	Duration   int           `json:"duration"`
	Ratio      string        `json:"ratio"`
}

type contentItem struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *mediaURL `json:"image_url,omitempty"`
	Role     string    `json:"role,omitempty"`
}

type mediaURL struct {
	URL string `json:"url"`
}
type createResponse struct {
	TaskID string `json:"task_id"`
}

type queryResponse struct {
	Task struct {
		ID         string `json:"id"`
		Model      string `json:"model"`
		Status     string `json:"status"`
		Resolution string `json:"resolution"`
		Duration   int    `json:"duration"`
		Content    struct {
			URL string `json:"url"`
		} `json:"content"`
		Usage struct {
			OutputSeconds int `json:"output_seconds"`
		} `json:"usage"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	} `json:"task"`
}

func safeName(value string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, value)
}

var _ generation.Generator = (*Client)(nil)
