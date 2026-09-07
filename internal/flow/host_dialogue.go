package flow

import "streaming-agent/internal/generation"

const (
	HostRole                = "开放式聊天频道主播"
	HostDialogueInstruction = "始终以开放式聊天频道主播的身份自然交流。保持人物外观和直播机位连续，但不要预设话题、活动或观众想让主播做什么；根据最近弹幕决定下一分钟，可以聊天、回答、展示物品、起身或切换话题。使用短而具体的口语，允许停顿、犹豫和反问。不要自称 AI，不说宣传文案，不跳出直播语境解释系统。"
)

func HostDialogueFlow() FlowDefinition {
	nodes := []NodeDefinition{
		{ID: "audience-window", Type: "audience_window", Label: "弹幕窗口"},
		{ID: "multimodal-observer", Type: "multimodal_observer", Label: "多模态观察"},
		{ID: "intent-cluster", Type: "intent_cluster", Label: "意图合并"},
		{ID: "dialogue-director", Type: "dialogue_director", Label: "对话导演"},
		{ID: "direction-plan", Type: "direction_plan", Label: "一分钟方向"},
		{ID: "generation-router", Type: "generation_router", Label: "生成模式"},
		{ID: "direction-writer", Type: "direction_writer", Label: "写入时间线"},
	}
	edges := make([]EdgeDefinition, 0, len(nodes)-1)
	for i := 1; i < len(nodes); i++ {
		edges = append(edges, EdgeDefinition{From: nodes[i-1].ID, To: nodes[i].ID})
	}
	return FlowDefinition{
		ID: "host-dialogue", Name: "开放聊天", Version: "2", Enabled: true, Kind: FlowKindPrimary,
		Description: "保持人物与直播机位连续，根据弹幕开放地调整未来一分钟；不预设话题或主播必须做的事情。",
		Nodes:       nodes, Edges: edges,
	}
}

func IdleHostDialogueRun() FlowRun {
	definition := HostDialogueFlow()
	nodes := make([]NodeRun, len(definition.Nodes))
	for i, node := range definition.Nodes {
		nodes[i] = NodeRun{ID: node.ID, Status: NodePending}
	}
	beats := make([]Beat, 12)
	intents := []string{
		"听清最近弹幕在说什么", "合并重复问题和情绪", "挑一个最值得先接的话题",
		"像平常聊天一样直接回应", "按观众意图展示或尝试一件事", "回应一条有代表性的弹幕",
		"自然停顿，观察新的反馈", "顺着观众选择继续深入", "接住追问或情绪变化",
		"需要时自然切换话题", "问一个没有标准答案的问题", "给下一轮弹幕留下空间",
	}
	for i := range beats {
		mode := generation.ModeFirstFrameToVideo
		anchors := [...]string{"chat-live-start → chat-live-end", "chat-live-end", "chat-live-start", "chat-live-end", "", "chat-live-start", "chat-live-start → chat-live-end", "chat-live-end", "chat-live-start", "", "chat-live-end", "chat-live-start"}
		var anchor *string
		if anchors[i] != "" {
			value := anchors[i]
			anchor = &value
		}
		if i == 0 || i == 6 {
			mode = generation.ModeFirstLastFrameToVideo
		}
		if i == 4 || i == 9 {
			mode = generation.ModeTextToVideo
		}
		beats[i] = Beat{
			Index: i, StartSeconds: i * 5, EndSeconds: (i + 1) * 5, Intent: intents[i],
			Mode: mode, AnchorFrame: anchor, Status: BeatPlanned,
		}
	}
	return FlowRun{
		FlowID: definition.ID, Status: RunIdle,
		Direction: DirectionPlan{
			CurrentLabel: "等待弹幕", TargetLabel: "开放回应",
			Summary: "等待观众开口后，再决定未来一分钟聊什么或做什么。", HorizonSeconds: 60,
			Axes: []DirectionAxis{
				{Name: "互动性", Current: 0, Target: 0},
				{Name: "亲近感", Current: 0, Target: 0},
				{Name: "叙事性", Current: 0, Target: 0},
			},
		},
		Audience: AudienceState{WindowSeconds: 20, Intents: []AudienceIntent{}},
		Nodes:    nodes, Beats: beats,
	}
}
