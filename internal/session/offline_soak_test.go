//go:build soak

package session_test

import (
	"os"
	"testing"
	"time"
)

func TestOfflinePostgresFFmpegSoak(t *testing.T) {
	duration, err := time.ParseDuration(os.Getenv("OFFLINE_SOAK_DURATION"))
	if err != nil || duration <= 0 {
		t.Fatal("OFFLINE_SOAK_DURATION must be a positive duration")
	}
	runOfflinePipeline(t, duration)
}
