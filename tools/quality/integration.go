package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// checkIntegration requires the actual database and media tests, not just green packages.
func checkIntegration(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	return integrationResult(file)
}

func integrationResult(input io.Reader) error {
	required := map[string]bool{
		"streaming-agent/internal/store/TestStoreMigrationRecoveryAndAtomicReady":            false,
		"streaming-agent/internal/streaming/ffmpeg/TestPrepareAddsAudioAndNormalizesVideo":   false,
		"streaming-agent/internal/streaming/ffmpeg/TestGenerateFallbackMatchesMediaContract": false,
		"streaming-agent/internal/session/TestOfflinePostgresFFmpeg":                         false,
	}
	decoder := json.NewDecoder(input)
	for {
		var event struct{ Action, Package, Test string }
		if err := decoder.Decode(&event); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("read test report: %w", err)
		}
		if event.Action == "skip" || event.Action == "fail" {
			return fmt.Errorf("integration %s: %s/%s", event.Action, event.Package, event.Test)
		}
		key := event.Package + "/" + event.Test
		if _, exists := required[key]; exists && event.Action == "pass" {
			required[key] = true
		}
	}
	for test, passed := range required {
		if !passed {
			return fmt.Errorf("required integration test did not pass: %s", test)
		}
	}
	return nil
}
