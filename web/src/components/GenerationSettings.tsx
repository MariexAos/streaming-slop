import { useModelServices } from "@/queries/services"
import { ProviderCredential } from "./ProviderCredential"
import { GenerationPricing } from "./GenerationPricing"
import { Card, CardHeader, CardTitle, CardContent } from "./ui/card"
export function GenerationSettings() {
  const { query } = useModelServices()
  if (query.error) return <p role="alert">{query.error.message}</p>
  if (!query.data) return <p>正在加载模型服务…</p>
  const data = query.data
  return (
    <Card>
      <CardHeader>
        <CardTitle>模型服务</CardTitle>
      </CardHeader>
      <CardContent className="space-y-6">
        <p className="text-sm text-neutral-500">
          分别管理供应商凭据。视频供应商双活，在运行页可随时切换后续任务的模型，无需重启。
        </p>
        <div className="grid gap-4 md:grid-cols-2">
          {data.credentials.map((c) => (
            <ProviderCredential key={c.provider} {...c} />
          ))}
        </div>
        {(["video", "text"] as const).map((kind) => (
          <section
            key={kind}
            className="space-y-3"
            aria-label={kind === "video" ? "视频生成模型" : "文本与视觉推理模型"}
          >
            <h3 className="font-semibold">
              {kind === "video" ? "视频生成模型" : "文本与视觉推理模型"}
            </h3>
            <p className="text-sm text-neutral-500">
              {kind === "video"
                ? "按输出视频秒计费，直播统一输出 720P。"
                : "用于理解弹幕和规划动作；输入、输出 token 分别计费。"}
            </p>
            {data.options
              .filter((o) => o.kind === kind)
              .map((option) => (
                <details key={option.provider + option.model} className="rounded-xl border p-4">
                  <summary className="cursor-pointer text-sm">
                    {option.provider === "minimax" ? "MiniMax" : "fal"} · {option.model} · 计费详情
                  </summary>
                  {option.prices.map((price) => (
                    <div key={price.variant} className="mt-4 border-t pt-3">
                      <p className="text-sm font-medium">
                        {kind === "video"
                          ? price.variant
                          : price.billingUnit === "input_token"
                            ? "输入 token（标准服务，≤512k 上下文）"
                            : "输出 token（标准服务，≤512k 上下文）"}
                      </p>
                      <GenerationPricing pricing={price} duration={5} />
                    </div>
                  ))}
                </details>
              ))}
          </section>
        ))}
      </CardContent>
    </Card>
  )
}
