import type { HTMLAttributes } from "react"
import { cn } from "@/lib/utils"

type BadgeProps = HTMLAttributes<HTMLSpanElement> & {
  tone?: "neutral" | "success" | "warning" | "danger" | "accent"
}

const tones = {
  neutral: "border-[var(--line)] bg-[var(--surface-raised)] text-[var(--text-muted)]",
  success: "border-emerald-500/25 bg-emerald-500/10 text-emerald-600 dark:text-emerald-300",
  warning: "border-amber-500/25 bg-amber-500/10 text-amber-700 dark:text-amber-300",
  danger: "border-rose-500/25 bg-rose-500/10 text-rose-600 dark:text-rose-300",
  accent: "border-cyan-500/25 bg-cyan-500/10 text-cyan-700 dark:text-cyan-300",
}

export function Badge({ className, tone = "neutral", ...props }: BadgeProps) {
  return (
    <span
      className={cn(
        "inline-flex min-h-6 items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-[11px] font-bold uppercase tracking-[0.12em]",
        tones[tone],
        className,
      )}
      {...props}
    />
  )
}
