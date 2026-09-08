import { useState } from "react"
import { useQwenSettings, saveResult } from "@/queries/config"
import { Button } from "./ui/button"
export function BackupSettings() {
  const { query, save } = useQwenSettings()
  const [draft, setDraft] = useState<string>()
  const [key, setKey] = useState("")
  const base = draft ?? query.data?.baseUrl ?? ""
  const submit = async () => {
    if (
      await saveResult(save.mutateAsync({ baseUrl: base.trim(), apiKey: key.trim() || undefined }))
    )
      setKey("")
  }
  return (
    <form
      className="space-y-4"
      onSubmit={(event) => {
        event.preventDefault()
        void submit()
      }}
    >
      <p className="text-sm">当前使用 MiniMax M3。此处仅保存 Qwen 备用配置。</p>
      <label className="block text-sm">
        OpenAI 兼容 Base URL
        <input
          className="mt-2 block w-full rounded-xl border p-3"
          value={base}
          onChange={(event) => setDraft(event.target.value)}
        />
      </label>
      <label className="block text-sm">
        Qwen API Key
        <input
          className="mt-2 block w-full rounded-xl border p-3"
          type="password"
          autoComplete="new-password"
          value={key}
          onChange={(event) => setKey(event.target.value)}
        />
      </label>
      <Button type="submit" disabled={!base.trim() || save.isPending}>
        保存 Qwen 配置
      </Button>
      {(query.error ?? save.error) && <p role="alert">{(query.error ?? save.error)?.message}</p>}
      {save.isSuccess && <p role="status">备用配置已保存。</p>}
    </form>
  )
}
