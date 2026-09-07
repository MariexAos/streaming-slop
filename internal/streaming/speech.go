package streaming

import (
	"context"
	"time"
)

type Speech interface {
	Synthesize(context.Context, string, string, time.Duration) ([]byte, error)
}
