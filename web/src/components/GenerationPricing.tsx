import type { GenerationConfig } from "@/lib/schema"

const units = {
  output_second: "生成视频秒",
  input_token: "输入 token",
  output_token: "输出 token",
  speech_character: "计费字符",
}
const money = new Intl.NumberFormat("zh-CN", {
  style: "currency",
  currency: "CNY",
  maximumFractionDigits: 6,
})

export function GenerationPricing({
  pricing,
  duration,
}: {
  pricing: NonNullable<GenerationConfig["pricing"]>
  duration: number
}) {
  return (
    <div className="space-y-2 py-3 text-sm">
      <p>
        标准价：{pricing.currency} {pricing.standardPrice} / {pricing.unitQuantity}{" "}
        {units[pricing.billingUnit]}
      </p>
      <p>
        当前人民币估算：{money.format(pricing.estimatedCny)} / {pricing.unitQuantity}{" "}
        {units[pricing.billingUnit]}
      </p>
      {pricing.billingUnit === "output_second" && (
        <p>
          已保存配置每 {duration} 秒约{" "}
          {money.format((pricing.estimatedCny * duration) / pricing.unitQuantity)}
        </p>
      )}
      {pricing.promotion && (
        <>
          <p>
            优惠价：{pricing.currency} {pricing.promotion.price} ·{" "}
            {pricing.promotionActive ? "生效中" : "已到期"}
          </p>
          <p>
            优惠到期：
            {pricing.promotion.expiresAt
              ? new Date(pricing.promotion.expiresAt).toLocaleString("zh-CN", { timeZone: "UTC" }) +
                " UTC"
              : "无到期时间"}
          </p>
          <p className="text-neutral-500">{pricing.promotion.note}</p>
        </>
      )}
      {pricing.currency !== "CNY" && (
        <p className="text-neutral-500">
          换算：1 {pricing.currency} ≈ ¥{pricing.exchangeRateToCny}；{pricing.exchangeRateAsOf}
          。人民币金额为估算，实际以账单为准。
        </p>
      )}
      <a className="underline" href={pricing.source} target="_blank" rel="noreferrer">
        官方计费说明
      </a>
    </div>
  )
}
