import { CharacterPanel } from "./CharacterPanel"
import { GenerationSettings } from "./GenerationSettings"
import { AudienceSettings } from "./AudienceSettings"
import { BackupSettings } from "./BackupSettings"
export function ConfigPage() {
  return (
    <div className="space-y-5">
      <div>
        <p className="eyebrow">开播配置</p>
        <h2 className="text-2xl font-semibold">为下一场直播做好准备</h2>
        <p className="mt-2 text-sm text-[var(--text-muted)]">
          修改后保存，开播前会重新检查。人物变更用于下一场。
        </p>
      </div>
      <CharacterPanel />
      <GenerationSettings />
      <details className="rounded-2xl border p-5">
        <summary className="py-2 text-sm font-semibold">Bilibili 弹幕接入（可选）</summary>
        <AudienceSettings />
      </details>
      <details className="rounded-2xl border p-5">
        <summary className="py-2 text-sm font-semibold">Qwen 备用配置（高级）</summary>
        <BackupSettings />
      </details>
    </div>
  )
}
