package session

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"os"
	"time"

	"streaming-agent/internal/generation"
	"streaming-agent/internal/live"

	"github.com/google/uuid"
)

// Ignore accounting and timeline identities, but retain all generation inputs.
func reuseKey(request generation.Request) [32]byte {
	request.BudgetID = ""
	request.PriceQuote = nil
	request.UnitPriceCNY = nil
	request.AttemptID, request.SegmentID, request.IdempotencyKey = "", "", ""
	request.ParentAssetID, request.ParentSegmentID = "", ""
	data, _ := json.Marshal(request)
	return sha256.Sum256(data)
}

func (r *Runtime) reusableAsset(request generation.Request) *live.VideoAsset {
	key := reuseKey(request)
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, attempt := range r.attempts {
		if attempt.Status != generation.AttemptSucceeded || reuseKey(attempt.Request) != key {
			continue
		}
		segment, ok := r.session.Timeline.Segment(attempt.SegmentID)
		if !ok || segment.Asset == nil || segment.Asset.AttemptID != attempt.ID {
			continue
		}
		asset := *segment.Asset
		if _, err := os.Stat(asset.NormalizedURI); err != nil {
			continue
		}
		asset.ID = live.AssetID(uuid.NewString())
		asset.AttemptID = request.AttemptID
		asset.VerifiedAt = time.Now().UTC()
		return &asset
	}
	return nil
}

func (r *Runtime) reuseAsset(ctx context.Context, sessionID live.SessionID, request generation.Request, asset live.VideoAsset) {
	for _, status := range []generation.AttemptStatus{generation.AttemptSubmitted, generation.AttemptRunning, generation.AttemptPreparing} {
		r.transitionAttempt(ctx, sessionID, request.AttemptID, status)
	}
	zero := 0.0
	r.acceptAsset(ctx, sessionID, request.AttemptID, generation.Result{CostCNY: &zero}, asset)
}
