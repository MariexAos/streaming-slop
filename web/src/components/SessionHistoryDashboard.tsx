import { useEffect, useState } from "react"
import { Clapperboard, Eye, History, MessageCircleMore, Play } from "lucide-react"
import { fetchSessionHistory, fetchSessionHistoryList } from "@/lib/api"
import type { SessionHistory, SessionHistorySummary } from "@/lib/schema"
import { formatNumber } from "@/lib/utils"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"

export function SessionHistoryDashboard() {
  const [sessions, setSessions] = useState<SessionHistorySummary[]>([])
  const [selectedID, setSelectedID] = useState<string | null>(null)
  const [loadedDetail, setDetail] = useState<SessionHistory | null>(null)
  const detail = loadedDetail?.session.id === selectedID ? loadedDetail : null
  const [segmentID, setSegmentID] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const controller = new AbortController()
    void fetchSessionHistoryList(controller.signal)
      .then((items) => {
        setSessions(items)
        setSelectedID((current) => current ?? items[0]?.id ?? null)
      })
      .catch((reason) => setError(reason instanceof Error ? reason.message : "无法读取直播记录。"))
    return () => controller.abort()
  }, [])

  useEffect(() => {
    if (!selectedID) return
    const controller = new AbortController()
    void fetchSessionHistory(selectedID, controller.signal)
      .then((value) => {
        setDetail(value)
        setSegmentID(value.segments.find((segment) => segment.playable)?.id ?? null)
        setError(null)
      })
      .catch((reason) => setError(reason instanceof Error ? reason.message : "无法读取直播详情。"))
    return () => controller.abort()
  }, [selectedID])

  const playableSegments = detail?.segments.filter((segment) => segment.playable) ?? []
  const selectedSegment = playableSegments.find((segment) => segment.id === segmentID) ?? null
  const playNext = () => {
    const index = playableSegments.findIndex((segment) => segment.id === segmentID)
    if (index >= 0 && index + 1 < playableSegments.length)
      setSegmentID(playableSegments[index + 1].id)
  }
  return (
    <div className="space-y-5">
      <div>
        <p className="eyebrow">历史运行</p>
        <h2 className="section-title">直播记录与 Flow 回放</h2>
      </div>
      {error && (
        <p role="alert" className="text-sm text-rose-600 dark:text-rose-300">
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
                  片段 {item.readyTotal}/{item.segmentTotal} · ¥{formatNumber(item.costCny, 2)}
                </p>
              </button>
            ))}
            {sessions.length === 0 && (
              <p className="text-sm text-[var(--text-muted)]">暂无直播记录。</p>
            )}
          </CardContent>
        </Card>

        <div className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle className="inline-flex items-center gap-2">
                <Clapperboard className="size-4 text-[var(--accent)]" />
                片段回放
              </CardTitle>
            </CardHeader>
            <CardContent>
              {selectedSegment ? (
                <video
                  key={selectedSegment.id}
                  src={`/api/v1/ops/media/${selectedSegment.id}`}
                  className="aspect-video w-full rounded-xl bg-black object-contain"
                  controls
                  autoPlay
                  muted
                  playsInline
                  onEnded={playNext}
                />
              ) : (
                <div className="grid aspect-video place-items-center rounded-xl bg-black text-sm text-white/70">
                  该直播没有可播放片段
                </div>
              )}
              <div className="mt-3 flex gap-2 overflow-x-auto pb-1">
                {playableSegments.map((segment) => (
                  <button
                    key={segment.id}
                    type="button"
                    onClick={() => setSegmentID(segment.id)}
                    className={`inline-flex shrink-0 items-center gap-1 rounded-lg border px-3 py-2 text-xs font-bold ${segmentID === segment.id ? "border-[var(--accent)] bg-[var(--accent)] text-slate-950" : "border-[var(--line)] bg-[var(--surface-raised)]"}`}
                  >
                    <Play className="size-3" />#{segment.sequence}
                  </button>
                ))}
              </div>
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
                    onClick={() => segment.playable && setSegmentID(segment.id)}
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
