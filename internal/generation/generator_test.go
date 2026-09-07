package generation

import (
	"testing"
	"time"
)

func TestGenerationSpecValidate(t *testing.T) {
	frame := &AssetRef{URL: "https://example.com/anchor.png"}
	tests := []struct {
		name string
		spec GenerationSpec
		ok   bool
	}{
		{"text", GenerationSpec{Mode: ModeTextToVideo, Prompt: "station exterior", Resolution: "768P", Duration: 5 * time.Second, Ratio: "16:9"}, true},
		{"first frame", GenerationSpec{Mode: ModeFirstFrameToVideo, Prompt: "host talks", FirstFrame: frame, Resolution: "768P", Duration: 5 * time.Second, Ratio: "adaptive"}, true},
		{"first last", GenerationSpec{Mode: ModeFirstLastFrameToVideo, Prompt: "host turns", FirstFrame: frame, LastFrame: frame, Resolution: "768P", Duration: 5 * time.Second, Ratio: "adaptive"}, true},
		{"text with frame", GenerationSpec{Mode: ModeTextToVideo, Prompt: "bad", FirstFrame: frame, Resolution: "768P", Duration: 5 * time.Second, Ratio: "16:9"}, false},
		{"image fixed ratio", GenerationSpec{Mode: ModeFirstFrameToVideo, Prompt: "bad", FirstFrame: frame, Resolution: "768P", Duration: 5 * time.Second, Ratio: "16:9"}, false},
		{"2k", GenerationSpec{Mode: ModeTextToVideo, Prompt: "bad", Resolution: "2K", Duration: 5 * time.Second, Ratio: "16:9"}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.spec.Validate()
			if (err == nil) != test.ok {
				t.Fatalf("Validate() error = %v, want valid=%v", err, test.ok)
			}
		})
	}
}
