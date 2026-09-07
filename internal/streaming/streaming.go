package streaming

import (
	"context"
	"time"
)

// PrepareRequest identifies an input asset and the normalized output path.
type PrepareRequest struct {
	SourcePath      string
	DestinationPath string
}

// Segment is a media asset that conforms to the runtime playback contract.
type Segment struct {
	Path      string
	Duration  time.Duration
	Width     int
	Height    int
	FrameRate int
	HasAudio  bool
}

// Preparer turns provider media or a generated safety slate into a Segment.
type Preparer interface {
	Prepare(context.Context, PrepareRequest) (Segment, error)
	GenerateFallback(context.Context, string) (Segment, error)
}

// Stats is the latest progress reported by the stream process.
type Stats struct {
	BitrateKbps   float64
	DroppedFrames uint64
	OutTime       time.Duration
	Restarts      uint64
	LastError     string
}

// Output writes normalized segments to one continuous media destination.
type Output interface {
	Start(context.Context) error
	Write(context.Context, Segment) error
	Stop(context.Context) error
	Stats() Stats
}
