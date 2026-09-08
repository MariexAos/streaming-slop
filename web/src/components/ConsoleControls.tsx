import type { LucideIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog"

export function NavButton({
  active,
  onClick,
  icon: Icon,
  children,
}: {
  active: boolean
  onClick: () => void
  icon: LucideIcon
  children: React.ReactNode
}) {
  return (
    <button
      type="button"
      aria-current={active ? "page" : undefined}
      onClick={onClick}
      className={`inline-flex min-h-11 items-center gap-1.5 rounded-lg px-3 text-xs font-bold transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--accent)] ${active ? "bg-[var(--accent)] text-white" : "text-[var(--text-muted)] hover:bg-[var(--surface-hover)] hover:text-[var(--text)]"}`}
    >
      <Icon className="size-3.5" aria-hidden="true" />
      {children}
    </button>
  )
}

export function ConfirmControl({
  children,
  title,
  description,
  confirmLabel,
  disabled,
  onConfirm,
  danger = false,
}: {
  children: React.ReactNode
  title: string
  description: string
  confirmLabel: string
  disabled: boolean
  onConfirm: () => void
  danger?: boolean
}) {
  return (
    <AlertDialog>
      <AlertDialogTrigger asChild>
        <Button variant={danger ? "danger" : "secondary"} disabled={disabled}>
          {children}
        </Button>
      </AlertDialogTrigger>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{title}</AlertDialogTitle>
          <AlertDialogDescription>{description}</AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel asChild>
            <Button variant="ghost">取消</Button>
          </AlertDialogCancel>
          <AlertDialogAction asChild>
            <Button variant={danger ? "danger" : "primary"} onClick={onConfirm}>
              {confirmLabel}
            </Button>
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
