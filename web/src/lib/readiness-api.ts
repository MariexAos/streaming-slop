import { readinessSchema } from "./journey"
export async function fetchReadiness(signal?: AbortSignal) {
  const response = await fetch("/api/v1/ops/readiness", { signal })
  if (!response.ok) throw new Error("无法检查开播条件，请重试")
  const value: unknown = await response.json()
  return readinessSchema.parse(value)
}
