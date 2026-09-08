package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"streaming-agent/internal/generation"
	video "streaming-agent/internal/generation/minimax"
	"streaming-agent/internal/live"
	"streaming-agent/internal/store"
	"streaming-agent/internal/streaming"
	"streaming-agent/internal/streaming/ffmpeg"
)

const comparisonPrompt = "固定机位的普通居家直播，同一个人物保持长深色头发和米白色针织开衫，自然呼吸、眨眼，轻微移动头部，从首帧平滑过渡到尾帧。看向镜头，偶尔看一下屏幕。不说话，不大幅度运动。保持相同脸部特征、服装、柔和室内光和虚化背景。不要运镜、切镜、字幕或水印。"

func compare480(ctx context.Context, s *store.Store, key string, anchors map[string]string, source, out string) error {
	timings := &measuredTransport{}
	raw, err := video.New(video.Config{APIKey: key, BaseURL: "https://api.minimax.cn", DataDir: out, HTTPClient: &http.Client{Transport: timings, Timeout: 30 * time.Second}})
	if err != nil {
		return err
	}
	client := generation.Budgeted{Generator: raw, Budget: s}
	path, err := comparisonSource(ctx, client, anchors, out)
	if timingErr := timings.save(out); timingErr != nil {
		return errors.Join(err, timingErr)
	}
	if err != nil {
		return err
	}
	if err := prepareComparison(ctx, source, path, out); err != nil {
		return err
	}
	budget, err := s.Spending(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("budget: %+v\n", budget)
	return saveJSON(out, "budget.json", budget)
}

func comparisonSource(ctx context.Context, client generation.Budgeted, anchors map[string]string, out string) (string, error) {
	var result generation.Result
	if err := readComparisonJSON(out, "result.json", &result); err == nil {
		return result.AssetURL, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	var request generation.Request
	err := readComparisonJSON(out, "submission.json", &request)
	if errors.Is(err, os.ErrNotExist) {
		return submitComparison(ctx, client, anchors, out)
	}
	if err != nil {
		return "", err
	}
	var job generation.Job
	if err := readComparisonJSON(out, "job.json", &job); err != nil {
		return "", fmt.Errorf("submission exists without readable job; reconcile before retrying: %w", err)
	}
	fmt.Println("resuming", job.ID)
	return awaitResultEvery(ctx, client, request, job.ID, out, 2*time.Second)
}

func submitComparison(ctx context.Context, client generation.Budgeted, anchors map[string]string, out string) (string, error) {
	request, err := client.BuildRequest(generation.Request{
		AttemptID: live.AttemptID(uuid.NewString()), Duration: 5 * time.Second,
		Spec: generation.GenerationSpec{Mode: generation.ModeFirstLastFrameToVideo,
			Prompt: comparisonPrompt, Resolution: "480P", Duration: 5 * time.Second, Ratio: "adaptive",
			FirstFrame: &generation.AssetRef{URL: anchors["chat-live-start"]},
			LastFrame:  &generation.AssetRef{URL: anchors["chat-live-end"]}},
	})
	if err != nil {
		return "", err
	}
	if err := saveJSON(out, "submission.json", request); err != nil {
		return "", err
	}
	started := time.Now()
	job, err := client.Submit(ctx, request)
	if err != nil {
		return "", err
	}
	if err := saveJSON(out, "job.json", job); err != nil {
		return "", err
	}
	fmt.Printf("submitted %s, quote CNY %.2f\n", job.ID, quote(request))
	path, err := awaitResultEvery(ctx, client, request, job.ID, out, 2*time.Second)
	if err != nil {
		return "", err
	}
	err = saveJSON(out, "generation-timing.json", map[string]any{
		"startedAt": started, "completedAt": time.Now(), "submitPollDownloadSeconds": time.Since(started).Seconds(),
		"pollIntervalSeconds": 2, "samePromptAsBaseline": false,
	})
	return path, err
}

func prepareComparison(ctx context.Context, baseline, generated, out string) error {
	preparer := ffmpeg.NewPreparer(ffmpeg.PreparerConfig{})
	timings := make(map[string]float64)
	for _, clip := range []struct{ name, source string }{{"baseline-720", baseline}, {"480-lanczos-720", generated}} {
		started := time.Now()
		_, err := preparer.Prepare(ctx, streaming.PrepareRequest{
			SourcePath: clip.source, DestinationPath: filepath.Join(out, clip.name+".ts"), Duration: 5 * time.Second,
		})
		if err != nil {
			return fmt.Errorf("prepare %s: %w", clip.name, err)
		}
		timings[clip.name] = time.Since(started).Seconds()
		fmt.Printf("%s preparation %.3fs\n", clip.name, timings[clip.name])
	}
	return saveJSON(out, "preparation-seconds.json", timings)
}

func readComparisonJSON(out, name string, value any) error {
	data, err := os.ReadFile(filepath.Join(out, name))
	if err != nil {
		return err
	}
	return json.Unmarshal(data, value)
}
