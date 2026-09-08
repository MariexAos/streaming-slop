import { z } from "zod"

const budgetSchema = z.object({
  sessionId: z.string().optional(),
  nextLimitMicros: z.number().int().positive(),
  limitMicros: z.number().int().positive(),
  chargedMicros: z.number().int().nonnegative(),
  reservedMicros: z.number().int().nonnegative(),
})
export type SessionBudgetState = z.infer<typeof budgetSchema>
export async function fetchBudget({ signal }: { signal?: AbortSignal } = {}) {
  const response = await fetch("/api/v1/budget", { signal })
  if (!response.ok) throw new Error("无法读取本场预算")
  return budgetSchema.parse(await response.json())
}
export async function saveBudget(limitMicros: number) {
  const response = await fetch("/api/v1/budget", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ limitMicros }),
  })
  if (!response.ok) throw new Error("预算保存失败，请重试")
}
export function budgetMicros(value: string): number | undefined {
  if (!/^\d+(\.\d{1,2})?$/.test(value)) return undefined
  const amount = Math.round(Number(value) * 1e6)
  return Number.isSafeInteger(amount) && amount > 0 ? amount : undefined
}
