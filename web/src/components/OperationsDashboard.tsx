import {
  AlertTriangle,
  CircleDollarSign,
  Clock3,
  Gauge,
  Layers3,
  Octagon,
  TimerReset,
  Zap,
} from "lucide-react"
import { formatClock, formatDuration, formatNumber, formatPercent } from "@/lib/utils"
import { toneForStatus, statusLabel, reasonLabel } from "@/lib/status"
import { useSnapshot } from "@/queries/ops"
import { useMockDanmaku, saveResult } from "@/queries/config"
import { useOpsStore } from "@/store/ops"
import { MetricCard } from "./MetricCard"
import { StatusTable, type StatusRow } from "./StatusTable"
import { Timeline } from "./Timeline"
import { LiveInteractionPanel } from "./LiveInteractionPanel"

export function OperationsDashboard() {
  const query = useSnapshot()
  const history = useOpsStore((state) => state.history)
  const mock = useMockDanmaku()
  const snapshot = query.data
  if (!snapshot) return null
  const generationRows: StatusRow[] = [
    {
      label: "调度模式",
      value: statusLabel(snapshot.generation.mode),
      status: toneForStatus(snapshot.generation.mode),
    },
    {
      label: "活跃任务",
      value: `${snapshot.generation.inFlight} / ${snapshot.generation.targetConcurrency}`,
    },
    {
      label: "延迟 P50 / P95",
      value: `${formatNumber(snapshot.generation.latencyP50Seconds)}秒 / ${formatNumber(snapshot.generation.latencyP95Seconds)}秒`,
    },
    {
      label: "最近错误",
      value: snapshot.generation.lastError ?? "无",
      status: snapshot.generation.lastError ? "danger" : "success",
    },
  ]
  const streamRows: StatusRow[] = [
    {
      label: "输出状态",
      value: statusLabel(snapshot.stream.status),
      status: toneForStatus(snapshot.stream.status),
    },
    {
      label: "码率",
      value:
        snapshot.stream.bitrateKbps === null
          ? "—"
          : `${formatNumber(snapshot.stream.bitrateKbps, 0)} kbps`,
    },
    {
      label: "丢帧 / 中断",
      value: `${snapshot.stream.droppedFramesTotal} / ${snapshot.stream.gapTotal}`,
    },
    {
      label: "最近错误",
      value: snapshot.stream.lastError ?? "无",
      status: snapshot.stream.lastError ? "danger" : "success",
    },
  ]
  const fallbackRows: StatusRow[] = [
    {
      label: "模式",
      value: snapshot.fallback.active ? (snapshot.fallback.forced ? "人工强制" : "自动") : "未启用",
      status: snapshot.fallback.active ? "warning" : "success",
    },
    { label: "原因", value: reasonLabel(snapshot.fallback.reason) },
    { label: "启用时间", value: formatClock(snapshot.fallback.since) },
    { label: "累计时长", value: formatDuration(snapshot.fallback.secondsTotal) },
  ]
  const previewSegment = [...snapshot.timeline]
    .reverse()
    .find(
      (segment) =>
        segment.source === "generated" &&
        ["ready", "committed", "playing", "played"].includes(segment.status),
    )

  return (
    <>
      <LiveInteractionPanel
        streamStatus={snapshot.stream.status}
        previewUrl={previewSegment ? `/api/v1/ops/media/${previewSegment.id}` : null}
        saving={mock.isPending}
        message={mock.isSuccess ? "模拟弹幕已进入最近 20 秒窗口。" : null}
        error={mock.error?.message ?? null}
        onSend={(value) => saveResult(mock.mutateAsync(value))}
      />

      <section aria-labelledby="kpi-heading">
        <div className="mb-3 flex items-end justify-between gap-4">
          <div>
            <p className="eyebrow">运行概览</p>
            <h2 id="kpi-heading" className="section-title">
              直播安全余量
            </h2>
          </div>
          <p className="hidden text-xs text-[var(--text-dim)] sm:block">
            观测时间 {formatClock(snapshot.observedAt)} · 已运行{" "}
            {formatDuration(snapshot.session.uptimeSeconds)}
          </p>
        </div>
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4 2xl:grid-cols-8">
          <MetricCard
            label="就绪缓冲"
            value={`${formatNumber(snapshot.buffer.readySeconds)}秒`}
            detail={`目标 ${snapshot.buffer.readyTargetSeconds}秒`}
            icon={Layers3}
            history={history.ready}
            tooltip="从当前播放点开始，可连续播放且已验证的媒体时长。"
          />
          <MetricCard
            label="已提交"
            value={`${formatNumber(snapshot.buffer.submittedSeconds)}秒`}
            detail={`目标 ${snapshot.buffer.submittedTargetSeconds}秒`}
            icon={Zap}
            history={history.submitted}
            tooltip="已有媒体或正在生成任务覆盖的连续未来时长。"
          />
          <MetricCard
            label="P95 延迟"
            value={`${formatNumber(snapshot.generation.latencyP95Seconds)}秒`}
            detail={`P50 ${formatNumber(snapshot.generation.latencyP50Seconds)}秒`}
            icon={Clock3}
            history={history.latencyP95}
            tooltip="成功生成任务的耗时分布。"
            warm
          />
          <MetricCard
            label="活跃任务"
            value={`${snapshot.generation.inFlight}`}
            detail={`目标并发 ${snapshot.generation.targetConcurrency}`}
            icon={TimerReset}
            progress={
              snapshot.generation.targetConcurrency
                ? (snapshot.generation.inFlight / snapshot.generation.targetConcurrency) * 100
                : 0
            }
            tooltip="当前运行中的供应商任务与调度目标。"
          />
          <MetricCard
            label="失败率"
            value={formatPercent(snapshot.generation.failureRate)}
            detail={`累计失败 ${snapshot.generation.failuresTotal} 次`}
            icon={Octagon}
            progress={Math.min(snapshot.generation.failureRate * 100, 100)}
            tooltip="最终失败的生成尝试占比。"
            warm
          />
          <MetricCard
            label="每小时成本"
            value={
              snapshot.generation.costPerLiveHourCny === null
                ? "—"
                : `¥${formatNumber(snapshot.generation.costPerLiveHourCny, 2)}`
            }
            detail={
              snapshot.generation.costCny === null
                ? "暂无成本样本"
                : `累计 ¥${formatNumber(snapshot.generation.costCny, 2)}`
            }
            icon={CircleDollarSign}
            tooltip="按一小时直播输出折算的生成成本。"
          />
          <MetricCard
            label="输出码率"
            value={
              snapshot.stream.bitrateKbps === null
                ? "—"
                : `${formatNumber(snapshot.stream.bitrateKbps, 0)}`
            }
            detail="kbps"
            icon={Gauge}
            history={history.bitrate}
            tooltip="当前 FFmpeg 推流码率。"
          />
          <MetricCard
            label="丢帧"
            value={`${snapshot.stream.droppedFramesTotal}`}
            detail={`推流中断 ${snapshot.stream.gapTotal} 次`}
            icon={AlertTriangle}
            progress={Math.min(snapshot.stream.droppedFramesTotal, 100)}
            tooltip="输出丢帧数及记录到的推流中断次数。"
            warm
          />
        </div>
      </section>

      <section aria-label="时间轴">
        <Timeline snapshot={snapshot} />
      </section>

      <section aria-labelledby="runtime-heading">
        <div className="mb-3">
          <p className="eyebrow">运行详情</p>
          <h2 id="runtime-heading" className="section-title">
            子系统状态
          </h2>
        </div>
        <div className="grid gap-3 lg:grid-cols-3">
          <StatusTable title="内容生成" rows={generationRows} />
          <StatusTable title="推流输出" rows={streamRows} />
          <StatusTable title="备用画面" rows={fallbackRows} />
        </div>
      </section>
    </>
  )
}
