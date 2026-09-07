//go:build integration || soak

package session_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"streaming-agent/internal/director"
	"streaming-agent/internal/generation"
	"streaming-agent/internal/live"
	"streaming-agent/internal/monitoring"
	"streaming-agent/internal/session"
	postgresstore "streaming-agent/internal/store"
	"streaming-agent/internal/streaming/ffmpeg"
)

func TestOfflinePostgresFFmpeg(t *testing.T) {
	runOfflinePipeline(t, 5*time.Second)
}

func runOfflinePipeline(t *testing.T, duration time.Duration) {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("TEST_DATABASE_URL is required")
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
	assertStreamContinuity(t, outputPath, duration)
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

func assertStreamContinuity(t *testing.T, path string, duration time.Duration) {
	t.Helper()
	output, err := exec.Command("ffprobe", "-v", "error", "-show_packets", "-show_entries", "packet=codec_type,dts_time", "-of", "json", path).Output()
	if err != nil {
		t.Fatal(err)
	}
	var probe struct {
		Packets []struct {
			Codec string `json:"codec_type"`
			DTS   string `json:"dts_time"`
		} `json:"packets"`
	}
	if err := json.Unmarshal(output, &probe); err != nil {
		t.Fatal(err)
	}
	first, last := map[string]float64{}, map[string]float64{}
	for _, packet := range probe.Packets {
		timestamp, err := strconv.ParseFloat(packet.DTS, 64)
		if err != nil {
			t.Fatalf("invalid %s DTS %q: %v", packet.Codec, packet.DTS, err)
		}
		previous, exists := last[packet.Codec]
		if exists && (timestamp <= previous || timestamp-previous > 0.25) {
			t.Fatalf("%s DTS discontinuity: %.3f -> %.3f", packet.Codec, previous, timestamp)
		}
		if !exists {
			first[packet.Codec] = timestamp
		}
		last[packet.Codec] = timestamp
	}
	for _, codec := range []string{"video", "audio"} {
		end, exists := last[codec]
		if !exists || end-first[codec] < duration.Seconds() {
			t.Fatalf("%s stream duration %.3fs is shorter than %s", codec, end-first[codec], duration)
		}
	}
	t.Logf("continuous media: video=%.3fs audio=%.3fs", last["video"]-first["video"], last["audio"]-first["audio"])
}
