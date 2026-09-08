package session

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"streaming-agent/internal/audience"
	"streaming-agent/internal/director"
	"streaming-agent/internal/generation"
	"streaming-agent/internal/live"
	"streaming-agent/internal/monitoring"
	"streaming-agent/internal/streaming"
)

type Store interface {
	CreateSession(context.Context, *live.LiveSession) error
	UpdateSession(context.Context, *live.LiveSession) error
	UpsertSegment(context.Context, live.SessionID, live.Segment) error
	ReplanSegments(context.Context, live.SessionID, []live.Segment) error
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
	RecordObserverRun(context.Context, live.SessionID, uint64, []string, audience.Observation, string) error
	AssetPath(context.Context, string) (string, bool, error)
}

type CharacterCatalog interface {
	Resolve(context.Context, string) (live.CharacterProfile, map[string]string, error)
}

type RuntimeConfig struct {
	BuildGeneration   func(context.Context, generation.Request) (generation.Request, error)
	GeneratorFor      func(context.Context, generation.Request) (generation.Generator, error)
	ResolveServices   func(context.Context, *live.LiveSession) (Services, error)
	CheckStart        func(context.Context) error
	Speech            streaming.Speech
	Continuous        bool
	Frames            generation.FrameExtractor
	Catalog           CharacterCatalog
	Audience          *audience.Window
	Observer          audience.Observer
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
	outputBase       time.Duration
	queuedUntil      time.Duration
	receipts         []playbackReceipt
	streamGaps       uint64
	lastGenErr       string
	stopWanted       bool
	directFails      map[int]int
	replanning       map[live.SegmentID]bool
	audienceRevision uint64
	audiencePosition time.Duration
	audienceError    string
	lastObservation  audience.Observation
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
	if active.Profile != nil && r.config.Catalog != nil {
		_, anchors, err := r.config.Catalog.Resolve(ctx, active.Profile.ID)
		if err != nil {
			return err
		}
		r.config.AnchorFrames = anchors
	}
	if active.Models == nil && len(attempts) > 0 {
		first := attempts[0]
		active.Models = &live.ModelSettings{Video: live.ModelSelection{Provider: first.Provider, Model: first.Request.Model, Resolution: first.Request.Spec.Resolution}, Text: live.ModelSelection{Provider: "minimax", Model: "MiniMax-M3"}}
	}
	if err := r.selectServices(ctx, active); err != nil {
		return err
	}
	for _, attempt := range attempts {
		if r.config.GeneratorFor == nil && !attempt.Terminal() && attempt.Provider != r.config.Provider {
			return fmt.Errorf("recover requires provider %s", attempt.Provider)
		}
	}
	now := time.Now().UTC()
	if active.Status != live.SessionRecovering {
		if err := active.Transition(live.SessionRecovering, now); err != nil {
			return fmt.Errorf("recover session state: %w", err)
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
	if r.config.CheckStart != nil {
		if err := r.config.CheckStart(ctx); err != nil {
			return err
		}
	}
	r.mu.Lock()
	if r.cancel != nil || (r.session != nil && r.session.Status != live.SessionStopped && r.session.Status != live.SessionFailed) {
		r.mu.Unlock()
		return fmt.Errorf("%w: session is already active", ErrInvalidState)
	}
	timelineValue, err := live.NewTimeline(r.config.CommitHorizon)
	if err != nil {
		r.mu.Unlock()
		return err
	}
	now := time.Now().UTC()
	world, profile, err := r.initialWorld(ctx)
	if err != nil {
		r.mu.Unlock()
		return err
	}
	created, err := live.NewSession(live.SessionID(uuid.NewString()), world, timelineValue, now)
	if err != nil {
		r.mu.Unlock()
		return err
	}
	if err := r.selectServices(ctx, &created); err != nil {
		r.mu.Unlock()
		return err
	}
	created.Profile = profile
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
	r.lastObservation = audience.Observation{}
	r.audienceRevision = 0
	r.audiencePosition = 0
	r.audienceError = ""
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
		defer func() { r.mu.Lock(); r.cancel = nil; r.mu.Unlock() }()
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

func (r *Runtime) run(ctx context.Context) {
	r.transition(ctx, live.SessionBuffering)
	schedulerTicker := time.NewTicker(r.config.SchedulerInterval)
	pollTicker := time.NewTicker(r.config.PollInterval)
	defer schedulerTicker.Stop()
	defer pollTicker.Stop()
	workers := newWorkGroup()
	defer workers.wg.Wait()
	workers.wg.Go(func() { r.playback(ctx) })

	for {
		select {
		case <-ctx.Done():
			return
		case name := <-workers.done:
			delete(workers.active, name)
		case <-schedulerTicker.C:
			workers.start(ctx, "audience", func(ctx context.Context) { r.replanAudience(ctx, 1) })
			workers.start(ctx, "planning", func(ctx context.Context) { r.plan(ctx, 1) })
			r.acknowledgePlayback(ctx)
			r.commit()
			r.updateFallback(ctx)
			workers.start(ctx, "submit", r.submit)
			r.maybeStartOutput(ctx)
			r.maybeStop(ctx)
			r.publish()
		case <-pollTicker.C:
			workers.start(ctx, "poll", r.poll)
			r.publish()
		}
	}
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
