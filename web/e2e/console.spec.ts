import { expect, test, type Page } from "@playwright/test"
import { snapshotFixture } from "../src/lib/snapshot.fixture"
import { servicesFixture } from "../src/lib/services.fixture"

const generation = {
  provider: "minimax",
  baseUrl: "https://example.com",
  model: "MiniMax-H3-Max",
  resolution: "768P",
  durationSeconds: 5,
  ratio: "16:9",
  apiKeyConfigured: true,
  unitPriceCnyPerSecond: 0.1,
}
const summary = (id: string) => ({
  id,
  status: "STOPPED",
  startedAt: "2026-09-01T12:00:00Z",
  endedAt: "2026-09-01T12:01:00Z",
  segmentTotal: 0,
  readyTotal: 0,
  costCny: 0,
})
const detail = (id: string) => ({
  session: summary(id),
  segments: [],
  observerRuns: [
    {
      revision: 1,
      messages: [],
      observation: { summary: `观察-${id}`, mood: "calm", intents: [] },
      error: null,
      createdAt: "2026-09-01T12:00:00Z",
    },
  ],
})

async function mockAPI(page: Page) {
  await page.route("**/live-preview/**", (route) =>
    route.fulfill({ contentType: "text/html", body: "<p>测试播放器</p>" }),
  )
  await page.route("**/api/v1/**", async (route) => {
    const path = new URL(route.request().url()).pathname
    if (path === "/api/v1/ops/events") {
      await route.fulfill({ contentType: "text/event-stream", body: ": connected\n\n" })
      return
    }
    const responses: Record<string, unknown> = {
      "/api/v1/ops/snapshot": snapshotFixture(),
      "/api/v1/ops/readiness": {
        ready: true,
        target: "本地测试",
        characterId: "host",
        characterName: "主播",
        credentialSaved: true,
        availableMicros: 30000000,
        minimumMicros: 22500000,
        blockers: [],
      },
      "/api/v1/config/generation": generation,
      "/api/v1/config/services": servicesFixture(),
      "/api/v1/budget": {
        limitMicros: 10000000,
        nextLimitMicros: 10000000,
        chargedMicros: 0,
        reservedMicros: 0,
      },
      "/api/v1/config/bilibili": {
        roomId: 123,
        cookieConfigured: true,
        status: "connected",
        lastError: null,
      },
      "/api/v1/config/qwen": {
        baseUrl: "https://example.com",
        observerModel: "qwen3.8-flash",
        directorModel: "qwen3.8-max",
        apiKeyConfigured: true,
      },
      "/api/v1/flows": { flows: [] },
      "/api/v1/sessions": { sessions: [summary("first"), summary("second")] },
      "/api/v1/sessions/first": detail("first"),
      "/api/v1/sessions/second": detail("second"),
    }
    if (path in responses) await route.fulfill({ json: responses[path] })
    else await route.fulfill({ status: 503, json: { code: "unavailable", message: "暂不可用" } })
  })
}

test.beforeEach(async ({ page }) => {
  await mockAPI(page)
})

test("loads the console and reports initialization failures", async ({ page }) => {
  await page.goto("/")
  await expect(page.getByRole("heading", { name: "Streaming Slop" })).toBeVisible()
  await page.route("**/ops/snapshot", (route) =>
    route.fulfill({ status: 503, json: { code: "offline", message: "服务暂不可用" } }),
  )
  await page.reload()
  await expect(page.getByText("服务暂不可用", { exact: true })).toBeVisible()
})

test("applies SSE snapshots and rejects stale revisions", async ({ page }) => {
  await page.route("**/ops/events", (route) =>
    route.fulfill({
      contentType: "text/event-stream",
      body: `event: snapshot\ndata: ${JSON.stringify(snapshotFixture(3))}\n\nevent: snapshot\ndata: ${JSON.stringify(snapshotFixture(2))}\n\n`,
    }),
  )
  await page.goto("/")
  await page.getByText("运行详情 · 缓冲、时间轴与推流状态", { exact: true }).click()
  await expect(page.getByText(/session-1 · 版本 3/)).toBeVisible()
})

test("disables pending commands and reports rejection", async ({ page }) => {
  let finish: (() => void) | undefined
  const pending = new Promise<void>((resolve) => {
    finish = resolve
  })
  await page.route("**/ops/stop", async (route) => {
    await pending
    await route.fulfill({ status: 409, json: { code: "conflict", message: "停止失败，请重试" } })
  })
  await page.goto("/")
  await page.getByRole("button", { name: "结束直播", exact: true }).click()
  await page.getByRole("button", { name: "停止会话", exact: true }).click()
  await expect(page.getByRole("button", { name: "结束直播", exact: true })).toBeDisabled()
  finish?.()
  await expect(page.getByText("停止失败，请重试", { exact: true })).toBeVisible()
  await expect(page.getByRole("button", { name: "结束直播", exact: true })).toBeEnabled()
})

