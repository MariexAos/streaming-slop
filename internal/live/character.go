package live

type ReferenceAsset struct {
	ID     string `json:"id"`
	Path   string `json:"path"`
	MIME   string `json:"mime"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// CharacterProfile is an immutable published version, pinned for a session.
type CharacterProfile struct {
	ID          string         `json:"id"`
	CharacterID string         `json:"characterId"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Scene       SceneState     `json:"scene"`
	Camera      CameraState    `json:"camera"`
	FirstFrame  ReferenceAsset `json:"firstFrame"`
	LastFrame   ReferenceAsset `json:"lastFrame"`
	VoiceID     string         `json:"voiceId,omitempty"`
}
