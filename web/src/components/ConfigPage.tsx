import { useQuery } from "@tanstack/react-query"
import { updateGenerationConfig, updateBilibiliConfig, updateQwenConfig } from "@/lib/api"
import {
  generationOptions,
  bilibiliOptions,
  qwenOptions,
  useConfigMutation,
  useMockDanmaku,
  saveResult,
} from "@/queries/config"
import { CharacterPanel } from "./CharacterPanel"
import { GenerationConfigPanel } from "./GenerationConfigPanel"

export function ConfigPage() {
  const generation = useQuery(generationOptions)
  const bilibili = useQuery(bilibiliOptions)
  const qwen = useQuery(qwenOptions)
  const saveGeneration = useConfigMutation(generationOptions.queryKey, updateGenerationConfig)
  const saveBilibili = useConfigMutation(bilibiliOptions.queryKey, updateBilibiliConfig)
  const saveQwen = useConfigMutation(qwenOptions.queryKey, updateQwenConfig)
  const mock = useMockDanmaku()
  return (
    <div className="space-y-5">
      <div>
        <p className="eyebrow">开播配置</p>
        <h2 className="text-3xl font-semibold tracking-tight">为下一场直播做好准备</h2>
        <p className="mt-2 text-sm text-[var(--text-muted)]">
          先确认人物，再配置生成服务。保存后返回「运行」启动直播。
        </p>
      </div>
      <CharacterPanel />
      <GenerationConfigPanel
        config={generation.data ?? null}
        saving={saveGeneration.isPending}
        message={saveGeneration.isSuccess ? "配置已保存，将用于后续新任务。" : null}
        error={(saveGeneration.error ?? generation.error)?.message ?? null}
        onSave={(value) => saveResult(saveGeneration.mutateAsync(value))}
        bilibiliConfig={bilibili.data ?? null}
        bilibiliSaving={saveBilibili.isPending}
        bilibiliMessage={saveBilibili.isSuccess ? "配置已保存，正在连接直播间弹幕。" : null}
        bilibiliError={(saveBilibili.error ?? bilibili.error)?.message ?? null}
        onSaveBilibili={(value) => saveResult(saveBilibili.mutateAsync(value))}
        qwenConfig={qwen.data ?? null}
        qwenSaving={saveQwen.isPending}
        qwenMessage={saveQwen.isSuccess ? "Qwen 备用配置已保存；当前使用 M3。" : null}
        qwenError={(saveQwen.error ?? qwen.error)?.message ?? null}
        onSaveQwen={(value) => saveResult(saveQwen.mutateAsync(value))}
        mockSaving={mock.isPending}
        mockMessage={mock.isSuccess ? "模拟弹幕已进入最近 20 秒窗口。" : null}
        mockError={mock.error?.message ?? null}
        onSendMock={(value) => saveResult(mock.mutateAsync(value))}
      />
    </div>
  )
}
