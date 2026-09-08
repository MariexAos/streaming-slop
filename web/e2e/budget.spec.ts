import { expect, test } from "@playwright/test"
import { budgetJourneyFixture } from "../src/lib/budget.fixture"

test("saves a per-session ceiling, blocks insufficient budget and shows live spending", async ({
  page,
}) => {
  const data = budgetJourneyFixture()
  let amount = data.budget.nextLimitMicros
  let active = false
  await page.route("**/api/v1/**", async (route) => {
    const path = new URL(route.request().url()).pathname
    if (path.endsWith("/events"))
      return route.fulfill({ contentType: "text/event-stream", body: ": connected\n\n" })
    if (path.endsWith("/budget")) {
      if (route.request().method() === "PUT") {
        const input: unknown = route.request().postDataJSON()
        expect(input).toEqual({ limitMicros: amount === 10000000 ? 1000000 : 10000000 })
        amount = amount === 10000000 ? 1000000 : 10000000
        return route.fulfill({ status: 204 })
      }
      return route.fulfill({
        json: {
          ...data.budget,
          nextLimitMicros: amount,
          limitMicros: amount,
          ...(active
            ? { sessionId: "session-1", chargedMicros: 100000, reservedMicros: 5000000 }
            : {}),
        },
      })
    }
    if (path.endsWith("/readiness"))
      return route.fulfill({
        json: {
          ...data.readiness,
          ready: amount >= 7550000,
          availableMicros: amount,
          blockers: amount < 7550000 ? ["本场预算不足"] : [],
        },
      })
    if (path.endsWith("/start")) {
      expect(amount).toBe(10000000)
      active = true
      return route.fulfill({
        json: { command: "start", status: "accepted", acceptedAt: "2026-09-08T00:00:00Z" },
      })
    }
    if (path.endsWith("/snapshot"))
      return route.fulfill({
        json: active
          ? {
              ...data.snapshot,
              revision: 2,
              session: { ...data.snapshot.session, status: "buffering" },
              controls: { ...data.snapshot.controls, canStart: false, canStop: true },
            }
          : data.snapshot,
      })
    if (path.endsWith("/services")) return route.fulfill({ json: data.services })
    return route.fulfill({ status: 503 })
  })
  await page.goto("/")
  const input = page.getByLabel("本场费用上限（人民币）")
  const start = page.getByRole("button", { name: "开始直播", exact: true })
  await expect(input).toHaveValue("10")
  await expect(start).toBeEnabled()
  await input.fill("1")
  await expect(start).toBeDisabled()
  await page.getByRole("button", { name: "保存本场预算" }).click()
  await expect(page.getByText("本场预算不足", { exact: true })).toBeVisible()
  await expect(start).toBeDisabled()
  await input.fill("10")
  await page.getByRole("button", { name: "保存本场预算" }).click()
  await expect(start).toBeEnabled()
  await start.click()
  await expect(page.getByText("剩余可用 ¥4.90", { exact: true })).toBeVisible()
  await expect(input).not.toBeVisible()
  await page.reload()
  await expect(page.getByText("剩余可用 ¥4.90", { exact: true })).toBeVisible()
})

test("failed budget save retains the draft and prevents starting with the wrong ceiling", async ({
  page,
}) => {
  const data = budgetJourneyFixture()
  await page.route("**/api/v1/**", async (route) => {
    const path = new URL(route.request().url()).pathname
    if (path.endsWith("/events"))
      return route.fulfill({ contentType: "text/event-stream", body: ": connected\n\n" })
    if (route.request().method() === "PUT") return route.fulfill({ status: 500 })
    const responses: Record<string, unknown> = {
      "/api/v1/budget": data.budget,
      "/api/v1/config/services": data.services,
      "/api/v1/ops/snapshot": data.snapshot,
      "/api/v1/ops/readiness": data.readiness,
    }
    return route.fulfill({ json: responses[path] ?? {} })
  })
  await page.goto("/")
  const input = page.getByLabel("本场费用上限（人民币）")
  await input.fill("20")
  await page.getByRole("button", { name: "保存本场预算" }).click()
  await expect(page.getByText("预算保存失败，请重试", { exact: true })).toBeVisible()
  await expect(input).toHaveValue("20")
  await expect(page.getByRole("button", { name: "开始直播", exact: true })).toBeDisabled()
})

test("cost page uses selected fal pricing and persisted reservations", async ({ page }) => {
  const data = budgetJourneyFixture()
  data.services.saved.video = {
    provider: "fal",
    model: "minimax/h3-max-turbo/image-to-video",
    resolution: "480P",
  }
  await page.route("**/api/v1/**", async (route) => {
    const path = new URL(route.request().url()).pathname
    if (path.endsWith("/events"))
      return route.fulfill({ contentType: "text/event-stream", body: ": connected\n\n" })
    const responses: Record<string, unknown> = {
      "/api/v1/config/services": data.services,
      "/api/v1/budget": {
        ...data.budget,
        sessionId: data.snapshot.session.id,
        chargedMicros: 50000,
        reservedMicros: 2096875,
      },
      "/api/v1/ops/snapshot": data.snapshot,
      "/api/v1/ops/readiness": data.readiness,
    }
    return route.fulfill({ json: responses[path] ?? {} })
  })
  await page.goto("/")
  await page.getByRole("button", { name: "成本", exact: true }).click()
  await expect(page.getByRole("cell", { name: "fal", exact: true })).toBeVisible()
  await expect(page.getByRole("cell", { name: "¥0.21", exact: true })).toBeVisible()
  await expect(page.getByText("¥2.1", { exact: true })).toBeVisible()
  await expect(page.getByText("¥0.05", { exact: true })).toBeVisible()
})
