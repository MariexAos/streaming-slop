import { z } from "zod"
import type { OpsSnapshot } from "./schema"

export const readinessSchema = z.object({
  ready: z.boolean(),
  target: z.string(),
  characterId: z.string(),
  characterName: z.string(),
  credentialSaved: z.boolean(),
  availableMicros: z.number(),
  minimumMicros: z.number(),
  blockers: z.array(z.string()),
})
export function journeyState(snapshot: OpsSnapshot) {
  const status = snapshot.session.status
  const titles = {
    stopped: snapshot.session.id ? "直播已结束" : "准备下一场直播",
    starting: "正在准备素材",
    buffering: "正在生成画面并积累缓冲",
    running: "直播进行中",
    recovering: "正在恢复直播",
    stopping: "正在结束直播",
    failed: "直播未能继续",
  }
  const active = ["starting", "buffering", "running", "recovering", "stopping"].includes(status)
  return {
    title: titles[status],
    active,
    preparing: active && status !== "running" && status !== "stopping",
    live: status === "running" && snapshot.stream.status === "live",
    error: snapshot.session.lastError ?? snapshot.stream.lastError ?? snapshot.generation.lastError,
  }
}
