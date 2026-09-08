package minimax

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"streaming-agent/internal/generation"
	"streaming-agent/internal/pricing"

	"github.com/google/uuid"
)

func (c *Client) Synthesize(ctx context.Context, text, voice string, maxDuration time.Duration) ([]byte, error) {
	if c.key() == "" || strings.TrimSpace(voice) == "" || strings.TrimSpace(text) == "" {
		return nil, errors.New("speech requires key, voice and text")
	}
	if len(text) > 2000 {
		return nil, errors.New("livestream dialogue too long")
	}
	rate, err := pricing.Lookup("minimax", "speech-2.8-turbo", "standard", time.Now())
	if err != nil {
		return nil, err
	}
	id := "speech:" + uuid.NewString()
	if err := c.Budget.Reserve(ctx, id, generation.Micros(rate.Reserve(float64(len(text)*2)))); err != nil {
		return nil, err
	}
	body := map[string]any{"model": "speech-2.8-turbo", "text": text, "stream": false, "output_format": "hex",
		"voice_setting": map[string]any{"voice_id": voice, "speed": 1, "vol": 1, "pitch": 0},
		"audio_setting": map[string]any{"sample_rate": 32000, "bitrate": 128000, "format": "mp3", "channel": 1}}
	var response struct {
		Data struct {
			Audio string `json:"audio"`
		} `json:"data"`
		Extra struct {
			Length     int64 `json:"audio_length"`
			Characters int64 `json:"usage_characters"`
		} `json:"extra_info"`
		Base struct {
			Code int `json:"status_code"`
		} `json:"base_resp"`
	}
	if err := c.post(ctx, "/v1/t2a_v2", body, &response); err != nil {
		return nil, err
	}
	if response.Base.Code != 0 {
		if response.Base.Code == 2013 {
			if err := c.Budget.Settle(ctx, id, 0); err != nil {
				return nil, err
			}
		}
		return nil, fmt.Errorf("speech error %d", response.Base.Code)
	}
	if response.Extra.Characters <= 0 {
		return nil, errors.New("speech usage missing")
	}
	if err := c.Budget.Settle(ctx, id, generation.Micros(rate.Estimate(float64(response.Extra.Characters)))); err != nil {
		return nil, err
	}
	if time.Duration(response.Extra.Length)*time.Millisecond > maxDuration {
		return nil, errors.New("speech exceeds clip duration; shorten dialogue")
	}
	data, err := hex.DecodeString(response.Data.Audio)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, errors.New("empty speech")
	}
	return data, nil
}
