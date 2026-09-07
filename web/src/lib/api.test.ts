import { afterEach, describe, expect, it, vi } from "vite-plus/test"
import { fetchSnapshot, openOpsEvents } from "./api"
import { snapshotFixture } from "./snapshot.fixture"

afterEach(() => vi.unstubAllGlobals())

describe("API boundary", () => {
  it("validates HTTP responses and forwards cancellation", async () => {
    const fetch = vi
      .fn()
      .mockResolvedValue(Response.json({ ...snapshotFixture(), revision: "invalid" }))
    vi.stubGlobal("fetch", fetch)
    const controller = new AbortController()
    await expect(fetchSnapshot(controller.signal)).rejects.toThrow()
    expect(fetch).toHaveBeenCalledWith(
      "/api/v1/ops/snapshot",
      expect.objectContaining({ signal: controller.signal }),
    )
  })

  it("validates SSE data and closes the connection on cleanup", () => {
    let listener: (event: MessageEvent<string>) => void = () => undefined
    const close = vi.fn()
    vi.stubGlobal(
      "EventSource",
      class {
        onopen: (() => void) | null = null
        onerror: (() => void) | null = null
        close = close
        addEventListener(_name: string, callback: typeof listener) {
          listener = callback
        }
      },
    )
    const onSnapshot = vi.fn()
    const onError = vi.fn()
    const cleanup = openOpsEvents({ onOpen: vi.fn(), onSnapshot, onError })
    listener(new MessageEvent("snapshot", { data: JSON.stringify(snapshotFixture()) }))
    expect(onSnapshot).toHaveBeenCalledOnce()
    listener(new MessageEvent("snapshot", { data: "{}" }))
    expect(onError).toHaveBeenCalledWith("实时更新数据不符合运行接口约定。")
    cleanup()
    expect(close).toHaveBeenCalledOnce()
  })
})
