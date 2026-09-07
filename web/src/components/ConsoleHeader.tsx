import type { ReactNode } from "react"
import { useMutation } from "@tanstack/react-query"
import { Activity, AlertTriangle, Pause, Play, Radio, RotateCcw, ShieldAlert } from "lucide-react"
import { sendCommand, type OpsCommand } from "@/lib/api"
import { toneForStatus, statusLabel, connectionLabel } from "@/lib/status"
import { useSnapshot } from "@/queries/ops"
import { useOpsStore } from "@/store/ops"
import { ConfirmControl, LoadingState } from "./ConsoleControls"
import { Badge } from "./ui/badge"
import { Button } from "./ui/button"

export function ConsoleHeader({ children }: { children: ReactNode }) {
  const query = useSnapshot()
  const connection = useOpsStore((state) => state.connection)
  const dataError = useOpsStore((state) => state.dataError)
  const mutation = useMutation({ mutationFn: sendCommand })
  const command = (value: OpsCommand) => mutation.mutate(value)
  const labels: Record<string, string> = {
    start: "启动",
    stop: "停止",
    "enable-fallback": "启用备用画面",
    "disable-fallback": "解除备用画面",
  }
  const commandMessage = mutation.isSuccess
    ? `命令已接受：${labels[mutation.data.command] ?? mutation.data.command}`
    : null
  const snapshot = query.data
  if (!snapshot)
    return (
      <>
        <LoadingState message={query.error?.message ?? dataError} />
        {children}
      </>
    )
  const statusTone = toneForStatus(snapshot.session.status)
  return (
    <>
      <header className="sticky top-0 z-40 border-b border-[var(--line)] bg-[color-mix(in_srgb,var(--background)_88%,transparent)] backdrop-blur-xl">
        <div className="mx-auto flex max-w-[1600px] flex-col gap-4 px-4 py-4 sm:px-6 xl:flex-row xl:items-center xl:justify-between xl:px-8">
          <div className="flex min-w-0 items-center gap-3">
            <div className="grid size-11 shrink-0 place-items-center rounded-2xl bg-[var(--brand)] text-white">
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

          <div className="flex flex-wrap items-center gap-2">
            <Badge tone={connection === "open" ? "success" : "warning"}>
              <Activity className="size-3" aria-hidden="true" />
              实时连接 {connectionLabel(connection)}
            </Badge>
            <span className="mx-1 hidden h-7 w-px bg-[var(--line)] lg:block" />
            <Button
              variant="primary"
              disabled={!snapshot.controls.canStart || mutation.isPending}
              onClick={() => command("start")}
            >
              <Play className="size-4" aria-hidden="true" />
              启动
            </Button>
            <ConfirmControl
              title="在下一个片段边界停止？"
              description="系统将停止新的规划与提交，并在下一个安全边界关闭推流。"
              confirmLabel="停止会话"
              disabled={!snapshot.controls.canStop || mutation.isPending}
              onConfirm={() => command("stop")}
            >
              <Pause className="size-4" aria-hidden="true" />
              停止
            </ConfirmControl>
            {snapshot.fallback.forced ? (
              <Button
                variant="secondary"
                disabled={!snapshot.controls.canDisableFallback || mutation.isPending}
                onClick={() => command("disable-fallback")}
              >
                <RotateCcw className="size-4" aria-hidden="true" />
                解除备用画面
              </Button>
            ) : snapshot.controls.canEnableFallback ? (
              <ConfirmControl
                title="强制切换到备用画面？"
                description="系统将在下一个片段边界切换到预制备用画面，缓冲恢复后可继续播放生成内容。"
                confirmLabel="启用备用画面"
                disabled={!snapshot.controls.canEnableFallback || mutation.isPending}
                onConfirm={() => command("enable-fallback")}
              >
                <ShieldAlert className="size-4" aria-hidden="true" />
                强制备用画面
              </ConfirmControl>
            ) : null}
          </div>
        </div>
        <div className="mx-auto max-w-[1600px] overflow-x-auto px-4 pb-3 sm:px-6 xl:px-8">
          {children}
        </div>
      </header>
      <div aria-live="polite" className="mx-auto max-w-[1600px] px-4 sm:px-6 xl:px-8">
        {(mutation.error?.message ?? dataError ?? query.error?.message) && (
          <p className="inline-flex items-center gap-2 text-sm font-medium text-rose-600">
            <AlertTriangle className="size-4" aria-hidden="true" />
            {mutation.error?.message ?? dataError ?? query.error?.message}
          </p>
        )}
        {commandMessage && !mutation.error?.message && (
          <p className="inline-flex items-center gap-2 text-sm font-medium text-emerald-600">
            <Activity className="size-4" aria-hidden="true" />
            {commandMessage}
          </p>
        )}
      </div>
    </>
  )
}
