package live

import (
	"testing"
	"time"
)

func TestTimelineAppendAndAdvance(t *testing.T) {
	timeline, err := NewTimeline(30 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	first, _ := NewSegment("one", 0, 0, 5*time.Second, testDirection())
	second, _ := NewSegment("two", 1, 5*time.Second, 10*time.Second, testDirection())
	if err := timeline.Append(first); err != nil {
		t.Fatal(err)
	}
	if err := timeline.Append(second); err != nil {
		t.Fatal(err)
	}
	broken, _ := NewSegment("broken", 3, 15*time.Second, 20*time.Second, testDirection())
	if err := timeline.Append(broken); err == nil {
		t.Fatal("expected non-contiguous append to fail")
	}

	timeline.Segments[0].Status = SegmentPlayed
	timeline.Segments[0].Asset = assetPointer(testAsset())
	if err := timeline.AdvancePlayhead(5 * time.Second); err != nil {
		t.Fatal(err)
	}
	if err := timeline.AdvancePlayhead(4 * time.Second); err == nil {
		t.Fatal("expected backwards playhead to fail")
	}
	if err := timeline.AdvancePlayhead(10 * time.Second); err == nil {
		t.Fatal("expected playhead to reject an unplayed segment")
	}
}

func assetPointer(asset VideoAsset) *VideoAsset { return &asset }
