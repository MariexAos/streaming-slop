import { useModelServices } from "@/queries/services"
import { useSessionBudget } from "@/queries/budget"
import { useFlows } from "@/queries/history"
import { useSnapshot } from "@/queries/ops"
import { useOpsStore } from "@/store/ops"
import { CostDashboard } from "./CostDashboard"
import { FlowDashboard } from "./FlowDashboard"

export function FlowPage() {
  const { catalog, current } = useFlows()
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
  const { query: services } = useModelServices()
  const { query: budget } = useSessionBudget()
  const history = useOpsStore((state) => state.history.cost)
  if (!snapshot.data) return null
  if (services.error || budget.error)
    return <p role="alert">{services.error?.message ?? budget.error?.message}</p>
  return (
    <CostDashboard
      snapshot={snapshot.data}
      services={services.data ?? null}
      budget={budget.data ?? null}
      history={history}
    />
  )
}
