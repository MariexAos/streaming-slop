import type { HTMLAttributes } from "react"
import { cn } from "@/lib/utils"

type BadgeProps = HTMLAttributes<HTMLSpanElement> & {
  tone?: "neutral" | "success" | "warning" | "danger" | "accent"
}

const tones = {
  neutral: "border-[var(--line)] bg-[var(--surface-raised)] text-[var(--text-muted)]",
  success: "border-emerald-500/25 bg-emerald-500/10 text-emerald-600",
  warning: "border-amber-500/25 bg-amber-500/10 text-amber-700",
  danger: "border-rose-500/25 bg-rose-500/10 text-rose-600",
  accent: "border-[var(--line)] bg-[var(--surface-raised)] text-[var(--text)]",
}

export function Badge({ className, tone = "neutral", ...props }: BadgeProps) {
  return (
    <span
      className={cn(
        "inline-flex min-h-6 items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-[11px] font-medium",
        tones[tone],
        className,
      )}
      {...props}
    />
  )
}
