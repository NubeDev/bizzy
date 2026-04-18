import { useState } from "react"
import { useParams } from "react-router-dom"
import { Loader2, CheckCircle, Circle, FlaskConical, ChevronDown } from "lucide-react"
import { useMyApps, useMyApp } from "@/hooks/use-my-apps"
import { ToolWorkbench } from "@/components/workshop/tool-workbench"
import { Badge } from "@/components/ui/badge"
import type { StoreApp } from "@/lib/types"

/** Standalone workshop page at /workshop — pick an app, test its tools. Single-column layout. */
export function WorkshopPage() {
  const { id } = useParams<{ id: string }>()
  const [selectedAppId, setSelectedAppId] = useState<string | null>(id ?? null)

  return (
    <div className="max-w-3xl mx-auto py-6 px-4">
      {/* App picker — inline dropdown-style */}
      <AppPickerInline selectedId={selectedAppId} onSelect={setSelectedAppId} />

      {/* Tool workbench */}
      {selectedAppId ? (
        <AppWorkbench appId={selectedAppId} />
      ) : (
        <EmptyState />
      )}
    </div>
  )
}

function EmptyState() {
  return (
    <div className="flex flex-col items-center justify-center py-20 text-center">
      <div className="w-14 h-14 rounded-none bg-secondary flex items-center justify-center mb-4 border border-border">
        <FlaskConical size={24} className="text-foreground" />
      </div>
      <h2 className="font-mono text-lg font-light mb-1">Tool Workshop</h2>
      <p className="text-sm text-muted-foreground max-w-sm">
        Select an app above to test its tools. Each tool runs in a sandboxed runtime with full HTTP tracing.
      </p>
    </div>
  )
}

function AppPickerInline({ selectedId, onSelect }: { selectedId: string | null; onSelect: (id: string) => void }) {
  const { data: apps, isLoading } = useMyApps()
  const [open, setOpen] = useState(false)

  const selectedApp = apps?.find(a => a.id === selectedId)

  return (
    <div className="mb-6">
      <button
        onClick={() => setOpen(!open)}
        className="w-full flex items-center justify-between px-4 py-3 border border-border rounded-none bg-background hover:bg-accent/50 transition-colors"
      >
        <div className="flex items-center gap-3">
          {selectedApp ? (
            <>
              <div
                className="w-7 h-7 rounded-none flex items-center justify-center text-[11px] font-mono text-white shrink-0"
                style={{ background: selectedApp.color || "#1f2228" }}
              >
                {selectedApp.displayName?.charAt(0) || "?"}
              </div>
              <div className="text-left">
                <p className="text-sm font-medium">{selectedApp.displayName || selectedApp.name}</p>
                <p className="text-[11px] text-muted-foreground">{selectedApp.tools?.length ?? 0} tools</p>
              </div>
            </>
          ) : (
            <span className="text-sm text-muted-foreground">Select an app to test...</span>
          )}
        </div>
        <ChevronDown size={14} className={`text-muted-foreground transition-transform ${open ? "rotate-180" : ""}`} />
      </button>

      {open && (
        <div className="border border-t-0 border-border rounded-none bg-background max-h-60 overflow-y-auto">
          {isLoading && (
            <div className="flex justify-center py-6">
              <Loader2 size={16} className="animate-spin text-muted-foreground" />
            </div>
          )}
          {apps?.map((app) => {
            const toolCount = app.tools?.length ?? 0
            const isActive = app.id === selectedId
            return (
              <button
                key={app.id}
                onClick={() => { onSelect(app.id); setOpen(false) }}
                className={`w-full flex items-center gap-3 px-4 py-2.5 text-left transition-colors ${
                  isActive
                    ? "bg-accent text-foreground"
                    : "text-muted-foreground hover:bg-accent/50 hover:text-foreground"
                }`}
              >
                <div
                  className="w-6 h-6 rounded-none flex items-center justify-center text-[10px] font-mono text-white shrink-0"
                  style={{ background: app.color || "#1f2228" }}
                >
                  {app.displayName?.charAt(0) || "?"}
                </div>
                <div className="min-w-0">
                  <p className="text-sm font-medium truncate">{app.displayName || app.name}</p>
                  <p className="text-[11px] text-muted-foreground">{toolCount} tool{toolCount !== 1 ? "s" : ""}</p>
                </div>
              </button>
            )
          })}
          {apps && apps.length === 0 && (
            <p className="text-xs text-muted-foreground px-4 py-6 text-center">
              No apps yet. Create one first.
            </p>
          )}
        </div>
      )}
    </div>
  )
}

