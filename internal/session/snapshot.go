package session

import (
	"context"
	"time"

	"streaming-agent/internal/generation"
	"streaming-agent/internal/live"
	"streaming-agent/internal/monitoring"
)

func (r *Runtime) PreviewAsset(segmentID string) (string, bool) {
	r.mu.Lock()
	if r.session != nil {
		segmentValue, ok := r.session.Timeline.Segment(live.SegmentID(segmentID))
		if ok && segmentValue.Asset != nil && segmentValue.Asset.Source == live.AssetSourceGenerated && segmentValue.Asset.URI != "" {
			r.mu.Unlock()
			return segmentValue.Asset.URI, true
		}
	}
	r.mu.Unlock()
	path, ok, _ := r.store.AssetPath(context.Background(), segmentID)
	return path, ok
}

func (r *Runtime) ListSessionHistory(ctx context.Context) ([]HistorySummary, error) {
	return r.store.ListSessionHistory(ctx)
}

func (r *Runtime) SessionHistory(ctx context.Context, id string) (History, error) {
	return r.store.SessionHistory(ctx, id)
}

func (r *Runtime) recordGenerationError(err error) {
	r.mu.Lock()
	r.lastGenErr = err.Error()
	r.mu.Unlock()
}

func (r *Runtime) saveAndPublish() {
	r.mu.Lock()
	if r.session == nil {
		r.mu.Unlock()
		return
	}
	if err := r.store.UpdateSession(context.Background(), r.session); err != nil {
		r.logger.Error("save session", "error", err)
	}
	r.mu.Unlock()
	r.publish()
}

func (r *Runtime) publish() {
	r.mu.Lock()
	var current *live.LiveSession
	if r.session != nil {
		copySession := *r.session
		copySession.Timeline.Segments = append([]live.Segment(nil), r.session.Timeline.Segments...)
		current = &copySession
	}
	attempts := append([]generation.Attempt(nil), r.attempts...)
	latencies := successfulLatencies(attempts)
	snapshot := monitoring.Project(monitoring.ProjectionInput{
		Now: time.Now().UTC(), Session: current, Attempts: attempts,
		TargetConcurrency: r.scheduler.TargetConcurrency(latencies),
		ReadyTarget:       r.config.ReadyTarget, SubmittedTarget: r.config.SubmittedTarget,
		FallbackActive: r.fallbackOn, FallbackForced: r.forced,
		FallbackReason: r.fallbackWhy, FallbackSince: r.fallbackAt, FallbackDuration: r.fallbackFor,
		StreamStats: r.output.Stats(), StreamStatus: streamStatus(r.streamOn, current),
		StreamGapTotal: r.streamGaps, GenerationLastError: r.lastGenErr,
		SegmentDuration: r.config.SegmentDuration,
	})
	r.mu.Unlock()
	snapshot = r.hub.Publish(snapshot)
	if r.metrics != nil {
		r.metrics.ObserveSnapshot(snapshot)
	}
}

func streamStatus(on bool, current *live.LiveSession) string {
	if current != nil && current.Status == live.SessionRecovering {
		return "recovering"
	}
	if on {
		return "live"
	}
	if current != nil && (current.Status == live.SessionStarting || current.Status == live.SessionBuffering) {
		return "starting"
	}
	if current != nil && current.Status == live.SessionFailed {
		return "failed"
	}
	return "stopped"
}
