import { Loader2, Plug, Power, PowerOff, Trash2, Wrench, MessageSquare, RefreshCw } from "lucide-react"
import { usePlugins, useTogglePlugin, useDeletePlugin } from "@/hooks/use-plugins"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import type { PluginSummary } from "@/lib/types"

function statusColor(status: string) {
  switch (status) {
    case "active":
      return "bg-emerald-500/15 text-emerald-600 dark:text-emerald-400"
    case "disabled":
      return "bg-zinc-500/15 text-zinc-500"
    case "crashed":
      return "bg-red-500/15 text-red-500"
    default:
      return "bg-zinc-500/15 text-zinc-500"
  }
}

function timeAgo(iso?: string) {
  if (!iso) return "never"
  const diff = Date.now() - new Date(iso).getTime()
  const secs = Math.floor(diff / 1000)
  if (secs < 60) return `${secs}s ago`
  const mins = Math.floor(secs / 60)
  if (mins < 60) return `${mins}m ago`
  const hrs = Math.floor(mins / 60)
  return `${hrs}h ago`
}

function PluginRow({ plugin }: { plugin: PluginSummary }) {
  const toggleMutation = useTogglePlugin()
  const deleteMutation = useDeletePlugin()

  const isActive = plugin.status === "active"
  const isDisabled = plugin.status === "disabled"

  const handleToggle = () => {
    toggleMutation.mutate({ name: plugin.name, enabled: !isActive && !isDisabled ? false : isDisabled })
  }

  const handleDelete = () => {
    if (!confirm(`Unload plugin "${plugin.name}"? It will need to reconnect.`)) return
    deleteMutation.mutate(plugin.name)
  }

  return (
    <tr className="border-b border-border/50 hover:bg-muted/30 transition-colors">
      {/* Name & description */}
      <td className="py-3 px-4">
        <div className="flex items-center gap-3">
          <div className="w-8 h-8 rounded-lg bg-primary/10 flex items-center justify-center shrink-0">
            <Plug size={14} className="text-primary" />
          </div>
          <div className="min-w-0">
            <p className="font-medium text-sm truncate">{plugin.name}</p>
            {plugin.description && (
              <p className="text-xs text-muted-foreground truncate max-w-[300px]">{plugin.description}</p>
            )}
          </div>
        </div>
      </td>

      {/* Version */}
      <td className="py-3 px-4">
        <span className="text-xs text-muted-foreground font-mono">{plugin.version}</span>
      </td>

      {/* Status */}
      <td className="py-3 px-4">
        <Badge variant="secondary" className={`text-[10px] uppercase tracking-wider font-semibold ${statusColor(plugin.status)}`}>
          {plugin.status}
        </Badge>
      </td>

      {/* Services */}
      <td className="py-3 px-4">
        <div className="flex gap-1 flex-wrap">
          {plugin.services.map((s) => (
            <Badge key={s} variant="outline" className="text-[10px]">{s}</Badge>
          ))}
        </div>
      </td>

      {/* Tools / Prompts */}
      <td className="py-3 px-4">
        <div className="flex items-center gap-3 text-xs text-muted-foreground">
          <span className="flex items-center gap-1" title="Tools">
            <Wrench size={11} /> {plugin.tool_count}
          </span>
          <span className="flex items-center gap-1" title="Prompts">
            <MessageSquare size={11} /> {plugin.prompt_count}
          </span>
        </div>
      </td>

      {/* Last heartbeat */}
      <td className="py-3 px-4">
        <span className="text-xs text-muted-foreground">{timeAgo(plugin.last_heartbeat)}</span>
      </td>

      {/* Actions */}
      <td className="py-3 px-4">
        <div className="flex items-center gap-1">
          <Button
            variant="ghost"
            size="icon"
            className="h-7 w-7"
            onClick={handleToggle}
            disabled={toggleMutation.isPending}
            title={isActive ? "Disable" : "Enable"}
          >
            {isActive ? <PowerOff size={13} /> : <Power size={13} />}
          </Button>
          <Button
            variant="ghost"
            size="icon"
            className="h-7 w-7 hover:text-destructive"
            onClick={handleDelete}
            disabled={deleteMutation.isPending}
            title="Unload"
          >
            <Trash2 size={13} />
          </Button>
        </div>
      </td>
    </tr>
  )
}

export function PluginsPage() {
  const { data: plugins, isLoading, refetch, isRefetching } = usePlugins()

  return (
    <div className="p-8 max-w-6xl mx-auto space-y-8">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-3xl font-light tracking-tight">Plugins</h2>
          <p className="text-sm text-muted-foreground mt-1">Connected plugins and their status</p>
        </div>
        <Button
          variant="outline"
          onClick={() => refetch()}
          disabled={isRefetching}
          className="rounded-full font-mono text-[11px] uppercase tracking-[1.4px] px-5"
        >
          <RefreshCw size={14} className={`mr-1.5 ${isRefetching ? "animate-spin" : ""}`} /> Refresh
        </Button>
      </div>

      {/* Content */}
      {isLoading ? (
        <div className="flex justify-center py-20">
          <Loader2 className="animate-spin text-muted-foreground" size={24} />
        </div>
      ) : !plugins?.length ? (
        <div className="text-center py-24 relative">
          <div className="absolute inset-0 dots-pattern opacity-30 pointer-events-none" />
          <div className="w-16 h-16 rounded-2xl bg-secondary flex items-center justify-center mx-auto mb-5 border border-border">
            <Plug size={24} className="text-foreground" />
          </div>
          <p className="text-lg font-light mb-2">No plugins connected</p>
          <p className="text-sm text-muted-foreground mb-8 max-w-sm mx-auto">
            Plugins connect automatically via NATS. Start a plugin process and it will appear here.
          </p>
        </div>
      ) : (
        <div className="border border-border rounded-lg overflow-hidden">
          <table className="w-full text-left">
            <thead>
              <tr className="border-b border-border bg-muted/40">
                <th className="py-2.5 px-4 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">Plugin</th>
                <th className="py-2.5 px-4 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">Version</th>
                <th className="py-2.5 px-4 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">Status</th>
                <th className="py-2.5 px-4 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">Services</th>
                <th className="py-2.5 px-4 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">Resources</th>
                <th className="py-2.5 px-4 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">Heartbeat</th>
                <th className="py-2.5 px-4 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">Actions</th>
              </tr>
            </thead>
            <tbody>
              {plugins.map((p) => (
                <PluginRow key={p.name} plugin={p} />
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
