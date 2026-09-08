import { useRef, useState } from "react"
import { Clapperboard, Eye, History, MessageCircleMore } from "lucide-react"
import { SessionReplayPlayer, type ReplayHandle } from "@/components/SessionReplayPlayer"
import { useSessions, useSessionHistory } from "@/queries/history"
import { formatNumber } from "@/lib/utils"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"

export function SessionHistoryDashboard() {
  const list = useSessions()
  const sessions = list.data ?? []
  const [selection, setSelectedID] = useState<string | null>(null)
  const selectedID = selection ?? sessions[0]?.id ?? null
  const history = useSessionHistory(selectedID)
  const detail = history.data
  const player = useRef<ReplayHandle>(null)
  const error = (list.error ?? history.error)?.message
  return (
    <div className="space-y-5">
      <div>
        <p className="eyebrow">历史运行</p>
        <h2 className="section-title">直播记录与 Flow 回放</h2>
      </div>
      {error && (
        <p role="alert" className="text-sm text-rose-600">
          {error}
        </p>
      )}
      <div className="grid gap-4 xl:grid-cols-[340px_minmax(0,1fr)]">
        <Card>
          <CardHeader>
            <CardTitle className="inline-flex items-center gap-2">
              <History className="size-4 text-[var(--accent)]" />
              直播列表
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-2">
            {sessions.map((item) => (
              <button
                key={item.id}
                type="button"
                onClick={() => setSelectedID(item.id)}
                className={`w-full rounded-xl border p-3 text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--accent)] ${selectedID === item.id ? "border-[var(--accent)] bg-[var(--surface-hover)]" : "border-[var(--line)] bg-[var(--surface-raised)]"}`}
              >
                <div className="flex items-center justify-between gap-2">
                  <span className="text-sm font-bold">{localTime(item.startedAt)}</span>
                  <Badge tone={item.status === "STOPPED" ? "neutral" : "warning"}>
                    {item.status}
                  </Badge>
                </div>
                <p className="mt-2 font-mono text-[10px] text-[var(--text-dim)]">{item.id}</p>
                <p className="mt-2 text-xs text-[var(--text-muted)]">
                  片段 {item.readyTotal}/{item.segmentTotal} · 已结算 ¥
                  {formatNumber(item.costCny, 2)}
                </p>
                {!!item.budgetLimitMicros && (
                  <p className="mt-1 text-xs text-[var(--text-muted)]">
                    本场上限 ¥{(item.budgetLimitMicros / 1e6).toFixed(2)} · 待结算预占 ¥
                    {((item.reservedMicros ?? 0) / 1e6).toFixed(4)}
                  </p>
                )}
              </button>
            ))}
            {sessions.length === 0 && (
              <p className="text-sm text-[var(--text-muted)]">暂无直播记录。</p>
            )}
          </CardContent>
        </Card>

        <div className="min-w-0 space-y-4">
          <Card>
            <CardHeader>
              <CardTitle className="inline-flex items-center gap-2">
                <Clapperboard className="size-4 text-[var(--accent)]" />
                整场回放
              </CardTitle>
            </CardHeader>
            <CardContent>
              {detail && (
                <SessionReplayPlayer
                  key={detail.session.id}
                  ref={player}
                  segments={detail.segments}
                />
              )}
            </CardContent>
          </Card>

          <div className="grid gap-4 lg:grid-cols-2">
            <Card>
              <CardHeader>
                <CardTitle className="inline-flex items-center gap-2">
                  <Eye className="size-4 text-[var(--accent)]" />
                  Observer 流转
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                {detail?.observerRuns.map((run) => (
                  <div
                    key={`${run.revision}-${run.createdAt}`}
                    className="rounded-xl border border-[var(--line)] bg-[var(--surface-raised)] p-3 text-sm"
                  >
                    <div className="flex justify-between gap-3">
                      <b>弹幕版本 {run.revision}</b>
                      <span className="text-xs text-[var(--text-dim)]">
                        {localTime(run.createdAt)}
                      </span>
                    </div>
                    <p className="mt-2 text-[var(--text-muted)]">{run.messages.join(" · ")}</p>
                    <p className="mt-2">{run.error ?? run.observation.summary}</p>
                  </div>
                ))}
                {detail?.observerRuns.length === 0 && (
                  <p className="text-sm text-[var(--text-muted)]">
                    该场直播没有 Observer 记录；新直播会开始保存。
                  </p>
                )}
              </CardContent>
            </Card>
            <Card>
              <CardHeader>
                <CardTitle className="inline-flex items-center gap-2">
                  <MessageCircleMore className="size-4 text-[var(--accent)]" />
                  Director 流转
                </CardTitle>
              </CardHeader>
              <CardContent className="max-h-[520px] space-y-3 overflow-y-auto">
                {detail?.segments.map((segment) => (
                  <button
                    key={segment.id}
                    type="button"
                    disabled={!segment.playable}
                    onClick={() => player.current?.seekSegment(segment.id)}
                    className="w-full rounded-xl border border-[var(--line)] bg-[var(--surface-raised)] p-3 text-left"
                  >
                    <div className="flex justify-between gap-3">
                      <b>片段 #{segment.sequence}</b>
                      <Badge tone={segment.playable ? "success" : "neutral"}>
                        {segment.status}
                      </Badge>
                    </div>
                    <p className="mt-2 text-sm">{segment.direction.action}</p>
                    <p className="mt-1 text-sm text-[var(--text-muted)]">
                      {segment.direction.dialogue || "（无台词）"}
                    </p>
                  </button>
                ))}
              </CardContent>
            </Card>
          </div>
        </div>
      </div>
    </div>
  )
}

function localTime(value: string) {
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  }).format(new Date(value))
}
