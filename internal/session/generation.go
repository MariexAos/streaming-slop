package session

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"streaming-agent/internal/generation"
	"streaming-agent/internal/live"
	"streaming-agent/internal/streaming"
	"streaming-agent/internal/timeline"
)

func (r *Runtime) submit(ctx context.Context) {
	r.mu.Lock()
	if r.session == nil || r.stopWanted {
		r.mu.Unlock()
		return
	}
	ready := timeline.ReadyAhead(r.session.Timeline)
	latencies := successfulLatencies(r.attempts)
	candidates := r.scheduler.Select(r.session.Timeline.Playhead, ready, r.session.Timeline.Segments, r.attempts, latencies)
	sessionID := r.session.ID
	r.mu.Unlock()

	for _, candidate := range candidates {
		r.reserveAndSubmit(ctx, sessionID, candidate)
	}
}

func (r *Runtime) reserveAndSubmit(ctx context.Context, sessionID live.SessionID, candidate generation.Candidate) {
	continuation, err := r.continuation(ctx, candidate.SegmentID)
	if err != nil {
		return
	}
	r.mu.Lock()
	if r.session == nil || r.session.ID != sessionID || r.stopWanted || !r.validContinuation(continuation) {
		r.mu.Unlock()
		return
	}
	segmentValue, ok := r.session.Timeline.Segment(candidate.SegmentID)
	if !ok || (segmentValue.Status != live.SegmentPlanned && segmentValue.Status != live.SegmentGenerating) {
		r.mu.Unlock()
		return
	}
	if segmentValue.Status == live.SegmentPlanned {
		if err := segmentValue.Transition(live.SegmentGenerating); err != nil {
			r.mu.Unlock()
			return
		}
	}
	now := time.Now().UTC()
	attempt, err := generation.NewAttempt(
		live.AttemptID(uuid.NewString()), segmentValue.ID, candidate.AttemptNumber,
		r.config.Provider, fmt.Sprintf("%s:%s:%d:%d", sessionID, segmentValue.ID, segmentValue.PlanRevision, candidate.AttemptNumber), now,
	)
	if err != nil {
		r.mu.Unlock()
		return
	}
	request := generation.Request{
		AttemptID: attempt.ID, SegmentID: segmentValue.ID, IdempotencyKey: attempt.IdempotencyKey,
		Duration: r.config.SegmentDuration, Direction: segmentValue.Direction,
		World: r.session.World, Spec: directedSpec(segmentValue.Direction, r.config.AnchorFrames),
	}
	request.ReferenceFrame = &generation.AssetRef{ID: "chat-live-start", URL: r.config.AnchorFrames["chat-live-start"]}
	if r.session.Profile != nil {
		request.CharacterVersion = r.session.Profile.ID
	}
	applyContinuation(&request, continuation)
	request.Spec.Duration = segmentValue.End - segmentValue.Start
	var buildErr error
	request, buildErr = r.buildRequest(request)
	if buildErr != nil {
		r.lastGenErr = buildErr.Error()
		r.mu.Unlock()
		return
	}
	attempt.PlanRevision = segmentValue.PlanRevision
	attempt.Request = request
	segmentCopy := *segmentValue
	r.mu.Unlock()

	if err := r.store.UpsertSegment(ctx, sessionID, segmentCopy); err != nil {
		r.recordGenerationError(err)
		return
	}
	if err := r.store.CreateAttempt(ctx, sessionID, &attempt); err != nil {
		r.recordGenerationError(err)
		return
	}
	r.mu.Lock()
	r.attempts = append(r.attempts, attempt)
	r.mu.Unlock()
	r.submitAttempt(ctx, sessionID, attempt, request)
}

