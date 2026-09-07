import { cn } from "@/lib/utils"

export function Progress({ value, className }: { value: number; className?: string }) {
  const normalized = Math.min(100, Math.max(0, value))
  return (
    <div
      role="progressbar"
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={Math.round(normalized)}
      className={cn("h-1.5 overflow-hidden rounded-full bg-[var(--track)]", className)}
    >
      <div
        className="h-full rounded-full bg-[var(--accent)] transition-[width] duration-500 motion-reduce:transition-none"
        style={{ width: `${normalized}%` }}
      />
    </div>
  )
}