test("retains configuration input after failure and clears the key after success", async ({
  page,
}) => {
  let fail = true
  await page.route("**/config/qwen", async (route) => {
    if (route.request().method() !== "PUT") return route.fallback()
    const input = route.request().postDataJSON() as { baseUrl: string; apiKey: string }
    expect(input.apiKey).toBe("test-key")
    if (fail) await route.fulfill({ status: 503, json: { code: "failed", message: "保存失败" } })
    else
      await route.fulfill({
        json: {
          baseUrl: input.baseUrl,
          observerModel: "qwen3.8-flash",
          directorModel: "qwen3.8-max",
          apiKeyConfigured: true,
        },
      })
  })
  await page.goto("/")
  await page.getByRole("button", { name: "配置", exact: true }).click()
  await page.getByText("Qwen 备用配置（高级）", { exact: true }).click()
  const base = page.getByLabel("OpenAI 兼容 Base URL")
  await base.fill("https://new.example.com")
  const key = page.getByLabel("Qwen API Key", { exact: true })
  await key.fill("test-key")
  await page.getByRole("button", { name: "保存 Qwen 配置" }).click()
  await expect(page.getByText("保存失败", { exact: true })).toBeVisible()
  await expect(key).toHaveValue("test-key")
  await expect(base).toHaveValue("https://new.example.com")
  fail = false
  await page.getByRole("button", { name: "保存 Qwen 配置" }).click()
  await expect(key).toHaveValue("")
})

test("does not show previous session details while switching", async ({ page }) => {
  let finish: (() => void) | undefined
  const pending = new Promise<void>((resolve) => {
    finish = resolve
  })
  await page.route("**/sessions/second", async (route) => {
    await pending
    await route.fulfill({ json: detail("second") })
  })
  await page.goto("/")
  await page.getByRole("button", { name: "记录", exact: true }).click()
  await expect(page.getByText("观察-first", { exact: true })).toBeVisible()
  await page.getByRole("button").filter({ hasText: "second" }).click()
  await expect(page.getByText("观察-first", { exact: true })).not.toBeVisible()
  finish?.()
  await expect(page.getByText("观察-second", { exact: true })).toBeVisible()
})

test("configuration failure does not block live operations", async ({ page }) => {
  await page.route("**/config/services", (route) =>
    route.fulfill({ status: 503, json: { code: "offline", message: "生成配置暂不可用" } }),
  )
  await page.goto("/")
  await expect(page.getByRole("heading", { name: "Streaming Slop" })).toBeVisible()
  await page.getByRole("button", { name: "配置", exact: true }).click()
  await expect(page.getByText("生成配置暂不可用", { exact: true })).toBeVisible()
})

test("background room refresh preserves an edited draft", async ({ page }) => {
  await page.goto("/")
  await page.getByRole("button", { name: "配置", exact: true }).click()
  await page.getByText("Bilibili 弹幕接入（可选）", { exact: true }).click()
  const room = page.getByLabel("直播间房间号")
  await expect(room).toHaveValue("123")
  await room.fill("789")
  await page.route("**/config/bilibili", (route) =>
    route.fulfill({
      json: { roomId: 456, cookieConfigured: true, status: "connected", lastError: null },
    }),
  )
  await page.waitForResponse("**/config/bilibili")
  await expect(room).toHaveValue("789")
})

test("switches persisted character versions and binds unknown submissions", async ({ page }) => {
  const first = {
    id: "version-1",
    characterId: "host",
    name: "主播",
    description: "white cardigan",
    firstFrame: { id: "a", width: 1280, height: 720 },
    lastFrame: { id: "b", width: 1280, height: 720 },
    scene: { location: "room", time: "day", lighting: "natural" },
    camera: { shot: "medium", angle: "fixed" },
  }
  const second = { ...first, id: "version-2" }
  let current = first
  let bound = false
  await page.route("**/api/v1/characters", (route) => route.fulfill({ json: [first, second] }))
  await page.route("**/api/v1/characters/current", async (route) => {
    if (route.request().method() === "PUT") {
      expect(route.request().postDataJSON()).toEqual({ versionId: "version-2" })
      current = second
      await route.fulfill({ status: 204 })
    } else await route.fulfill({ json: current })
  })
  await page.route("**/api/v1/budget", (route) =>
    route.fulfill({
      json: {
        limitMicros: 10000000,
        nextLimitMicros: 10000000,
        chargedMicros: 2500000,
        reservedMicros: 2500000,
      },
    }),
  )
  await page.route("**/api/v1/attempts/unresolved", (route) =>
    route.fulfill({
      json: bound
        ? []
        : [{ id: "attempt-1", provider: "minimax", jobId: "", status: "PENDING_SUBMIT" }],
    }),
  )
  await page.route("**/api/v1/attempts/attempt-1/job", async (route) => {
    expect(route.request().postDataJSON()).toEqual({ jobId: "provider-job" })
    bound = true
    await route.fulfill({ status: 204 })
  })
  await page.goto("/")
  await page.getByRole("button", { name: "配置", exact: true }).click()
  await page.getByLabel("人物版本").selectOption("version-2")
  await expect(page.getByLabel("人物版本")).toHaveValue("version-2")
  await expect(page.getByText(/在途预占 ¥2.5000/)).toBeVisible()
  await page.getByLabel("任务编号 attempt-1").fill("provider-job")
  await page.getByRole("button", { name: "绑定任务", exact: true }).click()
  await expect(page.getByLabel("任务编号 attempt-1")).toHaveCount(0)
})

