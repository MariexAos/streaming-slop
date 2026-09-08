package session

import (
	"fmt"
	"streaming-agent/internal/generation"
)

type StartReadiness struct {
	Ready           bool     `json:"ready"`
	Target          string   `json:"target"`
	CharacterID     string   `json:"characterId"`
	CharacterName   string   `json:"characterName"`
	AvailableMicros int64    `json:"availableMicros"`
	MinimumMicros   int64    `json:"minimumMicros"`
	CredentialSaved bool     `json:"credentialSaved"`
	Blockers        []string `json:"blockers"`
}

// EvaluateStart uses a video-only lower bound; inference still uses atomic reservations.
func EvaluateStart(characterID, name string, credential bool, available int64, price *float64, bufferSeconds float64) StartReadiness {
	r := StartReadiness{CharacterID: characterID, CharacterName: name, CredentialSaved: credential,
		AvailableMicros: max(0, available), Blockers: []string{}}
	if characterID == "" {
		r.Blockers = append(r.Blockers, "请选择有效人物并检查参考图片")
	}
	if !credential {
		r.Blockers = append(r.Blockers, "请保存生成与导演服务的凭据")
	}
	if price == nil || *price <= 0 || bufferSeconds <= 0 {
		r.Blockers = append(r.Blockers, "无法确定启动费用，请检查生成配置")
	} else {
		r.MinimumMicros = generation.Micros(*price * bufferSeconds)
		if r.AvailableMicros < r.MinimumMicros {
			r.Blockers = append(r.Blockers, fmt.Sprintf("启动视频缓冲至少需要 ¥%.2f，当前不足 ¥%.2f（另需推理费用）", float64(r.MinimumMicros)/1e6, float64(r.MinimumMicros-r.AvailableMicros)/1e6))
		}
	}
	r.Ready = len(r.Blockers) == 0
	return r
}
