import { create } from "zustand"
import type { OpsSnapshot } from "@/lib/schema"

type ConnectionState = "connecting" | "open" | "reconnecting" | "closed"

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
  revision: number
  connection: ConnectionState
  dataError: string | null
  history: MetricHistory
  applySnapshot: (snapshot: OpsSnapshot) => void
  setConnection: (connection: ConnectionState) => void
  setDataError: (message: string | null) => void
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
  revision: -1,
  connection: "connecting",
  dataError: null,
  history: emptyHistory,
  applySnapshot: (snapshot) => {
    if (snapshot.revision <= get().revision) return

    const point = (value: number | null): MetricPoint => ({
      observedAt: snapshot.observedAt,
      value,
    })
    set((state) => ({
      revision: snapshot.revision,
      dataError: null,
      history: {
        ready: append(state.history.ready, point(snapshot.buffer.readySeconds)),
        submitted: append(state.history.submitted, point(snapshot.buffer.submittedSeconds)),
        latencyP95: append(state.history.latencyP95, point(snapshot.generation.latencyP95Seconds)),
        bitrate: append(state.history.bitrate, point(snapshot.stream.bitrateKbps)),
        cost: append(state.history.cost, point(snapshot.generation.costCny)),
      },
    }))
  },
  setConnection: (connection) => set({ connection }),
  setDataError: (dataError) => set({ dataError }),
}))
