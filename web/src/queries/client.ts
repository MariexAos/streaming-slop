import { QueryClient, replaceEqualDeep } from "@tanstack/react-query"
import { opsSnapshotSchema } from "@/lib/schema"

export const snapshotKey = ["ops", "snapshot"] as const

export function createQueryClient() {
  const client = new QueryClient({
    defaultOptions: {
      queries: { staleTime: 60_000, retry: false, refetchOnWindowFocus: false },
      mutations: { retry: false },
    },
  })
  client.setQueryDefaults(snapshotKey, {
    staleTime: Infinity,
    gcTime: Infinity,
    structuralSharing: (previous, incoming) => {
      const old = opsSnapshotSchema.safeParse(previous)
      const next = opsSnapshotSchema.parse(incoming)
      if (old.success && old.data.revision >= next.revision) return previous
      return replaceEqualDeep(previous, incoming)
    },
  })
  return client
}
