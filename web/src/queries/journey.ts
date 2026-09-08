import { queryOptions, useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { sendCommand } from "@/lib/api"
import { fetchReadiness } from "@/lib/readiness-api"
import { snapshotKey } from "./client"

const readinessOptions = queryOptions({
  queryKey: ["readiness"],
  queryFn: ({ signal }) => fetchReadiness(signal),
  refetchInterval: 5000,
  staleTime: 0,
})
export function useReadiness() {
  return useQuery(readinessOptions)
}
export function useLiveCommand() {
  const client = useQueryClient()
  return useMutation({
    mutationKey: ["live-command"],
    mutationFn: sendCommand,
    retry: false,
    onSettled: async () => {
      await Promise.all([
        client.invalidateQueries({ queryKey: snapshotKey }),
        client.invalidateQueries({ queryKey: ["budget"] }),
        client.invalidateQueries({ queryKey: ["model-services"] }),
        client.invalidateQueries({ queryKey: readinessOptions.queryKey }),
      ])
    },
  })
}
