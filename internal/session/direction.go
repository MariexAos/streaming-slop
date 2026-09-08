package session

import (
	"context"
	"strings"
	"time"
	"unicode"

	"streaming-agent/internal/director"
	"streaming-agent/internal/live"
)

func (r *Runtime) direct(ctx context.Context, input director.Input) (live.Direction, error) {
	pause := r.conversationContext(&input)
	if pause {
		input.Guidance += " 本段为停顿：dialogue 必须为空；继续执行当前可见动作或观众指定的动作；不说话，不安排看屏幕、看弹幕、读弹幕或机械点头。"
	} else {
		input.Guidance += " 优先用画面和动作回应用户，不要先念出或解释弹幕。本段可以说一句简短口语，5 秒约 12—20 个汉字；没有新内容可不说。不要复述 recentDialogue。"
	}
	for range 2 {
		value, err := r.director.Direct(ctx, input)
		if err != nil {
			return live.Direction{}, err
		}
		if pause && strings.TrimSpace(value.Dialogue) != "" {
			input.Guidance += " 上次仍输出台词，请改为空字符串并取消说话动作。"
			continue
		}
		if repeatedDialogue(value.Dialogue, input.RecentDialogue) {
			input.Guidance += " 上次台词重复了近期内容。补充真正的新信息，或保持安静。"
			continue
		}
		return value, nil
	}
	// A quiet beat is preferable to forcing another sentence or spending on
	// repeated correction requests.
	return director.IdleDirection(), nil
}

func (r *Runtime) conversationContext(input *director.Input) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	pause := false
	if r.session != nil {
		for _, segment := range r.session.Timeline.Segments {
			if segment.Start >= input.Position || segment.End < input.Position-60*time.Second {
				continue
			}
			if text := strings.TrimSpace(segment.Direction.Dialogue); text != "" {
				input.RecentDialogue = append(input.RecentDialogue, text)
				pause = pause || segment.Start > input.Position-15*time.Second
			}
		}
	}
	if len(input.RecentDialogue) == 0 && input.Previous != nil && strings.TrimSpace(input.Previous.Dialogue) != "" {
		input.RecentDialogue = append(input.RecentDialogue, input.Previous.Dialogue)
		pause = true
	}
	return pause
}

func repeatedDialogue(text string, recent []string) bool {
	normalize := func(text string) string {
		return strings.Map(func(r rune) rune {
			if unicode.IsSpace(r) || unicode.IsPunct(r) {
				return -1
			}
			return unicode.ToLower(r)
		}, text)
	}
	text = normalize(text)
	if text == "" {
		return false
	}
	for _, previous := range recent {
		if text == normalize(previous) {
			return true
		}
	}
	return false
}
