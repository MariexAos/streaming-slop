package models

import "testing"

func TestValidateCapabilityAndProviderPairs(t *testing.T) {
	for _, tc := range []struct {
		name        string
		video, text Selection
		valid       bool
	}{
		{"minimax", Selection{Provider: "minimax", Model: "MiniMax-H3-Max", Resolution: "480P"}, Selection{Provider: "minimax", Model: "MiniMax-M3", Resolution: ""}, true},
		{"fal", Selection{Provider: "fal", Model: "minimax/h3-max-turbo/image-to-video", Resolution: "768P"}, Selection{Provider: "minimax", Model: "MiniMax-M3", Resolution: ""}, true},
		{"wrong provider", Selection{Provider: "minimax", Model: "minimax/h3-max-turbo/image-to-video", Resolution: "480P"}, Selection{Provider: "minimax", Model: "MiniMax-M3", Resolution: ""}, false},
		{"text as video", Selection{Provider: "minimax", Model: "MiniMax-M3", Resolution: "480P"}, Selection{Provider: "minimax", Model: "MiniMax-M3", Resolution: ""}, false},
		{"video as text", Selection{Provider: "minimax", Model: "MiniMax-H3-Max", Resolution: "480P"}, Selection{Provider: "minimax", Model: "MiniMax-H3-Max", Resolution: ""}, false},
		{"unsupported resolution", Selection{Provider: "minimax", Model: "MiniMax-H3-Max", Resolution: "1080P"}, Selection{Provider: "minimax", Model: "MiniMax-M3", Resolution: ""}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := Validate(Settings{Video: tc.video, Text: tc.text}); (err == nil) != tc.valid {
				t.Fatalf("Validate() = %v, valid=%v", err, tc.valid)
			}
		})
	}
}
