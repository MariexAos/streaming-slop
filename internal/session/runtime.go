package session

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"

	"streaming-agent/internal/audience"
	"streaming-agent/internal/director"
	"streaming-agent/internal/flow"
	"streaming-agent/internal/generation"
	"streaming-agent/internal/live"
	"streaming-agent/internal/monitoring"
	"streaming-agent/internal/observer"
	"streaming-agent/internal/streaming"
	"streaming-agent/internal/timeline"
)

type Store interface {
	CreateSession(context.Context, *live.LiveSession) error
	UpdateSession(context.Context, *live.LiveSession) error
	UpsertSegment(context.Context, live.SessionID, live.Segment) error
	CreateAttempt(context.Context, live.SessionID, *generation.Attempt) error
	UpdateAttempt(context.Context, live.SessionID, *generation.Attempt) error
	MarkReady(context.Context, live.SessionID, live.AttemptID, live.VideoAsset) error
	MarkFallbackReady(context.Context, live.SessionID, live.SegmentID, live.VideoAsset) error
	StartStreamRun(context.Context, live.SessionID) (string, error)
	FinishStreamRun(context.Context, live.SessionID, string, string, streaming.Stats, string) error
	ActiveSession(context.Context) (*live.LiveSession, error)
	ListAttempts(context.Context, live.SessionID) ([]generation.Attempt, error)
	ListSessionHistory(context.Context) ([]HistorySummary, error)
	SessionHistory(context.Context, string) (History, error)
	RecordObserverRun(context.Context, live.SessionID, uint64, []string, observer.Observation, string) error
	AssetPath(context.Context, string) (string, bool, error)
}

type RuntimeConfig struct {
	Audience          *audience.Window
	Observer          observer.Observer
	StorySeed         string
	Character         string
	AnchorFrames      map[string]string
	SegmentDuration   time.Duration
	CommitHorizon     time.Duration
	ReadyTarget       time.Duration
	SubmittedTarget   time.Duration
	PlannedHorizon    time.Duration
	SchedulerInterval time.Duration
	PollInterval      time.Duration
	FallbackExitReady time.Duration
	DataDir           string
	Provider          string
}

type Runtime struct {
	mu sync.Mutex

	config    RuntimeConfig
	store     Store
	director  director.Director
	generator generation.Generator
	preparer  streaming.Preparer
	output    streaming.Output
	scheduler generation.Scheduler
	hub       *monitoring.Hub
	metrics   *monitoring.Metrics
	logger    *slog.Logger

	session          *live.LiveSession
	attempts         []generation.Attempt
	fallback         streaming.Segment
	fallbackOn       bool
	forced           bool
	fallbackWhy      string
	fallbackAt       *time.Time
	fallbackFor      time.Duration
	streamOn         bool
	streamRunID      string
	streamGaps       uint64
	lastGenErr       string
	stopWanted       bool
	directFails      map[int]int
	audienceRevision uint64
	lastObservation  observer.Observation
	cancel           context.CancelFunc
}

func NewRuntime(
	config RuntimeConfig,
	store Store,
	directorClient director.Director,
	generator generation.Generator,
	preparer streaming.Preparer,
	output streaming.Output,
	scheduler generation.Scheduler,
	hub *monitoring.Hub,
	metrics *monitoring.Metrics,
	logger *slog.Logger,
) *Runtime {
	if logger == nil {
		logger = slog.Default()
	}
	return &Runtime{
		config: config, store: store, director: directorClient, generator: generator,
		preparer: preparer, output: output, scheduler: scheduler, hub: hub, metrics: metrics,
		logger: logger, directFails: make(map[int]int),
	}
}

func (r *Runtime) Recover(ctx context.Context) error {
	active, err := r.store.ActiveSession(ctx)
	if err != nil {
		return fmt.Errorf("load active session: %w", err)
	}
	if active == nil {
		r.publish()
		return nil
	}
	attempts, err := r.store.ListAttempts(ctx, active.ID)
	if err != nil {
		return fmt.Errorf("load active attempts: %w", err)
	}
	now := time.Now().UTC()
	if active.Status != live.SessionRecovering {
		if err := active.Transition(live.SessionRecovering, now); err != nil {
			return fmt.Errorf("recover session state: %w", err)
		}
	}
	for i := range active.Timeline.Segments {
		if active.Timeline.Segments[i].Status == live.SegmentPlaying {
			active.Timeline.Segments[i].Status = live.SegmentCommitted
		}
	}
	if err := r.store.UpdateSession(ctx, active); err != nil {
		return fmt.Errorf("save recovering session: %w", err)
	}
	r.mu.Lock()
	r.session = active
	r.attempts = attempts
	r.mu.Unlock()
	return r.launch(ctx)
}

