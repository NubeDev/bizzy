import { useState } from "react"
import { Link, useNavigate } from "react-router-dom"
import { Plus, Loader2, Trash2, Pencil, Sparkles, Package, Wrench, MessageSquare, Download } from "lucide-react"
import { useMyApps, useCreateApp, useDeleteApp } from "@/hooks/use-my-apps"
import { VisibilityBadge } from "@/components/store/visibility-badge"
import { AppCardBase, AppIcon, PatternArea, ColorPill } from "@/components/store/app-card-base"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"

export function MyAppsPage() {
  const { data: apps, isLoading } = useMyApps()
  const createMutation = useCreateApp()
  const deleteMutation = useDeleteApp()
  const navigate = useNavigate()

  const [showCreate, setShowCreate] = useState(false)
  const [newName, setNewName] = useState("")
  const [newDisplayName, setNewDisplayName] = useState("")

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    const app = await createMutation.mutateAsync({
      name: newName.trim().toLowerCase().replace(/\s+/g, "-"),
      displayName: newDisplayName.trim() || newName.trim(),
    })
    setShowCreate(false)
    setNewName("")
    setNewDisplayName("")
    navigate(`/my-apps/${app.id}/edit`)
  }

  const handleDelete = async (id: string, name: string) => {
    if (!confirm(`Delete "${name}"? This cannot be undone.`)) return
    await deleteMutation.mutateAsync(id)
  }

  return (
    <div className="p-8 max-w-6xl mx-auto space-y-8">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-3xl font-light tracking-tight">My Apps</h2>
          <p className="text-sm text-muted-foreground mt-1">Manage and build your applications</p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={() => navigate("/my-apps/create")} className="rounded-full font-mono text-[11px] uppercase tracking-[1.4px] px-5">
            <Sparkles size={14} className="mr-1.5" /> AI Wizard
          </Button>
          <Button onClick={() => setShowCreate(true)} className="rounded-full font-mono text-[11px] uppercase tracking-[1.4px] px-5">
            <Plus size={14} className="mr-1.5" /> Create App
          </Button>
        </div>
      </div>

      {/* Create dialog */}
      <Dialog open={showCreate} onOpenChange={setShowCreate}>
        <DialogContent>
          <form onSubmit={handleCreate}>
            <DialogHeader>
              <DialogTitle>Create New App</DialogTitle>
            </DialogHeader>
            <div className="space-y-4 py-4">
              <div className="space-y-2">
                <Label>Slug (lowercase, hyphens)</Label>
                <Input
                  value={newName}
                  onChange={(e) => setNewName(e.target.value)}
                  placeholder="my-cool-app"
                  autoFocus
                />
              </div>
              <div className="space-y-2">
                <Label>Display Name</Label>
                <Input
                  value={newDisplayName}
                  onChange={(e) => setNewDisplayName(e.target.value)}
                  placeholder="My Cool App"
                />
              </div>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setShowCreate(false)}>
                Cancel
              </Button>
              <Button type="submit" disabled={!newName.trim() || createMutation.isPending}>
                {createMutation.isPending ? "Creating..." : "Create"}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {isLoading ? (
        <div className="flex justify-center py-20">
          <Loader2 className="animate-spin text-muted-foreground" size={24} />
        </div>
      ) : !apps?.length ? (
        <div className="text-center py-24 relative">
          <div className="absolute inset-0 dots-pattern opacity-30 pointer-events-none" />
          <div className="w-16 h-16 rounded-2xl bg-secondary flex items-center justify-center mx-auto mb-5 border border-border">
            <Package size={24} className="text-foreground" />
          </div>
          <p className="text-lg font-light mb-2">No apps yet</p>
          <p className="text-sm text-muted-foreground mb-8 max-w-sm mx-auto">Create your first app to get started.</p>
          <div className="flex gap-3 justify-center">
            <Button onClick={() => navigate("/my-apps/create")} className="rounded-full font-mono text-[11px] uppercase tracking-[1.4px] px-5">
              <Sparkles size={14} className="mr-1.5" /> Create with AI
            </Button>
            <Button variant="outline" onClick={() => setShowCreate(true)} className="rounded-full font-mono text-[11px] uppercase tracking-[1.4px] px-5">
              <Plus size={14} className="mr-1.5" /> Manual
            </Button>
          </div>
        </div>
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-6">
          {apps.map((app) => {
            const color = app.color || '#6366f1'
            return (
              <div key={app.id} className="group relative h-full">
                <AppCardBase
                  color={color}
                  stats={
                    <>
                      <span className="flex items-center gap-1">
                        <Wrench size={11} /> {app.tools?.length || 0}
                      </span>
                      <span className="flex items-center gap-1">
                        <MessageSquare size={11} /> {app.prompts?.length || 0}
                      </span>
                      <span className="flex items-center gap-1">
                        <Download size={11} /> {app.installCount}
                      </span>
                    </>
                  }
                  footer={
                    <div className="flex items-center gap-2">
                      <VisibilityBadge visibility={app.visibility} />
                      <ColorPill label={`v${app.version}`} color={color} />
                    </div>
                  }
                >
                  {/* Title + meta */}
                  <div className="px-5 pt-5 pb-3">
                    <div className="flex items-start gap-3 mb-3">
                      <AppIcon name={app.displayName} color={color} />
                      <div className="min-w-0 flex-1">
                        <p className="font-semibold text-[15px] leading-tight truncate">{app.displayName}</p>
                        <p className="text-xs text-muted-foreground mt-0.5">{app.description || "No description"}</p>
                      </div>
                    </div>
                  </div>

                  {/* Visual pattern area */}
                  <PatternArea
                    name={app.displayName}
                    color={color}
                    className="flex-1 mx-4 mb-3 min-h-[80px]"
                  />

                  {/* Hover action buttons — floating top-right */}
                  <div className="absolute top-3 right-3 flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity z-10">
                    <Button variant="secondary" size="icon" asChild className="rounded-xl h-8 w-8 shadow-sm">
                      <Link to={`/my-apps/${app.id}/edit`} title="Edit">
                        <Pencil size={13} />
                      </Link>
                    </Button>
                    <Button
                      variant="secondary"
                      size="icon"
                      onClick={() => handleDelete(app.id, app.displayName)}
                      className="rounded-xl h-8 w-8 shadow-sm hover:text-destructive"
                      title="Delete"
                    >
                      <Trash2 size={13} />
                    </Button>
                  </div>
                </AppCardBase>
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
