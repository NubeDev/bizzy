/**
 * App Builder — three-panel layout for building complete apps with AI.
 *
 * Left:   AI Chat (drives generation)
 * Center: File Tree + Editor (view/edit generated files)
 * Right:  Live Preview + Tool Testing
 */
import { useState, useCallback, useEffect, useMemo } from "react"
import { useNavigate, useParams } from "react-router-dom"
import { Allotment } from "allotment"
import "allotment/dist/style.css"
import { Save, Loader2, Check } from "lucide-react"
import { useCreateApp, useAddTool, useAddPrompt, useUpdateApp, useUpdateTool, useUpdatePrompt, useMyApp, useMyApps } from "@/hooks/use-my-apps"
import { ChatPanel } from "./chat-panel"
import { FileTree } from "./file-tree"
import { FileEditor, EmptyEditor } from "./file-editor"
import { PreviewPanel } from "./preview-panel"
import { emptyProject, type AppProject, type AppFile } from "./types"
import { Button } from "@/components/ui/button"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import type { StoreApp } from "@/lib/types"

/** Convert a saved StoreApp into the builder's AppProject format */
function appToProject(app: StoreApp): AppProject {
  const files: AppFile[] = []

  // app.yaml
  const hosts = app.permissions?.allowedHosts?.length ? app.permissions.allowedHosts.map(h => `    - "${h}"`).join("\n") : ""
  files.push({
    path: "app.yaml", type: "yaml",
    content: `name: ${app.name}\nversion: ${app.version || "1.0.0"}\ndescription: "${app.description || ""}"\ncategory: ${app.category || "utilities"}\npermissions:\n  allowedHosts:\n${hosts || "    []"}\n  defaultToolClass: ${app.permissions?.defaultToolClass || "read-only"}\n`,
  })

  // Tools
  for (const t of app.tools || []) {
    files.push({ path: `tools/${t.name}.json`, type: "json", content: JSON.stringify({ name: t.name, description: t.description, toolClass: t.toolClass, mode: t.mode || undefined, params: t.params }, null, 2) })
    files.push({ path: `tools/${t.name}.js`, type: "js", content: t.script })
  }

  // Prompts
  for (const p of app.prompts || []) {
    const args = p.arguments?.map(a => `  - name: ${a.name}\n    description: ${a.description}\n    required: ${a.required}`).join("\n") || ""
    files.push({ path: `prompts/${p.name}.md`, type: "md", content: `---\nname: ${p.name}\ndescription: ${p.description}\n${args ? `arguments:\n${args}\n` : ""}---\n\n${p.body}` })
  }

  // UI Components
  for (const ui of app.uiComponents || []) {
    files.push({ path: `ui/${ui.name}.tsx`, type: "tsx", content: ui.code })
  }

  return { name: app.name, displayName: app.displayName, description: app.description, category: app.category, files }
}

