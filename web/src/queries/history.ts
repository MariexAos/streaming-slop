import { skipToken, useQuery } from "@tanstack/react-query"
import {
  fetchSessionHistory,
  fetchSessionHistoryList,
  fetchFlowCatalog,
  fetchCurrentFlow,
} from "@/lib/api"
export function useSessions() {
  return useQuery({
    queryKey: ["sessions"],
    queryFn: ({ signal }) => fetchSessionHistoryList(signal),
  })
}
export function useSessionHistory(id: string | null) {
  return useQuery({
    queryKey: ["sessions", id],
    queryFn: id ? ({ signal }) => fetchSessionHistory(id, signal) : skipToken,
  })
}
export function useFlows() {
  const catalog = useQuery({
    queryKey: ["flows"],
    queryFn: ({ signal }) => fetchFlowCatalog(signal),
  })
  const current = useQuery({
    queryKey: ["flows", "current"],
    queryFn: ({ signal }) => fetchCurrentFlow(signal),
    refetchInterval: 5000,
  })
  return { catalog, current }
}
