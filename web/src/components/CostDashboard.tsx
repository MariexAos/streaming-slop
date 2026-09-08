import type { ModelServices } from "@/lib/services-api"
import type { SessionBudgetState } from "@/lib/budget"
import {
  BadgeDollarSign,
  CircleDollarSign,
  Clock3,
  Coins,
  TimerReset,
  WalletCards,
} from "lucide-react"
import type { OpsSnapshot } from "@/lib/schema"
import type { MetricPoint } from "@/store/ops"
import { formatNumber } from "@/lib/utils"
import { MetricCard } from "@/components/MetricCard"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Table, TableBody, TableCell, TableHead, TableRow } from "@/components/ui/table"

export function CostDashboard({
  snapshot,
  services,
  budget,
  history,
}: {
  snapshot: OpsSnapshot
  services: ModelServices | null
  budget: SessionBudgetState | null
  history: MetricPoint[]
}) {
  const selected = services?.saved.video
  const price = services?.options
    .find(
      (option) =>
        option.kind === "video" &&
        option.provider === selected?.provider &&
        option.model === selected?.model,
    )
    ?.prices.find((quote) => quote.variant === selected?.resolution)
  const config = selected
    ? {
        ...selected,
        durationSeconds: 5,
        unitPriceCnyPerSecond: price ? price.estimatedCny / price.unitQuantity : null,
      }
    : null
  const clipCost =
    config?.unitPriceCnyPerSecond != null
      ? config.unitPriceCnyPerSecond * config.durationSeconds
      : null
  const currentBudget = budget?.sessionId === snapshot.session.id ? budget : null
  const total = currentBudget ? currentBudget.chargedMicros / 1e6 : null
  const hourly =
    total !== null && snapshot.session.uptimeSeconds > 0
      ? (total * 3600) / snapshot.session.uptimeSeconds
      : null
  const pending = currentBudget ? currentBudget.reservedMicros / 1e6 : null

  return (
    <div className="space-y-5">
      <section aria-labelledby="cost-heading">
        <div className="mb-3">
          <p className="eyebrow">实时成本</p>
          <h2 id="cost-heading" className="section-title">
            本场预算与视频报价
          </h2>
        </div>
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-6">
          <MetricCard
            label="本场已结算"
            value={money(total)}
            detail={`${snapshot.generation.succeededTotal} 个成功片段`}
            icon={WalletCards}
            history={history}
            tooltip="本场账本已结算费用，包含视频、推理与语音；不含待结算预占。"
          />
          <MetricCard
            label="新片段估价"
            value={money(clipCost)}
            detail={`${config?.durationSeconds ?? 5} 秒生成`}
            icon={Coins}
            tooltip="按当前模型、分辨率和输出秒数计算。"
          />
          <MetricCard
            label="实时每小时"
            value={money(hourly)}
            detail="按当前会话耗时折算"
            icon={Clock3}
            tooltip="累计成本除以当前会话运行小时数。"
          />
          <MetricCard
            label="本场预占"
            value={money(pending)}
            detail="含已生成但尚未结算的任务"
            icon={TimerReset}
            tooltip="来自本场持久化账本，不用当前单价重算旧任务。"
            warm
          />
          <MetricCard
            label="成功任务"
            value={`${snapshot.generation.succeededTotal}`}
            detail={`失败 ${snapshot.generation.failuresTotal} 次`}
            icon={BadgeDollarSign}
            tooltip="已完成并计入成本的生成任务数量。"
          />
          <MetricCard
            label="单价"
            value={money(config?.unitPriceCnyPerSecond ?? null)}
            detail="每输出秒"
            icon={CircleDollarSign}
            tooltip="当前所选供应商、模型与画质的有效价格，包含有效优惠及人民币换算。"
          />
        </div>
      </section>

      <Card>
        <CardHeader>
          <CardTitle>新任务计费配置</CardTitle>
        </CardHeader>
        <CardContent>
          <Table>
            <thead>
              <TableRow>
                <TableHead>供应商</TableHead>
                <TableHead>模型</TableHead>
                <TableHead>分辨率</TableHead>
                <TableHead>时长</TableHead>
                <TableHead className="text-right">预计单片成本</TableHead>
              </TableRow>
            </thead>
            <TableBody>
              <TableRow>
                <TableCell>{config?.provider ?? "—"}</TableCell>
                <TableCell className="font-mono text-xs">{config?.model ?? "—"}</TableCell>
                <TableCell>{config?.resolution ?? "—"}</TableCell>
                <TableCell>{config ? `${config.durationSeconds} 秒` : "—"}</TableCell>
                <TableCell className="text-right font-mono font-semibold">
                  {money(clipCost)}
                </TableCell>
              </TableRow>
            </TableBody>
          </Table>
          <p className="mt-4 text-xs leading-5 text-[var(--text-dim)]">
            新任务按所选供应商的有效价格估算人民币费用；优惠到期自动恢复标准价。已提交任务保留原始报价，预占不是实际扣费，最终以供应商账单为准。
          </p>
        </CardContent>
      </Card>
    </div>
  )
}

function money(value: number | null) {
  return value === null ? "—" : `¥${formatNumber(value, 2)}`
}
