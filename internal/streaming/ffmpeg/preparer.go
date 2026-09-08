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
	Duration       time.Duration
}

type Preparer struct {
	ffmpegPath     string
	ffprobePath    string
	commandTimeout time.Duration
	duration       time.Duration
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
	if config.Duration <= 0 {
		config.Duration = segmentDuration
	}
	return &Preparer{
		duration:       config.Duration,
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
	duration := request.Duration
	if duration <= 0 {
		duration = p.duration
	}
	seconds := fmt.Sprintf("%.3f", duration.Seconds())
	hasAudio, err := p.ProbeAudio(ctx, request.SourcePath)
	if err != nil {
		return streaming.Segment{}, err
	}

	args := []string{"-hide_banner", "-loglevel", "error", "-y", "-i", request.SourcePath}
	if request.AudioPath != "" {
		args = append(args, "-i", request.AudioPath)
	} else if !hasAudio {
		args = append(args, "-f", "lavfi", "-t", seconds, "-i", "anullsrc=channel_layout=stereo:sample_rate=48000")
	}
	args = append(args,
		"-map", "0:v:0",
	)
	if hasAudio && request.AudioPath == "" {
		args = append(args, "-map", "0:a:0")
	} else {
		args = append(args, "-map", "1:a:0")
	}
	args = append(args,
		"-vf", "scale=1280:720:force_original_aspect_ratio=decrease:flags=lanczos,pad=1280:720:(ow-iw)/2:(oh-ih)/2,setsar=1,fps=30,trim=duration="+seconds+",setpts=PTS-STARTPTS",
		"-af", "aresample=48000,apad=pad_dur="+seconds+",atrim=duration="+seconds+",asetpts=PTS-STARTPTS",
		"-c:v", "libx264", "-preset", "veryfast", "-pix_fmt", "yuv420p",
		"-r", "30", "-g", "60", "-keyint_min", "60", "-sc_threshold", "0",
		"-c:a", "aac", "-ar", "48000", "-ac", "2",
		"-t", seconds, "-muxdelay", "0", "-muxpreload", "0", "-f", "mpegts",
		request.DestinationPath,
	)
	if err := p.run(ctx, args...); err != nil {
		return streaming.Segment{}, fmt.Errorf("normalize media: %w", err)
	}
	result := normalizedSegment(request.DestinationPath)
	result.Duration = duration
	return result, nil
}

func (p *Preparer) GenerateFallback(ctx context.Context, destinationPath string) (streaming.Segment, error) {
	if destinationPath == "" {
		return streaming.Segment{}, fmt.Errorf("generate fallback: destination path is required")
	}
	seconds := fmt.Sprintf("%.3f", p.duration.Seconds())
	args := []string{
		"-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "color=c=0x111827:s=1280x720:r=30:d=" + seconds,
		"-f", "lavfi", "-i", "anullsrc=channel_layout=stereo:sample_rate=48000:d=" + seconds,
		"-map", "0:v:0", "-map", "1:a:0",
		"-c:v", "libx264", "-preset", "veryfast", "-pix_fmt", "yuv420p",
		"-r", "30", "-g", "60", "-keyint_min", "60", "-sc_threshold", "0",
		"-c:a", "aac", "-ar", "48000", "-ac", "2",
		"-t", seconds, "-muxdelay", "0", "-muxpreload", "0", "-f", "mpegts",
		destinationPath,
	}
	if err := p.run(ctx, args...); err != nil {
		return streaming.Segment{}, fmt.Errorf("generate fallback: %w", err)
	}
	result := normalizedSegment(destinationPath)
	result.Duration = p.duration
	return result, nil
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