func (r *Runtime) Start(ctx context.Context) error {
	r.mu.Lock()
	if r.session != nil && r.session.Status != live.SessionStopped && r.session.Status != live.SessionFailed {
		r.mu.Unlock()
		return fmt.Errorf("%w: session is already active", ErrInvalidState)
	}
	timelineValue, err := live.NewTimeline(r.config.CommitHorizon)
	if err != nil {
		r.mu.Unlock()
		return err
	}
	now := time.Now().UTC()
	world := live.WorldState{
		Character: live.CharacterState{Name: r.config.Character, Goal: r.config.StorySeed, Emotion: "calm"},
		Scene: live.SceneState{
			Location: "普通住处里的聊天直播角落，固定手机或网络摄像头，背景保留自然生活痕迹",
			Time:     "持续直播时段", Lighting: "不均匀的室内灯和屏幕光，轻微曝光过度，保留真实手机画质",
		},
		Camera: live.CameraState{Shot: "loose medium close-up", Angle: "slightly low and tilted fixed livestream view"},
	}
	created, err := live.NewSession(live.SessionID(uuid.NewString()), world, timelineValue, now)
	if err != nil {
		r.mu.Unlock()
		return err
	}
	r.session = &created
	r.attempts = nil
	r.fallbackOn = false
	r.forced = false
	r.fallbackWhy = ""
	r.fallbackAt = nil
	r.fallbackFor = 0
	r.streamOn = false
	r.streamGaps = 0
	r.lastGenErr = ""
	r.stopWanted = false
	r.directFails = make(map[int]int)
	r.lastObservation = observer.Observation{}
	r.mu.Unlock()

	if err := r.store.CreateSession(ctx, &created); err != nil {
		r.mu.Lock()
		r.session = nil
		r.mu.Unlock()
		return fmt.Errorf("%w: create session: %w", ErrUnavailable, err)
	}
	return r.launch(ctx)
}

func (r *Runtime) launch(parent context.Context) error {
	r.mu.Lock()
	if r.cancel != nil {
		r.mu.Unlock()
		return fmt.Errorf("%w: runtime loop is already active", ErrInvalidState)
	}
	ctx, cancel := context.WithCancel(context.WithoutCancel(parent))
	r.cancel = cancel
	r.mu.Unlock()

	go func() {
		if err := r.ensureFallback(ctx); err != nil {
			r.fail(ctx, fmt.Errorf("prepare fallback: %w", err))
			return
		}
		r.run(ctx)
	}()
	return nil
}

func (r *Runtime) Stop(ctx context.Context) error {
	r.mu.Lock()
	if r.session == nil || r.session.Status == live.SessionStopped || r.session.Status == live.SessionFailed || r.stopWanted {
		r.mu.Unlock()
		return fmt.Errorf("%w: session is not running", ErrInvalidState)
	}
	r.stopWanted = true
	now := time.Now().UTC()
	if r.session.Status == live.SessionStarting || r.session.Status == live.SessionBuffering || r.session.Status == live.SessionRunning || r.session.Status == live.SessionRecovering {
		if err := r.session.Transition(live.SessionStopping, now); err != nil {
			r.mu.Unlock()
			return fmt.Errorf("%w: %w", ErrInvalidState, err)
		}
		if err := r.store.UpdateSession(ctx, r.session); err != nil {
			r.mu.Unlock()
			return fmt.Errorf("%w: save stopping session: %w", ErrUnavailable, err)
		}
	}
	r.mu.Unlock()
	r.publish()
	return nil
}

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

