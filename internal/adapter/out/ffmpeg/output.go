package ffmpeg

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"streaming-agent/internal/streaming"
)

type OutputConfig struct {
	FFmpegPath string
	RTMPURL    string
}

var errOutputPipe = errors.New("ffmpeg output pipe failed")

type Output struct {
	config OutputConfig

	mu       sync.Mutex
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	cancel   context.CancelFunc
	wait     chan error
	started  bool
	restarts uint64

	statsMu sync.RWMutex
	stats   streaming.Stats
}

func NewOutput(config OutputConfig) (*Output, error) {
	if config.FFmpegPath == "" {
		config.FFmpegPath = "ffmpeg"
	}
	if strings.TrimSpace(config.RTMPURL) == "" {
		return nil, fmt.Errorf("RTMP URL is required")
	}
	return &Output{config: config}, nil
}

func (o *Output) Start(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.cmd != nil {
		return fmt.Errorf("stream output is already running")
	}
	o.restarts = 0
	if err := o.startLocked(); err != nil {
		return err
	}
	o.started = true
	o.statsMu.Lock()
	o.stats.Restarts = 0
	o.statsMu.Unlock()
	return nil
}

func (o *Output) Write(ctx context.Context, segment streaming.Segment) error {
	if segment.Path == "" {
		return fmt.Errorf("write stream segment: path is required")
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.started {
		return fmt.Errorf("write stream segment: output is not started")
	}
	if err := o.ensureRunningLocked(); err != nil {
		return fmt.Errorf("write stream segment: %w", err)
	}
	if err := o.writeFileLocked(ctx, segment.Path); err == nil {
		return nil
	} else if !errors.Is(err, errOutputPipe) {
		return fmt.Errorf("write stream segment: %w", err)
	}
	if err := o.restartLocked(); err != nil {
		return fmt.Errorf("write stream segment: %w", err)
	}
	if err := o.writeFileLocked(ctx, segment.Path); err != nil {
		return fmt.Errorf("write stream segment after restart: %w", err)
	}
	return nil
}

// Restart replaces a failed process at most once for this Output instance.
func (o *Output) Restart(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.started {
		return fmt.Errorf("restart stream output: output is not started")
	}
	return o.restartLocked()
}

func (o *Output) Stop(ctx context.Context) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.started = false
	if o.cmd == nil {
		return nil
	}
	_ = o.stdin.Close()
	select {
	case err := <-o.wait:
		o.clearProcessLocked()
		if err != nil {
			return fmt.Errorf("stop stream output: %w", err)
		}
		return nil
	case <-ctx.Done():
		o.cancel()
		<-o.wait
		o.clearProcessLocked()
		return ctx.Err()
	}
}

func (o *Output) Stats() streaming.Stats {
	o.statsMu.RLock()
	defer o.statsMu.RUnlock()
	return o.stats
}

func (o *Output) startLocked() error {
	processCtx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(processCtx, o.config.FFmpegPath,
		"-hide_banner", "-loglevel", "error", "-nostdin",
		"-re", "-fflags", "+genpts", "-f", "mpegts", "-i", "pipe:0",
		"-map", "0:v:0", "-map", "0:a:0", "-c", "copy",
		"-f", "flv", "-flvflags", "no_duration_filesize",
		"-progress", "pipe:2", o.config.RTMPURL,
	)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("open ffmpeg stdin: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("open ffmpeg progress: %w", err)
	}
	cmd.Stdout = io.Discard
	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("start ffmpeg output: %w", err)
	}
	o.cmd = cmd
	o.stdin = stdin
	o.cancel = cancel
	o.wait = make(chan error, 1)
	go o.readProgress(stderr)
	go func(wait chan<- error) { wait <- cmd.Wait() }(o.wait)
	return nil
}

func (o *Output) ensureRunningLocked() error {
	if o.cmd == nil {
		return o.restartLocked()
	}
	select {
	case <-o.wait:
		o.clearProcessLocked()
		o.setLastError("ffmpeg output exited")
		if err := o.restartLocked(); err != nil {
			return err
		}
		return nil
	default:
		return nil
	}
}

func (o *Output) restartLocked() error {
	if o.restarts >= 1 {
		return fmt.Errorf("ffmpeg restart limit reached")
	}
	if o.cmd != nil {
		_ = o.stdin.Close()
		o.cancel()
		<-o.wait
		o.clearProcessLocked()
	}
	o.restarts++
	o.statsMu.Lock()
	o.stats.Restarts = o.restarts
	o.statsMu.Unlock()
	if err := o.startLocked(); err != nil {
		return fmt.Errorf("restart ffmpeg output: %w", err)
	}
	return nil
}

func (o *Output) writeFileLocked(ctx context.Context, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open normalized segment: %w", err)
	}
	defer func() { _ = file.Close() }()
	buffer := make([]byte, 64<<10)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		read, readErr := file.Read(buffer)
		if read > 0 {
			if _, err := o.stdin.Write(buffer[:read]); err != nil {
				return fmt.Errorf("%w: %w", errOutputPipe, err)
			}
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return fmt.Errorf("read normalized segment: %w", readErr)
		}
	}
}

func (o *Output) readProgress(reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		key, value, ok := strings.Cut(scanner.Text(), "=")
		if !ok {
			continue
		}
		o.statsMu.Lock()
		switch key {
		case "bitrate":
			value = strings.TrimSuffix(value, "kbits/s")
			if parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil {
				o.stats.BitrateKbps = parsed
			}
		case "drop_frames":
			if parsed, err := strconv.ParseUint(value, 10, 64); err == nil {
				o.stats.DroppedFrames = parsed
			}
		case "out_time_us":
			if parsed, err := strconv.ParseInt(value, 10, 64); err == nil {
				o.stats.OutTime = time.Duration(parsed) * time.Microsecond
			}
		}
		o.statsMu.Unlock()
	}
}

func (o *Output) clearProcessLocked() {
	o.cmd = nil
	o.stdin = nil
	o.cancel = nil
	o.wait = nil
}

func (o *Output) setLastError(message string) {
	o.statsMu.Lock()
	o.stats.LastError = message
	o.statsMu.Unlock()
}

var _ streaming.Output = (*Output)(nil)
