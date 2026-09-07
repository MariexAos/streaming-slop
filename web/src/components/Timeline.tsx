import { CirclePlay, Film, ShieldCheck } from "lucide-react"
import type { OpsSnapshot } from "@/lib/schema"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"

const WINDOW_SECONDS = 90

const segmentColors: Record<string, string> = {
  planned: "border-slate-400/30 bg-slate-400/10",
  generating: "border-violet-400/40 bg-violet-400/15",
  ready: "border-cyan-400/50 bg-cyan-400/18",
  committed: "border-blue-400/50 bg-blue-400/20",
  playing: "border-emerald-400/60 bg-emerald-400/25",
  played: "border-slate-500/20 bg-slate-500/10",
}

export function Timeline({ snapshot }: { snapshot: OpsSnapshot }) {
  const start = snapshot.buffer.playheadSeconds
  const end = start + WINDOW_SECONDS
  const segments = snapshot.timeline.filter(
    (segment) => segment.endSeconds > start && segment.startSeconds < end,
  )
  const commitPosition = Math.min(
    100,
    (snapshot.buffer.commitHorizonSeconds / WINDOW_SECONDS) * 100,
  )

  return (
    <Card>
      <CardHeader className="flex-col gap-3 sm:flex-row sm:items-center">
        <div>
          <div className="flex items-center gap-2">
            <Film className="size-4 text-[var(--accent)]" aria-hidden="true" />
            <CardTitle>时间轴窗口</CardTitle>
          </div>
          <p className="mt-1 text-xs text-[var(--text-muted)]">从当前播放点开始的未来 90 秒</p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Badge tone="success">
            <CirclePlay className="size-3" aria-hidden="true" />
            播放点 {Math.floor(start)}秒
          </Badge>
          <Badge tone="accent">
            <ShieldCheck className="size-3" aria-hidden="true" />
            提交边界 +{snapshot.buffer.commitHorizonSeconds}秒
          </Badge>
        </div>
      </CardHeader>
      <CardContent>
        <div className="overflow-x-auto pb-2">
          <div className="min-w-[760px]">
            <div className="relative mb-2 h-6 text-[10px] font-semibold text-[var(--text-dim)]">
              {[0, 15, 30, 45, 60, 75, 90].map((tick) => (
                <span
                  key={tick}
                  className="absolute -translate-x-1/2"
                  style={{ left: `${(tick / 90) * 100}%` }}
                >
                  +{tick}s
                </span>
              ))}
            </div>
            <div className="relative h-28 overflow-hidden rounded-xl border border-[var(--line)] bg-[var(--timeline)]">
              <div className="absolute inset-y-0 left-0 z-20 w-0.5 bg-emerald-400" />
              <div
                className="absolute inset-y-0 z-10 border-l border-dashed border-cyan-400/80"
                style={{ left: `${commitPosition}%` }}
              >
                <span className="absolute left-2 top-2 whitespace-nowrap text-[10px] font-bold uppercase tracking-wider text-cyan-600">
                  提交边界
                </span>
              </div>
              {[15, 30, 45, 60, 75].map((tick) => (
                <div
                  key={tick}
                  className="absolute inset-y-0 border-l border-[var(--line)]"
                  style={{ left: `${(tick / 90) * 100}%` }}
                />
              ))}
              <ol className="absolute inset-x-0 bottom-3 top-9 list-none">
                {segments.map((segment) => {
                  const segmentStart = Math.max(segment.startSeconds, start)
                  const segmentEnd = Math.min(segment.endSeconds, end)
                  const left = ((segmentStart - start) / WINDOW_SECONDS) * 100
                  const width = ((segmentEnd - segmentStart) / WINDOW_SECONDS) * 100
                  return (
                    <li
                      key={segment.id}
                      className={`absolute inset-y-0 overflow-hidden rounded-md border px-1.5 py-2 ${segmentColors[segment.status]}`}
                      style={{ left: `${left}%`, width: `calc(${width}% - 2px)` }}
                      title={`片段 ${segment.sequence}：${segmentLabel(segment.status)}，${sourceLabel(segment.source)}`}
                    >
                      <span className="block truncate font-mono text-[10px] font-bold text-[var(--text)]">
                        #{segment.sequence}
                      </span>
                      <span className="mt-1 block truncate text-[9px] uppercase tracking-wide text-[var(--text-muted)]">
                        {segmentLabel(segment.status)}
                      </span>
                      {segment.source === "fallback" && (
                        <span className="mt-1 block text-[9px] font-bold text-amber-600">备用</span>
                      )}
                    </li>
                  )
                })}
              </ol>
              {segments.length === 0 && (
                <div className="absolute inset-0 grid place-items-center text-sm text-[var(--text-muted)]">
                  当前窗口内没有片段
                </div>
              )}
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}

function segmentLabel(status: string) {
  return (
    (
      {
        planned: "已规划",
        generating: "生成中",
        ready: "已就绪",
        committed: "已提交",
        playing: "播放中",
        played: "已播放",
      } as Record<string, string>
    )[status] ?? status
  )
}

function sourceLabel(source: string) {
  return source === "fallback" ? "备用画面" : "生成画面"
}