function AppWorkbench({ appId }: { appId: string }) {
  const { data: app, isLoading } = useMyApp(appId)
  const [activeToolName, setActiveToolName] = useState<string | null>(null)
  const [testedTools, setTestedTools] = useState<Set<string>>(new Set())

  if (isLoading || !app) {
    return (
      <div className="flex justify-center py-20">
        <Loader2 className="animate-spin text-muted-foreground" size={24} />
      </div>
    )
  }

  const tools = app.tools || []
  const activeTool = tools.find((t) => t.name === activeToolName) || tools[0] || null
  // For testing, fall back to ["*"] (allow all) when the app has no hosts configured.
  const allowedHosts = app.permissions?.allowedHosts?.length ? app.permissions.allowedHosts : ["*"]

  const settings: Record<string, string> = {}
  const secrets: Record<string, string> = {}
  for (const def of app.settings || []) {
    if (def.type === "secret") {
      secrets[def.key] = def.default || ""
    } else {
      settings[def.key] = def.default || ""
    }
  }

  return (
    <div className="space-y-4">
      {/* Tool tabs — horizontal */}
      {tools.length > 1 && (
        <div className="flex flex-wrap gap-1 border-b border-border pb-2">
          {tools.map((t) => {
            const tested = testedTools.has(t.name)
            const isActive = activeTool?.name === t.name
            return (
              <button
                key={t.name}
                onClick={() => setActiveToolName(t.name)}
                className={`flex items-center gap-1.5 px-3 py-1.5 text-xs font-mono rounded-none transition-colors ${
                  isActive
                    ? "bg-accent text-foreground border border-border"
                    : "text-muted-foreground hover:bg-accent/50 hover:text-foreground"
                }`}
              >
                {tested ? (
                  <CheckCircle size={12} className="text-emerald-400 shrink-0" />
                ) : (
                  <Circle size={12} className="text-muted-foreground/40 shrink-0" />
                )}
                {t.name}
              </button>
            )
          })}
        </div>
      )}

      {/* Active tool workbench */}
      {activeTool ? (
        <div>
          <div className="flex items-center gap-2 mb-1">
            <code className="text-sm font-semibold font-mono">{app.name}.{activeTool.name}</code>
            <Badge variant="outline" className="text-[10px]">{activeTool.toolClass}</Badge>
          </div>
          {activeTool.description && (
            <p className="text-xs text-muted-foreground mb-4">{activeTool.description}</p>
          )}
          {allowedHosts.length > 0 && (
            <p className="text-[11px] text-muted-foreground mb-4">
              Allowed hosts: {allowedHosts.map(h => <code key={h} className="mx-0.5">{h}</code>)}
            </p>
          )}
          <ToolWorkbench
            key={`${appId}-${activeTool.name}`}
            tool={activeTool}
            allowedHosts={allowedHosts}
            settings={settings}
            secrets={secrets}
            onScriptChange={() => {
              setTestedTools((prev) => {
                const next = new Set(prev)
                next.add(activeTool.name)
                return next
              })
            }}
          />
        </div>
      ) : (
        <div className="flex items-center justify-center py-16 text-sm text-muted-foreground">
          {tools.length ? "Select a tool to test." : "This app has no tools yet."}
        </div>
      )}
    </div>
  )
}
