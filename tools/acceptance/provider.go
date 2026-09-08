package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"streaming-agent/internal/director"
	"streaming-agent/internal/generation"
	video "streaming-agent/internal/generation/minimax"
	inference "streaming-agent/internal/inference/minimax"
	"streaming-agent/internal/live"
	"streaming-agent/internal/store"
	"streaming-agent/internal/streaming/ffmpeg"

	"github.com/google/uuid"
)

func errorsIsNotFound(err error) bool { return errors.Is(err, store.ErrNotFound) }
func testProvider(ctx context.Context, s *store.Store, key string, p live.CharacterProfile, anchors map[string]string, mode, source, out string) error {
	media := ffmpeg.NewPreparer(ffmpeg.PreparerConfig{})
	m3 := inference.New(key, s)
	input := generation.Request{Spec: generation.GenerationSpec{FirstFrame: &generation.AssetRef{URL: anchors["chat-live-start"]}}}
	if mode == "generate" {
		next, err := generateNext(ctx, s, m3, key, p, anchors, source, out)
		if err != nil {
			return err
		}
		source = next
	} else if mode != "inspect" {
		return fmt.Errorf("unknown mode %s", mode)
	}
	frames, err := media.Frames(ctx, source)
	if err != nil {
		return err
	}
	review, err := m3.Review(ctx, input, frames)
	if err != nil {
		return err
	}
	if err = saveJSON(out, "review.json", review); err != nil {
		return err
	}
	fmt.Printf("visual review accepted=%t reason=%s\n", review.Accepted, review.Reason)
	b, err := s.Spending(ctx)
	if err != nil {
		return err
	}
	if err = saveJSON(out, "budget.json", b); err != nil {
		return err
	}
	if !review.Accepted {
		return fmt.Errorf("visual acceptance rejected: %s", review.Reason)
	}
	return nil
}
func generateNext(ctx context.Context, s *store.Store, m3 *inference.Client, key string, p live.CharacterProfile, anchors map[string]string, source, out string) (string, error) {
	marker := filepath.Join(out, "submission.json")
	if _, err := os.Stat(marker); err == nil {
		return "", errors.New("submission record exists; do not resubmit")
	}
	frames, err := ffmpeg.NewPreparer(ffmpeg.PreparerConfig{}).Frames(ctx, source)
	if err != nil {
		return "", err
	}
	d, err := m3.Direct(ctx, director.Input{Duration: 5 * time.Second, World: live.WorldState{Character: live.CharacterState{Name: p.Description}, Scene: p.Scene, Camera: p.Camera}, Audience: &director.AudienceInput{Messages: []string{"看一下弹幕，轻轻点头，不用说话"}}, StorySeed: "自然固定机位直播，保持人物和背景，不说话"})
	if err != nil {
		return "", err
	}
	d.Dialogue = ""
	raw, err := video.New(video.Config{APIKey: key, BaseURL: "https://api.minimax.cn", DataDir: out})
	if err != nil {
		return "", err
	}
	client := generation.Budgeted{Generator: raw, Budget: s}
	request, err := client.BuildRequest(generation.Request{AttemptID: live.AttemptID(uuid.NewString()), Duration: 5 * time.Second, Direction: d,
		Spec: generation.GenerationSpec{Mode: generation.ModeFirstFrameToVideo, FirstFrame: &generation.AssetRef{URL: frames[len(frames)-1]}, Duration: 5 * time.Second, Resolution: "768P", Ratio: "adaptive"}})
	if err != nil {
		return "", err
	}
	if err = saveJSON(out, "submission.json", map[string]any{"state": "submitting", "attemptId": request.AttemptID, "quoteCny": quote(request), "characterVersion": p.ID, "source": source, "direction": d}); err != nil {
		return "", err
	}
	job, err := client.Submit(ctx, request)
	if err != nil {
		return "", err
	}
	if err = saveJSON(out, "job.json", job); err != nil {
		return "", err
	}
	fmt.Println("submitted", job.ID)
	return awaitResult(ctx, client, request, job.ID, out)
}
func awaitResult(ctx context.Context, c generation.Budgeted, request generation.Request, id, out string) (string, error) {
	return awaitResultEvery(ctx, c, request, id, out, 5*time.Second)
}

func awaitResultEvery(ctx context.Context, c generation.Budgeted, request generation.Request, id, out string, interval time.Duration) (string, error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
			job, err := c.Status(ctx, id)
			if err != nil {
				return "", err
			}
			fmt.Println("task", job.Status)
			if job.Status == generation.JobFailed || job.Status == generation.JobCancelled {
				return "", fmt.Errorf("generation %s: %s", job.Status, job.ErrorMessage)
			}
			if job.Status != generation.JobCompleted {
				continue
			}
			return finishResult(ctx, c, request, id, out)
		}
	}
}

func finishResult(ctx context.Context, c generation.Budgeted, request generation.Request, id, out string) (string, error) {
	result, err := c.Result(ctx, id)
	if err != nil {
		return "", err
	}
	if err = c.ReconcileCost(ctx, string(request.AttemptID), result.CostCNY); err != nil {
		return "", err
	}
	if err = saveJSON(out, "result.json", result); err != nil {
		return "", err
	}
	return result.AssetURL, nil
}
