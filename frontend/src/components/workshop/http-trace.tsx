import type { HTTPLogEntry } from "@/hooks/use-test-tool"

interface Props {
  entries: HTTPLogEntry[]
}

function statusColor(status: number): string {
  if (status >= 200 && status < 300) return "text-emerald-400"
  if (status >= 300 && status < 400) return "text-yellow-400"
  if (status >= 400) return "text-destructive"
  return "text-muted-foreground"
}

export function HttpTrace({ entries }: Props) {
  if (!entries.length) return null

  return (
    <div className="space-y-1.5">
      <h4 className="text-xs font-mono font-medium text-muted-foreground uppercase tracking-wider">HTTP Log</h4>
      <div className="rounded-none border border-border overflow-hidden">
        <table className="w-full text-xs font-mono">
          <thead>
            <tr className="bg-muted/50 text-muted-foreground">
              <th className="text-left px-3 py-1.5 font-medium">Method</th>
              <th className="text-left px-3 py-1.5 font-medium">URL</th>
              <th className="text-right px-3 py-1.5 font-medium">Status</th>
              <th className="text-right px-3 py-1.5 font-medium">Duration</th>
              <th className="text-left px-3 py-1.5 font-medium">Redirect</th>
            </tr>
          </thead>
          <tbody>
            {entries.map((e, i) => (
              <tr key={i} className="border-t border-border/50 hover:bg-accent/30">
                <td className="px-3 py-1.5 font-semibold">{e.method}</td>
                <td className="px-3 py-1.5 text-foreground/80 max-w-[400px] truncate">{e.url}</td>
                <td className={`px-3 py-1.5 text-right font-semibold ${statusColor(e.status)}`}>
                  {e.status || "ERR"}
                </td>
                <td className="px-3 py-1.5 text-right text-muted-foreground">
                  {Math.round(e.duration_ms)}ms
                </td>
                <td className="px-3 py-1.5 text-yellow-400 max-w-[200px] truncate">
                  {e.redirected_from ? `from ${e.redirected_from}` : ""}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
