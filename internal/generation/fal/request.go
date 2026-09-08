package fal

import (
	"fmt"
	"strings"
	"time"

	"streaming-agent/internal/generation"
	"streaming-agent/internal/pricing"
)

const TurboModel = "minimax/h3-max-turbo/image-to-video"

func (c *Client) compile(input generation.Request) (submitRequest, error) {
	spec, err := (Compiler{}).Compile(PromptInput{Direction: input.Direction, ReferenceImages: input.References})
	if err != nil {
		return submitRequest{}, err
	}
	prompt := input.Spec.Prompt
	if prompt == "" {
		prompt = "Character: " + input.World.Character.Name + "\nScene: " + input.World.Scene.Location + "\nLighting: " + input.World.Scene.Lighting + "\n" + spec.Prompt
	}
	duration := input.Spec.Duration
	if duration == 0 {
		duration = input.Duration
	}
	if duration == 0 {
		duration = 5 * time.Second
	}
	if duration < 5*time.Second || duration > 15*time.Second || duration%time.Second != 0 {
		return submitRequest{}, fmt.Errorf("fal duration must be an integer between 5 and 15 seconds")
	}
	resolution := input.Spec.Resolution
	if resolution == "" {
		resolution = c.resolution
	}
	if resolution != "480P" && resolution != "768P" {
		return submitRequest{}, fmt.Errorf("unsupported fal resolution %q", resolution)
	}
	request := submitRequest{Prompt: prompt, Duration: int(duration / time.Second), Resolution: resolution, PromptExpansionMode: "balanced", EnableSafetyChecker: true}
	switch c.model {
	case DefaultModel:
		if input.Spec.FirstFrame != nil || input.Spec.LastFrame != nil {
			return request, fmt.Errorf("reference-to-video does not support exact first/last frames")
		}
		request.AspectRatio = "16:9"
		request.ReferenceImageURLs = spec.ReferenceImageURLs
		request.ReferenceVideoURLs = spec.ReferenceVideoURLs
		request.ReferenceAudioURLs = spec.ReferenceAudioURLs
	case TurboModel, "minimax/h3-max/image-to-video":
		if len(input.References) > 0 {
			return request, fmt.Errorf("image-to-video does not support reference arrays")
		}
		if input.Spec.FirstFrame != nil {
			request.ImageURL = input.Spec.FirstFrame.URL
		}
		if input.Spec.LastFrame != nil {
			request.EndImageURL = input.Spec.LastFrame.URL
		}
	default:
		return request, fmt.Errorf("unsupported fal endpoint %q", c.model)
	}
	if strings.TrimSpace(request.Prompt) == "" {
		return request, fmt.Errorf("fal prompt is required")
	}
	return request, nil
}

func (c *Client) BuildRequest(input generation.Request) (generation.Request, error) {
	if input.Model != "" && input.Model != c.model {
		return input, fmt.Errorf("request model differs from configured fal endpoint")
	}
	request, err := c.compile(input)
	if err != nil {
		return input, err
	}
	q, err := pricing.Lookup("fal", c.model, request.Resolution, time.Now())
	if err != nil {
		return input, err
	}
	input.Model = c.model
	input.Spec.Prompt = request.Prompt
	input.Spec.Resolution = request.Resolution
	input.Spec.Duration = time.Duration(request.Duration) * time.Second
	input.Duration = input.Spec.Duration
	input.PriceQuote = &q
	price := q.Reserve(1)
	input.UnitPriceCNY = &price
	return input, nil
}
