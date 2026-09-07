import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query"
import {
  fetchGenerationConfig,
  fetchBilibiliConfig,
  fetchQwenConfig,
  sendMockDanmaku,
} from "@/lib/api"

export const generationOptions = queryOptions({
  queryKey: ["config", "generation"],
  queryFn: ({ signal }) => fetchGenerationConfig(signal),
})
export const bilibiliOptions = queryOptions({
  queryKey: ["config", "bilibili"],
  queryFn: ({ signal }) => fetchBilibiliConfig(signal),
  refetchInterval: 5000,
})
export const qwenOptions = queryOptions({
  queryKey: ["config", "qwen"],
  queryFn: ({ signal }) => fetchQwenConfig(signal),
})

export function useConfigMutation<T, V>(key: readonly string[], save: (value: V) => Promise<T>) {
  const client = useQueryClient()
  return useMutation({
    mutationFn: save,
    onMutate: () => client.cancelQueries({ queryKey: key }),
    onSuccess: async (data) => {
      await client.cancelQueries({ queryKey: key })
      client.setQueryData(key, data)
    },
  })
}

export function useMockDanmaku() {
  return useMutation({ mutationFn: sendMockDanmaku })
}

export async function saveResult<T>(operation: Promise<T>): Promise<boolean> {
  try {
    await operation
    return true
  } catch {
    return false
  }
}
