package ffmpeg

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"streaming-agent/internal/streaming"
)

const segmentDuration = 5 * time.Second

type PreparerConfig struct {
	FFmpegPath     string
	FFprobePath    string
	CommandTimeout time.Duration
}

type Preparer struct {
	ffmpegPath     string
	ffprobePath    string
	commandTimeout time.Duration
}

func NewPreparer(config PreparerConfig) *Preparer {
	if config.FFmpegPath == "" {
		config.FFmpegPath = "ffmpeg"
	}
	if config.FFprobePath == "" {
		config.FFprobePath = "ffprobe"
	}
	if config.CommandTimeout <= 0 {
		config.CommandTimeout = 2 * time.Minute
	}
	return &Preparer{
		ffmpegPath:     config.FFmpegPath,
		ffprobePath:    config.FFprobePath,
		commandTimeout: config.CommandTimeout,
	}
}

func (p *Preparer) ProbeAudio(ctx context.Context, sourcePath string) (bool, error) {
	commandCtx, cancel := context.WithTimeout(ctx, p.commandTimeout)
	defer cancel()
	output, err := exec.CommandContext(commandCtx, p.ffprobePath,
		"-v", "error",
		"-select_streams", "a:0",
		"-show_entries", "stream=index",
		"-of", "csv=p=0",
		sourcePath,
	).Output()
	if err != nil {
		return false, fmt.Errorf("probe input audio: %w", err)
	}
	return strings.TrimSpace(string(output)) != "", nil
}

func (p *Preparer) Prepare(ctx context.Context, request streaming.PrepareRequest) (streaming.Segment, error) {
	if request.SourcePath == "" || request.DestinationPath == "" {
		return streaming.Segment{}, fmt.Errorf("prepare media: source and destination paths are required")
	}
	hasAudio, err := p.ProbeAudio(ctx, request.SourcePath)
	if err != nil {
		return streaming.Segment{}, err
	}

	args := []string{"-hide_banner", "-loglevel", "error", "-y", "-i", request.SourcePath}
	if !hasAudio {
		args = append(args, "-f", "lavfi", "-t", "5", "-i", "anullsrc=channel_layout=stereo:sample_rate=48000")
	}
	args = append(args,
		"-map", "0:v:0",
	)
	if hasAudio {
		args = append(args, "-map", "0:a:0")
	} else {
		args = append(args, "-map", "1:a:0")
	}
	args = append(args,
		"-vf", "scale=1280:720:force_original_aspect_ratio=decrease,pad=1280:720:(ow-iw)/2:(oh-ih)/2,setsar=1,fps=30,trim=duration=5,setpts=PTS-STARTPTS",
		"-af", "aresample=48000,apad=pad_dur=5,atrim=duration=5,asetpts=PTS-STARTPTS",
		"-c:v", "libx264", "-preset", "veryfast", "-pix_fmt", "yuv420p",
		"-r", "30", "-g", "60", "-keyint_min", "60", "-sc_threshold", "0",
		"-c:a", "aac", "-ar", "48000", "-ac", "2",
		"-t", "5", "-muxdelay", "0", "-muxpreload", "0", "-f", "mpegts",
		request.DestinationPath,
	)
	if err := p.run(ctx, args...); err != nil {
		return streaming.Segment{}, fmt.Errorf("normalize media: %w", err)
	}
	return normalizedSegment(request.DestinationPath), nil
}

func (p *Preparer) GenerateFallback(ctx context.Context, destinationPath string) (streaming.Segment, error) {
	if destinationPath == "" {
		return streaming.Segment{}, fmt.Errorf("generate fallback: destination path is required")
	}
	args := []string{
		"-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "color=c=0x111827:s=1280x720:r=30:d=5",
		"-f", "lavfi", "-i", "anullsrc=channel_layout=stereo:sample_rate=48000:d=5",
		"-map", "0:v:0", "-map", "1:a:0",
		"-c:v", "libx264", "-preset", "veryfast", "-pix_fmt", "yuv420p",
		"-r", "30", "-g", "60", "-keyint_min", "60", "-sc_threshold", "0",
		"-c:a", "aac", "-ar", "48000", "-ac", "2",
		"-t", "5", "-muxdelay", "0", "-muxpreload", "0", "-f", "mpegts",
		destinationPath,
	}
	if err := p.run(ctx, args...); err != nil {
		return streaming.Segment{}, fmt.Errorf("generate fallback: %w", err)
	}
	return normalizedSegment(destinationPath), nil
}

func (p *Preparer) run(ctx context.Context, args ...string) error {
	commandCtx, cancel := context.WithTimeout(ctx, p.commandTimeout)
	defer cancel()
	var stderr bytes.Buffer
	cmd := exec.CommandContext(commandCtx, p.ffmpegPath, args...)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func normalizedSegment(path string) streaming.Segment {
	return streaming.Segment{
		Path:      path,
		Duration:  segmentDuration,
		Width:     1280,
		Height:    720,
		FrameRate: 30,
		HasAudio:  true,
	}
}

var _ streaming.Preparer = (*Preparer)(nil)
