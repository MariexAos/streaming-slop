package ffmpeg

import (
	"context"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"streaming-agent/internal/streaming"
)

func TestPrepareAddsAudioAndNormalizesVideo(t *testing.T) {
	requireMediaTools(t)
	directory := t.TempDir()
	source := filepath.Join(directory, "source.mp4")
	destination := filepath.Join(directory, "normalized.ts")
	command := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "testsrc2=s=320x240:r=24:d=1", "-an", "-c:v", "libx264", source)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generate fixture: %v: %s", err, output)
	}

	preparer := NewPreparer(PreparerConfig{CommandTimeout: 30 * time.Second})
	hasAudio, err := preparer.ProbeAudio(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	if hasAudio {
		t.Fatal("fixture unexpectedly has audio")
	}
	segment, err := preparer.Prepare(context.Background(), streaming.PrepareRequest{
		SourcePath: source, DestinationPath: destination,
	})
	if err != nil {
		t.Fatal(err)
	}
	if segment.Duration != 5*time.Second || !segment.HasAudio || segment.Width != 1280 || segment.Height != 720 || segment.FrameRate != 30 {
		t.Fatalf("segment = %+v", segment)
	}
	assertMediaContract(t, destination)
}

func TestGenerateFallbackMatchesMediaContract(t *testing.T) {
	requireMediaTools(t)
	destination := filepath.Join(t.TempDir(), "fallback.ts")
	preparer := NewPreparer(PreparerConfig{CommandTimeout: 30 * time.Second})
	if _, err := preparer.GenerateFallback(context.Background(), destination); err != nil {
		t.Fatal(err)
	}
	assertMediaContract(t, destination)
}

func requireMediaTools(t *testing.T) {
	t.Helper()
	for _, name := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(name); err != nil {
			t.Skip(name + " is not installed")
		}
	}
}

func assertMediaContract(t *testing.T, path string) {
	t.Helper()
	output, err := exec.Command("ffprobe", "-v", "error", "-show_streams", "-show_format", "-of", "json", path).Output()
	if err != nil {
		t.Fatal(err)
	}
	var probe struct {
		Streams []struct {
			CodecType   string `json:"codec_type"`
			CodecName   string `json:"codec_name"`
			Width       int    `json:"width"`
			Height      int    `json:"height"`
			PixelFormat string `json:"pix_fmt"`
			FrameRate   string `json:"avg_frame_rate"`
			SampleRate  string `json:"sample_rate"`
			Channels    int    `json:"channels"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if err := json.Unmarshal(output, &probe); err != nil {
		t.Fatal(err)
	}
	var videoOK, audioOK bool
	for _, stream := range probe.Streams {
		switch stream.CodecType {
		case "video":
			videoOK = stream.CodecName == "h264" && stream.Width == 1280 && stream.Height == 720 && stream.PixelFormat == "yuv420p" && stream.FrameRate == "30/1"
		case "audio":
			audioOK = stream.CodecName == "aac" && stream.SampleRate == "48000" && stream.Channels == 2
		}
	}
	duration, err := strconv.ParseFloat(probe.Format.Duration, 64)
	if err != nil {
		t.Fatal(err)
	}
	if !videoOK || !audioOK || duration < 4.9 || duration > 5.1 {
		t.Fatalf("media contract failed: video=%v audio=%v duration=%v probe=%s", videoOK, audioOK, duration, output)
	}
}
