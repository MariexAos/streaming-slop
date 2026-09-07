import { beforeEach, describe, expect, it } from "vite-plus/test"
import { opsSnapshotSchema } from "@/lib/schema"
import { snapshotFixture } from "@/lib/snapshot.fixture"
import { emptyHistory, useOpsStore } from "@/store/ops"

describe("ops store", () => {
  beforeEach(() => {
    useOpsStore.setState({
      snapshot: null,
      history: emptyHistory,
      commandError: null,
      commandMessage: null,
      dataError: null,
    })
  })

  it("ignores stale revisions", () => {
    useOpsStore.getState().applySnapshot(opsSnapshotSchema.parse(snapshotFixture(2)))
    useOpsStore.getState().applySnapshot(opsSnapshotSchema.parse(snapshotFixture(1)))
    expect(useOpsStore.getState().snapshot?.revision).toBe(2)
    expect(useOpsStore.getState().history.ready).toHaveLength(1)
  })

  it("caps every metric history at 120 snapshots", () => {
    for (let revision = 1; revision <= 125; revision += 1) {
      useOpsStore.getState().applySnapshot(
        opsSnapshotSchema.parse({
          ...snapshotFixture(revision),
          observedAt: new Date(Date.UTC(2026, 8, 1, 12, 0, revision)).toISOString(),
        }),
      )
    }
    const history = useOpsStore.getState().history
    expect(history.ready).toHaveLength(120)
    expect(history.submitted).toHaveLength(120)
    expect(history.latencyP95).toHaveLength(120)
    expect(history.bitrate).toHaveLength(120)
  })
})