func (r *Runtime) submitAttempt(ctx context.Context, sessionID live.SessionID, attempt generation.Attempt, request generation.Request) {
	job, err := r.generator.Submit(ctx, request)
	if err != nil {
		if errors.Is(err, generation.ErrBudgetExceeded) {
			r.failAttempt(ctx, attempt.ID, "budget", err)
			_ = r.Stop(ctx)
			return
		}
		r.recordGenerationError(fmt.Errorf("submission outcome unknown for attempt %s; reconcile before retry: %w", attempt.ID, err))
		return
	}
	r.mu.Lock()
	current := r.attempt(attempt.ID)
	if current != nil {
		current.ProviderJobID = job.ID
		_ = current.Transition(generation.AttemptSubmitted, time.Now().UTC())
		if err := r.store.UpdateAttempt(ctx, sessionID, current); err != nil {
			r.lastGenErr = err.Error()
		}
	}
	r.mu.Unlock()
}

func (r *Runtime) poll(ctx context.Context) {
	r.mu.Lock()
	if r.session == nil {
		r.mu.Unlock()
		return
	}
	sessionID := r.session.ID
	var pending []generation.Attempt
	for _, attempt := range r.attempts {
		if attempt.ProviderJobID != "" && !attempt.Terminal() {
			pending = append(pending, attempt)
		}
	}
	r.mu.Unlock()

	for _, attempt := range pending {
		job, err := r.generator.Status(ctx, attempt.ProviderJobID)
		if err != nil {
			r.recordGenerationError(err)
			continue
		}
		switch job.Status {
		case generation.JobQueued:
			continue
		case generation.JobRunning:
			r.transitionAttempt(ctx, sessionID, attempt.ID, generation.AttemptRunning)
		case generation.JobFailed, generation.JobCancelled:
			zero := 0.0
			if err := r.recordCost(ctx, sessionID, attempt.ID, &zero); err != nil {
				r.recordGenerationError(err)
				continue
			}
			r.failAttempt(ctx, attempt.ID, job.ErrorCode, errors.New(job.ErrorMessage))
		case generation.JobCompleted:
			r.completeAttempt(ctx, sessionID, attempt.ID, attempt.ProviderJobID)
		}
	}
}

func (r *Runtime) completeAttempt(ctx context.Context, sessionID live.SessionID, attemptID live.AttemptID, jobID string) {
	r.mu.Lock()
	attempt := r.attempt(attemptID)
	if attempt == nil || attempt.Terminal() {
		r.mu.Unlock()
		return
	}
	status := attempt.Status
	duration := attempt.Request.Duration
	if duration <= 0 {
		duration = r.config.SegmentDuration
	}
	r.mu.Unlock()
	if status == generation.AttemptSubmitted {
		r.transitionAttempt(ctx, sessionID, attemptID, generation.AttemptRunning)
		status = generation.AttemptRunning
	}
	if status == generation.AttemptRunning {
		r.transitionAttempt(ctx, sessionID, attemptID, generation.AttemptPreparing)
	}

	result, err := r.generator.Result(ctx, jobID)
	if err != nil {
		r.failAttempt(ctx, attemptID, "result", err)
		return
	}
	if err := r.recordCost(ctx, sessionID, attemptID, result.CostCNY); err != nil {
		r.recordGenerationError(err)
		return
	}
	audio, err := r.speech(ctx, attemptID, result.AssetURL)
	if err != nil {
		r.failAttempt(ctx, attemptID, "speech", err)
		return
	}
	destination := result.AssetURL + ".normalized.ts"
	prepared, err := r.preparer.Prepare(ctx, streaming.PrepareRequest{AudioPath: audio, SourcePath: result.AssetURL, DestinationPath: destination, Duration: duration})
	if err != nil {
		r.failAttempt(ctx, attemptID, "normalize", err)
		return
	}
	observed, err := r.inspect(ctx, attemptID, result.AssetURL)
	if err != nil {
		r.failAttempt(ctx, attemptID, "visual_review", err)
		return
	}
	now := time.Now().UTC()
	asset := live.VideoAsset{
		Observed: observed, ID: live.AssetID(uuid.NewString()), AttemptID: attemptID, Source: live.AssetSourceGenerated,
		URI: result.AssetURL, NormalizedURI: prepared.Path, Duration: prepared.Duration,
		Width: prepared.Width, Height: prepared.Height, FPS: prepared.FrameRate, VerifiedAt: now,
	}
	r.acceptAsset(ctx, sessionID, attemptID, result, asset)
}

