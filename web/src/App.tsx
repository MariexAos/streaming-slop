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
        <a href="#main-content" className="sr-only focus:not-sr-only focus:block focus:p-4">
          跳到主要内容
        </a>
        <ConsoleHeader>
          <nav className="flex w-max items-center gap-1" aria-label="管理看板">
            <NavButton
              active={view === "operations"}
              onClick={() => setView("operations")}
              icon={LayoutDashboard}
            >
              运行
            </NavButton>
            <NavButton active={view === "flow"} onClick={() => setView("flow")} icon={Workflow}>
              生成流程
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
        <main
          id="main-content"
          className="mx-auto max-w-[1600px] space-y-5 px-4 py-6 sm:px-6 xl:px-8"
        >
          {view === "operations" ? (
            <OperationsDashboard onConfigure={() => setView("config")} />
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
