import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useFlows, useCreateFlow, useDeleteFlow } from '@/hooks/use-flows'
import { Plus, Trash2, Copy, GitBranch, Clock } from 'lucide-react'
import { api } from '@/lib/api'
import { formatDate } from '@/lib/utils'

export function FlowsPage() {
  const { data: flows, isLoading } = useFlows()
  const createFlow = useCreateFlow()
  const deleteFlow = useDeleteFlow()
  const navigate = useNavigate()
  const [creating, setCreating] = useState(false)
  const [showNameInput, setShowNameInput] = useState(false)
  const [newName, setNewName] = useState('')

  const handleCreate = async () => {
    const name = newName.trim() || `untitled-${Date.now().toString(36)}`
    setCreating(true)
    try {
      const flow = await createFlow.mutateAsync({
        name,
        description: '',
        nodes: [
          { id: 'trigger-1', type: 'trigger', label: 'Trigger', position: { x: 100, y: 200 } },
          { id: 'output-1', type: 'output', label: 'Output', position: { x: 500, y: 200 } },
        ],
        edges: [
          { id: 'e1', source: 'trigger-1', sourceHandle: 'output', target: 'output-1', targetHandle: 'input' },
        ],
      })
      navigate(`/flows/${flow.id}`)
    } finally {
      setCreating(false)
      setShowNameInput(false)
      setNewName('')
    }
  }

  const handleDuplicate = async (e: React.MouseEvent, id: string) => {
    e.stopPropagation()
    const dup = await api.duplicateFlow(id)
    navigate(`/flows/${dup.id}`)
  }

  const handleDelete = (e: React.MouseEvent, id: string) => {
    e.stopPropagation()
    deleteFlow.mutate(id)
  }

  return (
    <div className="max-w-3xl mx-auto px-6 py-10">
      {/* Header */}
      <div className="flex items-end justify-between mb-8">
        <div>
          <h1 className="text-xl font-medium tracking-tight">Flows</h1>
          <p className="text-xs text-muted-foreground mt-1">
            Visual DAG workflows — drag nodes, connect edges, execute.
          </p>
        </div>

        {showNameInput ? (
          <div className="flex items-center gap-2">
            <input
              autoFocus
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              onKeyDown={(e) => { if (e.key === 'Enter') handleCreate(); if (e.key === 'Escape') setShowNameInput(false) }}
              placeholder="flow name..."
              className="px-3 py-1.5 text-xs bg-background border border-border rounded w-48 outline-none focus:border-primary"
            />
            <button
              onClick={handleCreate}
              disabled={creating}
              className="px-3 py-1.5 text-xs bg-primary text-primary-foreground rounded hover:opacity-90 transition-opacity"
            >
              Create
            </button>
            <button
              onClick={() => setShowNameInput(false)}
              className="px-2 py-1.5 text-xs text-muted-foreground hover:text-foreground"
            >
              Cancel
            </button>
          </div>
        ) : (
          <button
            onClick={() => setShowNameInput(true)}
            className="flex items-center gap-1.5 px-4 py-1.5 text-xs bg-primary text-primary-foreground rounded hover:opacity-90 transition-opacity"
          >
            <Plus className="w-3.5 h-3.5" />
            New Flow
          </button>
        )}
      </div>

      {/* List */}
      {isLoading ? (
        <div className="text-xs text-muted-foreground py-8 text-center">Loading flows...</div>
      ) : !flows?.length ? (
        <div className="text-center py-20 border border-dashed border-border rounded-lg">
          <GitBranch className="w-10 h-10 text-muted-foreground/20 mx-auto mb-4" />
          <p className="text-sm font-medium text-muted-foreground">No flows yet</p>
          <p className="text-xs text-muted-foreground/60 mt-1 mb-4">
            Create your first flow to start building visual workflows
          </p>
          <button
            onClick={() => setShowNameInput(true)}
            className="inline-flex items-center gap-1.5 px-4 py-1.5 text-xs bg-primary text-primary-foreground rounded hover:opacity-90 transition-opacity"
          >
            <Plus className="w-3.5 h-3.5" />
            New Flow
          </button>
        </div>
      ) : (
        <div className="space-y-1.5">
          {flows.map((flow) => (
            <div
              key={flow.id}
              onClick={() => navigate(`/flows/${flow.id}`)}
              className="flex items-center gap-4 px-4 py-3 rounded-lg border border-border hover:border-primary/30 hover:bg-accent/30 cursor-pointer transition-all group"
            >
              <div className="w-8 h-8 rounded bg-primary/10 flex items-center justify-center shrink-0">
                <GitBranch className="w-4 h-4 text-primary" />
              </div>

              <div className="flex-1 min-w-0">
                <div className="text-sm font-medium">{flow.name}</div>
                <div className="flex items-center gap-3 mt-0.5">
                  {flow.description && (
                    <span className="text-[10px] text-muted-foreground truncate">{flow.description}</span>
                  )}
                  <span className="text-[10px] text-muted-foreground/60 flex items-center gap-1">
                    <Clock className="w-2.5 h-2.5" />
                    {formatDate(flow.updated_at)}
                  </span>
                </div>
              </div>

              <div className="flex items-center gap-3 text-[10px] text-muted-foreground">
                <span>{flow.nodes?.length || 0} nodes</span>
                <span className="text-muted-foreground/40">v{flow.version}</span>
              </div>

              <div className="flex items-center gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity">
                <button
                  onClick={(e) => handleDuplicate(e, flow.id)}
                  className="p-1.5 hover:bg-accent rounded transition-colors"
                  title="Duplicate"
                >
                  <Copy className="w-3.5 h-3.5 text-muted-foreground" />
                </button>
                <button
                  onClick={(e) => handleDelete(e, flow.id)}
                  className="p-1.5 hover:bg-red-500/10 rounded transition-colors"
                  title="Delete"
                >
                  <Trash2 className="w-3.5 h-3.5 text-red-400" />
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
