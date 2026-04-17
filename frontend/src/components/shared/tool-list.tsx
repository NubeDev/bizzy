/**
 * Shared tool list with ToolCard + Add Tool form.
 * Used in app-editor Tools tab and anywhere else that needs tool management.
 */
import { useState } from "react"
import { Plus, Trash2, Sparkles, FlaskConical, History } from "lucide-react"
import { AIToolEditor } from "@/components/store/ai-tool-editor"
import { ToolHistory } from "@/components/store/tool-history"
import { ToolTester } from "@/components/shared/tool-tester"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import { Badge } from "@/components/ui/badge"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import type { StoreTool } from "@/lib/types"

interface ToolListProps {
  appId: string
  tools: StoreTool[]
  onAdd: (tool: StoreTool) => void
  onUpdate: (name: string, tool: StoreTool, changeSummary?: string) => void
  onDelete: (name: string) => void
  addPending?: boolean
}

const DEFAULT_SCRIPT = `// Tool script
function handle(params) {
  return { result: "hello" };
}
`

export function ToolList({ appId, tools, onAdd, onUpdate, onDelete, addPending }: ToolListProps) {
  const [showNewTool, setShowNewTool] = useState(false)
  const [newTool, setNewTool] = useState<StoreTool>({
    name: "", description: "", toolClass: "read-only", params: {}, script: DEFAULT_SCRIPT,
  })

  const handleAddTool = () => {
    if (!newTool.name) return
    onAdd(newTool)
    setNewTool({ name: "", description: "", toolClass: "read-only", params: {}, script: DEFAULT_SCRIPT })
    setShowNewTool(false)
  }

  return (
    <div className="space-y-3">
      <div className="flex justify-end">
        <Button onClick={() => setShowNewTool(true)} size="sm" className="rounded-none font-mono text-xs uppercase tracking-wider">
          <Plus size={14} className="mr-1" /> Add Tool
        </Button>
      </div>

      {showNewTool && (
        <div className="rounded-none border border-border bg-accent/30 p-5 space-y-4">
          <h3 className="text-sm font-mono font-medium">New Tool</h3>
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-2">
              <Label className="text-xs text-muted-foreground">Name</Label>
              <Input value={newTool.name} onChange={(e) => setNewTool({ ...newTool, name: e.target.value })}
                placeholder="check_status" className="font-mono rounded-none bg-background/60 border-border" />
            </div>
            <div className="space-y-2">
              <Label className="text-xs text-muted-foreground">Class</Label>
              <Select value={newTool.toolClass} onValueChange={(v) => setNewTool({ ...newTool, toolClass: v })}>
                <SelectTrigger className="rounded-none bg-background/60 border-border"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="read-only">read-only</SelectItem>
                  <SelectItem value="read-write">read-write</SelectItem>
                  <SelectItem value="destructive">destructive</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          <div className="space-y-2">
            <Label className="text-xs text-muted-foreground">Description</Label>
            <Input value={newTool.description} onChange={(e) => setNewTool({ ...newTool, description: e.target.value })}
              className="rounded-none bg-background/60 border-border" />
          </div>
          <div className="space-y-2">
            <Label className="text-xs text-muted-foreground">Script (JavaScript)</Label>
            <Textarea value={newTool.script} onChange={(e) => setNewTool({ ...newTool, script: e.target.value })}
              rows={10} className="font-mono text-sm rounded-none bg-background/60 border-border" />
          </div>
          <div className="flex gap-2">
            <Button onClick={handleAddTool} disabled={!newTool.name || addPending} size="sm" className="rounded-none font-mono text-xs uppercase tracking-wider">
              Add Tool
            </Button>
            <Button variant="outline" size="sm" onClick={() => setShowNewTool(false)} className="rounded-none">Cancel</Button>
          </div>
        </div>
      )}

      {tools.map((tool) => (
        <ToolCard
          key={tool.name}
          appId={appId}
          tool={tool}
          onUpdate={(t, summary) => onUpdate(tool.name, t, summary)}
          onDelete={() => onDelete(tool.name)}
        />
      ))}

      {!tools.length && !showNewTool && (
        <p className="text-sm text-muted-foreground text-center py-12">No tools yet. Add one to get started.</p>
      )}
    </div>
  )
}

// --- ToolCard ---

