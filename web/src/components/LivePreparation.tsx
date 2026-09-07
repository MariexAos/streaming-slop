import { useQuery } from "@tanstack/react-query"
import { ArrowRight, Check, Circle } from "lucide-react"
import { fetchBudget, fetchCharacter } from "@/lib/characters"
import { generationOptions } from "@/queries/config"
import { Button } from "./ui/button"

export function LivePreparation({
  onConfigure,
  active,
}: {
  onConfigure: () => void
  active: boolean
}) {
  const character = useQuery({ queryKey: ["character"], queryFn: fetchCharacter })
  const budget = useQuery({ queryKey: ["budget"], queryFn: fetchBudget, refetchInterval: 5000 })
  const generation = useQuery(generationOptions)
  const balance = budget.data
    ? Math.max(
        0,
        budget.data.limitMicros - budget.data.chargedMicros - budget.data.reservedMicros,
      ) / 1e6
    : null
  const steps = [
    {
      label: "直播人物",
      value: character.data?.name ?? (character.isPending ? "读取中…" : "请检查人物配置"),
      ready: !!character.data,
    },
    {
      label: "生成服务",
      value: generation.data?.apiKeyConfigured
        ? "已配置"
        : generation.isPending
          ? "读取中…"
          : "请配置 API Key",
      ready: !!generation.data?.apiKeyConfigured,
    },
    {
      label: "可用额度",
      value:
        balance === null
          ? budget.isPending
            ? "读取中…"
            : "暂时无法读取"
          : `¥${balance.toFixed(2)}`,
      ready: balance !== null && balance > 0,
    },
  ]
  return (
    <section aria-label="开播准备" className="space-y-5 py-2">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <p className="eyebrow">直播工作台</p>
          <h2 className="text-3xl font-semibold tracking-tight">
            {active ? "直播进行中" : "准备下一场直播"}
          </h2>
          <p className="mt-2 text-sm text-[var(--text-muted)]">
            {active
              ? "在下方预览画面、发送互动；需要时展开运行详情。"
              : "确认人物、生成服务和额度后，点击上方「启动」。生成会消耗 API 额度。"}
          </p>
        </div>
        <Button onClick={onConfigure}>
          检查开播配置
          <ArrowRight className="size-4" aria-hidden="true" />
        </Button>
      </div>
      <dl className="grid gap-4 rounded-2xl bg-[var(--surface-raised)] p-5 sm:grid-cols-3">
        {steps.map((step) => (
          <div key={step.label} className="flex items-center gap-3">
            {step.ready ? (
              <Check className="size-4" aria-label="已就绪" />
            ) : (
              <Circle className="size-4 text-[var(--text-muted)]" aria-label="待确认" />
            )}
            <div>
              <dt className="text-xs text-[var(--text-muted)]">{step.label}</dt>
              <dd className="mt-1 text-sm font-medium">{step.value}</dd>
            </div>
          </div>
        ))}
      </dl>
    </section>
  )
}
