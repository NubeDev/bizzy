import { useState, useEffect } from "react"
import { useParams, Link } from "react-router-dom"
import { ArrowLeft, Plus, Trash2, Check, Loader2, Save, Globe, Sparkles } from "lucide-react"
import {
  useMyApp, useUpdateApp, usePublishApp,
  useAddTool, useDeleteTool, useUpdateTool,
  useAddPrompt, useDeletePrompt, useUpdatePrompt,
} from "@/hooks/use-my-apps"
import { VisibilityBadge } from "@/components/store/visibility-badge"
import { AIWorkshop } from "@/components/store/ai-workshop"
import { categoryLabel } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import { Badge } from "@/components/ui/badge"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import type { StoreTool, StorePrompt, StoreApp } from "@/lib/types"

const categories = [
  "iot-devices", "analytics", "devops", "marketing",
  "design", "utilities", "integrations", "automation",
]

export function AppEditorPage() {
  const { id } = useParams<{ id: string }>()
  const { data: app, isLoading } = useMyApp(id!)
  const updateMutation = useUpdateApp()
  const publishMutation = usePublishApp()
  const addToolMutation = useAddTool()
  const deleteToolMutation = useDeleteTool()
  const updateToolMutation = useUpdateTool()
  const addPromptMutation = useAddPrompt()
  const deletePromptMutation = useDeletePrompt()
  const updatePromptMutation = useUpdatePrompt()

  const [draft, setDraft] = useState<Partial<StoreApp>>({})
  const [dirty, setDirty] = useState(false)

  const [showNewTool, setShowNewTool] = useState(false)
  const [newTool, setNewTool] = useState<StoreTool>({
    name: "", description: "", toolClass: "read-only", params: {},
    script: '// Tool script\nfunction handle(params) {\n  return { result: "hello" };\n}\n',
  })

  const [showNewPrompt, setShowNewPrompt] = useState(false)
  const [newPrompt, setNewPrompt] = useState<StorePrompt>({ name: "", description: "", body: "" })

  useEffect(() => {
    if (app) {
      setDraft({
        displayName: app.displayName,
        description: app.description,
        longDescription: app.longDescription,
        version: app.version,
        icon: app.icon,
        color: app.color,
        category: app.category,
        tags: app.tags,
      })
    }
  }, [app])

  if (isLoading || !app) {
    return (
      <div className="flex justify-center py-20">
        <Loader2 className="animate-spin text-muted-foreground" size={24} />
      </div>
    )
  }

  const updateField = (field: string, value: unknown) => {
    setDraft((d) => ({ ...d, [field]: value }))
    setDirty(true)
  }

  const handleSave = async () => {
    await updateMutation.mutateAsync({ id: app.id, data: draft })
    setDirty(false)
  }

  const handlePublish = async () => {
    await publishMutation.mutateAsync(app.id)
  }

  const handleAddTool = async () => {
    if (!newTool.name) return
    await addToolMutation.mutateAsync({ appId: app.id, tool: newTool })
    setNewTool({
      name: "", description: "", toolClass: "read-only", params: {},
      script: '// Tool script\nfunction handle(params) {\n  return { result: "hello" };\n}\n',
    })
    setShowNewTool(false)
  }

  const handleAddPrompt = async () => {
    if (!newPrompt.name) return
    await addPromptMutation.mutateAsync({ appId: app.id, prompt: newPrompt })
    setNewPrompt({ name: "", description: "", body: "" })
    setShowNewPrompt(false)
  }

  const checks = [
    { label: "Display name set", ok: !!app.displayName },
    { label: `Description (${app.description?.length || 0}/20 chars)`, ok: (app.description?.length || 0) >= 20 },
    { label: "Category selected", ok: !!app.category },
    { label: "At least one tool or prompt", ok: (app.tools?.length || 0) + (app.prompts?.length || 0) > 0 },
  ]
  const canPublish = checks.every((c) => c.ok)

  return (
    <div className="p-6 max-w-5xl mx-auto space-y-5">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Button variant="ghost" size="icon" asChild className="rounded-none hover:bg-accent">
            <Link to="/my-apps"><ArrowLeft size={18} /></Link>
          </Button>
          <div
            className="w-10 h-10 rounded-none flex items-center justify-center text-sm font-mono font-light text-white"
            style={{ background: app.color || '#1f2228' }}
          >
            {app.displayName.charAt(0)}
          </div>
          <div>
            <h2 className="font-bold text-lg">{app.displayName}</h2>
            <div className="flex items-center gap-2">
              <span className="font-mono text-[10px] text-muted-foreground border border-border px-1.5 py-0.5">v{app.version}</span>
              <VisibilityBadge visibility={app.visibility} />
            </div>
          </div>
        </div>
        {dirty && (
          <Button onClick={handleSave} disabled={updateMutation.isPending} className="rounded-none font-mono text-xs uppercase tracking-wider">
            <Save size={14} className="mr-1.5" />
            {updateMutation.isPending ? "Saving..." : "Save Changes"}
          </Button>
        )}
      </div>

      {/* Tabs */}
      <Tabs defaultValue="ai">
        <TabsList className="bg-transparent border-b border-border/30 rounded-none w-full justify-start gap-0 p-0 h-auto">
          <TabsTrigger value="ai" className="rounded-none border-b-2 border-transparent data-[state=active]:border-primary data-[state=active]:text-primary data-[state=active]:bg-transparent data-[state=active]:shadow-none px-4 py-2.5 text-[13px] text-muted-foreground transition-colors"><Sparkles size={14} className="mr-1.5" />AI Workshop</TabsTrigger>
          <TabsTrigger value="details" className="rounded-none border-b-2 border-transparent data-[state=active]:border-primary data-[state=active]:text-primary data-[state=active]:bg-transparent data-[state=active]:shadow-none px-4 py-2.5 text-[13px] text-muted-foreground transition-colors">Details</TabsTrigger>
          <TabsTrigger value="tools" className="rounded-none border-b-2 border-transparent data-[state=active]:border-primary data-[state=active]:text-primary data-[state=active]:bg-transparent data-[state=active]:shadow-none px-4 py-2.5 text-[13px] text-muted-foreground transition-colors">Tools ({app.tools?.length || 0})</TabsTrigger>
          <TabsTrigger value="prompts" className="rounded-none border-b-2 border-transparent data-[state=active]:border-primary data-[state=active]:text-primary data-[state=active]:bg-transparent data-[state=active]:shadow-none px-4 py-2.5 text-[13px] text-muted-foreground transition-colors">Prompts ({app.prompts?.length || 0})</TabsTrigger>
          <TabsTrigger value="settings" className="rounded-none border-b-2 border-transparent data-[state=active]:border-primary data-[state=active]:text-primary data-[state=active]:bg-transparent data-[state=active]:shadow-none px-4 py-2.5 text-[13px] text-muted-foreground transition-colors">Settings</TabsTrigger>
          <TabsTrigger value="permissions" className="rounded-none border-b-2 border-transparent data-[state=active]:border-primary data-[state=active]:text-primary data-[state=active]:bg-transparent data-[state=active]:shadow-none px-4 py-2.5 text-[13px] text-muted-foreground transition-colors">Permissions</TabsTrigger>
          <TabsTrigger value="publish" className="rounded-none border-b-2 border-transparent data-[state=active]:border-primary data-[state=active]:text-primary data-[state=active]:bg-transparent data-[state=active]:shadow-none px-4 py-2.5 text-[13px] text-muted-foreground transition-colors">Publish</TabsTrigger>
        </TabsList>

        {/* AI Workshop tab */}
        <TabsContent value="ai" className="mt-4">
          <AIWorkshop appId={app.id} appName={app.displayName} appDescription={app.description} />
        </TabsContent>

        {/* Details tab */}
        <TabsContent value="details" className="mt-6">
          <div className="space-y-5 max-w-lg">
              <div className="space-y-2">
                <Label className="text-xs text-muted-foreground">Display Name</Label>
                <Input value={draft.displayName || ""} onChange={(e) => updateField("displayName", e.target.value)} className="rounded-none bg-accent/40 border-border" />
              </div>
              <div className="space-y-2">
                <Label className="text-xs text-muted-foreground">Slug</Label>
                <Input value={app.name} disabled className="rounded-none bg-accent/40 border-border" />
              </div>
              <div className="space-y-2">
                <Label className="text-xs text-muted-foreground">Description</Label>
                <Input value={draft.description || ""} onChange={(e) => updateField("description", e.target.value)} className="rounded-none bg-accent/40 border-border" />
              </div>
              <div className="space-y-2">
                <Label className="text-xs text-muted-foreground">Long Description (markdown)</Label>
                <Textarea
                  value={draft.longDescription || ""}
                  onChange={(e) => updateField("longDescription", e.target.value)}
                  rows={5}
                  className="font-mono text-sm rounded-none bg-accent/40 border-border"
                />
              </div>
              <div className="space-y-2">
                <Label className="text-xs text-muted-foreground">Category</Label>
                <Select value={draft.category || ""} onValueChange={(v) => updateField("category", v)}>
                  <SelectTrigger className="rounded-none bg-accent/40 border-border">
                    <SelectValue placeholder="Select category..." />
                  </SelectTrigger>
                  <SelectContent>
                    {categories.map((c) => <SelectItem key={c} value={c}>{categoryLabel(c)}</SelectItem>)}
                  </SelectContent>
                </Select>
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-2">
                  <Label className="text-xs text-muted-foreground">Icon</Label>
                  <Input value={draft.icon || ""} onChange={(e) => updateField("icon", e.target.value)} placeholder="e.g. terminal" className="rounded-none bg-accent/40 border-border" />
                </div>
                <div className="space-y-2">
                  <Label className="text-xs text-muted-foreground">Color</Label>
                  <Input value={draft.color || ""} onChange={(e) => updateField("color", e.target.value)} placeholder="#6366f1" className="rounded-none bg-accent/40 border-border" />
                </div>
              </div>
              <div className="space-y-2">
                <Label className="text-xs text-muted-foreground">Version</Label>
                <Input value={draft.version || ""} onChange={(e) => updateField("version", e.target.value)} className="rounded-none bg-accent/40 border-border" />
              </div>
          </div>
        </TabsContent>

        {/* Tools tab */}
        <TabsContent value="tools" className="mt-6 space-y-3">
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
                    <Input
                      value={newTool.name}
                      onChange={(e) => setNewTool({ ...newTool, name: e.target.value })}
                      placeholder="check_status"
                      className="font-mono rounded-none bg-background/60 border-border"
                    />
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
                  <Input
                    value={newTool.description}
                    onChange={(e) => setNewTool({ ...newTool, description: e.target.value })}
                    className="rounded-none bg-background/60 border-border"
                  />
                </div>
                <div className="space-y-2">
                  <Label className="text-xs text-muted-foreground">Script (JavaScript)</Label>
                  <Textarea
                    value={newTool.script}
                    onChange={(e) => setNewTool({ ...newTool, script: e.target.value })}
                    rows={10}
                    className="font-mono text-sm rounded-none bg-background/60 border-border"
                  />
                </div>
                <div className="flex gap-2">
                  <Button onClick={handleAddTool} disabled={!newTool.name || addToolMutation.isPending} size="sm" className="rounded-none font-mono text-xs uppercase tracking-wider">
                    Add Tool
                  </Button>
                  <Button variant="outline" size="sm" onClick={() => setShowNewTool(false)} className="rounded-none">Cancel</Button>
                </div>
            </div>
          )}

          {app.tools?.map((tool) => (
            <ToolCard
              key={tool.name}
              tool={tool}
              onUpdate={(t) => updateToolMutation.mutate({ appId: app.id, name: tool.name, tool: t })}
              onDelete={() => deleteToolMutation.mutate({ appId: app.id, name: tool.name })}
            />
          ))}

          {!app.tools?.length && !showNewTool && (
            <p className="text-sm text-muted-foreground text-center py-12">No tools yet. Add one to get started.</p>
          )}
        </TabsContent>

        {/* Prompts tab */}
        <TabsContent value="prompts" className="mt-6 space-y-3">
          <div className="flex justify-end">
            <Button onClick={() => setShowNewPrompt(true)} size="sm" className="rounded-none font-mono text-xs uppercase tracking-wider">
              <Plus size={14} className="mr-1" /> Add Prompt
            </Button>
          </div>

          {showNewPrompt && (
            <div className="rounded-none border border-border bg-accent/30 p-5 space-y-4">
              <h3 className="text-sm font-mono font-medium">New Prompt</h3>
                <div className="space-y-2">
                  <Label className="text-xs text-muted-foreground">Name</Label>
                  <Input value={newPrompt.name} onChange={(e) => setNewPrompt({ ...newPrompt, name: e.target.value })} placeholder="setup-guide" className="rounded-none bg-background/60 border-border" />
                </div>
                <div className="space-y-2">
                  <Label className="text-xs text-muted-foreground">Description</Label>
                  <Input value={newPrompt.description} onChange={(e) => setNewPrompt({ ...newPrompt, description: e.target.value })} className="rounded-none bg-background/60 border-border" />
                </div>
                <div className="space-y-2">
                  <Label className="text-xs text-muted-foreground">Body (markdown)</Label>
                  <Textarea
                    value={newPrompt.body}
                    onChange={(e) => setNewPrompt({ ...newPrompt, body: e.target.value })}
                    rows={6}
                    className="font-mono text-sm rounded-none bg-background/60 border-border"
                  />
                </div>
                <div className="flex gap-2">
                  <Button onClick={handleAddPrompt} disabled={!newPrompt.name || addPromptMutation.isPending} size="sm" className="rounded-none font-mono text-xs uppercase tracking-wider">
                    Add Prompt
                  </Button>
                  <Button variant="outline" size="sm" onClick={() => setShowNewPrompt(false)} className="rounded-none">Cancel</Button>
                </div>
            </div>
          )}

          {app.prompts?.map((prompt) => (
            <PromptCard
              key={prompt.name}
              prompt={prompt}
              onUpdate={(p) => updatePromptMutation.mutate({ appId: app.id, name: prompt.name, prompt: p })}
              onDelete={() => deletePromptMutation.mutate({ appId: app.id, name: prompt.name })}
            />
          ))}

          {!app.prompts?.length && !showNewPrompt && (
            <p className="text-sm text-muted-foreground text-center py-12">No prompts yet.</p>
          )}
        </TabsContent>

        {/* Settings tab */}
        <TabsContent value="settings" className="mt-6">
          <div className="space-y-4 max-w-lg">
              <p className="text-sm text-muted-foreground">
                Define the settings users fill in when they install your app.
              </p>
              {app.settings?.map((s, i) => (
                <div key={i} className="flex items-center gap-2 p-3 rounded-none bg-accent/40 text-sm">
                  <code>{s.key}</code>
                  <Badge variant="outline" className="text-[10px] rounded-none font-mono">{s.type}</Badge>
                  {s.required && <Badge variant="secondary" className="text-[10px] rounded-none font-mono">required</Badge>}
                </div>
              ))}
              {!app.settings?.length && (
                <p className="text-xs text-muted-foreground">No settings defined.</p>
              )}
              <p className="text-xs text-muted-foreground">
                Edit settings via the API: PUT /api/my/apps/{"{id}"} with a &quot;settings&quot; array.
              </p>
          </div>
        </TabsContent>

        {/* Permissions tab */}
        <TabsContent value="permissions" className="mt-6">
          <div className="space-y-5 max-w-lg">
              <div className="space-y-2">
                <Label className="text-xs text-muted-foreground">Allowed Hosts (one per line)</Label>
                <Textarea
                  value={app.permissions?.allowedHosts?.join("\n") || ""}
                  onChange={(e) => {
                    const hosts = e.target.value.split("\n").map((h) => h.trim()).filter(Boolean)
                    updateField("permissions", { ...app.permissions, allowedHosts: hosts })
                  }}
                  rows={4}
                  placeholder="*.example.com"
                  className="font-mono text-sm rounded-none bg-accent/40 border-border"
                />
              </div>
              <div className="space-y-2">
                <Label className="text-xs text-muted-foreground">Default Tool Class</Label>
                <Select
                  value={app.permissions?.defaultToolClass || "read-only"}
                  onValueChange={(v) => updateField("permissions", { ...app.permissions, defaultToolClass: v })}
                >
                  <SelectTrigger className="rounded-none bg-accent/40 border-border"><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="read-only">read-only</SelectItem>
                    <SelectItem value="read-write">read-write</SelectItem>
                    <SelectItem value="destructive">destructive</SelectItem>
                  </SelectContent>
                </Select>
              </div>
          </div>
        </TabsContent>

        {/* Publish tab */}
        <TabsContent value="publish" className="mt-6">
          <div className="space-y-5 max-w-lg">
              <h3 className="text-sm font-medium">Publishing Checklist</h3>
              <div className="space-y-3">
              {checks.map((c, i) => (
                <div key={i} className="flex items-center gap-3 text-sm">
                  <div className={`w-5 h-5 rounded-none flex items-center justify-center ${c.ok ? 'bg-emerald-500/20' : 'bg-muted'}`}>
                    <Check size={12} className={c.ok ? "text-emerald-400" : "text-muted-foreground/30"} />
                  </div>
                  <span className={c.ok ? "" : "text-muted-foreground"}>{c.label}</span>
                </div>
              ))}
              </div>

              <div className="h-px bg-border/50" />

              <div className="flex items-center gap-2">
                <span className="text-sm text-muted-foreground">Current:</span>
                <VisibilityBadge visibility={app.visibility} />
              </div>

              {app.visibility !== "public" && (
                <Button
                  onClick={handlePublish}
                  disabled={!canPublish || publishMutation.isPending}
                  className="rounded-none font-mono text-xs uppercase tracking-wider"
                >
                  <Globe size={14} className="mr-1.5" />
                  {publishMutation.isPending ? "Publishing..." : "Publish to Store"}
                </Button>
              )}

              {publishMutation.isError && (
                <p className="text-sm text-destructive">
                  {(publishMutation.error as Error).message}
                </p>
              )}
          </div>
        </TabsContent>
      </Tabs>
    </div>
  )
}