function ToolCard({ appId, tool, onUpdate, onDelete }: {
  appId: string; tool: StoreTool; onUpdate: (t: StoreTool, summary?: string) => void; onDelete: () => void
}) {
  const [expanded, setExpanded] = useState(false)
  const [aiEditing, setAiEditing] = useState(false)
  const [showHistory, setShowHistory] = useState(false)
  const [showTest, setShowTest] = useState(false)
  const [editingParams, setEditingParams] = useState(false)
  const [newParamName, setNewParamName] = useState("")
  const [newParamType, setNewParamType] = useState("string")

  const closeAll = () => { setExpanded(false); setAiEditing(false); setShowHistory(false); setShowTest(false) }

  const handleAddParam = () => {
    if (!newParamName.trim()) return
    onUpdate({ ...tool, params: { ...tool.params, [newParamName.trim()]: { type: newParamType, required: false, description: "" } } })
    setNewParamName("")
    setNewParamType("string")
  }

  const handleDeleteParam = (name: string) => {
    const params = { ...tool.params }
    delete params[name]
    onUpdate({ ...tool, params })
  }

  const handleUpdateParam = (name: string, field: string, value: unknown) => {
    const param = { ...tool.params[name], [field]: value }
    onUpdate({ ...tool, params: { ...tool.params, [name]: param } })
  }

  return (
    <div className="rounded-none border border-border bg-accent/30 p-4">
      <div className="flex items-center justify-between cursor-pointer" onClick={() => { closeAll(); setExpanded(!expanded) }}>
        <div className="flex items-center gap-2">
          <code className="text-sm font-medium font-mono">{tool.name}</code>
          <Badge variant="secondary" className="text-[10px] rounded-none font-mono">{tool.toolClass}</Badge>
          {tool.mode === "qa" && <Badge variant="outline" className="text-[10px] rounded-none font-mono">QA</Badge>}
          <span className="text-[11px] text-muted-foreground">{Object.keys(tool.params).length} params</span>
        </div>
        <div className="flex items-center gap-1">
          <Button variant="ghost" size="icon" className="h-7 w-7 rounded-none text-muted-foreground hover:text-foreground" title="Edit with AI"
            onClick={(e) => { e.stopPropagation(); closeAll(); setAiEditing(!aiEditing) }}>
            <Sparkles size={14} />
          </Button>
          <Button variant="ghost" size="icon" className="h-7 w-7 rounded-none text-muted-foreground hover:text-foreground" title="Test tool"
            onClick={(e) => { e.stopPropagation(); closeAll(); setShowTest(!showTest) }}>
            <FlaskConical size={14} />
          </Button>
          <Button variant="ghost" size="icon" className="h-7 w-7 rounded-none text-muted-foreground hover:text-foreground" title="Version history"
            onClick={(e) => { e.stopPropagation(); closeAll(); setShowHistory(!showHistory) }}>
            <History size={14} />
          </Button>
          <Button variant="ghost" size="icon" className="h-7 w-7 hover:text-destructive rounded-none"
            onClick={(e) => { e.stopPropagation(); onDelete() }}>
            <Trash2 size={14} />
          </Button>
        </div>
      </div>

      {aiEditing && (
        <div className="mt-3">
          <AIToolEditor tool={tool} onApply={(updated) => { onUpdate(updated, "AI edit"); setAiEditing(false) }} onClose={() => setAiEditing(false)} />
        </div>
      )}

      {showHistory && (
        <div className="mt-3">
          <ToolHistory appId={appId} toolName={tool.name} onReverted={() => setShowHistory(false)} />
        </div>
      )}

      {showTest && (
        <div className="mt-3 border border-border rounded-none p-4">
          <ToolTester key={`test-${tool.name}`} tool={tool} showScript />
        </div>
      )}

      {expanded && !aiEditing && !showHistory && !showTest && (
        <div className="mt-3 space-y-3">
          <p className="text-xs text-muted-foreground">{tool.description}</p>

          {/* Params editor */}
          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <Label className="text-xs font-mono text-muted-foreground">Params</Label>
              <button onClick={() => setEditingParams(!editingParams)}
                className="text-[11px] font-mono text-muted-foreground hover:text-foreground transition-colors">
                {editingParams ? "Done" : "Edit params"}
              </button>
            </div>

            {Object.entries(tool.params).map(([name, def]) => (
              <div key={name} className="flex items-center gap-2 text-xs">
                <code className="font-mono text-foreground">{name}</code>
                <Badge variant="outline" className="text-[10px] rounded-none font-mono">{def.type}</Badge>
                {def.required && <Badge variant="secondary" className="text-[10px] rounded-none font-mono">required</Badge>}
                {editingParams && (
                  <>
                    <Input value={def.description} onChange={(e) => handleUpdateParam(name, "description", e.target.value)}
                      placeholder="description" className="h-6 text-[11px] rounded-none bg-background/60 border-border font-mono flex-1" />
                    <button onClick={() => handleUpdateParam(name, "required", !def.required)}
                      className={`text-[10px] font-mono px-1.5 py-0.5 border rounded-none ${def.required ? 'border-foreground text-foreground' : 'border-border text-muted-foreground'}`}>
                      req
                    </button>
                    <Button variant="ghost" size="icon" className="h-5 w-5 rounded-none hover:text-destructive" onClick={() => handleDeleteParam(name)}>
                      <Trash2 size={10} />
                    </Button>
                  </>
                )}
                {!editingParams && def.description && (
                  <span className="text-muted-foreground truncate">{def.description}</span>
                )}
              </div>
            ))}

            {editingParams && (
              <div className="flex items-center gap-2">
                <Input value={newParamName} onChange={(e) => setNewParamName(e.target.value)} placeholder="new_param"
                  className="h-6 text-[11px] rounded-none bg-background/60 border-border font-mono w-32"
                  onKeyDown={(e) => { if (e.key === "Enter") handleAddParam() }} />
                <Select value={newParamType} onValueChange={setNewParamType}>
                  <SelectTrigger className="h-6 w-24 rounded-none bg-background/60 border-border text-[11px] font-mono"><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="string">string</SelectItem>
                    <SelectItem value="number">number</SelectItem>
                    <SelectItem value="boolean">boolean</SelectItem>
                  </SelectContent>
                </Select>
                <Button size="sm" variant="outline" className="h-6 rounded-none text-[11px] font-mono" onClick={handleAddParam} disabled={!newParamName.trim()}>
                  <Plus size={10} className="mr-0.5" /> Add
                </Button>
              </div>
            )}

            {Object.keys(tool.params).length === 0 && !editingParams && (
              <p className="text-[11px] text-muted-foreground">No params.
                <button onClick={() => setEditingParams(true)} className="ml-1 underline hover:text-foreground">Add some</button>
              </p>
            )}
          </div>

          {/* Script */}
          <Textarea value={tool.script} onChange={(e) => onUpdate({ ...tool, script: e.target.value })}
            rows={8} className="font-mono text-xs rounded-none bg-background/60 border-border" />
        </div>
      )}
    </div>
  )
}
