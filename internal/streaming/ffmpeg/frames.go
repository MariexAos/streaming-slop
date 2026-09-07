package ffmpeg

import (
	"context"
	"encoding/base64"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Frames samples the start, midpoint and last decoded frame of a clip.
func (p *Preparer) Frames(ctx context.Context, path string) ([]string, error) {
	duration, err := p.mediaDuration(ctx, path)
	if err != nil {
		return nil, err
	}
	frames := make([]string, 0, 3)
	for _, offset := range []string{"start", "middle", "end"} {
		args := []string{"-v", "error"}
		switch offset {
		case "middle":
			args = append(args, "-ss", fmt.Sprintf("%.3f", duration/2))
		case "end":
			args = append(args, "-sseof", "-1")
		}
		filter := "scale=768:-2"
		if offset == "end" {
			filter = "reverse," + filter
		}
		args = append(args, "-i", path, "-frames:v", "1", "-vf", filter, "-f", "image2pipe", "-vcodec", "png", "-")
		commandCtx, cancel := context.WithTimeout(ctx, p.commandTimeout)
		data, err := exec.CommandContext(commandCtx, p.ffmpegPath, args...).Output()
		cancel()
		if err != nil {
			return nil, fmt.Errorf("extract %s frame: %w", offset, err)
		}
		if len(data) == 0 {
			return nil, fmt.Errorf("empty %s frame", offset)
		}
		frames = append(frames, "data:image/png;base64,"+base64.StdEncoding.EncodeToString(data))
	}
	return frames, nil
}

func (p *Preparer) mediaDuration(ctx context.Context, path string) (float64, error) {
	commandCtx, cancel := context.WithTimeout(ctx, p.commandTimeout)
	defer cancel()
	data, err := exec.CommandContext(commandCtx, p.ffprobePath, "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", path).Output()
	if err != nil {
		return 0, fmt.Errorf("probe media duration: %w", err)
	}
	duration, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
	if err != nil {
		return 0, err
	}
	if duration <= 0 {
		return 0, fmt.Errorf("invalid media duration")
	}
	return duration, nil
}
