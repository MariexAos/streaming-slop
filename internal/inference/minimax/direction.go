package minimax

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"streaming-agent/internal/director"
	"streaming-agent/internal/live"
)

// Keep strict domain validation: malformed model output must be corrected,
// not silently truncated or accepted with unknown fields.
func (c *Client) Direct(ctx context.Context, input director.Input) (live.Direction, error) {
	data, err := json.Marshal(input)
	if err != nil {
		return live.Direction{}, err
	}
	schema, err := json.Marshal(director.Schema())
	if err != nil {
		return live.Direction{}, err
	}
	instruction := director.Instruction + ` Available anchors: chat-live-start, chat-live-end.
Return exactly ONE JSON object containing action, dialogue, emotion, camera and continuity.
Do not return a JSON Schema. Do not copy schema keywords such as properties, required or additionalProperties into the result. No Markdown fences, commentary or second JSON value.
Example of the output shape (choose your own content): {"action":"轻轻点头","dialogue":"今天窗边的光挺舒服。","emotion":"calm","camera":{"shot":"medium close-up","movement":"none","angle":"fixed"},"continuity":{"notes":"保持人物与房间一致","anchors":["chat-live-start"]}}
Validation schema (definition only, not output): ` + string(schema)
	parts := []Part{{Type: "text", Text: string(data)}}
	var decodeErr error
	for range 2 {
		result, err := c.complete(ctx, instruction, parts)
		if err != nil {
			return live.Direction{}, err
		}
		value, err := director.DecodeDirection(directionJSON(result))
		if err == nil {
			return value, nil
		}
		decodeErr = err
		parts = []Part{{Type: "text", Text: string(data)}, {Type: "text", Text: "The previous response failed validation: " + err.Error() + ". Generate the requested direction again as one JSON instance, not a schema. Only the six defined fields are allowed."}}
	}
	return live.Direction{}, fmt.Errorf("MiniMax direction invalid after 2 responses: %w", decodeErr)
}

// A single enclosing Markdown fence is presentation, not part of the object.
// Everything inside it still goes through strict, single-value JSON decoding.
func directionJSON(data []byte) []byte {
	text := strings.TrimSpace(string(data))
	for _, prefix := range []string{"```json\n", "```\n"} {
		if strings.HasPrefix(text, prefix) && strings.HasSuffix(text, "\n```") {
			return []byte(strings.TrimSuffix(strings.TrimPrefix(text, prefix), "\n```"))
		}
	}
	return []byte(text)
}
