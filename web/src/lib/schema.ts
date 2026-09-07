import { z } from "zod"

const nullableText = z.string().nullable()
const nullableNumber = z.number().finite().nullable()
const nonNegative = z.number().finite().nonnegative()

const sessionStatusSchema = z.enum([
  "stopped",
  "starting",
  "buffering",
  "running",
  "stopping",
  "recovering",
  "failed",
])

const segmentStatusSchema = z.enum([
  "planned",
  "generating",
  "ready",
  "committed",
  "playing",
  "played",
])

const generationModeSchema = z.enum(["normal", "high", "critical", "fallback", "pause"])

const streamStatusSchema = z.enum(["stopped", "starting", "live", "recovering", "failed"])

export const opsSnapshotSchema = z
  .object({
    revision: z.number().int().nonnegative(),
    observedAt: z.string().datetime({ offset: true }),
    session: z
      .object({
        id: nullableText,
        status: sessionStatusSchema,
        startedAt: z.string().datetime({ offset: true }).nullable(),
        uptimeSeconds: nonNegative,
        lastError: nullableText,
      })
      .strict(),
    buffer: z
      .object({
        playheadSeconds: nonNegative,
        commitHorizonSeconds: nonNegative,
        readySeconds: nonNegative,
        submittedSeconds: nonNegative,
        readyTargetSeconds: nonNegative,
        submittedTargetSeconds: nonNegative,
      })
      .strict(),
    timeline: z.array(
      z
        .object({
          id: z.string().min(1),
          sequence: z.number().int().nonnegative(),
          startSeconds: nonNegative,
          endSeconds: nonNegative,
          status: segmentStatusSchema,
          source: z.enum(["generated", "fallback"]),
        })
        .strict()
        .refine((segment) => segment.endSeconds > segment.startSeconds, {
          message: "segment endSeconds must be greater than startSeconds",
        }),
    ),
    generation: z
      .object({
        mode: generationModeSchema,
        inFlight: z.number().int().nonnegative(),
        targetConcurrency: z.number().int().nonnegative(),
        latencyP50Seconds: nullableNumber,
        latencyP95Seconds: nullableNumber,
        failuresTotal: z.number().int().nonnegative(),
        failureRate: nonNegative,
        succeededTotal: z.number().int().nonnegative(),
        costCny: nullableNumber,
        costPerLiveHourCny: nullableNumber,
        lastError: nullableText,
      })
      .strict(),
    stream: z
      .object({
        status: streamStatusSchema,
        bitrateKbps: nullableNumber,
        droppedFramesTotal: z.number().int().nonnegative(),
        gapTotal: z.number().int().nonnegative(),
        lastError: nullableText,
      })
      .strict(),
    fallback: z
      .object({
        active: z.boolean(),
        forced: z.boolean(),
        reason: nullableText,
        since: z.string().datetime({ offset: true }).nullable(),
        secondsTotal: nonNegative,
      })
      .strict(),
    controls: z
      .object({
        canStart: z.boolean(),
        canStop: z.boolean(),
        canEnableFallback: z.boolean(),
        canDisableFallback: z.boolean(),
      })
      .strict(),
  })
  .strict()

export const commandResponseSchema = z
  .object({
    command: z.enum(["start", "stop", "fallback", "force-fallback", "mock-danmaku"]),
    status: z.literal("accepted"),
    acceptedAt: z.string().datetime({ offset: true }),
  })
  .strict()

export const apiErrorSchema = z
  .object({
    code: z.string().min(1),
    message: z.string().min(1),
  })
  .strict()

export const generationConfigSchema = z
  .object({
    provider: z.string().min(1),
    baseUrl: z.string().url(),
    model: z.literal("MiniMax-H3-Max"),
    resolution: z.literal("768P"),
    durationSeconds: z.literal(5),
    ratio: z.literal("16:9"),
    apiKeyConfigured: z.boolean(),
    unitPriceCnyPerSecond: nonNegative,
  })
  .strict()

export const bilibiliConfigSchema = z
  .object({
    roomId: z.number().int().nonnegative(),
    cookieConfigured: z.boolean(),
    status: z.enum(["disconnected", "connecting", "connected", "failed"]),
    lastError: nullableText,
  })
  .strict()

export const qwenConfigSchema = z
  .object({
    baseUrl: z.string().url(),
    observerModel: z.literal("qwen3.8-flash"),
    directorModel: z.literal("qwen3.8-max"),
    apiKeyConfigured: z.boolean(),
  })
  .strict()

const flowNodeStatusSchema = z.enum(["pending", "running", "completed", "failed", "skipped"])
const generationModeNameSchema = z.enum([
  "text_to_video",
  "first_frame_to_video",
  "first_last_frame_to_video",
])

