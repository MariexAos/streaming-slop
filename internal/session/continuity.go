package session

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"streaming-agent/internal/generation"
	"streaming-agent/internal/live"
)

type continuation struct {
	SegmentID live.SegmentID
	AssetID   live.AssetID
	Frame     string
}

func (r *Runtime) continuation(ctx context.Context, id live.SegmentID) (continuation, error) {
	if !r.config.Continuous {
		return continuation{}, nil
	}
	r.mu.Lock()
	segment, ok := r.session.Timeline.Segment(id)
	if !ok || segment.Sequence == 0 {
		r.mu.Unlock()
		return continuation{}, nil
	}
	previous := r.session.Timeline.Segments[segment.Sequence-1]
	r.mu.Unlock()
	if previous.Asset == nil {
		return continuation{}, errors.New("waiting for preceding clip")
	}
	if previous.Asset.Source == live.AssetSourceFallback {
		return continuation{}, nil
	}
	frames, err := r.config.Frames.Frames(ctx, previous.Asset.URI)
	if err != nil {
		return continuation{}, err
	}
	return continuation{SegmentID: previous.ID, AssetID: previous.Asset.ID, Frame: frames[len(frames)-1]}, nil
}
func (r *Runtime) validContinuation(c continuation) bool {
	if c.AssetID == "" {
		return true
	}
	previous, ok := r.session.Timeline.Segment(c.SegmentID)
	return ok && previous.Asset != nil && previous.Asset.ID == c.AssetID
}
func applyContinuation(request *generation.Request, c continuation) {
	if c.Frame == "" {
		return
	}
	request.ParentSegmentID = c.SegmentID
	request.ParentAssetID = c.AssetID
	request.Spec.AnchorVersion = fmt.Sprintf("%x", sha256.Sum256([]byte(c.Frame)))
	request.Spec.FirstFrame = &generation.AssetRef{ID: string(c.AssetID), URL: c.Frame}
	request.Spec.LastFrame = request.ReferenceFrame
	request.Spec.Mode = generation.ModeFirstFrameToVideo
	if request.Spec.LastFrame != nil {
		request.Spec.Mode = generation.ModeFirstLastFrameToVideo
	}
	request.Spec.Ratio = "adaptive"
}
