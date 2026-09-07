import type { MetricPoint } from "@/store/ops"

export function Sparkline({
  points,
  tone = "accent",
}: {
  points: MetricPoint[]
  tone?: "accent" | "warm"
}) {
  const values = points.flatMap((point, index) =>
    point.value === null ? [] : [{ x: index, value: point.value }],
  )
  if (values.length < 2) {
    return <div className="h-10 rounded-lg bg-[var(--surface-raised)]" aria-hidden="true" />
  }

  const min = Math.min(...values.map((point) => point.value))
  const max = Math.max(...values.map((point) => point.value))
  const spread = max - min || 1
  const width = 180
  const height = 40
  const path = values
    .map((point, index) => {
      const x = (point.x / Math.max(points.length - 1, 1)) * width
      const y = height - 4 - ((point.value - min) / spread) * (height - 8)
      return `${index === 0 ? "M" : "L"}${x.toFixed(1)} ${y.toFixed(1)}`
    })
    .join(" ")

  return (
    <svg
      viewBox={`0 0 ${width} ${height}`}
      className="h-10 w-full"
      preserveAspectRatio="none"
      aria-hidden="true"
    >
      <path
        d={path}
        fill="none"
        stroke={tone === "warm" ? "var(--warm)" : "var(--accent)"}
        strokeWidth="2.5"
        vectorEffect="non-scaling-stroke"
      />
    </svg>
  )
}
