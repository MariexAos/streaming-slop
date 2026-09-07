package minimax

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"streaming-agent/internal/generation"

	"github.com/google/uuid"
)

type Client struct {
	APIKey  string
	BaseURL string
	HTTP    *http.Client
	Budget  generation.Budget
}

func New(key string, budget generation.Budget) *Client {
	return &Client{APIKey: key, BaseURL: "https://api.minimax.cn", HTTP: &http.Client{Timeout: 60 * time.Second}, Budget: budget}
}

type Part struct {
	Type   string `json:"type"`
	Text   string `json:"text,omitempty"`
	Source any    `json:"source,omitempty"`
}
type message struct {
	Role    string `json:"role"`
	Content []Part `json:"content"`
}
type request struct {
	Model     string            `json:"model"`
	System    string            `json:"system"`
	Messages  []message         `json:"messages"`
	MaxTokens int               `json:"max_tokens,omitempty"`
	Thinking  map[string]string `json:"thinking,omitempty"`
}

func (c *Client) complete(ctx context.Context, instruction string, parts []Part) ([]byte, error) {
	if c.APIKey == "" {
		return nil, errors.New("MiniMax API key required")
	}
	body := request{Model: "MiniMax-M3", System: instruction, Messages: []message{{Role: "user", Content: parts}}}
	var count struct {
		InputTokens int64 `json:"input_tokens"`
	}
	if err := c.post(ctx, "/anthropic/v1/messages/count_tokens", body, &count); err != nil {
		return nil, err
	}
	if count.InputTokens <= 0 || count.InputTokens > 100_000 {
		return nil, errors.New("invalid or excessive input token count")
	}
	id := "m3:" + uuid.NewString()
	// Standard M3: CNY 2.10/M input + 8.40/M output. Reserve output ceiling.
	quote := generation.Micros(float64(count.InputTokens)*2.1/1e6 + 1024*8.4/1e6)
	if err := c.Budget.Reserve(ctx, id, quote); err != nil {
		return nil, err
	}
	body.MaxTokens = 1024
	body.Thinking = map[string]string{"type": "disabled"}
	var response struct {
		Content    []Part `json:"content"`
		StopReason string `json:"stop_reason"`
		Usage      struct {
			Input  int64 `json:"input_tokens"`
			Output int64 `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := c.post(ctx, "/anthropic/v1/messages", body, &response); err != nil {
		return nil, err
	}
	cost := generation.Micros(float64(response.Usage.Input)*2.1/1e6 + float64(response.Usage.Output)*8.4/1e6)
	if err := c.Budget.Settle(ctx, id, cost); err != nil {
		return nil, err
	}
	if response.StopReason == "max_tokens" {
		return nil, errors.New("M3 output truncated")
	}
	for _, part := range response.Content {
		if part.Type == "text" {
			return []byte(strings.TrimSpace(part.Text)), nil
		}
	}
	return nil, errors.New("M3 returned no text")
}
func (c *Client) post(ctx context.Context, path string, body, out any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("MiniMax %s: %w", path, err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != 200 {
		return fmt.Errorf("MiniMax %s HTTP %d", path, res.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(res.Body, 2<<20)).Decode(out)
}
