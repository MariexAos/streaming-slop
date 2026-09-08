import type { SessionBudgetState } from "@/lib/budget"
import { Button } from "./ui/button"

export function SessionBudget({
  active,
  data,
  amount,
  onChange,
  onSave,
  dirty,
  valid,
  saving,
  error,
}: {
  active: boolean
  data?: SessionBudgetState
  amount: string
  onChange: (value: string) => void
  onSave: () => void
  dirty: boolean
  valid: boolean
  saving: boolean
  error?: string
}) {
  return (
    <div className="space-y-3 rounded-2xl border p-5" aria-label="本场预算">
      <h3 className="font-semibold">本场预算</h3>
      {!data ? (
        <p>正在加载预算…</p>
      ) : active ? (
        <>
          <p>
            上限 ¥{(data.limitMicros / 1e6).toFixed(2)} · 已结算 ¥
            {(data.chargedMicros / 1e6).toFixed(4)} · 在途预占 ¥
            {(data.reservedMicros / 1e6).toFixed(4)}
          </p>
          <p className="text-sm">
            剩余可用 ¥
            {(
              Math.max(0, data.limitMicros - data.chargedMicros - data.reservedMicros) / 1e6
            ).toFixed(2)}
          </p>
          <p className="text-sm text-neutral-500">
            视频、推理和语音共用本场额度。额度不足时不再发起付费请求；待结算费用继续保留预占。
          </p>
        </>
      ) : (
        <>
          <label htmlFor="session-budget" className="block text-sm">
            本场费用上限（人民币）
          </label>
          <div className="flex flex-wrap items-center gap-3">
            <input
              id="session-budget"
              inputMode="decimal"
              className="w-40 rounded-xl border p-3"
              value={amount}
              disabled={saving}
              onChange={(event) => onChange(event.target.value)}
            />
            {dirty && (
              <Button disabled={!valid || saving} onClick={onSave}>
                {saving ? "正在保存…" : "保存本场预算"}
              </Button>
            )}
          </div>
          {dirty && (
            <p className="text-sm" role="status">
              {valid ? "保存后才能开播。" : "请输入大于零的金额，最多两位小数。"}
            </p>
          )}
          <p className="text-sm text-neutral-500">
            每场独立计费，历史消费不扣减本场额度。开播时固定上限，包含视频、推理和语音；下次沿用此金额，可在开播前修改。
          </p>
        </>
      )}
      {error && (
        <p role="alert" className="text-sm text-rose-600">
          {error}
        </p>
      )}
    </div>
  )
}
