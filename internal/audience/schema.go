package audience

func ObservationSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{
		"summary": map[string]any{"type": "string"}, "mood": map[string]any{"type": "string"},
		"intents": map[string]any{"type": "array", "items": map[string]any{"type": "object", "properties": map[string]any{
			"label": map[string]any{"type": "string"}, "support": map[string]any{"type": "number", "minimum": 0, "maximum": 1},
		}, "required": []string{"label", "support"}, "additionalProperties": false}},
	}, "required": []string{"summary", "mood", "intents"}, "additionalProperties": false}
}
