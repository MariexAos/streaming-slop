package generation

import (
	"context"
	"streaming-agent/internal/live"
)

type Review struct {
	Accepted bool            `json:"accepted"`
	Reason   string          `json:"reason"`
	Observed live.WorldDelta `json:"observed"`
}
type Inspector interface {
	Review(context.Context, Request, []string) (Review, error)
}
type FrameExtractor interface {
	Frames(context.Context, string) ([]string, error)
}
