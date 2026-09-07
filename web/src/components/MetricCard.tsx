import type { LucideIcon } from "lucide-react"
import { Card, CardContent } from "@/components/ui/card"
import { Progress } from "@/components/ui/progress"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { Sparkline } from "@/components/Sparkline"
import type { MetricPoint } from "@/store/ops"

export function MetricCard({
  label,
  value,
  detail,
  icon: Icon,
  history,
  progress,
  tooltip,
  warm,
}: {
  label: string
  value: string
  detail: string
  icon: LucideIcon
  history?: MetricPoint[]
  progress?: number
  tooltip: string
  warm?: boolean
}) {
  return (
    <Card className="min-w-0 overflow-hidden">
      <CardContent className="pb-4">
        <div className="flex items-center justify-between gap-3">
          <p className="text-xs font-bold uppercase tracking-[0.14em] text-[var(--text-dim)]">
            {label}
          </p>
          <Tooltip>
            <TooltipTrigger asChild>
              <span
                className="inline-flex size-8 items-center justify-center rounded-lg bg-[var(--surface-raised)] text-[var(--text-muted)]"
                tabIndex={0}
              >
                <Icon className="size-4" aria-hidden="true" />
                <span className="sr-only">关于{label}</span>
              </span>
            </TooltipTrigger>
            <TooltipContent>{tooltip}</TooltipContent>
          </Tooltip>
        </div>
        <p className="mt-4 font-mono text-2xl font-semibold tracking-tight text-[var(--text)]">
          {value}
        </p>
        <p className="mt-1 min-h-5 text-xs text-[var(--text-muted)]">{detail}</p>
        {progress !== undefined ? (
          <Progress value={progress} className="mt-4" />
        ) : history ? (
          <div className="mt-2">
            <Sparkline points={history} tone={warm ? "warm" : "accent"} />
            <span className="sr-only">最近 {history.length} 个快照的趋势。</span>
          </div>
        ) : null}
      </CardContent>
    </Card>
  )
}
