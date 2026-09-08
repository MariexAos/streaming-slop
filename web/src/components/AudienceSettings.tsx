import { useState } from "react"
import { useBilibiliSettings, saveResult } from "@/queries/config"
import { Button } from "./ui/button"
export function AudienceSettings() {
  const { query, save } = useBilibiliSettings()
  const [draft, setDraft] = useState<string>()
  const [cookie, setCookie] = useState("")
  const room = draft ?? String(query.data?.roomId || "")
  const valid = Number.isInteger(Number(room)) && Number(room) > 0
  const submit = async () => {
    if (
      valid &&
      (await saveResult(
        save.mutateAsync({ roomId: Number(room), cookie: cookie.trim() || undefined }),
      ))
    )
      setCookie("")
  }
  return (
    <form
      className="space-y-4"
      onSubmit={(event) => {
        event.preventDefault()
        void submit()
      }}
    >
      <p className="text-sm">仅接入 Bilibili 弹幕，不会更改推流目标。本地测试无需配置。</p>
      <label className="block text-sm">
        直播间房间号
        <input
          className="mt-2 block w-full rounded-xl border p-3"
          inputMode="numeric"
          value={room}
          onChange={(event) => setDraft(event.target.value)}
        />
      </label>
      <label className="block text-sm">
        Cookie
        <input
          className="mt-2 block w-full rounded-xl border p-3"
          type="password"
          autoComplete="new-password"
          value={cookie}
          onChange={(event) => setCookie(event.target.value)}
          placeholder="留空保持不变"
        />
      </label>
      <Button type="submit" disabled={!valid || save.isPending}>
        保存弹幕配置
      </Button>
      {(query.error ?? save.error) && <p role="alert">{(query.error ?? save.error)?.message}</p>}
      {save.isSuccess && <p role="status">已保存，弹幕连接状态：{query.data?.status}</p>}
    </form>
  )
}