func (r *Runtime) CurrentFlow() flow.FlowRun {
	r.mu.Lock()
	defer r.mu.Unlock()
	run := flow.IdleHostDialogueRun()
	r.applyAudience(&run)
	if r.session == nil {
		return run
	}
	startedAt := r.session.CreatedAt
	run.StartedAt = &startedAt
	switch r.session.Status {
	case live.SessionFailed:
		run.Status = flow.RunFailed
	case live.SessionStopped:
		return run
	default:
		run.Status = flow.RunRunning
	}
	segments := r.session.Timeline.Segments
	if len(segments) == 0 {
		return run
	}
	targetSegment := segments[len(segments)-1]
	for _, segment := range segments {
		if segment.Status == live.SegmentPlanned {
			targetSegment = segment
			break
		}
	}
	target := targetSegment.Direction.Action
	if targetSegment.Direction.Dialogue != "" {
		target = "回应：" + targetSegment.Direction.Dialogue
	}
	run.Direction.CurrentLabel = "自然聊天"
	run.Direction.TargetLabel = target
	run.Direction.Summary = targetSegment.Direction.Action
	for i := range run.Direction.Axes {
		run.Direction.Axes[i].Current = 35
		run.Direction.Axes[i].Target = 55
	}
	if targetSegment.Direction.Dialogue != "" {
		run.Direction.Axes[0].Target = 75
		run.Direction.Axes[1].Target = 65
	}
	for i := 3; i < len(run.Nodes); i++ {
		run.Nodes[i].Status = flow.NodeCompleted
		run.Nodes[i].Summary = "已应用到当前时间线"
	}
	playhead := r.session.Timeline.Playhead
	for _, segment := range segments {
		if segment.End <= playhead {
			continue
		}
		index := segment.Sequence % len(run.Beats)
		run.Beats[index].Status = beatStatus(segment.Status)
		if run.Direction.EffectiveInSeconds == 0 && segment.Status == live.SegmentPlanned {
			run.Direction.EffectiveInSeconds = (segment.Start - playhead).Seconds()
		}
	}
	return run
}

func (r *Runtime) applyAudience(run *flow.FlowRun) {
	if r.config.Audience == nil {
		return
	}
	snapshot := r.config.Audience.Snapshot(time.Now().UTC())
	run.Audience.WindowSeconds = snapshot.WindowSeconds
	run.Audience.MessageCount = snapshot.MessageCount
	run.Audience.Summary = snapshot.Summary
	run.Audience.Intents = make([]flow.AudienceIntent, 0, len(snapshot.Intents))
	for _, intent := range snapshot.Intents {
		run.Audience.Intents = append(run.Audience.Intents, flow.AudienceIntent{Label: intent.Label, Support: intent.Support})
	}
	if snapshot.MessageCount == 0 {
		return
	}
	run.Nodes[0].Status = flow.NodeCompleted
	run.Nodes[0].Summary = fmt.Sprintf("最近 %d 秒收到 %d 条弹幕", snapshot.WindowSeconds, snapshot.MessageCount)
	if r.lastObservation.Summary != "" {
		run.Nodes[1].Status = flow.NodeCompleted
		run.Nodes[1].Summary = r.lastObservation.Summary
	} else {
		run.Nodes[1].Status = flow.NodePending
		run.Nodes[1].Summary = "等待 Qwen Observer"
	}
	run.Nodes[2].Status = flow.NodeCompleted
	run.Nodes[2].Summary = fmt.Sprintf("合并为 %d 个候选方向", len(snapshot.Intents))
}

func (r *Runtime) directorAudience(now time.Time) *director.AudienceInput {
	if r.config.Audience == nil {
		return nil
	}
	snapshot := r.config.Audience.Snapshot(now)
	if snapshot.MessageCount == 0 {
		return nil
	}
	r.mu.Lock()
	observation := r.lastObservation
	r.mu.Unlock()
	return observedAudience(snapshot, observation)
}