test("keeps preparation and keyboard navigation usable on a narrow screen in dark OS mode", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await page.emulateMedia({ colorScheme: "dark" })
  await page.goto("/")
  await expect(page.locator("body")).toHaveCSS("background-color", "rgb(255, 255, 255)")
  await expect(page.getByRole("region", { name: "开播准备" })).toBeVisible()
  await expect(page.getByRole("heading", { name: "直播安全余量" })).not.toBeVisible()
  await page.getByText("运行详情 · 缓冲、时间轴与推流状态", { exact: true }).press("Enter")
  await expect(page.getByRole("heading", { name: "直播安全余量" })).toBeVisible()
  await page.getByRole("button", { name: "配置", exact: true }).click()
  await expect(page.getByRole("heading", { name: "为下一场直播做好准备" })).toBeVisible()
  await page.getByRole("button", { name: "运行", exact: true }).click()
  await expect(page.getByRole("region", { name: "开播准备" })).toBeVisible()
  expect(await page.locator("body").evaluate((body) => body.scrollWidth <= window.innerWidth)).toBe(
    true,
  )
})

test("start preparation stays on the page and does not claim to be live", async ({ page }) => {
  let current = {
    ...snapshotFixture(),
    session: { ...snapshotFixture().session, status: "stopped" },
    controls: { ...snapshotFixture().controls, canStart: true, canStop: false },
  }
  await page.route("**/ops/snapshot", (route) => route.fulfill({ json: current }))
  await page.route("**/ops/start", async (route) => {
    current = {
      ...current,
      revision: 2,
      session: { ...current.session, status: "buffering" },
      controls: { ...current.controls, canStart: false, canStop: true },
    }
    await route.fulfill({
      json: { command: "start", status: "accepted", acceptedAt: "2026-09-08T00:00:00Z" },
    })
  })
  await page.goto("/")
  await expect(page.getByRole("button", { name: "开始直播", exact: true })).toBeEnabled()
  await page.getByRole("button", { name: "修改开播配置" }).click()
  await expect(page.getByRole("heading", { name: "为下一场直播做好准备" })).toBeVisible()
  await page.getByRole("button", { name: "返回开播摘要" }).click()
  await page.getByRole("button", { name: "开始直播", exact: true }).click()
  await expect(page.getByRole("heading", { name: "正在生成画面并积累缓冲" })).toBeVisible()
  await expect(page.getByRole("button", { name: "停止准备" })).toBeVisible()
  await expect(page.getByRole("textbox", { name: "弹幕内容" })).not.toBeVisible()
  await page.reload()
  await expect(page.getByRole("heading", { name: "正在生成画面并积累缓冲" })).toBeVisible()
})

test("insufficient startup budget blocks generation with an explanation", async ({ page }) => {
  const snapshot = snapshotFixture()
  await page.route("**/ops/snapshot", (route) =>
    route.fulfill({
      json: {
        ...snapshot,
        session: { ...snapshot.session, status: "stopped" },
        controls: { ...snapshot.controls, canStart: true, canStop: false },
      },
    }),
  )
  await page.route("**/ops/readiness", (route) =>
    route.fulfill({
      json: {
        ready: false,
        target: "本地测试",
        characterId: "host",
        characterName: "主播",
        credentialSaved: true,
        availableMicros: 1000000,
        minimumMicros: 22500000,
        blockers: ["启动额度不足"],
      },
    }),
  )
  await page.goto("/")
  await expect(page.getByText("启动额度不足", { exact: true })).toBeVisible()
  await expect(page.getByRole("button", { name: "开始直播", exact: true })).toBeDisabled()
})

