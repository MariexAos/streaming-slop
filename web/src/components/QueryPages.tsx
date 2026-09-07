import { useQuery } from "@tanstack/react-query"
import { fetchFlowCatalog, fetchCurrentFlow } from "@/lib/api"
import { generationOptions } from "@/queries/config"
import { useSnapshot } from "@/queries/ops"
import { useOpsStore } from "@/store/ops"
import { CostDashboard } from "./CostDashboard"
import { FlowDashboard } from "./FlowDashboard"

export function FlowPage() {
  const catalog = useQuery({
    queryKey: ["flows"],
    queryFn: ({ signal }) => fetchFlowCatalog(signal),
  })
  const current = useQuery({
    queryKey: ["flows", "current"],
    queryFn: ({ signal }) => fetchCurrentFlow(signal),
    refetchInterval: 5000,
  })
  return (
    <FlowDashboard
      catalog={catalog.data ?? null}
      current={current.data ?? null}
      error={(catalog.error ?? current.error)?.message ?? null}
    />
  )
}

export function CostPage() {
  const snapshot = useSnapshot()
  const config = useQuery(generationOptions)
  const history = useOpsStore((state) => state.history.cost)
  if (!snapshot.data) return null
  return <CostDashboard snapshot={snapshot.data} config={config.data ?? null} history={history} />
}
