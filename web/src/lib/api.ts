import {
  apiErrorSchema,
  commandResponseSchema,
  opsSnapshotSchema,
  generationConfigSchema,
  bilibiliConfigSchema,
  qwenConfigSchema,
  flowCatalogSchema,
  currentFlowSchema,
  sessionHistoryListSchema,
  sessionHistorySchema,
  type CommandResponse,
  type OpsSnapshot,
  type GenerationConfig,
  type BilibiliConfig,
  type QwenConfig,
  type FlowCatalog,
  type CurrentFlow,
  type SessionHistorySummary,
  type SessionHistory,
} from "@/lib/schema"

export type OpsCommand = "start" | "stop" | "enable-fallback" | "disable-fallback"

export async function fetchSnapshot(signal?: AbortSignal): Promise<OpsSnapshot> {
  const response = await fetch("/api/v1/ops/snapshot", {
    headers: { Accept: "application/json" },
    signal,
  })
  return parseResponse(response, (value) => opsSnapshotSchema.parse(value))
}

export async function sendCommand(command: OpsCommand): Promise<CommandResponse> {
  const isFallback = command === "enable-fallback" || command === "disable-fallback"
  const response = await fetch(isFallback ? "/api/v1/ops/fallback" : `/api/v1/ops/${command}`, {
    method: isFallback ? "PUT" : "POST",
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
    },
    body: JSON.stringify(isFallback ? { forced: command === "enable-fallback" } : {}),
  })
  return parseResponse(response, (value) => commandResponseSchema.parse(value))
}

export async function fetchGenerationConfig(signal?: AbortSignal): Promise<GenerationConfig> {
  const response = await fetch("/api/v1/config/generation", {
    headers: { Accept: "application/json" },
    signal,
  })
  return parseResponse(response, (value) => generationConfigSchema.parse(value))
}

export async function updateGenerationConfig(
  config: Pick<GenerationConfig, "model" | "resolution" | "durationSeconds" | "ratio"> & {
    apiKey?: string
  },
): Promise<GenerationConfig> {
  const response = await fetch("/api/v1/config/generation", {
    method: "PUT",
    headers: { Accept: "application/json", "Content-Type": "application/json" },
    body: JSON.stringify(config),
  })
  return parseResponse(response, (value) => generationConfigSchema.parse(value))
}

export async function fetchBilibiliConfig(signal?: AbortSignal): Promise<BilibiliConfig> {
  const response = await fetch("/api/v1/config/bilibili", {
    headers: { Accept: "application/json" },
    signal,
  })
  return parseResponse(response, (value) => bilibiliConfigSchema.parse(value))
}

export async function updateBilibiliConfig(config: {
  roomId: number
  cookie?: string
}): Promise<BilibiliConfig> {
  const response = await fetch("/api/v1/config/bilibili", {
    method: "PUT",
    headers: { Accept: "application/json", "Content-Type": "application/json" },
    body: JSON.stringify(config),
  })
  return parseResponse(response, (value) => bilibiliConfigSchema.parse(value))
}

export async function sendMockDanmaku(message: {
  username: string
  text: string
}): Promise<CommandResponse> {
  const response = await fetch("/api/v1/config/bilibili/mock", {
    method: "POST",
    headers: { Accept: "application/json", "Content-Type": "application/json" },
    body: JSON.stringify(message),
  })
  return parseResponse(response, (value) => commandResponseSchema.parse(value))
}

export async function fetchQwenConfig(signal?: AbortSignal): Promise<QwenConfig> {
  const response = await fetch("/api/v1/config/qwen", {
    headers: { Accept: "application/json" },
    signal,
  })
  return parseResponse(response, (value) => qwenConfigSchema.parse(value))
}

export async function updateQwenConfig(config: {
  baseUrl: string
  apiKey?: string
}): Promise<QwenConfig> {
  const response = await fetch("/api/v1/config/qwen", {
    method: "PUT",
    headers: { Accept: "application/json", "Content-Type": "application/json" },
    body: JSON.stringify(config),
  })
  return parseResponse(response, (value) => qwenConfigSchema.parse(value))
}

export async function fetchFlowCatalog(signal?: AbortSignal): Promise<FlowCatalog> {
  const response = await fetch("/api/v1/flows", { headers: { Accept: "application/json" }, signal })
  return parseResponse(response, (value) => flowCatalogSchema.parse(value))
}

export async function fetchCurrentFlow(signal?: AbortSignal): Promise<CurrentFlow> {
  const response = await fetch("/api/v1/flows/current", {
    headers: { Accept: "application/json" },
    signal,
  })
  return parseResponse(response, (value) => currentFlowSchema.parse(value))
}

export async function fetchSessionHistoryList(
  signal?: AbortSignal,
): Promise<SessionHistorySummary[]> {
  const response = await fetch("/api/v1/sessions", {
    headers: { Accept: "application/json" },
    signal,
  })
  return parseResponse(response, (value) => sessionHistoryListSchema.parse(value).sessions)
}

export async function fetchSessionHistory(
  id: string,
  signal?: AbortSignal,
): Promise<SessionHistory> {
  const response = await fetch(`/api/v1/sessions/${encodeURIComponent(id)}`, {
    headers: { Accept: "application/json" },
    signal,
  })
  return parseResponse(response, (value) => sessionHistorySchema.parse(value))
}

export function openOpsEvents(handlers: {
  onOpen: () => void
  onSnapshot: (snapshot: OpsSnapshot) => void
  onError: (message?: string) => void
}) {
  const events = new EventSource("/api/v1/ops/events")

  events.onopen = handlers.onOpen
  events.onerror = () => handlers.onError()
  events.addEventListener("snapshot", (event) => {
    try {
      handlers.onSnapshot(opsSnapshotSchema.parse(JSON.parse((event as MessageEvent<string>).data)))
    } catch {
      handlers.onError("实时更新数据不符合运行接口约定。")
    }
  })

  return () => events.close()
}

async function parseResponse<T>(response: Response, parse: (value: unknown) => T): Promise<T> {
  const payload: unknown = await response.json()
  if (!response.ok) {
    const error = apiErrorSchema.parse(payload)
    throw new Error(error.message)
  }
  return parse(payload)
}
