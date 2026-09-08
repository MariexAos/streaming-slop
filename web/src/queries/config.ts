import { queryOptions, useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import {
  updateBilibiliConfig,
  updateQwenConfig,
  fetchBilibiliConfig,
  fetchQwenConfig,
  sendMockDanmaku,
} from "@/lib/api"

const bilibiliOptions = queryOptions({
  queryKey: ["config", "bilibili"],
  queryFn: ({ signal }) => fetchBilibiliConfig(signal),
  refetchInterval: 5000,
})
const qwenOptions = queryOptions({
  queryKey: ["config", "qwen"],
  queryFn: ({ signal }) => fetchQwenConfig(signal),
})

function useConfigMutation<T, V>(key: readonly string[], save: (value: V) => Promise<T>) {
  const client = useQueryClient()
  return useMutation({
    mutationFn: save,
    onMutate: () => client.cancelQueries({ queryKey: key }),
    onSuccess: async (data) => {
      await client.cancelQueries({ queryKey: key })
      client.setQueryData(key, data)
      await client.invalidateQueries({ queryKey: ["readiness"] })
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

export function useQwenSettings() {
  const query = useQuery(qwenOptions)
  const save = useConfigMutation(qwenOptions.queryKey, updateQwenConfig)
  return { query, save }
}
export function useBilibiliSettings() {
  const query = useQuery(bilibiliOptions)
  const save = useConfigMutation(bilibiliOptions.queryKey, updateBilibiliConfig)
  return { query, save }
}
