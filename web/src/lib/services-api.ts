import { z } from "zod"
import { pricingSchema } from "./schema"

const selectionSchema = z
  .object({
    provider: z.enum(["minimax", "fal"]),
    model: z.string().min(1),
    resolution: z.enum(["480P", "768P"]).optional(),
  })
  .strict()
const settingsSchema = z.object({ video: selectionSchema, text: selectionSchema }).strict()
const optionSchema = z
  .object({
    provider: z.enum(["minimax", "fal"]),
    model: z.string(),
    kind: z.enum(["video", "text"]),
    resolutions: z.array(z.enum(["480P", "768P"])),
    prices: z.array(pricingSchema),
  })
  .strict()
const servicesSchema = z
  .object({
    active: settingsSchema.nullable(),
    saved: settingsSchema,
    options: z.array(optionSchema),
    credentials: z.array(
      z.object({ provider: z.enum(["minimax", "fal"]), configured: z.boolean() }).strict(),
    ),
  })
  .strict()
export type ModelServices = z.infer<typeof servicesSchema>
export type ModelSettings = ModelServices["saved"]
export type ModelOption = ModelServices["options"][number]
export type Provider = ModelSettings["video"]["provider"]

async function responseData(response: Response) {
  if (!response.ok) {
    const error = z.object({ message: z.string() }).safeParse(await response.json())
    throw new Error(error.success ? error.data.message : "模型服务请求失败")
  }
  return servicesSchema.parse(await response.json())
}
export async function fetchServices(signal?: AbortSignal) {
  return responseData(await fetch("/api/v1/config/services", { signal }))
}
export async function saveServices(settings: ModelSettings) {
  return responseData(
    await fetch("/api/v1/config/services", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(settings),
    }),
  )
}
export async function saveCredential(input: { provider: Provider; apiKey: string }) {
  return responseData(
    await fetch("/api/v1/config/services/credentials", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(input),
    }),
  )
}
