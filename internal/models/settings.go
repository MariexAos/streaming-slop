// Package models defines the supported service selections independently of providers.
package models

import (
	"fmt"
	"time"

	"streaming-agent/internal/live"
	"streaming-agent/internal/pricing"
)

type Selection = live.ModelSelection
type Settings = live.ModelSettings

func Validate(s Settings) error {
	if err := ValidateVideo(s.Video); err != nil {
		return err
	}
	if s.Text.Provider != "minimax" || s.Text.Model != "MiniMax-M3" || s.Text.Resolution != "" {
		return fmt.Errorf("text inference currently supports MiniMax-M3 on minimax")
	}
	return nil
}
func ValidateVideo(s Selection) error {
	if s.Resolution != "480P" && s.Resolution != "768P" {
		return fmt.Errorf("video resolution must be 480P or 768P")
	}
	if (s.Provider != "minimax" || s.Model != "MiniMax-H3-Max") && (s.Provider != "fal" || s.Model != "minimax/h3-max-turbo/image-to-video") {
		return fmt.Errorf("unsupported video provider/model")
	}
	return nil
}

type Option struct {
	Provider    string          `json:"provider"`
	Model       string          `json:"model"`
	Kind        string          `json:"kind"`
	Resolutions []string        `json:"resolutions"`
	Prices      []pricing.Quote `json:"prices"`
}

func Options(at time.Time) ([]Option, error) {
	definitions := []struct {
		provider, model, kind string
		variants, resolutions []string
	}{
		{"minimax", "MiniMax-H3-Max", "video", []string{"480P", "768P"}, []string{"480P", "768P"}},
		{"fal", "minimax/h3-max-turbo/image-to-video", "video", []string{"480P", "768P"}, []string{"480P", "768P"}},
		{"minimax", "MiniMax-M3", "text", []string{"standard-input-up-to-512k", "standard-output-up-to-512k"}, []string{}},
	}
	result := make([]Option, 0, len(definitions))
	for _, d := range definitions {
		o := Option{Provider: d.provider, Model: d.model, Kind: d.kind, Resolutions: d.resolutions, Prices: []pricing.Quote{}}
		for _, variant := range d.variants {
			q, err := pricing.Lookup(d.provider, d.model, variant, at)
			if err != nil {
				return nil, err
			}
			o.Prices = append(o.Prices, q)
		}
		result = append(result, o)
	}
	return result, nil
}
