package server

import (
	"fmt"
	"streaming-agent/internal/session"

	"streaming-agent/internal/live"
	"streaming-agent/internal/server/flow"
)

func currentFlow(state session.View) flow.FlowRun {
	run := flow.IdleHostDialogueRun()
	applyAudience(&run, state)
	if state.Session == nil {
		return run
	}
	startedAt := state.Session.CreatedAt
	run.StartedAt = &startedAt
	switch state.Session.Status {
	case live.SessionFailed:
		run.Status = flow.RunFailed
	case live.SessionStopped:
		return run
	default:
		run.Status = flow.RunRunning
	}
	segments := state.Session.Timeline.Segments
	if len(segments) == 0 {
		return run
	}
	targetSegment := segments[len(segments)-1]
	for _, segment := range segments {
		if segment.Status == live.SegmentPlanned {
			targetSegment = segment
			break
		}
	}
	target := targetSegment.Direction.Action
	if targetSegment.Direction.Dialogue != "" {
		target = "回应：" + targetSegment.Direction.Dialogue
	}
	run.Direction.CurrentLabel = "自然聊天"
	run.Direction.TargetLabel = target
	run.Direction.Summary = targetSegment.Direction.Action
	for i := range run.Direction.Axes {
		run.Direction.Axes[i].Current = 35
		run.Direction.Axes[i].Target = 55
	}
	if targetSegment.Direction.Dialogue != "" {
		run.Direction.Axes[0].Target = 75
		run.Direction.Axes[1].Target = 65
	}
	for i := 3; i < len(run.Nodes); i++ {
		run.Nodes[i].Status = flow.NodeCompleted
		run.Nodes[i].Summary = "已应用到当前时间线"
	}
	playhead := state.Session.Timeline.Playhead
	for _, segment := range segments {
		if segment.End <= playhead {
			continue
		}
		index := segment.Sequence % len(run.Beats)
		run.Beats[index].Status = beatStatus(segment.Status)
		if run.Direction.EffectiveInSeconds == 0 && segment.Status == live.SegmentPlanned {
			run.Direction.EffectiveInSeconds = (segment.Start - playhead).Seconds()
		}
	}
	return run
}

func applyAudience(run *flow.FlowRun, state session.View) {
	snapshot := state.Audience
	run.Audience.WindowSeconds = snapshot.WindowSeconds
	run.Audience.MessageCount = snapshot.MessageCount
	run.Audience.Summary = snapshot.Summary
	run.Audience.Intents = make([]flow.AudienceIntent, 0, len(snapshot.Intents))
	for _, intent := range snapshot.Intents {
		run.Audience.Intents = append(run.Audience.Intents, flow.AudienceIntent{Label: intent.Label, Support: intent.Support})
	}
	if snapshot.MessageCount == 0 {
		return
	}
	run.Nodes[0].Status = flow.NodeCompleted
	run.Nodes[0].Summary = fmt.Sprintf("最近 %d 秒收到 %d 条弹幕", snapshot.WindowSeconds, snapshot.MessageCount)
	if state.Observation.Summary != "" {
		run.Nodes[1].Status = flow.NodeCompleted
		run.Nodes[1].Summary = state.Observation.Summary
	} else {
		run.Nodes[1].Status = flow.NodePending
		run.Nodes[1].Summary = "等待新互动（无需模型观察）"
	}
	run.Nodes[2].Status = flow.NodeCompleted
	run.Nodes[2].Summary = fmt.Sprintf("合并为 %d 个候选方向", len(snapshot.Intents))
}

func beatStatus(status live.SegmentStatus) flow.BeatStatus {
	switch status {
	case live.SegmentGenerating:
		return flow.BeatGenerating
	case live.SegmentReady:
		return flow.BeatReady
	case live.SegmentCommitted, live.SegmentPlaying, live.SegmentPlayed:
		return flow.BeatLocked
	default:
		return flow.BeatPlanned
	}
}
