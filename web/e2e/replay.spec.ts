import { expect, test } from "@playwright/test"
import { replayFixture, replayVideo } from "./replay.fixture"
import { snapshotFixture } from "../src/lib/snapshot.fixture"

test("plays a session on one timeline with chapter seeking and replay", async ({ page }) => {
  const media = replayVideo()
  await page.route("**/api/v1/**", async (route) => {
    const path = new URL(route.request().url()).pathname
    if (path.includes("/ops/media/")) {
      const range = route.request().headers()["range"]
      const start = range ? Number(range.match(/bytes=(\d+)/)?.[1] ?? 0) : 0
      await route.fulfill({
        status: range ? 206 : 200,
        contentType: "video/webm",
        headers: {
          "Accept-Ranges": "bytes",
          ...(range
            ? { "Content-Range": `bytes ${start}-${media.length - 1}/${media.length}` }
            : {}),
        },
        body: media.subarray(start),
      })
      return
    }
    const responses: Record<string, unknown> = {
      "/api/v1/ops/snapshot": snapshotFixture(),
      "/api/v1/sessions": { sessions: [replayFixture.session] },
      "/api/v1/sessions/replay": replayFixture,
    }
    if (path in responses) await route.fulfill({ json: responses[path] })
    else await route.fulfill({ status: 503, json: { code: "unavailable", message: "暂不可用" } })
  })
  await page.goto("/")
  await page.getByRole("button", { name: "记录", exact: true }).click()
  const video = page.locator("video")
  const progress = page.getByRole("slider", { name: "整场播放进度" })
  await expect(progress).toHaveAttribute("max", "4")
  await expect(page.getByRole("button", { name: "跳到片段 #2", exact: true })).toHaveCount(0)
  await expect
    .poll(() => video.evaluate((element: HTMLVideoElement) => element.readyState))
    .toBeGreaterThanOrEqual(2)
  const previewSize = await video.boundingBox()
  for (const sequence of [3, 1, 3, 1]) {
    await page.getByRole("button", { name: `跳到片段 #${sequence}`, exact: true }).click()
    await expect(video).toHaveAttribute("src", `/api/v1/ops/media/clip-${sequence}`)
    await expect
      .poll(async () => {
        const bounds = await video.boundingBox()
        return { width: bounds?.width, height: bounds?.height }
      })
      .toEqual({ width: previewSize?.width, height: previewSize?.height })
  }
  await progress.fill("3")
  await expect(video).toHaveAttribute("src", "/api/v1/ops/media/clip-3")
  await expect
    .poll(() => video.evaluate((element: HTMLVideoElement) => element.currentTime))
    .toBeCloseTo(1, 1)
  expect(await video.evaluate((element: HTMLVideoElement) => element.paused)).toBe(true)
  await page.getByRole("button", { name: "跳到片段 #1", exact: true }).click()
  await page.getByRole("button", { name: "静音", exact: true }).click()
  await page.getByLabel("播放速度").selectOption("2")
  await page.getByRole("button", { name: "播放回放", exact: true }).click()
  await expect(video).toHaveAttribute("src", "/api/v1/ops/media/clip-3")
  expect(
    await video.evaluate(
      (element: HTMLVideoElement) => element.muted && element.playbackRate === 2,
    ),
  ).toBe(true)
  await expect(progress).toHaveValue("4")
  await page.getByRole("button", { name: "播放回放", exact: true }).click()
  await expect(video).toHaveAttribute("src", "/api/v1/ops/media/clip-1")
  await page.getByRole("button", { name: "暂停回放", exact: true }).click()
  await page.getByRole("button").filter({ hasText: "片段 #3" }).click()
  await expect(video).toHaveAttribute("src", "/api/v1/ops/media/clip-3")
  await expect(progress).toHaveValue("2")
  expect(await video.evaluate((element: HTMLVideoElement) => element.paused)).toBe(true)
})
