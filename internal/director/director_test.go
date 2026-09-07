package director

import (
	"testing"
)

func TestDecodeDirectionNestedShape(t *testing.T) {
	direction, err := DecodeDirection([]byte(`{
		"action":"wave","dialogue":"hello","emotion":"warm",
		"camera":{"shot":"medium","movement":"locked","angle":"eye-level"},
		"continuity":{"notes":"same room","anchors":["wardrobe"]}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if direction.Camera.Shot != "medium" || direction.Camera.Movement != "locked" || direction.Camera.Angle != "eye-level" {
		t.Fatalf("camera = %+v", direction.Camera)
	}
	if direction.Continuity.Notes != "same room" || len(direction.Continuity.Anchors) != 1 {
		t.Fatalf("continuity = %+v", direction.Continuity)
	}
}

func TestDecodeDirectionRejectsUnknownNestedField(t *testing.T) {
	_, err := DecodeDirection([]byte(`{
		"action":"wave","dialogue":"","emotion":"warm",
		"camera":{"shot":"medium","movement":"locked","angle":"eye-level","zoom":2},
		"continuity":{"notes":"same room","anchors":[]}
	}`))
	if err == nil {
		t.Fatal("expected strict decode error")
	}
}

func TestIdleDirectionIsDeterministicAndValid(t *testing.T) {
	first := IdleDirection()
	second := IdleDirection()
	if first.Action != second.Action || first.Camera != second.Camera || first.Continuity.Notes != second.Continuity.Notes {
		t.Fatalf("idle direction changed: first=%+v second=%+v", first, second)
	}
	if err := first.Validate(); err != nil {
		t.Fatal(err)
	}
}
