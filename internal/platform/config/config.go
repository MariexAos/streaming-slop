package config

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

const HTTPAddr = "127.0.0.1:8080"

type Config struct {
	DatabaseURL              string
	AnthropicAPIKey          string
	AnthropicModel           string
	AnthropicBaseURL         string
	QwenAPIKey               string
	QwenBaseURL              string
	QwenObserverModel        string
	QwenDirectorModel        string
	MiniMaxAPIKey            string
	MiniMaxBaseURL           string
	MiniMaxModel             string
	MiniMaxResolution        string
	MiniMaxRatio             string
	BilibiliRoomID           int
	BilibiliCookie           string
	GenerationMaxConcurrency int
	RTMPURL                  string
	StorySeed                string
	Character                string
	ReferenceImageURLs       []string
	DataDir                  string
	FFmpegPath               string
	FFprobePath              string
	SegmentDuration          time.Duration
	CommitHorizon            time.Duration
	ReadyTarget              time.Duration
	SubmittedTarget          time.Duration
	PlannedHorizon           time.Duration
	SchedulerInterval        time.Duration
	GenerationPoll           time.Duration
	InitialConcurrency       int
	GenerationAttempts       int
	FallbackExitReady        time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		DatabaseURL:              strings.TrimSpace(os.Getenv("DATABASE_URL")),
		AnthropicAPIKey:          strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY")),
		AnthropicModel:           envOr("ANTHROPIC_MODEL", "claude-haiku-4-5-20251001"),
		AnthropicBaseURL:         envOr("ANTHROPIC_BASE_URL", "https://api.anthropic.com"),
		QwenAPIKey:               strings.TrimSpace(os.Getenv("DASHSCOPE_API_KEY")),
		QwenBaseURL:              envOr("QWEN_BASE_URL", "https://dashscope.aliyuncs.com/compatible-mode/v1"),
		QwenObserverModel:        envOr("QWEN_OBSERVER_MODEL", "qwen3.8-flash"),
		QwenDirectorModel:        envOr("QWEN_DIRECTOR_MODEL", "qwen3.8-max"),
		MiniMaxAPIKey:            strings.TrimSpace(os.Getenv("MINIMAX_API_KEY")),
		MiniMaxBaseURL:           envOr("MINIMAX_BASE_URL", "https://api.minimaxi.com"),
		MiniMaxModel:             envOr("MINIMAX_MODEL", "MiniMax-H3-Max"),
		MiniMaxResolution:        envOr("MINIMAX_RESOLUTION", "768P"),
		MiniMaxRatio:             envOr("MINIMAX_RATIO", "16:9"),
		BilibiliRoomID:           envInt("BILIBILI_ROOM_ID", 0),
		BilibiliCookie:           strings.TrimSpace(os.Getenv("BILIBILI_COOKIE")),
		GenerationMaxConcurrency: envInt("MINIMAX_MAX_CONCURRENCY", 4),
		RTMPURL:                  strings.TrimSpace(os.Getenv("RTMP_URL")),
		StorySeed:                strings.TrimSpace(os.Getenv("STORY_SEED")),
		Character:                envOr("CHARACTER_DESCRIPTION", "长深色头发、穿宽松白色针织上衣的成年女性聊天主播；自然皮肤与普通生活感，固定低角度横屏直播机位"),
		ReferenceImageURLs:       splitCSV(os.Getenv("REFERENCE_IMAGE_URLS")),
		DataDir:                  envOr("DATA_DIR", "./data"),
		FFmpegPath:               envOr("FFMPEG_PATH", "ffmpeg"),
		FFprobePath:              envOr("FFPROBE_PATH", "ffprobe"),
		SegmentDuration:          5 * time.Second,
		CommitHorizon:            30 * time.Second,
		ReadyTarget:              45 * time.Second,
		SubmittedTarget:          90 * time.Second,
		PlannedHorizon:           180 * time.Second,
		SchedulerInterval:        time.Second,
		GenerationPoll:           2 * time.Second,
		InitialConcurrency:       3,
		GenerationAttempts:       2,
		FallbackExitReady:        30 * time.Second,
	}

	var missing []string
	for name, value := range map[string]string{
		"DATABASE_URL": cfg.DatabaseURL,
		"RTMP_URL":     cfg.RTMPURL,
		"STORY_SEED":   cfg.StorySeed,
	} {
		if value == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return Config{}, errors.New("missing required environment: " + strings.Join(missing, ", "))
	}
	if cfg.GenerationMaxConcurrency < 1 {
		return Config{}, errors.New("MINIMAX_MAX_CONCURRENCY must be positive")
	}
	return cfg, nil
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func envInt(name string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func splitCSV(value string) []string {
	var values []string
	for item := range strings.SplitSeq(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			values = append(values, item)
		}
	}
	return values
}
