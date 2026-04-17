/**
 * Displays revision history for a tool and allows reverting to a previous version.
 */
import { useState } from "react"
import { History, RotateCcw, Loader2, ChevronDown, ChevronRight } from "lucide-react"
import { useRevisions, useRevertRevision, type Revision } from "@/hooks/use-revisions"
import { Button } from "@/components/ui/button"
import type { StoreTool } from "@/lib/types"

interface Props {
  appId: string
  toolName: string
  onReverted: () => void
}

export function ToolHistory({ appId, toolName, onReverted }: Props) {
  const { data: revisions, isLoading } = useRevisions(appId, "tool", toolName)
  const revertMutation = useRevertRevision(appId, "tool", toolName)
  const [expandedRev, setExpandedRev] = useState<number | null>(null)

  if (isLoading) {
    return (
      <div className="flex items-center gap-2 text-xs text-muted-foreground py-2">
        <Loader2 size={12} className="animate-spin" /> Loading history...
      </div>
    )
  }

  if (!revisions || revisions.length === 0) {
    return (
      <p className="text-[11px] text-muted-foreground py-2">No edit history yet. Changes will appear here after you edit this tool.</p>
    )
  }

  const handleRevert = async (rev: number) => {
    await revertMutation.mutateAsync(rev)
    onReverted()
  }

  return (
    <div className="space-y-1">
      <div className="flex items-center gap-1.5 mb-2">
        <History size={12} className="text-muted-foreground" />
        <span className="text-[11px] font-mono font-medium text-muted-foreground uppercase tracking-wider">
          History ({revisions.length})
        </span>
      </div>
      {revisions.map((rev) => {
        const isExpanded = expandedRev === rev.revision
        const toolData = rev.data as StoreTool | null
        const age = formatAge(rev.createdAt)

        return (
          <div key={rev.id} className="border border-border rounded-none text-xs">
            <button
              onClick={() => setExpandedRev(isExpanded ? null : rev.revision)}
              className="w-full flex items-center gap-2 px-2.5 py-1.5 text-left hover:bg-accent/50 transition-colors"
            >
              {isExpanded ? <ChevronDown size={10} /> : <ChevronRight size={10} />}
              <span className="font-mono text-muted-foreground">r{rev.revision}</span>
              <span className="truncate flex-1">{rev.changeSummary}</span>
              <span className="text-muted-foreground shrink-0">{age}</span>
            </button>
            {isExpanded && toolData && (
              <div className="px-2.5 pb-2.5 space-y-2 border-t border-border">
                <details className="mt-2">
                  <summary className="cursor-pointer text-muted-foreground hover:text-foreground transition-colors text-[11px]">
                    View script ({typeof toolData.script === 'string' ? toolData.script.split('\n').length : '?'} lines)
                  </summary>
                  <pre className="mt-1 p-2 rounded-none bg-background overflow-auto text-[10px] leading-relaxed border border-border max-h-40">
                    {typeof toolData.script === 'string' ? toolData.script : JSON.stringify(toolData, null, 2)}
                  </pre>
                </details>
                <Button
                  size="sm"
                  variant="outline"
                  className="rounded-none text-[11px] h-6"
                  onClick={() => handleRevert(rev.revision)}
                  disabled={revertMutation.isPending}
                >
                  {revertMutation.isPending ? (
                    <Loader2 size={10} className="mr-1 animate-spin" />
                  ) : (
                    <RotateCcw size={10} className="mr-1" />
                  )}
                  Revert to this version
                </Button>
              </div>
            )}
          </div>
        )
      })}
    </div>
  )
}

function formatAge(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime()
  const mins = Math.floor(diff / 60000)
  if (mins < 1) return "just now"
  if (mins < 60) return `${mins}m ago`
  const hrs = Math.floor(mins / 60)
  if (hrs < 24) return `${hrs}h ago`
  const days = Math.floor(hrs / 24)
  return `${days}d ago`
}
