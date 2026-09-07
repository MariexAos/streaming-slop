package main

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"streaming-agent/internal/streaming"
	"streaming-agent/internal/streaming/ffmpeg"
)

func testRTMP(ctx context.Context, source, out string, duration time.Duration) error {
	if duration < 10*time.Second || duration > 15*time.Minute {
		return fmt.Errorf("RTMP duration must be 10s–15m")
	}
	media := ffmpeg.NewPreparer(ffmpeg.PreparerConfig{})
	clip, err := media.Prepare(ctx, streaming.PrepareRequest{SourcePath: source, DestinationPath: filepath.Join(out, "clip.ts"), Duration: 5 * time.Second})
	if err != nil {
		return err
	}
	url := "rtmp://127.0.0.1:1935/live/acceptance"
	sender, err := ffmpeg.NewOutput(ffmpeg.OutputConfig{RTMPURL: url})
	if err != nil {
		return err
	}
	if err = sender.Start(ctx); err != nil {
		return err
	}
	defer func() {
		stop, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = sender.Stop(stop)
	}()
	done := make(chan error, 1)
	go func() {
		for range int(duration/(5*time.Second)) + 6 {
			if err := sender.Write(ctx, clip); err != nil {
				done <- err
				return
			}
		}
		done <- sender.Stop(ctx)
	}()
	// Wait for the RTMP publisher to report actual emitted media before subscribing.
	deadline := time.Now().Add(20 * time.Second)
	for sender.Stats().OutTime == 0 {
		if time.Now().After(deadline) {
			return fmt.Errorf("publisher did not start")
		}
		select {
		case err := <-done:
			return fmt.Errorf("publisher ended early: %w", err)
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
	if err := waitPublisher(ctx, url); err != nil {
		return err
	}
	output := filepath.Join(out, "received.flv")
	cmd := exec.CommandContext(ctx, "ffmpeg", "-v", "error", "-y", "-i", url, "-t", strconv.FormatFloat(duration.Seconds(), 'f', 0, 64), "-c", "copy", output)
	if log, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("RTMP receiver: %w: %s", err, log)
	}
	probe := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-show_entries", "format=duration:stream=codec_type,codec_name,duration,width,height", "-of", "json", output)
	data, err := probe.Output()
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return verifyReceived(out, data, duration)
}

func waitPublisher(ctx context.Context, url string) error {
	var last error
	for range 5 {
		probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		data, err := exec.CommandContext(probeCtx, "ffprobe", "-v", "error", "-show_entries", "stream=codec_type", "-of", "csv=p=0", url).Output()
		cancel()
		if err == nil && len(data) > 0 {
			return nil
		}
		last = err
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return fmt.Errorf("RTMP publisher not available: %w", last)
}