export function AppBuilder() {
  const navigate = useNavigate()
  const { id: existingAppId } = useParams<{ id: string }>()
  const { data: existingApp } = useMyApp(existingAppId || "")

  const [project, setProject] = useState<AppProject>(emptyProject)
  const [selectedPath, setSelectedPath] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)
  const [savedId, setSavedId] = useState<string | null>(existingAppId || null)
  const [seeded, setSeeded] = useState(false)
  const [pendingMessage, setPendingMessage] = useState<string | null>(null)

  // Builder chat history — loaded from backend so conversation persists across page reloads
  const [chatHistory, setChatHistory] = useState<{ messages: { role: string; content: string }[]; claude_session_id: string } | null>(null)

  // Seed project from existing app when loaded
  useEffect(() => {
    if (existingApp && !seeded) {
      setProject(appToProject(existingApp))
      setSeeded(true)
      if (existingApp.tools?.length) setSelectedPath(`tools/${existingApp.tools[0].name}.js`)
    }
  }, [existingApp, seeded])

  // Load builder chat history for existing apps
  useEffect(() => {
    if (!existingAppId) return
    fetch(`/api/my/apps/${encodeURIComponent(existingAppId)}/chat`)
      .then(r => r.ok ? r.json() : null)
      .then(data => { if (data?.messages?.length) setChatHistory(data) })
      .catch(() => {})
  }, [existingAppId])

  const createAppMutation = useCreateApp()
  const updateAppMutation = useUpdateApp()
  const addToolMutation = useAddTool()
  const updateToolMutation = useUpdateTool()
  const addPromptMutation = useAddPrompt()
  const updatePromptMutation = useUpdatePrompt()

  const selectedFile = project.files.find(f => f.path === selectedPath) || null

  // Tool runner for live previews — runs scripts via test-tool endpoint (no saved app needed)
  const toolPairs = useMemo(() => {
    const helpers = project.files.find(f => f.path === "tools/_helpers.js")
    const tools: { name: string; script: string; helpers?: string }[] = []
    for (const f of project.files) {
      if (f.type !== "json" || !f.path.startsWith("tools/")) continue
      const name = f.path.replace("tools/", "").replace(".json", "")
      const jsFile = project.files.find(j => j.path === `tools/${name}.js`)
      if (!jsFile) continue
      try {
        const schema = JSON.parse(f.content)
        tools.push({ name: schema.name || name, script: jsFile.content, helpers: helpers?.content })
      } catch { /* skip */ }
    }
    return tools
  }, [project.files])

  const builderToolRunFn = useCallback(async (toolName: string, params?: Record<string, unknown>) => {
    const shortName = toolName.includes(".") ? toolName.split(".").pop()! : toolName
    const tool = toolPairs.find(t => t.name === shortName)
    if (!tool) throw new Error(`Tool "${shortName}" not found in project`)
    const script = tool.helpers ? tool.helpers + "\n\n" + tool.script : tool.script
    const res = await fetch("/api/apps/test-tool", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ script, params: params || {}, allowedHosts: ["*"], timeout: "30s" }),
    })
    if (!res.ok) {
      const body = await res.json().catch(() => ({}))
      throw new Error(body.error || res.statusText)
    }
    const result = await res.json()
    if (result.error) throw new Error(result.error)
    return result.output
  }, [toolPairs])

  // Called when AI generates file blocks
  const handleFilesGenerated = useCallback((newFiles: AppFile[]) => {
    setProject(prev => {
      const updated = { ...prev }
      const fileMap = new Map(prev.files.map(f => [f.path, f]))

      for (const nf of newFiles) {
        fileMap.set(nf.path, nf)

        // Auto-extract app metadata from app.yaml
        if (nf.path === "app.yaml") {
          const nameMatch = nf.content.match(/^name:\s*(.+)$/m)
          const descMatch = nf.content.match(/^description:\s*"?(.+?)"?\s*$/m)
          if (nameMatch) updated.name = nameMatch[1].trim()
          if (descMatch) updated.description = descMatch[1].trim()
        }
      }

      updated.files = Array.from(fileMap.values())
      return updated
    })

    // Auto-select the first generated file if nothing selected
    if (!selectedPath && newFiles.length > 0) {
      setSelectedPath(newFiles[0].path)
    }
  }, [selectedPath])

  // Update file content when manually edited
  const handleUpdateFile = useCallback((path: string, content: string) => {
    setProject(prev => ({
      ...prev,
      files: prev.files.map(f => f.path === path ? { ...f, content, dirty: true } : f),
    }))
  }, [])

  // Save the project as a real app (handles both create and update)
  const handleSave = async () => {
    setSaving(true)
    try {
      // Parse app.yaml for metadata
      const appYaml = project.files.find(f => f.path === "app.yaml")
      let appName = project.name || "my-app"
      let description = project.description || ""
      let category = project.category || "utilities"

      if (appYaml) {
        const nameMatch = appYaml.content.match(/^name:\s*(.+)$/m)
        const descMatch = appYaml.content.match(/^description:\s*"?(.+?)"?\s*$/m)
        if (nameMatch) appName = nameMatch[1].trim()
        if (descMatch) description = descMatch[1].trim()
      }

      let appId = savedId

      if (appId) {
        // --- UPDATE existing app ---
        await updateAppMutation.mutateAsync({
          id: appId,
          data: { description, category },
        })
      } else {
        // --- CREATE new app ---
        const app = await createAppMutation.mutateAsync({
          name: appName,
          displayName: appName.replace(/-/g, " ").replace(/\b\w/g, c => c.toUpperCase()),
          description,
          category,
        })
        appId = app.id
      }

      // Collect existing tool/prompt names so we know whether to add or update
      const existingToolNames = new Set(existingApp?.tools?.map(t => t.name) || [])
      const existingPromptNames = new Set(existingApp?.prompts?.map(p => p.name) || [])

      // Save tools (parse .json + .js pairs)
      const helpers = project.files.find(f => f.path === "tools/_helpers.js")?.content || ""

      for (const f of project.files) {
        if (f.type !== "json" || !f.path.startsWith("tools/")) continue
        const toolFileName = f.path.replace("tools/", "").replace(".json", "")
        const jsFile = project.files.find(j => j.path === `tools/${toolFileName}.js`)
        if (!jsFile) continue

        try {
          const schema = JSON.parse(f.content)
          const toolName = schema.name || toolFileName
          const toolData = {
            name: toolName,
            description: schema.description || "",
            toolClass: schema.toolClass || "read-only",
            mode: schema.mode || "",
            params: schema.params || {},
            script: helpers ? helpers + "\n\n" + jsFile.content : jsFile.content,
          }

          if (existingToolNames.has(toolName)) {
            await updateToolMutation.mutateAsync({
              appId: appId!, name: toolName, tool: toolData, changeSummary: "builder save",
            })
          } else {
            await addToolMutation.mutateAsync({ appId: appId!, tool: toolData })
          }
        } catch { /* skip invalid json */ }
      }

      // Save prompts (parse .md frontmatter)
      for (const f of project.files) {
        if (f.type !== "md" || !f.path.startsWith("prompts/")) continue
        const frontmatterMatch = f.content.match(/^---\n([\s\S]*?)\n---\n([\s\S]*)$/)
        if (!frontmatterMatch) continue

        const fm = frontmatterMatch[1]
        const body = frontmatterMatch[2].trim()
        const nameMatch = fm.match(/^name:\s*(.+)$/m)
        const descMatch = fm.match(/^description:\s*(.+)$/m)
        if (!nameMatch) continue

        const promptName = nameMatch[1].trim()
        const promptData = {
          name: promptName,
          description: descMatch?.[1]?.trim() || "",
          body,
        }

        if (existingPromptNames.has(promptName)) {
          await updatePromptMutation.mutateAsync({
            appId: appId!, name: promptName, prompt: promptData,
          })
        } else {
          await addPromptMutation.mutateAsync({ appId: appId!, prompt: promptData })
        }
      }

      // Save UI components
      const uiComponents = project.files
        .filter(f => f.type === "tsx" && f.path.startsWith("ui/"))
        .map(f => ({
          name: f.path.replace("ui/", "").replace(".tsx", ""),
          code: f.content,
        }))

      await updateAppMutation.mutateAsync({
        id: appId!,
        data: {
          description,
          longDescription: description.length >= 20 ? description : undefined,
          uiComponents: uiComponents.length > 0 ? uiComponents : undefined,
        },
      })

      setSavedId(appId)
    } catch (err) {
      console.error("Save failed:", err)
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="h-[calc(100vh-64px)] flex flex-col">
      {/* Top bar */}
      <TopBar
        project={project}
        savedId={savedId}
        saving={saving}
        onSave={handleSave}
        onLoadApp={(app) => { navigate(`/my-apps/create/${app.name}`); setProject(appToProject(app)); setSavedId(app.id); setSeeded(true); setSelectedPath(app.tools?.length ? `tools/${app.tools[0].name}.js` : "app.yaml") }}
        onNewApp={() => { navigate('/my-apps/create'); setProject(emptyProject()); setSavedId(null); setSeeded(false); setSelectedPath(null) }}
      />

      {/* Three-panel resizable layout */}
      <div className="flex-1 min-h-0">
        <Allotment>
          {/* Left: Chat */}
          <Allotment.Pane preferredSize={320} minSize={250} maxSize={500}>
            <ChatPanel
              project={project}
              onFilesGenerated={handleFilesGenerated}
              pendingMessage={pendingMessage}
              onPendingMessageConsumed={() => setPendingMessage(null)}
              appName={existingAppId || project.name || undefined}
              initialMessages={chatHistory?.messages?.map(m => {
                let content = m.content
                // Strip the system prompt prefix — sessions store the full enriched prompt,
                // but the user only typed the short message after "User: "
                if (m.role === 'user') {
                  const userMatch = content.match(/\n\nUser:\s*([\s\S]+)$/)
                  if (userMatch) content = userMatch[1].trim()
                }
                return { role: m.role as 'user' | 'assistant', content }
              }) || undefined}
              initialClaudeSessionId={chatHistory?.claude_session_id || undefined}
              onClearHistory={() => {
                if (existingAppId) {
                  fetch(`/api/my/apps/${encodeURIComponent(existingAppId)}/chat`, { method: 'DELETE' }).catch(() => {})
                }
                setChatHistory(null)
              }}
            />
          </Allotment.Pane>

          {/* Center: File Tree + Editor */}
          <Allotment.Pane preferredSize={500} minSize={300}>
            <Allotment>
              <Allotment.Pane preferredSize={170} minSize={120} maxSize={280}>
                <FileTree files={project.files} selectedPath={selectedPath} onSelect={setSelectedPath} />
              </Allotment.Pane>
              <Allotment.Pane minSize={200}>
                {selectedFile ? (
                  <FileEditor file={selectedFile} onUpdate={handleUpdateFile} toolRunFn={builderToolRunFn} />
                ) : (
                  <EmptyEditor />
                )}
              </Allotment.Pane>
            </Allotment>
          </Allotment.Pane>

          {/* Right: Preview */}
          <Allotment.Pane preferredSize={400} minSize={250} maxSize={700}>
            <PreviewPanel project={project} selectedFile={selectedFile} onRequestFix={setPendingMessage} appName={existingAppId || project.name} />
          </Allotment.Pane>
        </Allotment>
      </div>
    </div>
  )
}

