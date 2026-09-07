import { useState } from "react"
import { History, LayoutDashboard, Settings, WalletCards, Workflow } from "lucide-react"
import { useOpsEvents } from "@/queries/ops"
import { ConsoleHeader } from "@/components/ConsoleHeader"
import { NavButton } from "@/components/ConsoleControls"
import { OperationsDashboard } from "@/components/OperationsDashboard"
import { ConfigPage } from "@/components/ConfigPage"
import { FlowPage, CostPage } from "@/components/QueryPages"
import { SessionHistoryDashboard } from "@/components/SessionHistoryDashboard"
import { TooltipProvider } from "@/components/ui/tooltip"

export function App() {
  const [view, setView] = useState("operations")
  useOpsEvents()
  return (
    <TooltipProvider delayDuration={250}>
      <div className="min-h-screen">
        <ConsoleHeader>
          <nav
            className="flex items-center gap-1 rounded-xl border border-[var(--line)] bg-[var(--surface-raised)] p-1"
            aria-label="管理看板"
          >
            <NavButton
              active={view === "operations"}
              onClick={() => setView("operations")}
              icon={LayoutDashboard}
            >
              运行
            </NavButton>
            <NavButton active={view === "flow"} onClick={() => setView("flow")} icon={Workflow}>
              Flow
            </NavButton>
            <NavButton
              active={view === "history"}
              onClick={() => setView("history")}
              icon={History}
            >
              记录
            </NavButton>
            <NavButton active={view === "cost"} onClick={() => setView("cost")} icon={WalletCards}>
              成本
            </NavButton>
            <NavButton active={view === "config"} onClick={() => setView("config")} icon={Settings}>
              配置
            </NavButton>
          </nav>
        </ConsoleHeader>
        <main className="mx-auto max-w-[1600px] space-y-5 px-4 py-6 sm:px-6 xl:px-8">
          {view === "operations" ? (
            <OperationsDashboard />
          ) : view === "flow" ? (
            <FlowPage />
          ) : view === "cost" ? (
            <CostPage />
          ) : view === "history" ? (
            <SessionHistoryDashboard />
          ) : (
            <ConfigPage />
          )}
        </main>
      </div>
    </TooltipProvider>
  )
}
