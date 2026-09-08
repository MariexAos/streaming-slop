package session

import (
	"context"
	"testing"
	"time"

	"streaming-agent/internal/director"
	"streaming-agent/internal/live"
)

type chattyDirector struct{ calls int }

func (d *chattyDirector) Direct(context.Context, director.Input) (live.Direction, error) {
	d.calls++
	value := director.IdleDirection()
	value.Dialogue = "这杯咖啡有点苦。"
	return value, nil
}
func TestSpeechFollowedByQuietBeat(t *testing.T) {
	previous := director.IdleDirection()
	previous.Dialogue = "刚泡的咖啡。"
	d := &chattyDirector{}
	r := Runtime{director: d}
	value, err := r.direct(context.Background(), director.Input{Previous: &previous})
	if err != nil || value.Dialogue != "" || d.calls != 2 {
		t.Fatalf("quiet beat missing: %+v %v", value, err)
	}
}
func TestConversationRemembersRecentDialogueAndAllowsSpeechAfterPause(t *testing.T) {
	first, _ := live.NewSegment("one", 0, 0, 5*time.Second, director.IdleDirection())
	first.Direction.Dialogue = "这杯咖啡有点苦。"
	r := Runtime{session: &live.LiveSession{Timeline: live.Timeline{Segments: []live.Segment{first}}}}
	input := director.Input{Position: 10 * time.Second}
	if !r.conversationContext(&input) {
		t.Fatal("speech too soon")
	}
	input = director.Input{Position: 15 * time.Second}
	if r.conversationContext(&input) || len(input.RecentDialogue) != 1 {
		t.Fatal("pause never ends or dialogue forgotten")
	}
	if !repeatedDialogue("这杯咖啡，有点苦！", input.RecentDialogue) || repeatedDialogue("换个杯子试试。", input.RecentDialogue) || repeatedDialogue("", input.RecentDialogue) {
		t.Fatal("incorrect duplicate detection")
	}
}
