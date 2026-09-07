package session_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"streaming-agent/internal/adapter/out/ffmpeg"
	postgresstore "streaming-agent/internal/adapter/out/storage/postgres"
	"streaming-agent/internal/director"
	"streaming-agent/internal/generation"
	"streaming-agent/internal/live"
	"streaming-agent/internal/monitoring"
	"streaming-agent/internal/session"
)

func TestOfflinePostgresFFmpegSoak(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	durationText := os.Getenv("OFFLINE_SOAK_DURATION")
	if databaseURL == "" || durationText == "" {
		t.Skip("TEST_DATABASE_URL and OFFLINE_SOAK_DURATION are required")
	}
	duration, err := time.ParseDuration(durationText)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	store, err := postgresstore.Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	dataDir := t.TempDir()
	preparer := ffmpeg.NewPreparer(ffmpeg.PreparerConfig{})
	source, err := preparer.GenerateFallback(ctx, filepath.Join(dataDir, "provider-source.ts"))
	if err != nil {
		t.Fatal(err)
	}
	outputPath := filepath.Join(dataDir, "stream.flv")
	output, err := ffmpeg.NewOutput(ffmpeg.OutputConfig{RTMPURL: outputPath})
	if err != nil {
		t.Fatal(err)
	}
	hub := monitoring.NewHub(monitoring.EmptySnapshot(time.Now().UTC(), 45*time.Second, 90*time.Second, 30*time.Second))
	scheduler := generation.NewScheduler(generation.DefaultSchedulerConfig(3))
	runtime := session.NewRuntime(session.RuntimeConfig{
		StorySeed: "offline acceptance", SegmentDuration: 5 * time.Second,
		CommitHorizon: 30 * time.Second, ReadyTarget: 45 * time.Second,
		SubmittedTarget: 90 * time.Second, PlannedHorizon: 180 * time.Second,
		SchedulerInterval: 50 * time.Millisecond, PollInterval: 50 * time.Millisecond,
		FallbackExitReady: 30 * time.Second, DataDir: dataDir,
	}, store, soakDirector{}, &soakGenerator{source: source.Path, dataDir: dataDir}, preparer,
		output, scheduler, hub, nil, nil)
	if err := runtime.Start(ctx); err != nil {
		t.Fatal(err)
	}
	waitForSnapshot(t, hub, 2*time.Minute, func(snapshot monitoring.Snapshot) bool {
		return snapshot.Session.Status == "running"
	})

	if err := runtime.ForceFallback(ctx, true); err != nil {
		t.Fatal(err)
	}
	waitForSnapshot(t, hub, 10*time.Second, func(snapshot monitoring.Snapshot) bool {
		return snapshot.Fallback.Active && snapshot.Fallback.Forced
	})
	time.Sleep(6 * time.Second)
	if err := runtime.ForceFallback(ctx, false); err != nil {
		t.Fatal(err)
	}
	waitForSnapshot(t, hub, 10*time.Second, func(snapshot monitoring.Snapshot) bool {
		return !snapshot.Fallback.Forced
	})

	time.Sleep(duration)
	if err := runtime.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	final := waitForSnapshot(t, hub, 20*time.Second, func(snapshot monitoring.Snapshot) bool {
		return snapshot.Session.Status == "stopped"
	})
	if final.Fallback.SecondsTotal < 5 {
		t.Fatalf("fallback seconds=%v, want at least 5", final.Fallback.SecondsTotal)
	}
	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() == 0 {
		t.Fatal("local FLV is empty")
	}
}

func waitForSnapshot(t *testing.T, hub *monitoring.Hub, timeout time.Duration, ready func(monitoring.Snapshot) bool) monitoring.Snapshot {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		snapshot := hub.Current()
		if snapshot.Session.Status == "failed" {
			message := "unknown error"
			if snapshot.Session.LastError != nil {
				message = *snapshot.Session.LastError
			}
			t.Fatalf("runtime failed: %s", message)
		}
		if ready(snapshot) {
			return snapshot
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("condition not reached; last snapshot: %#v", hub.Current())
	return monitoring.Snapshot{}
}

type soakDirector struct{}

func (soakDirector) Direct(context.Context, director.Input) (live.Direction, error) {
	return director.IdleDirection(), nil
}

type soakGenerator struct {
	source  string
	dataDir string
}

func (g *soakGenerator) Submit(_ context.Context, request generation.Request) (generation.Job, error) {
	return generation.Job{ID: string(request.AttemptID), Status: generation.JobQueued}, nil
}
func (*soakGenerator) Status(_ context.Context, id string) (generation.Job, error) {
	return generation.Job{ID: id, Status: generation.JobCompleted}, nil
}
func (g *soakGenerator) Result(_ context.Context, id string) (generation.Result, error) {
	path := filepath.Join(g.dataDir, id+".ts")
	if err := os.Link(g.source, path); err != nil {
		return generation.Result{}, err
	}
	return generation.Result{JobID: id, AssetURL: path}, nil
}
func (*soakGenerator) Cancel(context.Context, string) error { return nil }
