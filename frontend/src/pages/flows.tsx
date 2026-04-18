import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useFlows, useCreateFlow, useDeleteFlow } from '@/hooks/use-flows'
import { Plus, Trash2, Copy, GitBranch } from 'lucide-react'
import { cn } from '@/lib/utils'
import { api } from '@/lib/api'

export function FlowsPage() {
  const { data: flows, isLoading } = useFlows()
  const createFlow = useCreateFlow()
  const deleteFlow = useDeleteFlow()
  const navigate = useNavigate()
  const [creating, setCreating] = useState(false)

  const handleCreate = async () => {
    setCreating(true)
    try {
      const flow = await createFlow.mutateAsync({
        name: `flow-${Date.now()}`,
        description: '',
        nodes: [
          {
            id: 'trigger-1',
            type: 'trigger',
            label: 'Trigger',
            position: { x: 50, y: 200 },
          },
          {
            id: 'output-1',
            type: 'output',
            label: 'Output',
            position: { x: 400, y: 200 },
          },
        ],
        edges: [
          {
            id: 'e-trigger-output',
            source: 'trigger-1',
            sourceHandle: 'output',
            target: 'output-1',
            targetHandle: 'input',
          },
        ],
      })
      navigate(`/flows/${flow.id}`)
    } finally {
      setCreating(false)
    }
  }

  const handleDuplicate = async (id: string) => {
    const dup = await api.duplicateFlow(id)
    navigate(`/flows/${dup.id}`)
  }

  return (
    <div className="max-w-4xl mx-auto px-6 py-8">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-lg font-medium">Flows</h1>
          <p className="text-xs text-muted-foreground mt-0.5">
            Visual DAG workflows — drag nodes, connect edges, execute.
          </p>
        </div>
        <button
          onClick={handleCreate}
          disabled={creating}
          className="flex items-center gap-1.5 px-3 py-1.5 text-xs bg-primary/10 text-primary rounded hover:bg-primary/20 transition-colors"
        >
          <Plus className="w-3.5 h-3.5" />
          New Flow
        </button>
      </div>

      {isLoading ? (
        <div className="text-xs text-muted-foreground">Loading...</div>
      ) : !flows?.length ? (
        <div className="text-center py-16">
          <GitBranch className="w-8 h-8 text-muted-foreground/30 mx-auto mb-3" />
          <p className="text-sm text-muted-foreground">No flows yet</p>
          <p className="text-xs text-muted-foreground mt-1">Create your first visual workflow</p>
        </div>
      ) : (
        <div className="space-y-1">
          {flows.map((flow) => (
            <div
              key={flow.id}
              className="flex items-center gap-3 px-4 py-3 rounded border border-border hover:bg-accent/50 cursor-pointer transition-colors group"
              onClick={() => navigate(`/flows/${flow.id}`)}
            >
              <GitBranch className="w-4 h-4 text-muted-foreground" />
              <div className="flex-1 min-w-0">
                <div className="text-sm font-medium">{flow.name}</div>
                {flow.description && (
                  <div className="text-[10px] text-muted-foreground truncate">{flow.description}</div>
                )}
              </div>
              <div className="text-[10px] text-muted-foreground">
                v{flow.version}
              </div>
              <div className="text-[10px] text-muted-foreground">
                {flow.nodes?.length || 0} nodes
              </div>
              <div className="flex items-center gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity">
                <button
                  onClick={(e) => { e.stopPropagation(); handleDuplicate(flow.id) }}
                  className="p-1 hover:bg-accent rounded"
                  title="Duplicate"
                >
                  <Copy className="w-3 h-3 text-muted-foreground" />
                </button>
                <button
                  onClick={(e) => { e.stopPropagation(); deleteFlow.mutate(flow.id) }}
                  className="p-1 hover:bg-red-500/10 rounded"
                  title="Delete"
                >
                  <Trash2 className="w-3 h-3 text-red-400" />
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
