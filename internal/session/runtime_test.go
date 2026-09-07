package session

import (
	"context"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"streaming-agent/internal/audience"
	"streaming-agent/internal/director"
	"streaming-agent/internal/generation"
	"streaming-agent/internal/live"
	"streaming-agent/internal/monitoring"
	"streaming-agent/internal/streaming"
)

func TestRuntimeStartsAsynchronouslyAndStopsAtBoundary(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		hub := monitoring.NewHub(monitoring.EmptySnapshot(time.Now().UTC(), 5*time.Second, 10*time.Second, 5*time.Second))
		preparer := &fakePreparer{}
		output := &fakeOutput{}
		schedulerConfig := generation.DefaultSchedulerConfig(1)
		schedulerConfig.InitialConcurrency = 1
		runtime := NewRuntime(RuntimeConfig{
			StorySeed: "a quiet room", SegmentDuration: 5 * time.Second,
			CommitHorizon: 5 * time.Second, ReadyTarget: 5 * time.Second,
			SubmittedTarget: 10 * time.Second, PlannedHorizon: 10 * time.Second,
			SchedulerInterval: 5 * time.Millisecond, PollInterval: 5 * time.Millisecond,
			FallbackExitReady: 5 * time.Second, DataDir: t.TempDir(),
		}, &memoryStore{}, fakeDirector{}, fakeGenerator{}, preparer, output,
			generation.NewScheduler(schedulerConfig), hub, nil, nil)

		t.Cleanup(func() {
			runtime.mu.Lock()
			cancel := runtime.cancel
			runtime.mu.Unlock()
			if cancel != nil {
				cancel()
			}
		})

		started := time.Now()
		if err := runtime.Start(context.Background()); err != nil {
			t.Fatal(err)
		}
		if time.Since(started) > 100*time.Millisecond {
			t.Fatal("start command did not return asynchronously")
		}
		waitForStatus(t, hub, "running")
		if preparer.fallbackPath == "" {
			t.Fatal("fallback destination was not provided")
		}
		waitForWrite(t, output)
		if err := runtime.Stop(context.Background()); err != nil {
			t.Fatal(err)
		}
		waitForStatus(t, hub, "stopped")
		synctest.Wait()
		output.mu.Lock()
		writes := output.writes
		output.mu.Unlock()
		time.Sleep(time.Second)
		synctest.Wait()
		output.mu.Lock()
		defer output.mu.Unlock()
		if output.writes != writes {
			t.Fatal("playback continued after stop")
		}
	})
}

func waitForStatus(t *testing.T, hub *monitoring.Hub, wanted string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if hub.Current().Session.Status == wanted {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("session status=%s, want %s", hub.Current().Session.Status, wanted)
}

func waitForWrite(t *testing.T, output *fakeOutput) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		output.mu.Lock()
		written := output.writes > 0
		output.mu.Unlock()
		if written {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("runtime did not play a segment")
}

type memoryStore struct{}

func (*memoryStore) CreateSession(context.Context, *live.LiveSession) error            { return nil }
func (*memoryStore) UpdateSession(context.Context, *live.LiveSession) error            { return nil }
func (*memoryStore) UpsertSegment(context.Context, live.SessionID, live.Segment) error { return nil }
func (*memoryStore) CreateAttempt(context.Context, live.SessionID, *generation.Attempt) error {
	return nil
}
func (*memoryStore) UpdateAttempt(context.Context, live.SessionID, *generation.Attempt) error {
	return nil
}
func (*memoryStore) MarkReady(context.Context, live.SessionID, live.AttemptID, live.VideoAsset) error {
	return nil
}
func (*memoryStore) MarkFallbackReady(context.Context, live.SessionID, live.SegmentID, live.VideoAsset) error {
	return nil
}
func (*memoryStore) StartStreamRun(context.Context, live.SessionID) (string, error) {
	return "run", nil
}
func (*memoryStore) FinishStreamRun(context.Context, live.SessionID, string, string, streaming.Stats, string) error {
	return nil
}
func (*memoryStore) ActiveSession(context.Context) (*live.LiveSession, error) { return nil, nil }
func (*memoryStore) ListAttempts(context.Context, live.SessionID) ([]generation.Attempt, error) {
	return nil, nil
}
func (*memoryStore) ListSessionHistory(context.Context) ([]HistorySummary, error) { return nil, nil }
func (*memoryStore) SessionHistory(context.Context, string) (History, error)      { return History{}, nil }
func (*memoryStore) RecordObserverRun(context.Context, live.SessionID, uint64, []string, audience.Observation, string) error {
	return nil
}
func (*memoryStore) AssetPath(context.Context, string) (string, bool, error) { return "", false, nil }

type fakeDirector struct{}

func TestGenerationSpecKeepsIdentityAcrossSequence(t *testing.T) {
	anchors := map[string]string{"chat-live-start": "data:start", "chat-live-end": "data:end"}
	for _, sequence := range []int{0, 3, 4, 9, 12} {
		spec := generationSpec(sequence, anchors)
		if spec.Mode != generation.ModeFirstFrameToVideo || spec.FirstFrame.ID != "chat-live-start" || spec.LastFrame != nil {
			t.Fatalf("sequence %d: %+v", sequence, spec)
		}
	}
}

func (fakeDirector) Direct(context.Context, director.Input) (live.Direction, error) {
	return director.IdleDirection(), nil
}

type fakeGenerator struct{}

func (fakeGenerator) Submit(_ context.Context, request generation.Request) (generation.Job, error) {
	return generation.Job{ID: string(request.AttemptID), Status: generation.JobQueued}, nil
}
func (fakeGenerator) Status(_ context.Context, id string) (generation.Job, error) {
	return generation.Job{ID: id, Status: generation.JobCompleted}, nil
}
func (fakeGenerator) Result(_ context.Context, id string) (generation.Result, error) {
	return generation.Result{JobID: id, AssetURL: "/tmp/fake.mp4"}, nil
}
func (fakeGenerator) Cancel(context.Context, string) error { return nil }

type fakePreparer struct {
	fallbackPath string
}

func (*fakePreparer) Prepare(_ context.Context, request streaming.PrepareRequest) (streaming.Segment, error) {
	return streaming.Segment{Path: request.DestinationPath, Duration: 5 * time.Second, Width: 1280, Height: 720, FrameRate: 30, HasAudio: true}, nil
}
func (p *fakePreparer) GenerateFallback(_ context.Context, path string) (streaming.Segment, error) {
	p.fallbackPath = path
	return streaming.Segment{Path: path, Duration: 5 * time.Second, Width: 1280, Height: 720, FrameRate: 30, HasAudio: true}, nil
}

type fakeOutput struct {
	mu     sync.Mutex
	writes int
	stats  streaming.Stats
}

func (*fakeOutput) Start(context.Context) error { return nil }
func (o *fakeOutput) Write(_ context.Context, segment streaming.Segment) error {
	o.mu.Lock()
	o.writes++
	o.stats.OutTime += segment.Duration
	o.mu.Unlock()
	time.Sleep(10 * time.Millisecond)
	return nil
}
func (*fakeOutput) Stop(context.Context) error { return nil }
func (o *fakeOutput) Stats() streaming.Stats   { o.mu.Lock(); defer o.mu.Unlock(); return o.stats }

func (*memoryStore) ReplanSegments(context.Context, live.SessionID, []live.Segment) error { return nil }
