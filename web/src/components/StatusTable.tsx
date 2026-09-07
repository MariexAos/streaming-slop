import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Table, TableBody, TableCell, TableHead, TableRow } from "@/components/ui/table"

export type StatusRow = {
  label: string
  value: string
  status?: "success" | "warning" | "danger" | "accent" | "neutral"
}

export function StatusTable({ title, rows }: { title: string; rows: StatusRow[] }) {
  return (
    <Card>
      <CardHeader><CardTitle>{title}</CardTitle></CardHeader>
      <CardContent className="pt-3">
        <Table>
          <thead><TableRow><TableHead>指标</TableHead><TableHead className="text-right">当前值</TableHead></TableRow></thead>
          <TableBody>
            {rows.map((row) => (
              <TableRow key={row.label}>
                <TableCell className="text-[var(--text-muted)]">{row.label}</TableCell>
                <TableCell className="text-right font-mono text-xs font-semibold text-[var(--text)]">
                  {row.status ? <Badge tone={row.status}>{row.value}</Badge> : row.value}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  )
}