func (r *Runtime) replanAudience(ctx context.Context, limit int) {
	if r.config.Audience == nil {
		return
	}
	snapshot := r.config.Audience.Snapshot(time.Now().UTC())
	r.mu.Lock()
	if snapshot.Revision == r.audienceRevision || r.session == nil || r.stopWanted {
		r.mu.Unlock()
		return
	}
	if snapshot.MessageCount == 0 {
		r.audienceRevision = snapshot.Revision
		r.mu.Unlock()
		return
	}
	type candidate struct {
		segment  live.Segment
		previous *live.Direction
	}
	candidates := make([]candidate, 0, limit)
	segments := r.session.Timeline.Segments
	for i, segment := range segments {
		if segment.Status != live.SegmentPlanned {
			continue
		}
		var previous *live.Direction
		if i > 0 {
			value := segments[i-1].Direction
			previous = &value
		}
		candidates = append(candidates, candidate{segment: segment, previous: previous})
		if len(candidates) == limit {
			break
		}
	}
	sessionID := r.session.ID
	world := r.session.World
	r.mu.Unlock()

	observation := observer.Observation{}
	if r.config.Observer != nil {
		value, err := r.config.Observer.Observe(ctx, observer.Input{Messages: snapshot.Messages})
		if err != nil {
			r.mu.Lock()
			r.lastGenErr = err.Error()
			observation = r.lastObservation
			r.mu.Unlock()
			if recordErr := r.store.RecordObserverRun(ctx, sessionID, snapshot.Revision, snapshot.Messages, observation, err.Error()); recordErr != nil {
				r.recordGenerationError(recordErr)
			}
			if observation.Summary == "" {
				r.mu.Lock()
				r.audienceRevision = snapshot.Revision
				r.mu.Unlock()
				return
			}
		} else {
			if err := r.store.RecordObserverRun(ctx, sessionID, snapshot.Revision, snapshot.Messages, value, ""); err != nil {
				r.recordGenerationError(err)
			}
			r.mu.Lock()
			r.lastObservation = value
			r.mu.Unlock()
			observation = value
		}
	}
	audienceInput := observedAudience(snapshot, observation)
	for _, candidate := range candidates {
		directionValue, err := r.director.Direct(ctx, director.Input{
			World: world, StorySeed: r.config.StorySeed, Previous: candidate.previous,
			Position: candidate.segment.Start, Audience: audienceInput,
		})
		if err != nil {
			r.mu.Lock()
			r.lastGenErr = err.Error()
			r.audienceRevision = snapshot.Revision
			r.mu.Unlock()
			return
		}
		r.mu.Lock()
		if r.session == nil || r.session.ID != sessionID {
			r.mu.Unlock()
			return
		}
		segmentValue, ok := r.session.Timeline.Segment(candidate.segment.ID)
		if !ok || segmentValue.Status != live.SegmentPlanned {
			r.mu.Unlock()
			continue
		}
		segmentValue.Direction = directionValue
		updated := *segmentValue
		r.mu.Unlock()
		if err := r.store.UpsertSegment(ctx, sessionID, updated); err != nil {
			r.recordGenerationError(err)
			return
		}
	}
	r.mu.Lock()
	r.audienceRevision = snapshot.Revision
	r.mu.Unlock()
}

func observedAudience(snapshot audience.Snapshot, observation observer.Observation) *director.AudienceInput {
	intents := make([]string, 0, len(observation.Intents))
	for _, intent := range observation.Intents {
		intents = append(intents, intent.Label)
	}
	return &director.AudienceInput{
		WindowSeconds: snapshot.WindowSeconds, Messages: snapshot.Messages,
		Summary: observation.Summary, Mood: observation.Mood, Intents: intents,
	}
}

func beatStatus(status live.SegmentStatus) flow.BeatStatus {
	switch status {
	case live.SegmentGenerating:
		return flow.BeatGenerating
	case live.SegmentReady:
		return flow.BeatReady
	case live.SegmentCommitted, live.SegmentPlaying, live.SegmentPlayed:
		return flow.BeatLocked
	default:
		return flow.BeatPlanned
	}
}

func (r *Runtime) run(ctx context.Context) {
	r.transition(ctx, live.SessionBuffering)
	schedulerTicker := time.NewTicker(r.config.SchedulerInterval)
	pollTicker := time.NewTicker(r.config.PollInterval)
	defer schedulerTicker.Stop()
	defer pollTicker.Stop()
	go r.playback(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-schedulerTicker.C:
			r.replanAudience(ctx, 4)
			r.plan(ctx, 4)
			r.commit()
			r.updateFallback(ctx)
			r.submit(ctx)
			r.maybeStartOutput(ctx)
			r.maybeStop(ctx)
			r.publish()
		case <-pollTicker.C:
			r.poll(ctx)
			r.publish()
		}
	}
}

