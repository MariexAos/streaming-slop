package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

func verifyReceived(out string, data []byte, expected time.Duration) error {
	var result struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
		Streams []struct {
			Type string `json:"codec_type"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return err
	}
	duration, err := strconv.ParseFloat(result.Format.Duration, 64)
	if err != nil {
		return err
	}
	types := map[string]bool{}
	for _, stream := range result.Streams {
		types[stream.Type] = true
	}
	if duration < expected.Seconds()-1 || !types["audio"] || !types["video"] {
		return fmt.Errorf("incomplete RTMP recording: %.3fs, streams=%v", duration, types)
	}
	return saveJSON(out, "rtmp.json", result)
}
