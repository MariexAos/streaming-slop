import { z } from "zod"

const reference = z.object({ id: z.string(), width: z.number(), height: z.number() })
const profile = z.object({
  id: z.string(),
  characterId: z.string(),
  name: z.string(),
  description: z.string(),
  firstFrame: reference,
  lastFrame: reference,
  scene: z.object({ location: z.string(), time: z.string(), lighting: z.string() }),
  camera: z.object({ shot: z.string(), angle: z.string() }),
  voiceId: z.string().optional(),
})
const attempts = z.array(
  z.object({ id: z.string(), provider: z.string(), jobId: z.string(), status: z.string() }),
)
export type CharacterProfile = z.infer<typeof profile>
async function request(
  path: string,
  method = "GET",
  body?: unknown,
  signal?: AbortSignal,
): Promise<unknown> {
  const response = await fetch(`/api/v1/${path}`, {
    method,
    signal,
    headers: { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  if (!response.ok) throw new Error(await response.text())
  if (response.status === 204) return null
  const value: unknown = await response.json()
  return value
}
export async function fetchCharacters({ signal }: { signal?: AbortSignal } = {}) {
  return z.array(profile).parse(await request("characters", "GET", undefined, signal))
}
export async function fetchCharacter({ signal }: { signal?: AbortSignal } = {}) {
  return profile.parse(await request("characters/current", "GET", undefined, signal))
}
export async function fetchUnresolved({ signal }: { signal?: AbortSignal } = {}) {
  return attempts.parse(await request("attempts/unresolved", "GET", undefined, signal))
}
export async function selectCharacter(versionId: string) {
  await request("characters/current", "PUT", { versionId })
}
export async function attachJob(input: { id: string; jobId: string }) {
  await request(`attempts/${encodeURIComponent(input.id)}/job`, "PUT", { jobId: input.jobId })
}
function fileData(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onerror = () => reject(new Error("读取图片失败"))
    reader.onload = () =>
      typeof reader.result === "string"
        ? resolve(reader.result.split(",")[1] ?? "")
        : reject(new Error("读取图片失败"))
    reader.readAsDataURL(file)
  })
}
export async function publishCharacter(input: {
  profile: CharacterProfile
  first?: File
  last?: File
}) {
  const [first, last] = await Promise.all([
    input.first ? (input.first ? fileData(input.first) : Promise.resolve("")) : Promise.resolve(""),
    input.last ? (input.last ? fileData(input.last) : Promise.resolve("")) : Promise.resolve(""),
  ])
  return profile.parse(await request("characters", "POST", { profile: input.profile, first, last }))
}