test("saving provider credentials refreshes readiness", async ({ page }) => {
  let saved = false
  const snapshot = snapshotFixture()
  await page.route("**/ops/snapshot", (route) =>
    route.fulfill({
      json: {
        ...snapshot,
        session: { ...snapshot.session, status: "stopped" },
        controls: { ...snapshot.controls, canStart: true, canStop: false },
      },
    }),
  )
  await page.route("**/ops/readiness", (route) =>
    route.fulfill({
      json: {
        ready: saved,
        target: "本地测试",
        characterId: "host",
        characterName: "主播",
        credentialSaved: saved,
        availableMicros: 30000000,
        minimumMicros: 22500000,
        blockers: saved ? [] : ["请保存凭据"],
      },
    }),
  )
  await page.route("**/config/services/credentials", async (route) => {
    expect(route.request().postDataJSON()).toEqual({ provider: "minimax", apiKey: "test-key" })
    saved = true
    await route.fulfill({ json: servicesFixture() })
  })
  await page.goto("/")
  await expect(page.getByRole("button", { name: "开始直播", exact: true })).toBeDisabled()
  await page.getByRole("button", { name: "修改开播配置" }).click()
  await page.getByLabel("MiniMax API Key", { exact: true }).fill("test-key")
  await page.getByRole("button", { name: "保存 MiniMax 凭据" }).click()
  await expect(page.getByLabel("MiniMax API Key", { exact: true })).toHaveValue("")
  await page.getByRole("button", { name: "返回开播摘要" }).click()
  await expect(page.getByRole("button", { name: "开始直播", exact: true })).toBeEnabled()
})

test("shows provider pricing, CNY estimate and promotion expiry from the API", async ({ page }) => {
  await page.goto("/")
  await page.getByRole("button", { name: "配置", exact: true }).click()
  await page
    .getByText("fal · minimax/h3-max-turbo/image-to-video · 计费详情", { exact: true })
    .click()
  await expect(page.getByText("标准价：USD 0.025 / 1 生成视频秒", { exact: true })).toBeVisible()
  await expect(
    page.getByText("当前人民币估算：¥0.041938 / 1 生成视频秒", { exact: true }),
  ).toBeVisible()
  await expect(page.getByText(/优惠到期：.*2026.*9.*14.*UTC/)).toBeVisible()
  await expect(page.getByText(/预算按有效价格预留/)).toBeVisible()
})

test("switches new video tasks during a live session without restarting", async ({ page }) => {
  let current = servicesFixture()
  let commands = 0
  await page.route("**/ops/start", () => {
    commands++
  })
  await page.route("**/ops/stop", () => {
    commands++
  })
  await page.route("**/config/services", async (route) => {
    if (route.request().method() === "PUT") {
      expect(route.request().postDataJSON()).toEqual({
        video: {
          provider: "fal",
          model: "minimax/h3-max-turbo/image-to-video",
          resolution: "480P",
        },
        text: current.saved.text,
      })
      current = {
        ...current,
        saved: {
          ...current.saved,
          video: {
            provider: "fal",
            model: "minimax/h3-max-turbo/image-to-video",
            resolution: "480P",
          },
        },
      }
    }
    await route.fulfill({ json: current })
  })
  await page.goto("/")
  const selector = page.getByLabel("后续视频任务供应商与模型")
  await selector.selectOption("fal:minimax/h3-max-turbo/image-to-video")
  await expect(selector).toHaveValue("fal:minimax/h3-max-turbo/image-to-video")
  await expect(page.getByRole("button", { name: "结束直播", exact: true })).toBeEnabled()
  await page.reload()
  await expect(selector).toHaveValue("fal:minimax/h3-max-turbo/image-to-video")
  expect(commands).toBe(0)
})

test("live preview uses the output stream and never falls back to a generated clip", async ({
  page,
}) => {
  const snapshot = snapshotFixture()
  snapshot.stream.status = "starting"
  await page.route("**/api/v1/ops/snapshot", (route) => route.fulfill({ json: snapshot }))
  await page.goto("/")
  const preview = page.getByRole("region", { name: "直播预览与互动" })
  await expect(preview.getByText("推流开始后显示直播画面，与 RTMP 输出同源")).toBeVisible()
  await expect(preview.locator("video")).toHaveCount(0)
  await expect(preview.locator("iframe")).toHaveCount(0)

  snapshot.stream.status = "live"
  await page.reload()
  await expect(preview.locator("iframe")).toHaveAttribute(
    "src",
    "/live-preview/?autoplay=true&muted=true&controls=true&playsinline=true",
  )
  await expect(preview.locator("video")).toHaveCount(0)
})
