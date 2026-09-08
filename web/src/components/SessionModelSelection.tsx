import { useModelServices } from "@/queries/services"
import type { ModelOption } from "@/lib/services-api"
import { GenerationPricing } from "./GenerationPricing"
export function SessionModelSelection() {
  const { query, save } = useModelServices()
  if (query.error) return <p role="alert">{query.error.message}</p>
  if (!query.data) return <p>正在加载本场模型…</p>
  const data = query.data
  const selected = data.saved.video
  const options = data.options.filter((o) => o.kind === "video")
  const option = options.find((o) => o.provider === selected.provider && o.model === selected.model)
  const choose = (next: ModelOption) =>
    save.mutate({
      ...data.saved,
      video: {
        provider: next.provider,
        model: next.model,
        resolution: next.resolutions.find((r) => r === selected.resolution) ?? next.resolutions[0],
      },
    })
  const price = option?.prices.find((p) => p.variant === selected.resolution)
  return (
    <div className="space-y-3 rounded-2xl border p-5" aria-label="本场模型选择">
      <label className="block text-sm" htmlFor="session-video-model">
        后续视频任务供应商与模型
      </label>
      <select
        id="session-video-model"
        className="w-full rounded-xl border bg-white p-3"
        value={selected.provider + ":" + selected.model}
        disabled={save.isPending}
        onChange={(e) => {
          const next = options.find((o) => o.provider + ":" + o.model === e.target.value)
          if (next) choose(next)
        }}
      >
        {options.map((o) => (
          <option key={o.model} value={o.provider + ":" + o.model}>
            {o.provider === "minimax" ? "MiniMax 直连" : "fal"} · {o.model}
          </option>
        ))}
      </select>
      <label htmlFor="session-resolution" className="block text-sm">
        生成画质
      </label>
      <select
        id="session-resolution"
        className="w-full rounded-xl border bg-white p-3"
        value={selected.resolution}
        disabled={save.isPending}
        onChange={(e) => {
          const resolution = option?.resolutions.find((r) => r === e.target.value)
          if (resolution) save.mutate({ ...data.saved, video: { ...selected, resolution } })
        }}
      >
        {option?.resolutions.map((r) => (
          <option key={r} value={r}>
            {r}
          </option>
        ))}
      </select>
      <p className="text-sm">本场文本与视觉推理：MiniMax · {data.saved.text.model}</p>
      <p className="text-sm text-neutral-500">
        两家供应商同时可用。切换立即用于新提交的视频任务；在途任务继续由原供应商完成，文本推理不变。
      </p>
      {price && (
        <details>
          <summary className="cursor-pointer text-sm">本场视频计费预估</summary>
          <GenerationPricing pricing={price} duration={5} />
          <p className="text-sm">预算按 ¥{price.reserveCny}/秒预留，推理与语音另计。</p>
        </details>
      )}
      {save.isPending && <p role="status">正在保存本场选择…</p>}
      {save.error && <p role="alert">{save.error.message}</p>}
    </div>
  )
}
