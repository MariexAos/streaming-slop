import type { ReactNode } from "react"
import { Activity, Radio } from "lucide-react"
import { statusLabel, connectionLabel, toneForStatus } from "@/lib/status"
import { useSnapshot } from "@/queries/ops"
import { useOpsStore } from "@/store/ops"
import { Badge } from "./ui/badge"
export function ConsoleHeader({ children }: { children: ReactNode }) {
  const query = useSnapshot()
  const connection = useOpsStore((state) => state.connection)
  return (
    <header className="sticky top-0 z-40 border-b border-[var(--line)] bg-white/95 backdrop-blur-xl">
      <div className="mx-auto flex max-w-[1600px] flex-wrap items-center justify-between gap-3 px-6 py-4">
        <div className="flex items-center gap-3">
          <Radio aria-hidden="true" className="size-5" />
          <h1 className="font-semibold">Streaming Slop</h1>
          <Badge tone={toneForStatus(query.data?.session.status ?? "stopped")}>
            {query.data ? statusLabel(query.data.session.status) : "连接中"}
          </Badge>
        </div>
        <Badge tone={connection === "open" ? "neutral" : "warning"}>
          <Activity aria-hidden="true" className="size-3" />
          实时连接 {connectionLabel(connection)}
        </Badge>
      </div>
      <div className="mx-auto max-w-[1600px] overflow-x-auto px-6 pb-3">{children}</div>
      {query.error && (
        <p role="alert" className="px-6 pb-3 text-sm text-rose-600">
          {query.error.message}
        </p>
      )}
    </header>
  )
}
