import { BadgeDollarSign, CircleDollarSign, Clock3, Coins, TimerReset, WalletCards } from "lucide-react"
import type { GenerationConfig, OpsSnapshot } from "@/lib/schema"
import type { MetricPoint } from "@/store/ops"
import { formatNumber } from "@/lib/utils"
import { MetricCard } from "@/components/MetricCard"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Table, TableBody, TableCell, TableHead, TableRow } from "@/components/ui/table"

export function CostDashboard({ snapshot, config, history }: { snapshot: OpsSnapshot; config: GenerationConfig | null; history: MetricPoint[] }) {
  const clipCost = config ? config.unitPriceCnyPerSecond * config.durationSeconds : null
  const total = snapshot.generation.costCny
  const hourly = snapshot.generation.costPerLiveHourCny
  const pending = clipCost === null ? null : clipCost * snapshot.generation.inFlight

  return (
    <div className="space-y-5">
      <section aria-labelledby="cost-heading">
        <div className="mb-3">
          <p className="eyebrow">实时成本</p>
          <h2 id="cost-heading" className="section-title">MiniMax 生成成本</h2>
        </div>
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-6">
          <MetricCard label="累计成本" value={money(total)} detail={`${snapshot.generation.succeededTotal} 个成功片段`} icon={WalletCards} history={history} tooltip="仅累计成功生成任务的已知成本。" />
          <MetricCard label="每片段成本" value={money(clipCost)} detail={`${config?.durationSeconds ?? 5} 秒生成`} icon={Coins} tooltip="按当前模型、分辨率和输出秒数计算。" />
          <MetricCard label="实时每小时" value={money(hourly)} detail="按当前会话耗时折算" icon={Clock3} tooltip="累计成本除以当前会话运行小时数。" />
          <MetricCard label="待结算估算" value={money(pending)} detail={`${snapshot.generation.inFlight} 个进行中任务`} icon={TimerReset} tooltip="进行中任务全部成功时的预计输出成本。" warm />
          <MetricCard label="成功任务" value={`${snapshot.generation.succeededTotal}`} detail={`失败 ${snapshot.generation.failuresTotal} 次`} icon={BadgeDollarSign} tooltip="已完成并计入成本的生成任务数量。" />
          <MetricCard label="单价" value={config ? `¥${formatNumber(config.unitPriceCnyPerSecond, 2)}` : "—"} detail="每输出秒" icon={CircleDollarSign} tooltip="MiniMax 中国区按量计费刊例价。" />
        </div>
      </section>

      <Card>
        <CardHeader><CardTitle>当前计费配置</CardTitle></CardHeader>
        <CardContent>
          <Table>
            <thead><TableRow><TableHead>供应商</TableHead><TableHead>模型</TableHead><TableHead>分辨率</TableHead><TableHead>时长</TableHead><TableHead className="text-right">预计单片成本</TableHead></TableRow></thead>
            <TableBody>
              <TableRow>
                <TableCell>{config?.provider ?? "—"}</TableCell>
                <TableCell className="font-mono text-xs">{config?.model ?? "—"}</TableCell>
                <TableCell>{config?.resolution ?? "—"}</TableCell>
                <TableCell>{config ? `${config.durationSeconds} 秒` : "—"}</TableCell>
                <TableCell className="text-right font-mono font-semibold">{money(clipCost)}</TableCell>
              </TableRow>
            </TableBody>
          </Table>
          <p className="mt-4 text-xs leading-5 text-[var(--text-dim)]">失败、取消或安全审核未通过的任务不计入成功成本；页面金额根据官方刊例价估算，最终以 MiniMax 账单为准。</p>
        </CardContent>
      </Card>
    </div>
  )
}

function money(value: number | null) {
  return value === null ? "—" : `¥${formatNumber(value, 2)}`
}
