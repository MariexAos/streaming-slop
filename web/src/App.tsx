import {
  Activity,
  AlertTriangle,
  CircleDollarSign,
  Clock3,
  Gauge,
  Layers3,
  LayoutDashboard,
  Octagon,
  Pause,
  Play,
  Radio,
  RotateCcw,
  ServerCog,
  ShieldAlert,
  TimerReset,
  Settings,
  Workflow,
  WalletCards,
  History,
  Zap,
} from "lucide-react"
import type { OpsCommand } from "@/lib/api"
import { formatClock, formatDuration, formatNumber, formatPercent } from "@/lib/utils"
import { useConsole } from "@/store/useConsole"
import { NavButton, ConfirmControl, LoadingState } from "@/components/ConsoleControls"
import { toneForStatus, statusLabel, connectionLabel, reasonLabel } from "@/lib/status"
import { MetricCard } from "@/components/MetricCard"
import { StatusTable, type StatusRow } from "@/components/StatusTable"
import { Timeline } from "@/components/Timeline"
import { CostDashboard } from "@/components/CostDashboard"
import { GenerationConfigPanel } from "@/components/GenerationConfigPanel"
import { FlowDashboard } from "@/components/FlowDashboard"
import { LiveInteractionPanel } from "@/components/LiveInteractionPanel"
import { SessionHistoryDashboard } from "@/components/SessionHistoryDashboard"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { TooltipProvider } from "@/components/ui/tooltip"

