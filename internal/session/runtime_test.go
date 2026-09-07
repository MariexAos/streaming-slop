package session

import (
	"context"
	"sync"
	"testing"
	"time"

	"streaming-agent/internal/audience"
	"streaming-agent/internal/director"
	"streaming-agent/internal/generation"
	"streaming-agent/internal/live"
	"streaming-agent/internal/monitoring"
	"streaming-agent/internal/observer"
	"streaming-agent/internal/streaming"
)

func TestRuntimeStartsAsynchronouslyAndStopsAtBoundary(t *testing.T) {
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
func (*memoryStore) RecordObserverRun(context.Context, live.SessionID, uint64, []string, observer.Observation, string) error {
	return nil
}
func (*memoryStore) AssetPath(context.Context, string) (string, bool, error) { return "", false, nil }

type fakeDirector struct{}

func TestGenerationSpecFollowsHostDialogueBeatModes(t *testing.T) {
	anchors := map[string]string{
		"chat-live-start": "data:start", "chat-live-end": "data:end",
	}
	first := generationSpec(0, anchors)
	if first.Mode != generation.ModeFirstLastFrameToVideo || first.FirstFrame == nil || first.LastFrame == nil {
		t.Fatalf("first beat spec = %+v", first)
	}
	transition := generationSpec(3, anchors)
	if transition.Mode != generation.ModeFirstFrameToVideo || transition.FirstFrame == nil || transition.FirstFrame.ID != "chat-live-end" {
		t.Fatalf("transition spec = %+v", transition)
	}
	cutaway := generationSpec(4, anchors)
	if cutaway.Mode != generation.ModeTextToVideo || cutaway.Ratio != "16:9" {
		t.Fatalf("cutaway spec = %+v", cutaway)
	}
}

func TestCurrentFlowProjectsPlannedDirection(t *testing.T) {
	timelineValue, err := live.NewTimeline(30 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	directionValue := director.IdleDirection()
	directionValue.Dialogue = "外面那圈灯刚亮，你们也看见了？"
	segmentValue, err := live.NewSegment("segment-1", 0, 0, 5*time.Second, directionValue)
	if err != nil {
		t.Fatal(err)
	}
	if err := timelineValue.Append(segmentValue); err != nil {
		t.Fatal(err)
	}
	sessionValue, err := live.NewSession("session-1", live.WorldState{}, timelineValue, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	audienceWindow := audience.NewWindow(20 * time.Second)
	audienceWindow.Add(audience.Message{ID: "one", Text: "看看桌上的杯子", At: time.Now().UTC()})
	runtime := Runtime{config: RuntimeConfig{Audience: audienceWindow}, session: &sessionValue}
	current := runtime.CurrentFlow()
	if current.Status != "running" || current.Direction.TargetLabel == "" || current.Nodes[3].Status != "completed" || current.Audience.MessageCount != 1 || current.Nodes[0].Status != "completed" {
		t.Fatalf("current flow = %+v", current)
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
func (o *fakeOutput) Write(context.Context, streaming.Segment) error {
	o.mu.Lock()
	o.writes++
	o.mu.Unlock()
	time.Sleep(10 * time.Millisecond)
	return nil
}
func (*fakeOutput) Stop(context.Context) error { return nil }
func (o *fakeOutput) Stats() streaming.Stats   { return o.stats }
