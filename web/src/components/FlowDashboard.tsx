import { useState } from "react"
import {
  ArrowRight,
  Bot,
  Camera,
  CheckCircle2,
  Clock3,
  Film,
  MessageCircleMore,
  Orbit,
  RadioTower,
  Sparkles,
  UserRound,
} from "lucide-react"
import type { CurrentFlow, FlowCatalog } from "@/lib/schema"
import { formatNumber, formatPercent } from "@/lib/utils"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"

const anchors = [
  ["chat-live-start", "直播首帧", "调整固定机位"],
  ["chat-live-end", "直播尾帧", "回到自然聊天"],
] as const

export function FlowDashboard({
  catalog,
  current,
  error,
}: {
  catalog: FlowCatalog | null
  current: CurrentFlow | null
  error: string | null
}) {
  const active = catalog?.flows.find((flow) => flow.id === current?.flowId) ?? catalog?.flows[0]
  return (
    <div className="space-y-5">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <p className="eyebrow">导演回路</p>
          <h2 className="section-title">未来一分钟正在往哪里走</h2>
        </div>
        <div className="flex items-center gap-2">
          <Badge tone={current?.status === "running" ? "success" : "neutral"}>
            <RadioTower className="size-3" />
            {flowStatus(current?.status)}
          </Badge>
          {active && (
            <Badge tone="accent">
              {active.name} · {active.version}
            </Badge>
          )}
        </div>
      </div>

      {error && <p className="text-sm text-rose-600">{error}</p>}
      {!active || !current ? (
        <EmptyFlow />
      ) : (
        <>
          <section className="grid gap-4 xl:grid-cols-[1fr_1.45fr]">
            <AudienceCard current={current} />
            <DirectionCard current={current} />
          </section>

          <Card>
            <CardHeader>
              <CardTitle>Node 执行链</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="flex snap-x gap-2 overflow-x-auto pb-2">
                {active.nodes.map((node, index) => {
                  const run = current.nodes.find((item) => item.id === node.id)
                  return (
                    <div className="flex shrink-0 items-center gap-2" key={node.id}>
                      <div
                        className={`w-44 rounded-xl border p-3 ${run?.status === "running" ? "border-[var(--accent)] bg-[color-mix(in_srgb,var(--accent)_9%,var(--surface-raised))]" : "border-[var(--line)] bg-[var(--surface-raised)]"}`}
                      >
                        <div className="flex items-center justify-between gap-2">
                          <NodeIcon type={node.type} />
                          <Badge tone={nodeTone(run?.status)}>{nodeStatus(run?.status)}</Badge>
                        </div>
                        <p className="mt-3 text-sm font-bold text-[var(--text)]">{node.label}</p>
                        <p className="mt-1 line-clamp-2 min-h-8 text-xs leading-4 text-[var(--text-dim)]">
                          {run?.summary || "等待上游输入"}
                        </p>
                        <p className="mt-2 font-mono text-[10px] text-[var(--text-dim)]">
                          {run?.durationMs == null ? "—" : `${formatNumber(run.durationMs, 0)}ms`}
                          {run?.costCny == null ? "" : ` · ¥${formatNumber(run.costCny, 3)}`}
                        </p>
                      </div>
                      {index < active.nodes.length - 1 && (
                        <ArrowRight
                          className="size-4 shrink-0 text-[var(--text-dim)]"
                          aria-hidden="true"
                        />
                      )}
                    </div>
                  )
                })}
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>未来 60 秒 DirectionPlan</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="grid gap-2 md:grid-cols-2 xl:grid-cols-4">
                {current.beats.map((beat) => (
                  <div
                    key={beat.index}
                    className="rounded-xl border border-[var(--line)] bg-[var(--surface-raised)] p-3"
                  >
                    <div className="flex items-center justify-between">
                      <span className="font-mono text-xs text-[var(--text-dim)]">
                        {beat.startSeconds}–{beat.endSeconds}s
                      </span>
                      <Badge tone={beatTone(beat.status)}>{beatLabel(beat.status)}</Badge>
                    </div>
                    <p className="mt-3 text-sm font-bold text-[var(--text)]">{beat.intent}</p>
                    <p className="mt-2 text-xs text-[var(--text-muted)]">{modeLabel(beat.mode)}</p>
                    <p className="mt-1 truncate font-mono text-[10px] text-[var(--text-dim)]">
                      {beat.anchorFrame ?? "无需 Anchor Frame"}
                    </p>
                  </div>
                ))}
              </div>
            </CardContent>
          </Card>
        </>
      )}

      <AnchorGallery />
    </div>
  )
}