// --- Sub-components ---

function ToolCard({ tool, onUpdate, onDelete }: {
  tool: StoreTool; onUpdate: (t: StoreTool) => void; onDelete: () => void
}) {
  const [expanded, setExpanded] = useState(false)

  return (
    <div className="rounded-none border border-border bg-accent/30 p-4">
        <div className="flex items-center justify-between cursor-pointer" onClick={() => setExpanded(!expanded)}>
          <div className="flex items-center gap-2">
            <code className="text-sm font-medium font-mono">{tool.name}</code>
            <Badge variant="secondary" className="text-[10px] rounded-none font-mono">{tool.toolClass}</Badge>
          </div>
          <Button
            variant="ghost"
            size="icon"
            className="h-7 w-7 hover:text-destructive rounded-none"
            onClick={(e) => { e.stopPropagation(); onDelete() }}
          >
            <Trash2 size={14} />
          </Button>
        </div>
        {expanded && (
          <div className="mt-3 space-y-2">
            <p className="text-xs text-muted-foreground">{tool.description}</p>
            <Textarea
              value={tool.script}
              onChange={(e) => onUpdate({ ...tool, script: e.target.value })}
              rows={8}
              className="font-mono text-xs rounded-none bg-background/60 border-border"
            />
          </div>
        )}
    </div>
  )
}

function PromptCard({ prompt, onUpdate, onDelete }: {
  prompt: StorePrompt; onUpdate: (p: StorePrompt) => void; onDelete: () => void
}) {
  const [expanded, setExpanded] = useState(false)

  return (
    <div className="rounded-none border border-border bg-accent/30 p-4">
        <div className="flex items-center justify-between cursor-pointer" onClick={() => setExpanded(!expanded)}>
          <code className="text-sm font-medium font-mono">{prompt.name}</code>
          <Button
            variant="ghost"
            size="icon"
            className="h-7 w-7 hover:text-destructive rounded-none"
            onClick={(e) => { e.stopPropagation(); onDelete() }}
          >
            <Trash2 size={14} />
          </Button>
        </div>
        {expanded && (
          <div className="mt-3 space-y-2">
            <p className="text-xs text-muted-foreground">{prompt.description}</p>
            <Textarea
              value={prompt.body}
              onChange={(e) => onUpdate({ ...prompt, body: e.target.value })}
              rows={6}
              className="font-mono text-xs rounded-none bg-background/60 border-border"
            />
          </div>
        )}
    </div>
  )
}
