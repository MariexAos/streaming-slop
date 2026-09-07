import { beforeEach, expect, it, vi } from "vite-plus/test"
import { QueryObserver } from "@tanstack/react-query"
import { opsSnapshotSchema, type OpsSnapshot } from "@/lib/schema"
import { snapshotFixture } from "@/lib/snapshot.fixture"
import { emptyHistory, useOpsStore } from "@/store/ops"
import { createQueryClient, snapshotKey } from "./client"
import { acceptSnapshot, snapshotOptions } from "./ops"

beforeEach(() => {
  useOpsStore.setState({ revision: -1, history: emptyHistory })
})

it("does not let a delayed HTTP snapshot overwrite a newer SSE revision", async () => {
  const client = createQueryClient()
  let resolve: ((response: Response) => void) | undefined
  const response = new Promise<Response>((done) => {
    resolve = done
  })
  const fetchMock = vi.spyOn(globalThis, "fetch").mockReturnValue(response)
  try {
    const request = client.fetchQuery(snapshotOptions(client))
    acceptSnapshot(client, opsSnapshotSchema.parse(snapshotFixture(3)))
    resolve?.(new Response(JSON.stringify(snapshotFixture(1)), { status: 200 }))
    await request
    expect(client.getQueryData<OpsSnapshot>(snapshotKey)?.revision).toBe(3)
    expect(useOpsStore.getState().history.ready).toHaveLength(1)
  } finally {
    fetchMock.mockRestore()
    client.clear()
  }
})

it("preserves unchanged selected data when other snapshot fields change", () => {
  const client = createQueryClient()
  acceptSnapshot(client, opsSnapshotSchema.parse(snapshotFixture(1)))
  const observer = new QueryObserver(client, {
    ...snapshotOptions(client),
    select: (snapshot) => snapshot.controls,
    notifyOnChangeProps: ["data"],
  })
  const notified = vi.fn()
  const unsubscribe = observer.subscribe(notified)
  acceptSnapshot(client, opsSnapshotSchema.parse(snapshotFixture(2)))
  expect(notified).not.toHaveBeenCalled()
  expect(useOpsStore.getState().history.ready).toHaveLength(2)
  unsubscribe()
  client.clear()
})
