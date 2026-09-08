import { useGuestStatus, useGuestMessage } from "@/queries/guest"
import { LiveInteractionPanel } from "./LiveInteractionPanel"

export function GuestPage() {
  const status = useGuestStatus()
  const message = useGuestMessage()
  const send = async (value: { username: string; text: string }) => {
    try {
      await message.mutateAsync(value)
      return true
    } catch {
      return false
    }
  }
  return (
    <main className="mx-auto min-h-screen max-w-6xl space-y-6 bg-white p-5 text-black sm:p-8">
      <header>
        <h1 className="text-2xl font-semibold">Streaming Slop</h1>
        <p className="mt-2 text-sm text-neutral-500">
          看看直播，发条消息。互动会在后续画面中体现。
        </p>
      </header>
      {status.error ? <p role="alert">连接暂时中断，正在重新连接…</p> : null}
      <LiveInteractionPanel
        streamStatus={status.data?.streamStatus ?? "stopped"}
        interactive={status.data?.interactive ?? false}
        saving={message.isPending}
        message={message.isSuccess ? "已发送，等待后续回应。" : (status.data?.message ?? null)}
        error={message.error?.message ?? null}
        onSend={send}
      />
      {!status.data?.interactive && (
        <p role="status" className="text-sm text-neutral-500">
          主播还没开播，稍等一会儿。
        </p>
      )}
    </main>
  )
}
