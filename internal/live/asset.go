package live

import (
	"strings"
	"time"
)

type AssetSource string

const (
	AssetSourceGenerated AssetSource = "generated"
	AssetSourceFallback  AssetSource = "fallback"
)

type VideoAsset struct {
	Observed      WorldDelta    `json:"observed,omitempty"`
	ID            AssetID       `json:"id"`
	AttemptID     AttemptID     `json:"attemptId,omitempty"`
	Source        AssetSource   `json:"source"`
	URI           string        `json:"uri"`
	NormalizedURI string        `json:"normalizedUri"`
	Duration      time.Duration `json:"duration"`
	Width         int           `json:"width"`
	Height        int           `json:"height"`
	FPS           int           `json:"fps"`
	Checksum      string        `json:"checksum,omitempty"`
	VerifiedAt    time.Time     `json:"verifiedAt"`
}

func (a VideoAsset) Playable() bool {
	return a.ID != "" && strings.TrimSpace(a.NormalizedURI) != "" && a.Duration > 0 && !a.VerifiedAt.IsZero()
}
