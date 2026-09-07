import type { LucideIcon } from "lucide-react"
import { Radio } from "lucide-react"
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
      className={`inline-flex min-h-9 items-center gap-1.5 rounded-lg px-3 text-xs font-bold transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--accent)] ${active ? "bg-[var(--accent)] text-slate-950" : "text-[var(--text-muted)] hover:bg-[var(--surface-hover)] hover:text-[var(--text)]"}`}
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

export function LoadingState({ message }: { message: string | null }) {
  return (
    <main className="grid min-h-screen place-items-center px-6">
      <div className="max-w-md text-center">
        <div className="mx-auto grid size-14 place-items-center rounded-2xl bg-[var(--brand)] text-slate-950">
          <Radio className="size-6 animate-pulse motion-reduce:animate-none" aria-hidden="true" />
        </div>
        <h1 className="mt-5 text-xl font-bold text-[var(--text)]">正在连接运行时</h1>
        <p className="mt-2 text-sm leading-6 text-[var(--text-muted)]">
          正在等待第一份有效运行快照。
        </p>
        {message && (
          <p role="alert" className="mt-4 text-sm text-rose-600 dark:text-rose-300">
            {message}
          </p>
        )}
      </div>
    </main>
  )
}