export const flowCatalogSchema = z
  .object({
    flows: z.array(
      z
        .object({
          id: z.string().min(1),
          name: z.string().min(1),
          version: z.string().min(1),
          enabled: z.boolean(),
          kind: z.enum(["primary", "modifier"]),
          description: z.string(),
          nodes: z.array(
            z
              .object({ id: z.string().min(1), type: z.string().min(1), label: z.string().min(1) })
              .strict(),
          ),
          edges: z.array(z.object({ from: z.string().min(1), to: z.string().min(1) }).strict()),
        })
        .strict(),
    ),
  })
  .strict()

export const currentFlowSchema = z
  .object({
    flowId: z.string().min(1),
    status: z.enum(["idle", "running", "completed", "failed"]),
    startedAt: z.string().datetime({ offset: true }).nullable(),
    direction: z
      .object({
        currentLabel: z.string(),
        targetLabel: z.string(),
        summary: z.string(),
        horizonSeconds: z.literal(60),
        effectiveInSeconds: nonNegative,
        axes: z.array(
          z
            .object({
              name: z.string().min(1),
              current: z.number().min(0).max(100),
              target: z.number().min(0).max(100),
            })
            .strict(),
        ),
      })
      .strict(),
    audience: z
      .object({
        windowSeconds: nonNegative,
        messageCount: z.number().int().nonnegative(),
        summary: z.string(),
        intents: z.array(
          z.object({ label: z.string().min(1), support: z.number().min(0).max(1) }).strict(),
        ),
      })
      .strict(),
    nodes: z.array(
      z
        .object({
          id: z.string().min(1),
          status: flowNodeStatusSchema,
          durationMs: nullableNumber,
          summary: z.string(),
          costCny: nullableNumber,
        })
        .strict(),
    ),
    beats: z.array(
      z
        .object({
          index: z.number().int().nonnegative(),
          startSeconds: nonNegative,
          endSeconds: nonNegative,
          intent: z.string().min(1),
          mode: generationModeNameSchema,
          anchorFrame: nullableText,
          status: z.enum(["planned", "submitted", "generating", "ready", "locked"]),
        })
        .strict(),
    ),
  })
  .strict()

const historySummarySchema = z
  .object({
    id: z.string().min(1),
    status: z.string().min(1),
    startedAt: z.string().datetime({ offset: true }),
    endedAt: z.string().datetime({ offset: true }),
    segmentTotal: z.number().int().nonnegative(),
    readyTotal: z.number().int().nonnegative(),
    costCny: nonNegative,
  })
  .strict()

const historyDirectionSchema = z
  .object({
    action: z.string(),
    dialogue: z.string(),
    emotion: z.string(),
    camera: z.object({ shot: z.string(), movement: z.string(), angle: z.string() }).strict(),
    continuity: z.object({ notes: z.string(), anchors: z.array(z.string()).optional() }).strict(),
  })
  .strict()

export const sessionHistoryListSchema = z
  .object({ sessions: z.array(historySummarySchema) })
  .strict()
export const sessionHistorySchema = z
  .object({
    session: historySummarySchema,
    segments: z.array(
      z
        .object({
          id: z.string().min(1),
          sequence: z.number().int().nonnegative(),
          startSeconds: nonNegative,
          endSeconds: nonNegative,
          status: z.string().min(1),
          direction: historyDirectionSchema,
          playable: z.boolean(),
        })
        .strict(),
    ),
    observerRuns: z.array(
      z
        .object({
          revision: z.number().int().nonnegative(),
          messages: z.array(z.string()),
          observation: z
            .object({
              summary: z.string(),
              mood: z.string(),
              intents: z.array(z.object({ label: z.string(), support: z.number() }).strict()),
            })
            .strict(),
          error: nullableText,
          createdAt: z.string().datetime({ offset: true }),
        })
        .strict(),
    ),
  })
  .strict()

export type OpsSnapshot = z.infer<typeof opsSnapshotSchema>
export type CommandResponse = z.infer<typeof commandResponseSchema>
export type GenerationConfig = z.infer<typeof generationConfigSchema>
export type BilibiliConfig = z.infer<typeof bilibiliConfigSchema>
export type QwenConfig = z.infer<typeof qwenConfigSchema>
export type FlowCatalog = z.infer<typeof flowCatalogSchema>
export type CurrentFlow = z.infer<typeof currentFlowSchema>
export type SessionHistorySummary = z.infer<typeof historySummarySchema>
export type SessionHistory = z.infer<typeof sessionHistorySchema>
