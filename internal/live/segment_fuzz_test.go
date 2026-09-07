package live

import (
	"reflect"
	"testing"
	"time"
)

func FuzzSegmentReplan(f *testing.F) {
	for _, freeze := range []int64{29, 30, 31} {
		for stage := uint8(0); stage < 6; stage++ {
			f.Add(freeze, stage)
		}
	}
	f.Fuzz(func(t *testing.T, freeze int64, stage uint8) {
		segment, err := NewSegment("segment", 6, 30*time.Second, 35*time.Second, testDirection())
		if err != nil {
			t.Fatal(err)
		}
		stages := []SegmentStatus{SegmentPlanned, SegmentGenerating, SegmentReady, SegmentCommitted, SegmentPlaying, SegmentPlayed}
		segment.Status = stages[int(stage)%len(stages)]
		if segment.Status != SegmentPlanned && segment.Status != SegmentGenerating {
			asset := testAsset()
			segment.Asset = &asset
		}
		before := segment
		replacement := testDirection()
		replacement.Action = "responds to the audience"
		// Bound conversion so generated seconds cannot overflow time.Duration.
		freezeAt := time.Duration(freeze%1000) * time.Second
		err = segment.Replan(replacement, freezeAt)
		mutable := (before.Status == SegmentPlanned || before.Status == SegmentGenerating || before.Status == SegmentReady) && freezeAt <= before.Start
		if !mutable {
			if err == nil || !reflect.DeepEqual(before, segment) {
				t.Fatalf("frozen segment changed: before=%+v after=%+v err=%v", before, segment, err)
			}
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		if segment.PlanRevision != before.PlanRevision+1 || segment.Status != SegmentPlanned || segment.Asset != nil || segment.Direction.Action != replacement.Action || segment.Start != before.Start || segment.End != before.End {
			t.Fatalf("invalid replacement: %+v", segment)
		}
	})
}
