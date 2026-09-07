package live

import "time"

type CharacterState struct {
	Name       string            `json:"name"`
	Goal       string            `json:"goal"`
	Emotion    string            `json:"emotion"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

type SceneState struct {
	Location string `json:"location"`
	Time     string `json:"time"`
	Lighting string `json:"lighting"`
}

type CameraState struct {
	Shot  string `json:"shot"`
	Angle string `json:"angle"`
}

type StoryState struct {
	Beat  string `json:"beat"`
	Notes string `json:"notes"`
}

type WorldEvent struct {
	At      time.Duration     `json:"at"`
	Kind    string            `json:"kind"`
	Summary string            `json:"summary"`
	Facts   map[string]string `json:"facts,omitempty"`
}

type WorldState struct {
	Character CharacterState `json:"character"`
	Scene     SceneState     `json:"scene"`
	Camera    CameraState    `json:"camera"`
	Story     StoryState     `json:"story"`
	Recent    []WorldEvent   `json:"recent,omitempty"`
}

type WorldDelta struct {
	Character *CharacterState `json:"character,omitempty"`
	Scene     *SceneState     `json:"scene,omitempty"`
	Camera    *CameraState    `json:"camera,omitempty"`
	Story     *StoryState     `json:"story,omitempty"`
	Events    []WorldEvent    `json:"events,omitempty"`
}

func (w *WorldState) Apply(delta WorldDelta, recentLimit int) {
	if delta.Character != nil {
		w.Character = *delta.Character
	}
	if delta.Scene != nil {
		w.Scene = *delta.Scene
	}
	if delta.Camera != nil {
		w.Camera = *delta.Camera
	}
	if delta.Story != nil {
		w.Story = *delta.Story
	}
	w.Recent = append(w.Recent, delta.Events...)
	if recentLimit > 0 && len(w.Recent) > recentLimit {
		w.Recent = append([]WorldEvent(nil), w.Recent[len(w.Recent)-recentLimit:]...)
	}
}
