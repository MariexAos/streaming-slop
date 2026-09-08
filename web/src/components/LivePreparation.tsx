import { SessionBudget } from "./SessionBudget"
import { useSessionBudget } from "@/queries/budget"
import { budgetMicros } from "@/lib/budget"
import { useState } from "react"
import { SessionModelSelection } from "./SessionModelSelection"
import { useServicesSaving } from "@/queries/services"
import type { OpsSnapshot } from "@/lib/schema"
import { journeyState } from "@/lib/journey"
import { formatDuration } from "@/lib/utils"
import { useReadiness, useLiveCommand } from "@/queries/journey"
import { Button } from "./ui/button"
import { ConfigPage } from "./ConfigPage"
import { LiveSessionControls } from "./LiveSessionControls"

export function LivePreparation({
  snapshot,
  onHistory,
}: {
  snapshot: OpsSnapshot
  onHistory: () => void
}) {
  const budget = useSessionBudget()
  const [budgetDraft, setBudgetDraft] = useState<string>()
  const amount = budgetDraft ?? String((budget.query.data?.nextLimitMicros ?? 0) / 1e6)
  const limit = budgetMicros(amount)
  const budgetDirty = budgetDraft !== undefined && limit !== budget.query.data?.nextLimitMicros
  const servicesSaving = useServicesSaving()
  const readiness = useReadiness()
  const command = useLiveCommand()
  const [editing, setEditing] = useState(false)
  const state = journeyState(snapshot)
  const data = readiness.data
  return (
    <section aria-label="开播准备" className="space-y-5 py-2">
      <div>
        <p className="eyebrow">直播工作台</p>
        <h2 className="text-3xl font-semibold tracking-tight">{state.title}</h2>
      </div>
      {state.error && (
        <p role="alert" className="rounded-xl bg-rose-50 p-4 text-sm text-rose-700">
          {state.error}
        </p>
      )}
      <SessionModelSelection />
      <SessionBudget
        active={state.active}
        data={budget.query.data}
        amount={amount}
        onChange={setBudgetDraft}
        dirty={budgetDirty}
        valid={limit !== undefined}
        saving={budget.save.isPending}
        error={budget.save.error?.message ?? budget.query.error?.message}
        onSave={() => {
          if (limit !== undefined)
            budget.save.mutate(limit, { onSuccess: () => setBudgetDraft(undefined) })
        }}
      />
      {state.active ? (
        <>
          <p className="text-sm text-[var(--text-muted)]">
            已运行 {formatDuration(snapshot.session.uptimeSeconds)} · 已生成可播放画面{" "}
            {snapshot.buffer.readySeconds.toFixed(0)} 秒 / 启动目标{" "}
            {snapshot.buffer.readyTargetSeconds} 秒
          </p>
          {state.preparing && (
            <p role="status" className="text-sm">
              准备完成后自动连接推流。生成期间会产生费用，可以停止准备。
            </p>
          )}
          {snapshot.fallback.active && (
            <p role="status" className="rounded-xl bg-amber-50 p-4 text-sm">
              正在播放备用画面，生成内容恢复后继续。
            </p>
          )}
          <LiveSessionControls snapshot={snapshot} />
        </>
      ) : (
        <>
          {readiness.error && (
            <div role="alert">
              <p>{readiness.error.message}</p>
              <Button onClick={() => void readiness.refetch()}>重新检查</Button>
            </div>
          )}
          {data && (
            <div className="grid gap-5 rounded-2xl bg-[var(--surface-raised)] p-5 sm:grid-cols-[160px_1fr]">
              {data.characterId && (
                <img
                  className="aspect-video w-full rounded-xl object-cover"
                  src={`/api/v1/characters/${data.characterId}/start`}
                  alt={`下一场人物：${data.characterName}`}
                />
              )}
              <dl className="grid gap-3 text-sm sm:grid-cols-2">
                <div>
                  <dt className="text-[var(--text-muted)]">人物</dt>
                  <dd>{data.characterName || "未选择"}</dd>
                </div>
                <div>
                  <dt className="text-[var(--text-muted)]">直播目标</dt>
                  <dd>{data.target}</dd>
                </div>
                <div>
                  <dt className="text-[var(--text-muted)]">本场可用额度</dt>
                  <dd>¥{(data.availableMicros / 1e6).toFixed(2)}</dd>
                </div>
                <div>
                  <dt className="text-[var(--text-muted)]">启动视频缓冲至少需要</dt>
                  <dd>¥{(data.minimumMicros / 1e6).toFixed(2)}，另需推理费用</dd>
                </div>
                <div className="sm:col-span-2">
                  <dt className="text-[var(--text-muted)]">生成服务</dt>
                  <dd>{data.credentialSaved ? "凭据已保存，未验证连接" : "需要配置凭据"}</dd>
                </div>
              </dl>
            </div>
          )}
          {!!data?.blockers.length && (
            <ul className="space-y-2 text-sm text-amber-800" aria-label="开播阻塞原因">
              {data.blockers.map((item) => (
                <li key={item}>{item}</li>
              ))}
            </ul>
          )}
          <div className="flex flex-wrap gap-3">
            <Button
              variant="primary"
              disabled={
                budgetDirty ||
                budget.save.isPending ||
                !budget.query.data ||
                budget.query.isError ||
                servicesSaving ||
                !data?.ready ||
                !snapshot.controls.canStart ||
                command.isPending ||
                readiness.isError
              }
              onClick={() => command.mutate("start")}
            >
              {command.isPending ? "正在启动…" : "开始直播"}
            </Button>
            <Button onClick={() => setEditing(!editing)}>
              {editing ? "收起配置" : "修改开播配置"}
            </Button>
          </div>
          {command.error && (
            <p role="alert" className="text-sm text-rose-600">
              {command.error.message}
            </p>
          )}
          {command.isSuccess && <p role="status">启动请求已接收，正在等待准备状态。</p>}
          {snapshot.session.id && <Button onClick={onHistory}>查看直播记录</Button>}
          {editing && (
            <div className="border-t border-[var(--line)] pt-5">
              <ConfigPage />
              <Button
                className="mt-4"
                onClick={() => {
                  setEditing(false)
                  void readiness.refetch()
                }}
              >
                返回开播摘要
              </Button>
            </div>
          )}
        </>
      )}
    </section>
  )
}