export function App() {
  const {
    store,
    view,
    setView,
    generationConfig,
    bilibiliConfig,
    qwenConfig,
    configSaving,
    configMessage,
    configError,
    bilibiliSaving,
    bilibiliMessage,
    bilibiliError,
    mockSaving,
    mockMessage,
    mockError,
    qwenSaving,
    qwenMessage,
    qwenError,
    flowCatalog,
    currentFlow,
    flowError,
    saveGenerationConfig,
    saveBilibiliConfig,
    sendMock,
    saveQwenConfig,
  } = useConsole()

  if (!store.snapshot) {
    return <LoadingState message={store.dataError} />
  }

  const { snapshot, history } = store
  const command = (value: OpsCommand) => void store.runCommand(value)
  const statusTone = toneForStatus(snapshot.session.status)
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
    <TooltipProvider delayDuration={250}>
      <div className="min-h-screen">
        <header className="sticky top-0 z-40 border-b border-[var(--line)] bg-[color-mix(in_srgb,var(--background)_88%,transparent)] backdrop-blur-xl">
          <div className="mx-auto flex max-w-[1600px] flex-col gap-4 px-4 py-4 sm:px-6 xl:flex-row xl:items-center xl:justify-between xl:px-8">
            <div className="flex min-w-0 items-center gap-3">
              <div className="grid size-11 shrink-0 place-items-center rounded-2xl bg-[var(--brand)] text-slate-950 shadow-[0_0_28px_color-mix(in_srgb,var(--accent)_28%,transparent)]">
                <Radio className="size-5" aria-hidden="true" />
              </div>
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2">
                  <h1 className="truncate text-base font-bold tracking-tight text-[var(--text)]">
                    直播生成控制台
                  </h1>
                  <Badge tone={statusTone}>
                    <span className="status-dot" />
                    {statusLabel(snapshot.session.status)}
                  </Badge>
                </div>
                <p className="mt-0.5 truncate font-mono text-[11px] text-[var(--text-dim)]">
                  {snapshot.session.id ?? "暂无活动会话"} · 版本 {snapshot.revision}
                </p>
              </div>
            </div>

            <nav
              className="flex items-center gap-1 rounded-xl border border-[var(--line)] bg-[var(--surface-raised)] p-1"
              aria-label="管理看板"
            >
              <NavButton
                active={view === "operations"}
                onClick={() => setView("operations")}
                icon={LayoutDashboard}
              >
                运行
              </NavButton>
              <NavButton active={view === "flow"} onClick={() => setView("flow")} icon={Workflow}>
                Flow
              </NavButton>
              <NavButton
                active={view === "history"}
                onClick={() => setView("history")}
                icon={History}
              >
                记录
              </NavButton>
              <NavButton
                active={view === "cost"}
                onClick={() => setView("cost")}
                icon={WalletCards}
              >
                成本
              </NavButton>
              <NavButton
                active={view === "config"}
                onClick={() => setView("config")}
                icon={Settings}
              >
                配置
              </NavButton>
            </nav>

            <div className="flex flex-wrap items-center gap-2">
              <Badge tone={store.connection === "open" ? "success" : "warning"}>
                <Activity className="size-3" aria-hidden="true" />
                实时连接 {connectionLabel(store.connection)}
              </Badge>
              <Badge tone={toneForStatus(snapshot.generation.mode)}>
                <ServerCog className="size-3" aria-hidden="true" />
                生成器 {statusLabel(snapshot.generation.mode)}
              </Badge>
              <Badge tone={toneForStatus(snapshot.stream.status)}>
                <Radio className="size-3" aria-hidden="true" />
                推流 {statusLabel(snapshot.stream.status)}
              </Badge>
              <span className="mx-1 hidden h-7 w-px bg-[var(--line)] lg:block" />
              <Button
                variant="primary"
                disabled={!snapshot.controls.canStart || store.pendingCommand !== null}
                onClick={() => command("start")}
              >
                <Play className="size-4" aria-hidden="true" />
                启动
              </Button>
              <ConfirmControl
                title="在下一个片段边界停止？"
                description="系统将停止新的规划与提交，并在下一个安全边界关闭推流。"
                confirmLabel="停止会话"
                disabled={!snapshot.controls.canStop || store.pendingCommand !== null}
                onConfirm={() => command("stop")}
              >
                <Pause className="size-4" aria-hidden="true" />
                停止
              </ConfirmControl>
              {snapshot.fallback.forced ? (
                <Button
                  variant="secondary"
                  disabled={!snapshot.controls.canDisableFallback || store.pendingCommand !== null}
                  onClick={() => command("disable-fallback")}
                >
                  <RotateCcw className="size-4" aria-hidden="true" />
                  解除备用画面
                </Button>
              ) : (
                <ConfirmControl
                  title="强制切换到备用画面？"
                  description="系统将在下一个片段边界切换到预制备用画面，缓冲恢复后可继续播放生成内容。"
                  confirmLabel="启用备用画面"
                  disabled={!snapshot.controls.canEnableFallback || store.pendingCommand !== null}
                  onConfirm={() => command("enable-fallback")}
                  danger
                >
                  <ShieldAlert className="size-4" aria-hidden="true" />
                  强制备用画面
                </ConfirmControl>
              )}
            </div>
          </div>
        </header>

        <main className="mx-auto max-w-[1600px] space-y-5 px-4 py-6 sm:px-6 xl:px-8">
          <div aria-live="polite" className="min-h-6">
            {(store.commandError || store.dataError) && (
              <p className="inline-flex items-center gap-2 text-sm font-medium text-rose-600 dark:text-rose-300">
                <AlertTriangle className="size-4" aria-hidden="true" />
                {store.commandError ?? store.dataError}
              </p>
            )}
            {store.commandMessage && !store.commandError && (
              <p className="inline-flex items-center gap-2 text-sm font-medium text-emerald-600 dark:text-emerald-300">
                <Activity className="size-4" aria-hidden="true" />
                {store.commandMessage}
              </p>
            )}
          </div>

          {view === "operations" ? (
            <>
              <LiveInteractionPanel
                streamStatus={snapshot.stream.status}
                previewUrl={previewSegment ? `/api/v1/ops/media/${previewSegment.id}` : null}
                saving={mockSaving}
                message={mockMessage}
                error={mockError}
                onSend={sendMock}
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
                        ? (snapshot.generation.inFlight / snapshot.generation.targetConcurrency) *
                          100
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
          ) : view === "flow" ? (
            <FlowDashboard catalog={flowCatalog} current={currentFlow} error={flowError} />
          ) : view === "cost" ? (
            <CostDashboard snapshot={snapshot} config={generationConfig} history={history.cost} />
          ) : view === "history" ? (
            <SessionHistoryDashboard />
          ) : (
            <GenerationConfigPanel
              config={generationConfig}
              saving={configSaving}
              message={configMessage}
              error={configError}
              onSave={saveGenerationConfig}
              bilibiliConfig={bilibiliConfig}
              bilibiliSaving={bilibiliSaving}
              bilibiliMessage={bilibiliMessage}
              bilibiliError={bilibiliError}
              onSaveBilibili={saveBilibiliConfig}
              mockSaving={mockSaving}
              mockMessage={mockMessage}
              mockError={mockError}
              onSendMock={sendMock}
              qwenConfig={qwenConfig}
              qwenSaving={qwenSaving}
              qwenMessage={qwenMessage}
              qwenError={qwenError}
              onSaveQwen={saveQwenConfig}
            />
          )}
        </main>
      </div>
    </TooltipProvider>
  )
}