function TopBar({ project, savedId, saving, onSave, onLoadApp, onNewApp }: {
  project: AppProject
  savedId: string | null
  saving: boolean
  onSave: () => void
  onLoadApp: (app: StoreApp) => void
  onNewApp: () => void
}) {
  const navigate = useNavigate()
  const { data: apps } = useMyApps()

  return (
    <div className="h-10 border-b border-border flex items-center justify-between px-4 shrink-0">
      <div className="flex items-center gap-3">
        <span className="text-xs font-mono font-medium uppercase tracking-wider" style={{ color: "#6366f1" }}>AI Architect</span>

        {/* App selector — open existing or start new */}
        <Select
          value={savedId || "__new__"}
          onValueChange={(v) => {
            if (v === "__new__") { onNewApp(); return }
            const app = apps?.find(a => a.id === v)
            if (app) onLoadApp(app)
          }}
        >
          <SelectTrigger className="h-7 w-48 rounded-none bg-background border-border text-xs font-mono">
            <SelectValue placeholder="New App" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__new__">+ New App</SelectItem>
            {apps?.map(a => (
              <SelectItem key={a.id} value={a.id}>
                {a.displayName || a.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <span className="text-[10px] text-muted-foreground">{project.files.length} files</span>
      </div>

      <div className="flex items-center gap-2">
        {savedId && (
          <Button size="sm" variant="ghost" className="rounded-none text-xs h-7" onClick={() => navigate(`/my-apps/${savedId}/edit`)}>
            Editor
          </Button>
        )}
        {savedId ? (
          <Button size="sm" className="rounded-none text-xs h-7" onClick={onSave} disabled={saving}>
            {saving ? <><Loader2 size={12} className="mr-1 animate-spin" /> Saving...</> : <><Save size={12} className="mr-1" /> Save</>}
          </Button>
        ) : (
          <Button size="sm" className="rounded-none text-xs h-7" onClick={onSave}
            disabled={saving || project.files.length <= 1}>
            {saving ? <><Loader2 size={12} className="mr-1 animate-spin" /> Saving...</> : <><Save size={12} className="mr-1" /> Save App</>}
          </Button>
        )}
        {savedId && (
          <span className="text-[10px] text-emerald-500 flex items-center gap-1"><Check size={10} /> Saved</span>
        )}
      </div>
    </div>
  )
}
