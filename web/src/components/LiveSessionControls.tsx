import type { OpsSnapshot } from "@/lib/schema"
import { journeyState } from "@/lib/journey"
import { useLiveCommand } from "@/queries/journey"
import { ConfirmControl } from "./ConsoleControls"
import { Button } from "./ui/button"

export function LiveSessionControls({ snapshot }: { snapshot: OpsSnapshot }) {
  const command = useLiveCommand()
  const state = journeyState(snapshot)
  return (
    <div className="space-y-3">
      <div className="flex flex-wrap gap-2">
        {snapshot.controls.canStop && (
          <ConfirmControl
            title={state.preparing ? "停止本次准备？" : "结束本场直播？"}
            description="系统将停止新的生成并安全关闭推流。已提交的生成任务可能继续计费。"
            confirmLabel="停止会话"
            disabled={command.isPending}
            onConfirm={() => command.mutate("stop")}
          >
            {state.preparing ? "停止准备" : "结束直播"}
          </ConfirmControl>
        )}
        {snapshot.fallback.forced ? (
          <Button disabled={command.isPending} onClick={() => command.mutate("disable-fallback")}>
            恢复生成画面
          </Button>
        ) : (
          snapshot.controls.canEnableFallback && (
            <details>
              <summary className="px-3 py-3 text-sm">更多操作</summary>
              <Button
                disabled={command.isPending}
                onClick={() => command.mutate("enable-fallback")}
              >
                切换备用画面
              </Button>
            </details>
          )
        )}
      </div>
      {command.error && (
        <p role="alert" className="text-sm text-rose-600">
          {command.error.message}
        </p>
      )}
      {command.isSuccess && (
        <p role="status" className="text-sm">
          请求已接收，等待运行状态更新。
        </p>
      )}
    </div>
  )
}
