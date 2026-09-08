import { expect, test } from "@playwright/test"

test("guest watches the stream and sends interaction without management controls", async ({
  page,
}) => {
  await page.route("**/api/guest/status", (route) =>
    route.fulfill({ json: { streamStatus: "live", interactive: true, message: "等待互动" } }),
  )
  await page.route("**/live-preview/**", (route) =>
    route.fulfill({ contentType: "text/html", body: "直播画面" }),
  )
  let received = ""
  await page.route("**/api/guest/messages", async (route) => {
    received = route.request().postData() ?? ""
    await route.fulfill({ status: 202, json: { status: "accepted" } })
  })
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto("/guest")
  await expect(page.getByTitle("MediaMTX 直播预览")).toBeVisible()
  await expect(page.getByRole("button", { name: "开始直播", exact: true })).toHaveCount(0)
  await page.getByLabel("弹幕内容").fill("聊聊今天的天气")
  await page.getByRole("button", { name: "发送弹幕", exact: true }).click()
  await expect(page.getByText("已发送，等待后续回应。")).toBeVisible()
  expect(received).toContain("聊聊今天的天气")
  await expect(page.getByLabel("弹幕内容")).toHaveValue("")
})
