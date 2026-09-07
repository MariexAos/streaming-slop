//go:build integration

package ffmpeg

import (
	"bytes"
	"context"
	"encoding/base64"
	"image/png"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFramesDecodeStartMiddleAndFinalFrame(t *testing.T) {
	ctx := context.Background()
	source := filepath.Join(t.TempDir(), "frames.mkv")
	log, err := exec.CommandContext(ctx, "ffmpeg", "-v", "error", "-f", "lavfi", "-i", "nullsrc=s=768x432:r=30:d=5,geq=r=N:g=0:b=0", "-c:v", "ffv1", source).CombinedOutput()
	if err != nil {
		t.Fatalf("fixture: %v: %s", err, log)
	}
	frames, err := NewPreparer(PreparerConfig{}).Frames(ctx, source)
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != 3 {
		t.Fatal("missing frame samples")
	}
	for i, expected := range []int{0, 75, 149} {
		raw, err := base64.StdEncoding.DecodeString(strings.SplitN(frames[i], ",", 2)[1])
		if err != nil {
			t.Fatal(err)
		}
		frame, err := png.Decode(bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		red, _, _, _ := frame.At(200, 200).RGBA()
		actual := int(red >> 8)
		if actual < expected-1 || actual > expected+1 {
			t.Fatalf("sample %d: frame=%d want %d", i, actual, expected)
		}
	}
}
