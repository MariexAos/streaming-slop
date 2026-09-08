package session

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"streaming-agent/internal/generation"
	"streaming-agent/internal/live"
	"streaming-agent/internal/streaming"
	"streaming-agent/internal/timeline"
)

func (r *Runtime) ForceFallback(_ context.Context, forced bool) error {
	r.mu.Lock()
	if r.session == nil || r.session.Status == live.SessionStopped || r.session.Status == live.SessionFailed {
		r.mu.Unlock()
		return fmt.Errorf("%w: session is not running", ErrInvalidState)
	}
	if r.forced == forced {
		r.mu.Unlock()
		return fmt.Errorf("%w: fallback override is already %t", ErrInvalidState, forced)
	}
	r.forced = forced
	if forced {
		r.activateFallbackLocked("operator")
	}
	r.mu.Unlock()
	r.publish()
	return nil
}

func (r *Runtime) commit() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.session == nil {
		return
	}
	_, _ = timeline.CommitReady(&r.session.Timeline)
}

func (r *Runtime) maybeStartOutput(ctx context.Context) {
	r.mu.Lock()
	if r.session == nil || r.streamOn || r.stopWanted || timeline.ReadyAhead(r.session.Timeline) < r.config.ReadyTarget {
		r.mu.Unlock()
		return
	}
	r.mu.Unlock()
	if err := r.output.Start(ctx); err != nil {
		r.fail(ctx, fmt.Errorf("start stream output: %w", err))
		return
	}
	r.mu.Lock()
	sessionID := r.session.ID
	r.mu.Unlock()
	streamRunID, err := r.store.StartStreamRun(ctx, sessionID)
	if err != nil {
		_ = r.output.Stop(ctx)
		r.fail(ctx, fmt.Errorf("record stream start: %w", err))
		return
	}
	r.mu.Lock()
	r.streamOn = true
	r.streamRunID = streamRunID
	r.outputBase = r.session.Timeline.Playhead
	r.queuedUntil = r.outputBase
	r.receipts = nil
	if r.session.Status == live.SessionBuffering || r.session.Status == live.SessionRecovering {
		_ = r.session.Transition(live.SessionRunning, time.Now().UTC())
	}
	r.mu.Unlock()
	r.saveAndPublish()
}

func (r *Runtime) playback(ctx context.Context) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		if !r.playNext(ctx) {
			return
		}
	}
}

func (r *Runtime) playNext(ctx context.Context) bool {
	r.mu.Lock()
	if r.session == nil || !r.streamOn {
		r.mu.Unlock()
		return true
	}
	if r.stopWanted {
		r.mu.Unlock()
		if err := r.output.Stop(ctx); err != nil {
			r.fail(ctx, err)
		}
		r.acknowledgePlayback(ctx)
		return false
	}
	if len(r.receipts) >= 2 {
		r.mu.Unlock()
		return true
	}
	_, _ = timeline.CommitReady(&r.session.Timeline)
	segmentValue, err := r.nextForOutputLocked()
	if err != nil {
		r.mu.Unlock()
		return true
	}
	media := r.fallback
	fallback := r.fallbackOn
	if !fallback {
		if segmentValue.Asset == nil {
			r.mu.Unlock()
			return true
		}
		media = streaming.Segment{
			Path: segmentValue.Asset.NormalizedURI, Duration: segmentValue.Asset.Duration,
			Width: segmentValue.Asset.Width, Height: segmentValue.Asset.Height,
			FrameRate: segmentValue.Asset.FPS, HasAudio: true,
		}
	}
	segmentID := segmentValue.ID
	segmentEnd := segmentValue.End
	fallback = fallback || segmentValue.Asset.Source == live.AssetSourceFallback
	r.mu.Unlock()
	r.saveAndPublish()

	if err := r.output.Write(ctx, media); err != nil {
		r.fail(ctx, fmt.Errorf("write stream segment: %w", err))
		return false
	}
	if r.output.Stats().Restarts > 0 {
		r.fail(ctx, fmt.Errorf("output restarted; recover from last confirmed playhead"))
		return false
	}
	r.mu.Lock()
	if r.session != nil {
		r.queuedUntil = segmentEnd
		r.receipts = append(r.receipts, playbackReceipt{ID: segmentID, End: segmentEnd, Fallback: fallback})
	}
	r.mu.Unlock()
	r.acknowledgePlayback(ctx)
	return true
}