func (r *Runtime) plan(ctx context.Context, limit int) {
	for range limit {
		r.mu.Lock()
		if r.session == nil || r.stopWanted {
			r.mu.Unlock()
			return
		}
		end := time.Duration(0)
		sequence := 0
		var previous *live.Direction
		if segments := r.session.Timeline.Segments; len(segments) > 0 {
			last := segments[len(segments)-1]
			end = last.End
			sequence = last.Sequence + 1
			copy := last.Direction
			previous = &copy
		}
		if end-r.session.Timeline.Playhead >= r.config.PlannedHorizon {
			r.mu.Unlock()
			return
		}
		world := r.session.World
		r.mu.Unlock()

		directionValue, err := r.director.Direct(ctx, director.Input{
			World: world, StorySeed: r.config.StorySeed, Previous: previous, Position: end,
			Audience: r.directorAudience(time.Now().UTC()),
		})
		if err != nil {
			r.mu.Lock()
			r.directFails[sequence]++
			failures := r.directFails[sequence]
			r.lastGenErr = err.Error()
			r.mu.Unlock()
			if failures < 2 {
				return
			}
			directionValue = director.IdleDirection()
		}
		segmentValue, err := live.NewSegment(
			live.SegmentID(uuid.NewString()), sequence, end, end+r.config.SegmentDuration, directionValue,
		)
		if err != nil {
			r.fail(ctx, err)
			return
		}
		r.mu.Lock()
		if r.session == nil || len(r.session.Timeline.Segments) != sequence {
			r.mu.Unlock()
			return
		}
		if err := r.session.Timeline.Append(segmentValue); err != nil {
			r.mu.Unlock()
			r.fail(ctx, err)
			return
		}
		sessionID := r.session.ID
		delete(r.directFails, sequence)
		r.mu.Unlock()
		if err := r.store.UpsertSegment(ctx, sessionID, segmentValue); err != nil {
			r.fail(ctx, fmt.Errorf("save planned segment: %w", err))
			return
		}
	}
}

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
	r.mu.Lock()
	if r.session == nil || r.session.ID != sessionID {
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
		r.config.Provider, fmt.Sprintf("%s:%s:%d", sessionID, segmentValue.ID, candidate.AttemptNumber), now,
	)
	if err != nil {
		r.mu.Unlock()
		return
	}
	request := generation.Request{
		AttemptID: attempt.ID, SegmentID: segmentValue.ID, IdempotencyKey: attempt.IdempotencyKey,
		Duration: r.config.SegmentDuration, Direction: segmentValue.Direction,
		World: r.session.World, Spec: generationSpec(segmentValue.Sequence, r.config.AnchorFrames),
	}
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
	job, err := r.generator.Submit(ctx, request)
	if err != nil {
		r.failAttempt(ctx, attempt.ID, "submit_unknown", err)
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

func generationSpec(sequence int, anchors map[string]string) generation.GenerationSpec {
	base := generation.GenerationSpec{Resolution: "768P", Duration: 5 * time.Second}
	phase := sequence % 12
	if phase == 4 || phase == 9 {
		base.Mode = generation.ModeTextToVideo
		base.Ratio = "16:9"
		return base
	}
	if phase == 0 || phase == 6 {
		first, firstOK := anchors["chat-live-start"]
		last, lastOK := anchors["chat-live-end"]
		if firstOK && lastOK {
			base.Mode = generation.ModeFirstLastFrameToVideo
			base.Ratio = "adaptive"
			base.FirstFrame = &generation.AssetRef{ID: "chat-live-start", URL: first}
			base.LastFrame = &generation.AssetRef{ID: "chat-live-end", URL: last}
			return base
		}
	}
	names := [...]string{"chat-live-start", "chat-live-end", "chat-live-start", "chat-live-end", "chat-live-start", "chat-live-end", "chat-live-start", "chat-live-end", "chat-live-start", "chat-live-end", "chat-live-end", "chat-live-start"}
	name := names[phase]
	if anchor, ok := anchors[name]; ok {
		base.Mode = generation.ModeFirstFrameToVideo
		base.Ratio = "adaptive"
		base.FirstFrame = &generation.AssetRef{ID: name, URL: anchor}
		return base
	}
	base.Mode = generation.ModeTextToVideo
	base.Ratio = "16:9"
	return base
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
	destination := result.AssetURL + ".normalized.ts"
	prepared, err := r.preparer.Prepare(ctx, streaming.PrepareRequest{SourcePath: result.AssetURL, DestinationPath: destination})
	if err != nil {
		r.failAttempt(ctx, attemptID, "normalize", err)
		return
	}
	now := time.Now().UTC()
	asset := live.VideoAsset{
		ID: live.AssetID(uuid.NewString()), AttemptID: attemptID, Source: live.AssetSourceGenerated,
		URI: result.AssetURL, NormalizedURI: prepared.Path, Duration: prepared.Duration,
		Width: prepared.Width, Height: prepared.Height, FPS: prepared.FrameRate, VerifiedAt: now,
	}
	r.mu.Lock()
	attempt = r.attempt(attemptID)
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
	r.mu.Unlock()
	if err := r.store.MarkReady(ctx, sessionID, attemptID, asset); err != nil {
		r.fail(ctx, fmt.Errorf("persist ready asset: %w", err))
		return
	}
	r.mu.Lock()
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
		r.mu.Lock()
		if r.session == nil || !r.streamOn || r.stopWanted {
			r.mu.Unlock()
			continue
		}
		_, _ = timeline.CommitReady(&r.session.Timeline)
		segmentValue, err := timeline.StartNext(&r.session.Timeline)
		if err != nil {
			r.mu.Unlock()
			continue
		}
		media := r.fallback
		fallback := r.fallbackOn
		if !fallback {
			if segmentValue.Asset == nil {
				r.mu.Unlock()
				continue
			}
			media = streaming.Segment{
				Path: segmentValue.Asset.NormalizedURI, Duration: segmentValue.Asset.Duration,
				Width: segmentValue.Asset.Width, Height: segmentValue.Asset.Height,
				FrameRate: segmentValue.Asset.FPS, HasAudio: true,
			}
		}
		segmentID := segmentValue.ID
		fallback = fallback || segmentValue.Asset.Source == live.AssetSourceFallback
		r.mu.Unlock()
		r.saveAndPublish()

		if err := r.output.Write(ctx, media); err != nil {
			r.fail(ctx, fmt.Errorf("write stream segment: %w", err))
			return
		}
		r.mu.Lock()
		if r.session != nil {
			_ = timeline.FinishPlaying(&r.session.Timeline, segmentID)
			if fallback {
				r.fallbackFor += media.Duration
			}
		}
		r.mu.Unlock()
		r.saveAndPublish()
	}
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
		sessionID := r.session.ID
		segmentID, asset, cancelled := r.ensureFallbackSegmentLocked()
		r.mu.Unlock()
		for i := range cancelled {
			if cancelled[i].ProviderJobID != "" {
				_ = r.generator.Cancel(ctx, cancelled[i].ProviderJobID)
			}
			if err := r.store.UpdateAttempt(ctx, sessionID, &cancelled[i]); err != nil {
				r.recordGenerationError(err)
			}
		}
		if asset != nil {
			if err := r.store.MarkFallbackReady(ctx, sessionID, segmentID, *asset); err != nil {
				r.fail(ctx, fmt.Errorf("persist fallback segment: %w", err))
			}
		}
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

func (r *Runtime) ensureFallbackSegmentLocked() (live.SegmentID, *live.VideoAsset, []generation.Attempt) {
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
		var cancelled []generation.Attempt
		for j := range r.attempts {
			attempt := &r.attempts[j]
			if attempt.SegmentID != segmentValue.ID || attempt.Terminal() {
				continue
			}
			attempt.ErrorCode = "fallback"
			attempt.ErrorMessage = "generation cancelled for fallback"
			if err := attempt.Transition(generation.AttemptCancelled, time.Now().UTC()); err == nil {
				cancelled = append(cancelled, *attempt)
			}
		}
		asset := live.VideoAsset{
			ID: live.AssetID(uuid.NewString()), Source: live.AssetSourceFallback,
			URI: r.fallback.Path, NormalizedURI: r.fallback.Path,
			Duration: r.fallback.Duration, Width: r.fallback.Width, Height: r.fallback.Height,
			FPS: r.fallback.FrameRate, VerifiedAt: time.Now().UTC(),
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

func (r *Runtime) maybeStop(ctx context.Context) {
	r.mu.Lock()
	if !r.stopWanted || r.session == nil {
		r.mu.Unlock()
		return
	}
	playing := false
	for _, segmentValue := range r.session.Timeline.Segments {
		playing = playing || segmentValue.Status == live.SegmentPlaying
	}
	streamOn := r.streamOn
	streamRunID := r.streamRunID
	sessionID := r.session.ID
	if playing {
		r.mu.Unlock()
		return
	}
	r.mu.Unlock()
	if streamOn {
		_ = r.output.Stop(ctx)
		_ = r.store.FinishStreamRun(ctx, sessionID, streamRunID, "STOPPED", r.output.Stats(), "")
	}
	r.mu.Lock()
	r.streamOn = false
	r.streamRunID = ""
	if r.session.Status == live.SessionStopping {
		_ = r.session.Transition(live.SessionStopped, time.Now().UTC())
	}
	cancel := r.cancel
	r.cancel = nil
	r.mu.Unlock()
	r.saveAndPublish()
	if cancel != nil {
		cancel()
	}
}

func (r *Runtime) transition(ctx context.Context, status live.SessionStatus) {
	r.mu.Lock()
	if r.session == nil {
		r.mu.Unlock()
		return
	}
	_ = r.session.Transition(status, time.Now().UTC())
	_ = r.store.UpdateSession(ctx, r.session)
	r.mu.Unlock()
	r.publish()
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

func (r *Runtime) fail(ctx context.Context, cause error) {
	r.logger.Error("runtime failed", "error", cause)
	r.mu.Lock()
	if r.session == nil {
		r.mu.Unlock()
		return
	}
	now := time.Now().UTC()
	if r.session.Status != live.SessionFailed {
		_ = r.session.Transition(live.SessionFailed, now)
	}
	r.session.Stream.LastError = cause.Error()
	streamOn := r.streamOn
	streamRunID := r.streamRunID
	sessionID := r.session.ID
	r.streamOn = false
	r.streamRunID = ""
	cancel := r.cancel
	r.cancel = nil
	_ = r.store.UpdateSession(ctx, r.session)
	r.mu.Unlock()
	if streamOn {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = r.output.Stop(stopCtx)
		stopCancel()
		_ = r.store.FinishStreamRun(context.Background(), sessionID, streamRunID, "FAILED", r.output.Stats(), cause.Error())
	}
	r.publish()
	if cancel != nil {
		cancel()
	}
}

func (r *Runtime) recordGenerationError(err error) {
	r.mu.Lock()
	r.lastGenErr = err.Error()
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

func (r *Runtime) saveAndPublish() {
	r.mu.Lock()
	if r.session == nil {
		r.mu.Unlock()
		return
	}
	if err := r.store.UpdateSession(context.Background(), r.session); err != nil {
		r.logger.Error("save session", "error", err)
	}
	sessionID := r.session.ID
	segments := append([]live.Segment(nil), r.session.Timeline.Segments...)
	r.mu.Unlock()
	for _, segmentValue := range segments {
		switch segmentValue.Status {
		case live.SegmentReady:
			if segmentValue.Asset != nil && segmentValue.Asset.Source == live.AssetSourceFallback {
				if err := r.store.MarkFallbackReady(context.Background(), sessionID, segmentValue.ID, *segmentValue.Asset); err != nil {
					r.logger.Error("save fallback segment", "segment_id", segmentValue.ID, "error", err)
				}
			}
		case live.SegmentCommitted, live.SegmentPlaying, live.SegmentPlayed:
			if err := r.store.UpsertSegment(context.Background(), sessionID, segmentValue); err != nil {
				r.logger.Error("save segment", "segment_id", segmentValue.ID, "error", err)
			}
		}
	}
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

func successfulLatencies(attempts []generation.Attempt) []time.Duration {
	var values []time.Duration
	for _, attempt := range attempts {
		if attempt.Status == generation.AttemptSucceeded && attempt.Latency > 0 {
			values = append(values, attempt.Latency)
		}
	}
	return values
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