function AudienceCard({ current }: { current: CurrentFlow }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>弹幕观察窗口</CardTitle>
      </CardHeader>
      <CardContent>
        <div className="flex items-center gap-3">
          <div className="grid size-10 place-items-center rounded-xl bg-[var(--surface-raised)]">
            <MessageCircleMore className="size-5 text-[var(--accent)]" />
          </div>
          <div>
            <p className="text-2xl font-black">{current.audience.messageCount}</p>
            <p className="text-xs text-[var(--text-dim)]">
              最近 {current.audience.windowSeconds} 秒有效弹幕
            </p>
          </div>
        </div>
        <p className="mt-4 rounded-xl bg-[var(--surface-raised)] p-3 text-sm leading-6 text-[var(--text-muted)]">
          {current.audience.summary || "等待弹幕形成稳定方向。"}
        </p>
        <div className="mt-4 space-y-3">
          {current.audience.intents.map((intent) => (
            <div key={intent.label}>
              <div className="mb-1 flex justify-between text-xs">
                <span>{intent.label}</span>
                <span>{formatPercent(intent.support)}</span>
              </div>
              <div className="h-2 overflow-hidden rounded-full bg-[var(--surface-raised)]">
                <div
                  className="h-full rounded-full bg-[var(--accent)]"
                  style={{ width: `${intent.support * 100}%` }}
                />
              </div>
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  )
}

function DirectionCard({ current }: { current: CurrentFlow }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>导演方向调整</CardTitle>
      </CardHeader>
      <CardContent>
        <div className="grid gap-3 sm:grid-cols-[1fr_auto_1fr]">
          <div className="rounded-xl bg-[var(--surface-raised)] p-3">
            <p className="text-xs text-[var(--text-dim)]">当前</p>
            <p className="mt-1 font-bold">{current.direction.currentLabel}</p>
          </div>
          <ArrowRight className="hidden self-center text-[var(--accent)] sm:block" />
          <div className="rounded-xl border border-[var(--accent)] bg-[color-mix(in_srgb,var(--accent)_8%,var(--surface-raised))] p-3">
            <p className="text-xs text-[var(--text-dim)]">目标</p>
            <p className="mt-1 font-bold">{current.direction.targetLabel}</p>
          </div>
        </div>
        <p className="mt-3 text-sm leading-6 text-[var(--text-muted)]">
          {current.direction.summary}
        </p>
        <div className="mt-4 grid gap-3 sm:grid-cols-2">
          {current.direction.axes.map((axis) => (
            <div key={axis.name} className="rounded-xl border border-[var(--line)] p-3">
              <div className="flex justify-between text-xs">
                <span>{axis.name}</span>
                <span className="font-mono">
                  {axis.current} → {axis.target}
                </span>
              </div>
              <div className="relative mt-3 h-2 rounded-full bg-[var(--surface-raised)]">
                <div
                  className="absolute h-full rounded-full bg-[var(--accent)]"
                  style={{ width: `${axis.target}%` }}
                />
                <span
                  className="absolute top-1/2 size-3 -translate-x-1/2 -translate-y-1/2 rounded-full border-2 border-[var(--text)] bg-[var(--background)]"
                  style={{ left: `${axis.current}%` }}
                />
              </div>
            </div>
          ))}
        </div>
        <p className="mt-4 flex items-center gap-2 text-xs text-[var(--text-dim)]">
          <Clock3 className="size-3.5" />
          计划覆盖 {current.direction.horizonSeconds} 秒，预计{" "}
          {formatNumber(current.direction.effectiveInSeconds, 0)} 秒后开始影响画面
        </p>
      </CardContent>
    </Card>
  )
}

function AnchorGallery() {
  return (
    <Card>
      <CardHeader>
        <CardTitle>普通聊天直播首尾帧</CardTitle>
      </CardHeader>
      <CardContent>
        <div className="grid gap-3 sm:grid-cols-2">
          {anchors.map(([file, label, use]) => (
            <AnchorCard key={file} file={file} label={label} use={use} />
          ))}
        </div>
      </CardContent>
    </Card>
  )
}

function AnchorCard({ file, label, use }: { file: string; label: string; use: string }) {
  const [missing, setMissing] = useState(false)
  return (
    <div className="overflow-hidden rounded-xl border border-[var(--line)] bg-[var(--surface-raised)]">
      <div className="aspect-video bg-[var(--surface-hover)]">
        {missing ? (
          <div className="grid h-full place-items-center text-[var(--text-dim)]">
            <Camera className="size-8" />
          </div>
        ) : (
          <img
            className="h-full w-full object-cover"
            src={`/anchorframes/${file}.png`}
            alt={`${label} Anchor Frame`}
            onError={() => setMissing(true)}
          />
        )}
      </div>
      <div className="flex items-center justify-between gap-3 p-3">
        <div>
          <p className="text-sm font-bold">{label}</p>
          <p className="text-xs text-[var(--text-dim)]">{use}</p>
        </div>
        <Badge tone={missing ? "warning" : "success"}>{missing ? "待生成" : "已就绪"}</Badge>
      </div>
    </div>
  )
}

function EmptyFlow() {
  return (
    <Card>
      <CardContent className="grid min-h-48 place-items-center py-10 text-center">
        <div>
          <Orbit className="mx-auto size-8 text-[var(--accent)]" />
          <p className="mt-3 font-bold">等待 Flow 运行数据</p>
          <p className="mt-1 text-sm text-[var(--text-dim)]">
            连接后将显示弹幕如何改变未来一分钟的导演方向。
          </p>
        </div>
      </CardContent>
    </Card>
  )
}
function NodeIcon({ type }: { type: string }) {
  const Icon = type.includes("audience")
    ? MessageCircleMore
    : type.includes("director")
      ? Bot
      : type.includes("generation") || type.includes("prompt")
        ? Film
        : type.includes("policy")
          ? CheckCircle2
          : type.includes("character")
            ? UserRound
            : Sparkles
  return <Icon className="size-4 text-[var(--accent)]" />
}
function flowStatus(value?: CurrentFlow["status"]) {
  return (
    { idle: "待机", running: "运行中", completed: "已完成", failed: "失败" } as Record<
      string,
      string
    >
  )[value ?? "idle"]
}
function nodeStatus(value?: CurrentFlow["nodes"][number]["status"]) {
  return (
    {
      pending: "等待",
      running: "执行中",
      completed: "完成",
      failed: "失败",
      skipped: "跳过",
    } as Record<string, string>
  )[value ?? "pending"]
}
function nodeTone(
  value?: CurrentFlow["nodes"][number]["status"],
): "neutral" | "accent" | "success" | "danger" {
  return value === "running"
    ? "accent"
    : value === "completed"
      ? "success"
      : value === "failed"
        ? "danger"
        : "neutral"
}
function beatTone(
  value: CurrentFlow["beats"][number]["status"],
): "neutral" | "accent" | "success" | "warning" {
  return value === "ready"
    ? "success"
    : value === "generating"
      ? "warning"
      : value === "submitted"
        ? "accent"
        : "neutral"
}
function beatLabel(value: CurrentFlow["beats"][number]["status"]) {
  return (
    {
      planned: "计划",
      submitted: "已提交",
      generating: "生成中",
      ready: "就绪",
      locked: "已锁定",
    } as Record<string, string>
  )[value]
}
function modeLabel(value: CurrentFlow["beats"][number]["mode"]) {
  return (
    {
      text_to_video: "文生视频",
      first_frame_to_video: "首帧图生视频",
      first_last_frame_to_video: "首尾帧图生视频",
    } as Record<string, string>
  )[value]
}