func (r *Runtime) updateFallback(ctx context.Context) {
	r.mu.Lock()
	if r.session == nil || !r.streamOn {
		r.mu.Unlock()
		return
	}
	ready := timeline.ReadyAhead(r.session.Timeline)
	if r.forced || (r.fallbackOn && ready < r.config.FallbackExitReady) || generation.Mode(ready) == generation.ModeFallback {
		reason := "buffer"
		if r.forced {
			reason = "operator"
		}
		r.activateFallbackLocked(reason)
		_, _, cancelled := r.ensureFallbackSegmentLocked(ctx)
		r.mu.Unlock()
		r.cancelFallbackAttempts(ctx, cancelled)
		return
	}
	if r.fallbackOn && ready >= r.config.FallbackExitReady {
		r.fallbackOn = false
		r.fallbackWhy = ""
		r.fallbackAt = nil
	}
	r.mu.Unlock()
}

func (r *Runtime) activateFallbackLocked(reason string) {
	if !r.fallbackOn {
		now := time.Now().UTC()
		r.fallbackAt = &now
	}
	r.fallbackOn = true
	r.fallbackWhy = reason
}

func (r *Runtime) ensureFallbackSegmentLocked(ctx context.Context) (live.SegmentID, *live.VideoAsset, []generation.Attempt) {
	for i := range r.session.Timeline.Segments {
		segmentValue := &r.session.Timeline.Segments[i]
		if segmentValue.End <= r.session.Timeline.Playhead {
			continue
		}
		if segmentValue.Start != r.session.Timeline.Playhead || segmentValue.Status == live.SegmentCommitted || segmentValue.Status == live.SegmentPlaying {
			return "", nil, nil
		}
		if segmentValue.Status == live.SegmentPlanned {
			_ = segmentValue.Transition(live.SegmentGenerating)
		}
		if segmentValue.Status != live.SegmentGenerating {
			return "", nil, nil
		}
		cancelled := r.cancelSegmentAttemptsLocked(segmentValue.ID)
		asset := live.VideoAsset{
			ID: live.AssetID(uuid.NewString()), Source: live.AssetSourceFallback,
			URI: r.fallback.Path, NormalizedURI: r.fallback.Path,
			Duration: r.fallback.Duration, Width: r.fallback.Width, Height: r.fallback.Height,
			FPS: r.fallback.FrameRate, VerifiedAt: time.Now().UTC(),
		}
		if err := r.store.MarkFallbackReady(ctx, r.session.ID, segmentValue.ID, asset); err != nil {
			r.lastGenErr = fmt.Sprintf("persist fallback segment: %v", err)
			return "", nil, nil
		}
		if err := segmentValue.MarkReady(asset); err != nil {
			return "", nil, cancelled
		}
		return segmentValue.ID, &asset, cancelled
	}
	return "", nil, nil
}

func (r *Runtime) ensureFallback(ctx context.Context) error {
	segmentValue, err := r.preparer.GenerateFallback(ctx, filepath.Join(r.config.DataDir, "fallback.ts"))
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.fallback = segmentValue
	r.mu.Unlock()
	return nil
}

func (r *Runtime) cancelFallbackAttempts(ctx context.Context, cancelled []generation.Attempt) {
	for _, attempt := range cancelled {
		if attempt.ProviderJobID != "" {
			client, err := r.generatorFor(ctx, attemptRequest(attempt))
			if err != nil {
				r.recordGenerationError(err)
				continue
			}
			if err := client.Cancel(ctx, attempt.ProviderJobID); err != nil {
				r.recordGenerationError(fmt.Errorf("cancel task %s: %w", attempt.ProviderJobID, err))
			}
		}
	}
}

func (r *Runtime) cancelSegmentAttemptsLocked(segmentID live.SegmentID) []generation.Attempt {
	var cancelled []generation.Attempt
	for j := range r.attempts {
		attempt := &r.attempts[j]
		if attempt.SegmentID != segmentID || attempt.Terminal() {
			continue
		}
		// Cancellation is a request. Keep polling until the provider confirms a terminal state.
		cancelled = append(cancelled, *attempt)
	}
	return cancelled
}
