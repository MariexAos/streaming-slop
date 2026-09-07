package flow

import (
	"strings"
	"testing"
)

func TestHostDialogueFlowAndIdleRun(t *testing.T) {
	definition := HostDialogueFlow()
	if definition.Kind != FlowKindPrimary || !definition.Enabled || len(definition.Edges) != len(definition.Nodes)-1 {
		t.Fatalf("definition = %+v", definition)
	}
	if !strings.Contains(HostDialogueInstruction, HostRole) || !strings.Contains(HostDialogueInstruction, "不要自称 AI") {
		t.Fatalf("host dialogue instruction does not lock role and tone: %q", HostDialogueInstruction)
	}
	run := IdleHostDialogueRun()
	if run.FlowID != definition.ID || run.Status != RunIdle {
		t.Fatalf("run = %+v", run)
	}
	if err := run.Direction.Validate(run.Beats); err != nil {
		t.Fatal(err)
	}
	for _, beat := range run.Beats {
		if beat.Status != BeatPlanned {
			t.Fatalf("beat = %+v", beat)
		}
	}
	if run.Beats[0].Mode != "first_last_frame_to_video" || run.Beats[4].Mode != "text_to_video" {
		t.Fatalf("transition modes = %q/%q", run.Beats[0].Mode, run.Beats[4].Mode)
	}
}
