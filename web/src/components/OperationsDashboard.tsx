import { useSnapshot } from "@/queries/ops"
import { useMockDanmaku, saveResult } from "@/queries/config"
import { journeyState } from "@/lib/journey"
import { LivePreparation } from "./LivePreparation"
import { RuntimeDetails } from "./RuntimeDetails"
import { LiveInteractionPanel } from "./LiveInteractionPanel"
export function OperationsDashboard({ onHistory }: { onHistory: () => void }) {
  const query = useSnapshot()
  const mock = useMockDanmaku()
  const snapshot = query.data
  if (!snapshot) return <p role="status">正在读取直播状态…</p>
  const state = journeyState(snapshot)
  return (
    <>
      <LivePreparation snapshot={snapshot} onHistory={onHistory} />
      {state.active && (
        <LiveInteractionPanel
          streamStatus={snapshot.stream.status}
          interactive={state.live}
          saving={mock.isPending}
          message={snapshot.interaction ?? (mock.isSuccess ? "已收到，等待规划采用。" : null)}
          error={mock.error?.message ?? null}
          onSend={(value) => saveResult(mock.mutateAsync(value))}
        />
      )}
      <RuntimeDetails />
    </>
  )
}
