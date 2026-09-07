package session

import (
	"context"
	"streaming-agent/internal/live"
)

type memoryReader interface {
	CharacterMemory(context.Context, string) ([]live.WorldEvent, error)
}

func (r *Runtime) initialWorld(ctx context.Context) (live.WorldState, *live.CharacterProfile, error) {
	world := live.WorldState{
		Character: live.CharacterState{Name: r.config.Character, Goal: r.config.StorySeed, Emotion: "calm"},
		Scene:     live.SceneState{Location: "普通住处的聊天直播角落", Time: "持续直播时段", Lighting: "自然室内光"},
		Camera:    live.CameraState{Shot: "medium close-up", Angle: "slightly low fixed livestream view"},
	}
	if r.config.Catalog == nil {
		return world, nil, nil
	}
	p, anchors, err := r.config.Catalog.Resolve(ctx, "")
	if err != nil {
		return world, nil, err
	}
	r.config.AnchorFrames = anchors
	world.Character.Name = p.Name
	world.Character.Attributes = map[string]string{"appearance": p.Description, "voiceId": p.VoiceID}
	if reader, ok := r.store.(memoryReader); ok {
		world.Recent, err = reader.CharacterMemory(ctx, p.CharacterID)
		if err != nil {
			return world, nil, err
		}
	}
	world.Scene = p.Scene
	world.Camera = p.Camera
	return world, &p, nil
}
