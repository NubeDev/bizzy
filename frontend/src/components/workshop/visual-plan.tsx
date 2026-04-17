import { useState } from "react"
import { Check, Pencil, Eye, Wrench, Trash2, RotateCcw } from "lucide-react"
import type { Spec } from "@json-render/core"
import { SpecRenderer } from "@/lib/json-render-registry"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Textarea } from "@/components/ui/textarea"

export interface VisualToolPlan {
  name: string
  description: string
  toolClass: string
  params: Record<string, { type: string; required: boolean; description: string; options?: string[] }>
  inputSpec?: Spec
  outputSpec?: Spec
  sampleOutput?: unknown
  enabled: boolean
}

export type ToolPlanStatus = "pending" | "approved" | "rejected" | "editing"

interface VisualPlanCardProps {
  tool: VisualToolPlan
  status: ToolPlanStatus
  onApprove: () => void
  onReject: () => void
  onRestore: () => void
  onRequestEdit: (feedback: string) => void
}

/**
 * Renders a single tool's visual plan: input form preview + sample output preview.
 * User can approve, reject, or request changes via chat.
 */
export function VisualPlanCard({ tool, status, onApprove, onReject, onRestore, onRequestEdit }: VisualPlanCardProps) {
  const [editMode, setEditMode] = useState(false)
  const [feedback, setFeedback] = useState("")

  const handleSubmitEdit = () => {
    if (!feedback.trim()) return
    onRequestEdit(feedback.trim())
    setFeedback("")
    setEditMode(false)
  }

  // Rejected state — collapsed with undo option
  if (status === "rejected") {
    return (
      <div className="border border-border rounded-none p-3 opacity-50 bg-muted/30">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Trash2 size={12} className="text-destructive" />
            <code className="text-sm line-through text-muted-foreground">{tool.name}</code>
            <Badge variant="outline" className="text-[10px] text-muted-foreground">{tool.toolClass}</Badge>
            <Badge variant="outline" className="text-[10px] text-destructive border-destructive/30">Rejected</Badge>
          </div>
          <Button size="sm" variant="ghost" className="rounded-none text-xs h-7" onClick={onRestore}>
            <RotateCcw size={10} className="mr-1" /> Undo
          </Button>
        </div>
      </div>
    )
  }

  return (
    <div className={`border border-border rounded-none p-4 space-y-4 ${status === "approved" ? "border-emerald-500/40 bg-emerald-500/5" : ""}`}>
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Wrench size={14} className="text-muted-foreground" />
          <code className="text-sm font-semibold">{tool.name}</code>
          <Badge variant="outline" className="text-[10px]">{tool.toolClass}</Badge>
          {status === "approved" && (
            <Badge variant="default" className="text-[10px] bg-emerald-600">
              <Check size={10} className="mr-0.5" /> Approved
            </Badge>
          )}
        </div>
        {/* Reject button — always available (top-right) */}
        {status !== "approved" && (
          <Button size="sm" variant="ghost" className="rounded-none text-xs h-7 text-muted-foreground hover:text-destructive" onClick={onReject}>
            <Trash2 size={12} />
          </Button>
        )}
      </div>
      <p className="text-xs text-muted-foreground">{tool.description}</p>

      {/* Input preview */}
      {tool.inputSpec && (
        <div className="space-y-1.5">
          <div className="flex items-center gap-1.5">
            <Eye size={12} className="text-muted-foreground" />
            <span className="text-[11px] font-mono font-medium text-muted-foreground uppercase tracking-wider">Input Preview</span>
          </div>
          <div className="border border-border rounded-none p-3 bg-background/60">
            <SpecRenderer spec={tool.inputSpec} />
          </div>
        </div>
      )}

      {/* Output preview */}
      {tool.outputSpec && (
        <div className="space-y-1.5">
          <div className="flex items-center gap-1.5">
            <Eye size={12} className="text-muted-foreground" />
            <span className="text-[11px] font-mono font-medium text-muted-foreground uppercase tracking-wider">Expected Output (sample data)</span>
          </div>
          <div className="border border-border rounded-none p-3 bg-background/60">
            <SpecRenderer spec={tool.outputSpec} />
          </div>
        </div>
      )}

      {/* No specs fallback */}
      {!tool.inputSpec && !tool.outputSpec && (
        <div className="text-xs text-muted-foreground py-4 text-center border border-dashed border-border rounded-none">
          No visual preview available for this tool.
          <br />
          <span className="text-[11px]">Params: {Object.keys(tool.params).join(", ") || "none"}</span>
        </div>
      )}

      {/* Actions */}
      {status !== "approved" && (
        <div className="flex items-center gap-2">
          <Button size="sm" className="rounded-none text-xs" onClick={onApprove}>
            <Check size={12} className="mr-1" /> Looks good
          </Button>
          <Button
            size="sm"
            variant="outline"
            className="rounded-none text-xs"
            onClick={() => setEditMode(!editMode)}
          >
            <Pencil size={12} className="mr-1" /> Change this
          </Button>
        </div>
      )}

      {/* Approved — allow undo back to pending */}
      {status === "approved" && (
        <div className="flex items-center gap-2">
          <Button size="sm" variant="ghost" className="rounded-none text-xs text-muted-foreground" onClick={onRestore}>
            <RotateCcw size={10} className="mr-1" /> Reconsider
          </Button>
          <Button size="sm" variant="ghost" className="rounded-none text-xs text-muted-foreground hover:text-destructive" onClick={onReject}>
            <Trash2 size={12} className="mr-1" /> Reject
          </Button>
        </div>
      )}

      {/* Edit feedback input */}
      {editMode && status === "pending" && (
        <div className="space-y-2">
          <Textarea
            value={feedback}
            onChange={(e) => setFeedback(e.target.value)}
            placeholder='e.g. "make it a card layout instead of a table", "add a date picker", "show rate as a big number"'
            rows={2}
            className="text-xs rounded-none"
            autoFocus
          />
          <div className="flex gap-2">
            <Button size="sm" className="rounded-none text-xs" onClick={handleSubmitEdit} disabled={!feedback.trim()}>
              Send
            </Button>
            <Button size="sm" variant="ghost" className="rounded-none text-xs" onClick={() => setEditMode(false)}>
              Cancel
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}
