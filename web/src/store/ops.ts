import { create } from "zustand"
import { sendCommand, type OpsCommand } from "@/lib/api"
import type { OpsSnapshot } from "@/lib/schema"

export type ConnectionState = "connecting" | "open" | "reconnecting" | "closed"

export type MetricPoint = {
  observedAt: string
  value: number | null
}

type MetricHistory = {
  ready: MetricPoint[]
  submitted: MetricPoint[]
  latencyP95: MetricPoint[]
  bitrate: MetricPoint[]
  cost: MetricPoint[]
}

type OpsStore = {
  snapshot: OpsSnapshot | null
  connection: ConnectionState
  pendingCommand: OpsCommand | null
  commandError: string | null
  commandMessage: string | null
  dataError: string | null
  history: MetricHistory
  applySnapshot: (snapshot: OpsSnapshot) => void
  setConnection: (connection: ConnectionState) => void
  setDataError: (message: string | null) => void
  runCommand: (command: OpsCommand) => Promise<void>
}

const HISTORY_LIMIT = 120

export const emptyHistory: MetricHistory = {
  ready: [],
  submitted: [],
  latencyP95: [],
  bitrate: [],
  cost: [],
}

function append(points: MetricPoint[], point: MetricPoint) {
  return [...points, point].slice(-HISTORY_LIMIT)
}

export const useOpsStore = create<OpsStore>((set, get) => ({
  snapshot: null,
  connection: "connecting",
  pendingCommand: null,
  commandError: null,
  commandMessage: null,
  dataError: null,
  history: emptyHistory,
  applySnapshot: (snapshot) => {
    const current = get().snapshot
    if (current && snapshot.revision <= current.revision) return

    const point = (value: number | null): MetricPoint => ({
      observedAt: snapshot.observedAt,
      value,
    })
    set((state) => ({
      snapshot,
      dataError: null,
      history: {
        ready: append(state.history.ready, point(snapshot.buffer.readySeconds)),
        submitted: append(state.history.submitted, point(snapshot.buffer.submittedSeconds)),
        latencyP95: append(
          state.history.latencyP95,
          point(snapshot.generation.latencyP95Seconds),
        ),
        bitrate: append(state.history.bitrate, point(snapshot.stream.bitrateKbps)),
        cost: append(state.history.cost, point(snapshot.generation.costCny)),
      },
    }))
  },
  setConnection: (connection) => set({ connection }),
  setDataError: (dataError) => set({ dataError }),
  runCommand: async (command) => {
    set({ pendingCommand: command, commandError: null, commandMessage: null })
    try {
      const response = await sendCommand(command)
      set({
        pendingCommand: null,
        commandMessage: `命令已接受：${commandLabel(response.command)}`,
      })
    } catch (error) {
      set({
        pendingCommand: null,
        commandError: error instanceof Error ? error.message : "命令执行失败。",
      })
    }
  },
}))

function commandLabel(command: string) {
  return ({ start: "启动", stop: "停止", fallback: "备用画面", "force-fallback": "强制备用画面" } as Record<string, string>)[command] ?? command
}
