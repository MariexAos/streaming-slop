import { z } from "zod"

const guestStatusSchema = z
  .object({
    streamStatus: z.string(),
    interactive: z.boolean(),
    message: z.string(),
  })
  .strict()

export async function fetchGuestStatus(signal?: AbortSignal) {
  const response = await fetch("/api/guest/status", { signal })
  if (!response.ok) throw new Error("暂时无法连接直播")
  return guestStatusSchema.parse(await response.json())
}

export async function sendGuestMessage(value: { username: string; text: string }) {
  const response = await fetch("/api/guest/messages", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(value),
  })
  if (!response.ok) throw new Error("发送失败，请确认直播已开始后重试")
}