func (r *Runtime) acceptAsset(ctx context.Context, sessionID live.SessionID, attemptID live.AttemptID, result generation.Result, asset live.VideoAsset) {
	now := asset.VerifiedAt
	r.mu.Lock()
	attempt := r.attempt(attemptID)
	if attempt == nil {
		r.mu.Unlock()
		return
	}
	attempt.CostCNY = result.CostCNY
	if err := r.store.UpdateAttempt(ctx, sessionID, attempt); err != nil {
		r.lastGenErr = err.Error()
		r.mu.Unlock()
		return
	}
	segmentID := attempt.SegmentID
	segment, valid := r.session.Timeline.Segment(segmentID)
	if !valid || segment.PlanRevision != attempt.PlanRevision || segment.Status != live.SegmentGenerating || attempt.Terminal() || !r.validContinuation(continuation{SegmentID: attempt.Request.ParentSegmentID, AssetID: attempt.Request.ParentAssetID}) {
		if !attempt.Terminal() {
			_ = attempt.Transition(generation.AttemptSucceeded, now)
			if err := r.store.UpdateAttempt(ctx, sessionID, attempt); err != nil {
				r.lastGenErr = err.Error()
			}
		}
		r.mu.Unlock()
		return
	}

	if err := r.store.MarkReady(ctx, sessionID, attemptID, asset); err != nil {
		r.mu.Unlock()
		r.recordGenerationError(fmt.Errorf("persist ready asset: %w", err))
		return
	}
	attempt = r.attempt(attemptID)
	segmentValue, ok := r.session.Timeline.Segment(segmentID)
	if attempt != nil {
		_ = attempt.Transition(generation.AttemptSucceeded, now)
		attempt.Version++
	}
	if ok {
		_ = segmentValue.MarkReady(asset)
	}
	r.mu.Unlock()
}

func (r *Runtime) transitionAttempt(ctx context.Context, sessionID live.SessionID, id live.AttemptID, status generation.AttemptStatus) {
	r.mu.Lock()
	attempt := r.attempt(id)
	if attempt == nil || attempt.Status == status || attempt.Terminal() {
		r.mu.Unlock()
		return
	}
	if err := attempt.Transition(status, time.Now().UTC()); err != nil {
		r.mu.Unlock()
		return
	}
	_ = r.store.UpdateAttempt(ctx, sessionID, attempt)
	r.mu.Unlock()
}

func (r *Runtime) failAttempt(ctx context.Context, id live.AttemptID, code string, cause error) {
	r.mu.Lock()
	attempt := r.attempt(id)
	if attempt == nil || attempt.Terminal() {
		r.mu.Unlock()
		return
	}
	attempt.ErrorCode = code
	if cause != nil {
		attempt.ErrorMessage = cause.Error()
		r.lastGenErr = cause.Error()
	}
	_ = attempt.Transition(generation.AttemptFailed, time.Now().UTC())
	sessionID := r.session.ID
	_ = r.store.UpdateAttempt(ctx, sessionID, attempt)
	r.mu.Unlock()
}

func (r *Runtime) attempt(id live.AttemptID) *generation.Attempt {
	for i := range r.attempts {
		if r.attempts[i].ID == id {
			return &r.attempts[i]
		}
	}
	return nil
}

func successfulLatencies(attempts []generation.Attempt) []time.Duration {
	var values []time.Duration
	for _, attempt := range attempts {
		if attempt.Status == generation.AttemptSucceeded && attempt.Latency > 0 {
			values = append(values, attempt.Latency)
		}
	}
	return values
}

func (r *Runtime) buildRequest(request generation.Request) (generation.Request, error) {
	if builder, ok := r.generator.(generation.RequestBuilder); ok {
		return builder.BuildRequest(request)
	}
	return request, nil
}
