import { forwardRef, type ButtonHTMLAttributes } from "react"
import { cn } from "@/lib/utils"

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: "primary" | "secondary" | "danger" | "ghost"
  size?: "default" | "icon"
}

const variants = {
  primary: "bg-[var(--accent)] text-white hover:bg-black",
  secondary:
    "border border-[var(--line-strong)] bg-[var(--surface-raised)] text-[var(--text)] hover:bg-[var(--surface-hover)]",
  danger: "bg-[var(--danger)] text-white hover:brightness-110",
  ghost: "text-[var(--text-muted)] hover:bg-[var(--surface-hover)] hover:text-[var(--text)]",
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant = "secondary", size = "default", type = "button", ...props }, ref) => (
    <button
      ref={ref}
      type={type}
      className={cn(
        "inline-flex min-h-11 items-center justify-center gap-2 rounded-xl px-4 text-sm font-semibold transition disabled:pointer-events-none disabled:opacity-40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--accent)] focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--background)]",
        size === "icon" && "size-11 px-0",
        variants[variant],
        className,
      )}
      {...props}
    />
  ),
)
Button.displayName = "Button"
