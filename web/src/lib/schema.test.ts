import { describe, expect, it } from "vitest"
import { bilibiliConfigSchema, currentFlowSchema, flowCatalogSchema, opsSnapshotSchema, qwenConfigSchema } from "@/lib/schema"

export function snapshotFixture(revision = 1) {
  return {
    revision,
    observedAt: "2026-09-01T12:00:00Z",
    session: { id: "session-1", status: "running", startedAt: "2026-09-01T11:59:00Z", uptimeSeconds: 60, lastError: null },
    buffer: { playheadSeconds: 10, commitHorizonSeconds: 30, readySeconds: 48, submittedSeconds: 86, readyTargetSeconds: 45, submittedTargetSeconds: 90 },
    timeline: [{ id: "segment-3", sequence: 3, startSeconds: 10, endSeconds: 15, status: "playing", source: "generated" }],
    generation: { mode: "normal", inFlight: 2, targetConcurrency: 3, latencyP50Seconds: 8, latencyP95Seconds: 12, failuresTotal: 1, failureRate: 0.05, succeededTotal: 3, costCny: 7.5, costPerLiveHourCny: 15, lastError: null },
    stream: { status: "live", bitrateKbps: 3200, droppedFramesTotal: 0, gapTotal: 0, lastError: null },
    fallback: { active: false, forced: false, reason: null, since: null, secondsTotal: 0 },
    controls: { canStart: false, canStop: true, canEnableFallback: true, canDisableFallback: false },
  }
}

describe("opsSnapshotSchema", () => {
  it("accepts the frozen snapshot contract", () => {
    expect(opsSnapshotSchema.parse(snapshotFixture()).revision).toBe(1)
  })

  it("rejects missing and unknown fields", () => {
    const missing = snapshotFixture() as Record<string, unknown>
    delete missing.controls
    expect(opsSnapshotSchema.safeParse(missing).success).toBe(false)
    expect(opsSnapshotSchema.safeParse({ ...snapshotFixture(), extra: true }).success).toBe(false)
  })

  it("rejects unknown statuses", () => {
    expect(opsSnapshotSchema.safeParse({ ...snapshotFixture(), session: { ...snapshotFixture().session, status: "unknown" } }).success).toBe(false)
  })
})

describe("flow schemas", () => {
  it("accepts the observable director loop contract", () => {
    expect(flowCatalogSchema.parse({ flows: [{
      id: "host-dialogue", name: "开放聊天", version: "2", enabled: true,
      kind: "primary", description: "日常互动", nodes: [{ id: "audience", type: "audience-window", label: "弹幕窗口" }], edges: [],
    }]}).flows).toHaveLength(1)
    expect(currentFlowSchema.parse({
      flowId: "host-dialogue", status: "running", startedAt: "2026-09-01T12:00:00Z",
      direction: { currentLabel: "平静聊天", targetLabel: "回应问题", summary: "按观众的选择继续聊。", horizonSeconds: 60, effectiveInSeconds: 45, axes: [{ name: "互动性", current: 30, target: 70 }] },
      audience: { windowSeconds: 20, messageCount: 12, summary: "观众想看看桌上的东西。", intents: [{ label: "展示物品", support: 0.72 }] },
      nodes: [{ id: "audience", status: "completed", durationMs: 12, summary: "收集完成", costCny: null }],
      beats: [{ index: 0, startSeconds: 0, endSeconds: 5, intent: "先接住问题", mode: "first_last_frame_to_video", anchorFrame: "chat-live-start → chat-live-end", status: "planned" }],
    }).direction.horizonSeconds).toBe(60)
  })
})

describe("bilibiliConfigSchema", () => {
  it("accepts only the secret-free connection projection", () => {
    expect(bilibiliConfigSchema.parse({ roomId: 123, cookieConfigured: true, status: "connected", lastError: null }).status).toBe("connected")
    expect(bilibiliConfigSchema.safeParse({ roomId: 123, cookieConfigured: true, status: "connected", lastError: null, cookie: "secret" }).success).toBe(false)
  })
})

describe("qwenConfigSchema", () => {
  it("accepts the fixed observer and director models without a secret", () => {
    expect(qwenConfigSchema.parse({ baseUrl: "https://dashscope.aliyuncs.com/compatible-mode/v1", observerModel: "qwen3.8-flash", directorModel: "qwen3.8-max", apiKeyConfigured: true }).directorModel).toBe("qwen3.8-max")
  })
})
