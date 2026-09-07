import { useState } from "react"
import { MessageCircleMore, Radio, Send } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"

export function LiveInteractionPanel({ streamStatus, previewUrl, saving, message, error, onSend }: {
  streamStatus: string
  previewUrl: string | null
  saving: boolean
  message: string | null
  error: string | null
  onSend: (value: { username: string; text: string }) => Promise<boolean>
}) {
  const [username, setUsername] = useState("本地观众")
  const [text, setText] = useState("")
  const fieldClass = "min-h-11 w-full rounded-xl border border-[var(--line-strong)] bg-[var(--surface-raised)] px-3 text-sm text-[var(--text)] outline-none focus:ring-2 focus:ring-[var(--accent)]"

  const send = async () => {
    const content = text.trim()
    if (!content) return
    if (await onSend({ username: username.trim() || "本地观众", text: content })) setText("")
  }

  return (
    <section className="grid gap-3 xl:grid-cols-[minmax(0,2fr)_minmax(320px,1fr)]" aria-label="直播预览与互动">
      <Card className="overflow-hidden">
        <CardHeader><CardTitle className="inline-flex items-center gap-2"><Radio className="size-4 text-[var(--accent)]" />本地直播预览</CardTitle></CardHeader>
        <CardContent>
          {streamStatus === "live" ? (
            <iframe
              title="MediaMTX 直播预览"
              src="http://127.0.0.1:8888/live-test/?autoplay=true&muted=true&controls=true&playsinline=true"
              className="aspect-video w-full rounded-xl border border-[var(--line)] bg-black"
              allow="autoplay; fullscreen"
            />
          ) : previewUrl ? (
            <video
              key={previewUrl}
              src={previewUrl}
              className="aspect-video w-full rounded-xl border border-[var(--line)] bg-black object-contain"
              controls
              autoPlay
              muted
              loop
              playsInline
            />
          ) : (
            <div className="relative aspect-video overflow-hidden rounded-xl border border-[var(--line)] bg-black">
              <img src="/anchorframes/chat-live-start.png" alt="主播预览占位画面" className="size-full object-cover opacity-55" />
              <div className="absolute inset-0 grid place-items-center bg-black/35 p-6 text-center text-white">
                <div><Radio className="mx-auto size-7" /><p className="mt-3 font-bold">{previewStatusLabel(streamStatus)}</p><p className="mt-1 text-xs text-white/75">首个片段生成后会在这里直接播放</p></div>
              </div>
            </div>
          )}
        </CardContent>
      </Card>
      <Card>
        <CardHeader><CardTitle className="inline-flex items-center gap-2"><MessageCircleMore className="size-4 text-[var(--accent)]" />发送测试弹幕</CardTitle></CardHeader>
        <CardContent>
          <form className="space-y-3" onSubmit={(event) => { event.preventDefault(); void send() }}>
            <label className="block text-xs font-semibold text-[var(--text-muted)]">昵称
              <input className={`${fieldClass} mt-2`} value={username} maxLength={40} onChange={(event) => setUsername(event.target.value)} />
            </label>
            <label className="block text-xs font-semibold text-[var(--text-muted)]">弹幕内容
              <input className={`${fieldClass} mt-2`} value={text} maxLength={200} onChange={(event) => setText(event.target.value)} placeholder="例如：聊聊你桌上的东西" />
            </label>
            <Button type="submit" variant="primary" disabled={saving || !text.trim()}>
              <Send className="size-4" aria-hidden="true" />{saving ? "发送中…" : "发送并影响后续方向"}
            </Button>
          </form>
          <div aria-live="polite" className="mt-3 min-h-10 text-xs leading-5">
            {error ? <span className="text-rose-600 dark:text-rose-300">{error}</span> : message ? <span className="text-emerald-600 dark:text-emerald-300">{message}</span> : <span className="text-[var(--text-dim)]">本地与 Bilibili 弹幕会合并进同一个最近 20 秒窗口，再交给 Observer。</span>}
          </div>
        </CardContent>
      </Card>
    </section>
  )
}

function previewStatusLabel(status: string) {
  if (status === "starting") return "正在准备直播画面"
  if (status === "failed") return "推流启动失败"
  return "推流已停止"
}
