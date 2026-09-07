package audience

import (
	"testing"
	"time"
)

func TestWindowPrunesDeduplicatesAndProjects(t *testing.T) {
	now := time.Now().UTC()
	window := NewWindow(20 * time.Second)
	window.Add(Message{ID: "old", Text: "旧消息", At: now.Add(-21 * time.Second)})
	window.Add(Message{ID: "one", Text: "看看桌上的杯子", At: now})
	window.Add(Message{ID: "one", Text: "重复", At: now})
	window.Add(Message{ID: "two", Text: "看看桌上的杯子", At: now})
	window.Add(Message{ID: "three", Text: "聊聊今天", At: now})

	snapshot := window.Snapshot(now)
	if snapshot.MessageCount != 3 || snapshot.Summary == "" || len(snapshot.Intents) != 2 {
		t.Fatalf("snapshot = %+v", snapshot)
	}
	if snapshot.Intents[0].Label != "看看桌上的杯子" || snapshot.Intents[0].Support != 2.0/3.0 {
		t.Fatalf("top intent = %+v", snapshot.Intents[0])
	}
}
