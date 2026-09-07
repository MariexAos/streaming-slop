package server

import (
	"streaming-agent/internal/audience"
	"streaming-agent/internal/director"
	"streaming-agent/internal/live"
	"streaming-agent/internal/session"
	"testing"
	"time"
)

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
	current := currentFlow(session.View{Session: &sessionValue, Audience: audienceWindow.Snapshot(time.Now().UTC())})
	if current.Status != "running" || current.Direction.TargetLabel == "" || current.Nodes[3].Status != "completed" || current.Audience.MessageCount != 1 || current.Nodes[0].Status != "completed" {
		t.Fatalf("current flow = %+v", current)
	}
}
