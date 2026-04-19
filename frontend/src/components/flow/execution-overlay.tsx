import { useState } from 'react'
import { useApproveNode, useRejectNode } from '@/hooks/use-flows'
import type { FlowRun, FlowRunStatus, NodeState } from '@/lib/types'
import { cn } from '@/lib/utils'
import { X, CheckCircle, XCircle, Loader2, Clock, Ban, ChevronRight } from 'lucide-react'

interface ExecutionOverlayProps {
  run?: FlowRun
  onClose: () => void
}

const statusConfig: Record<FlowRunStatus, { label: string; color: string; icon: React.ReactNode }> = {
  pending: { label: 'Pending', color: 'text-muted-foreground', icon: <Clock className="w-3.5 h-3.5" /> },
  running: { label: 'Running', color: 'text-blue-500', icon: <Loader2 className="w-3.5 h-3.5 animate-spin" /> },
  waiting_approval: { label: 'Waiting Approval', color: 'text-amber-500', icon: <Clock className="w-3.5 h-3.5" /> },
  completed: { label: 'Completed', color: 'text-green-500', icon: <CheckCircle className="w-3.5 h-3.5" /> },
  failed: { label: 'Failed', color: 'text-red-500', icon: <XCircle className="w-3.5 h-3.5" /> },
  cancelled: { label: 'Cancelled', color: 'text-muted-foreground', icon: <Ban className="w-3.5 h-3.5" /> },
}

export function ExecutionOverlay({ run, onClose }: ExecutionOverlayProps) {
  const approve = useApproveNode()
  const reject = useRejectNode()
  const [expandedNode, setExpandedNode] = useState<string | null>(null)

  if (!run) return null

  const config = statusConfig[run.status]

  const waitingNodes = Object.entries(run.node_states || {}).filter(
    ([, state]) => state.status === 'waiting',
  )

  return (
    <div className="absolute bottom-4 right-4 w-96 max-h-[500px] bg-card border border-border rounded shadow-lg overflow-hidden z-50">
      {/* Header */}
      <div className="flex items-center justify-between px-3 py-2 border-b border-border">
        <div className="flex items-center gap-2">
          <span className={config.color}>{config.icon}</span>
          <span className="text-xs font-medium">{run.flow_name}</span>
          <span className={cn('text-[10px]', config.color)}>{config.label}</span>
        </div>
        <button onClick={onClose} className="p-0.5 hover:bg-accent rounded">
          <X className="w-3 h-3" />
        </button>
      </div>

      {/* Node states */}
      <div className="overflow-y-auto max-h-[350px] p-2 space-y-0.5">
        {Object.entries(run.node_states || {})
          .filter(([id]) => !id.includes(':iter-'))
          .map(([nodeId, state]) => (
            <NodeRow
              key={nodeId}
              nodeId={nodeId}
              state={state}
              expanded={expandedNode === nodeId}
              onToggle={() => setExpandedNode(expandedNode === nodeId ? null : nodeId)}
            />
          ))}
      </div>

      {/* Approval actions */}
      {waitingNodes.length > 0 && (
        <div className="border-t border-border p-2 space-y-1">
          {waitingNodes.map(([nodeId]) => (
            <div key={nodeId} className="flex items-center gap-1">
              <span className="text-[10px] text-muted-foreground flex-1 font-mono truncate">
                {nodeId}
              </span>
              <button
                onClick={() => approve.mutate({ runId: run.id, nodeId })}
                disabled={approve.isPending}
                className="px-2 py-0.5 text-[10px] bg-green-500/10 text-green-500 rounded hover:bg-green-500/20 transition-colors"
              >
                Approve
              </button>
              <button
                onClick={() => reject.mutate({ runId: run.id, nodeId })}
                disabled={reject.isPending}
                className="px-2 py-0.5 text-[10px] bg-red-500/10 text-red-400 rounded hover:bg-red-500/20 transition-colors"
              >
                Reject
              </button>
            </div>
          ))}
        </div>
      )}

      {/* Flow output */}
      {run.output && (
        <div className="border-t border-border px-3 py-2">
          <div className="text-[10px] font-mono uppercase tracking-wider text-muted-foreground mb-1">Flow Output</div>
          <pre className="text-[10px] font-mono text-foreground bg-background p-1.5 rounded border border-border overflow-x-auto max-h-[80px]">
            {JSON.stringify(run.output, null, 2)}
          </pre>
        </div>
      )}

      {/* Error */}
      {run.error && (
        <div className="border-t border-red-500/20 px-3 py-2 text-[10px] text-red-400">
          {run.error}
        </div>
      )}

      {/* Footer */}
      <div className="border-t border-border px-3 py-1.5 flex items-center gap-2 text-[10px] text-muted-foreground">
        <span className="font-mono">{run.id}</span>
        <span className="ml-auto">v{run.flow_version}</span>
      </div>
    </div>
  )
}

function NodeRow({ nodeId, state, expanded, onToggle }: {
  nodeId: string
  state: NodeState
  expanded: boolean
  onToggle: () => void
}) {
  const hasData = state.input != null || state.output != null
  return (
    <div>
      <div
        onClick={hasData ? onToggle : undefined}
        className={cn(
          'flex items-center gap-2 px-2 py-1 rounded text-xs',
          hasData && 'cursor-pointer hover:bg-accent/50',
          state.status === 'running' && 'bg-blue-500/10',
          state.status === 'completed' && 'bg-green-500/5',
          state.status === 'failed' && 'bg-red-500/5',
          state.status === 'waiting' && 'bg-amber-500/10',
        )}
      >
        {hasData && (
          <ChevronRight className={cn('w-3 h-3 text-muted-foreground transition-transform', expanded && 'rotate-90')} />
        )}
        <NodeStatusIcon status={state.status} />
        <span className="font-mono truncate flex-1">{nodeId}</span>
        {state.duration_ms != null && state.status === 'completed' && (
          <span className="text-[10px] text-muted-foreground">
            {state.duration_ms < 1000 ? `${state.duration_ms}ms` : `${(state.duration_ms / 1000).toFixed(1)}s`}
          </span>
        )}
        {state.error && (
          <span className="text-[10px] text-red-400 truncate max-w-[100px]" title={state.error}>
            {state.error}
          </span>
        )}
      </div>

      {/* Expanded: show input/output port values */}
      {expanded && (
        <div className="ml-6 mr-2 mb-1 space-y-1">
          {state.input != null && (
            <PortValueDisplay label="Input" value={state.input} />
          )}
          {state.output != null && (
            <PortValueDisplay label="Output" value={state.output} />
          )}
        </div>
      )}
    </div>
  )
}

function PortValueDisplay({ label, value }: { label: string; value: unknown }) {
  const text = typeof value === 'string' ? value : JSON.stringify(value, null, 2)
  return (
    <div>
      <div className="text-[9px] font-mono uppercase tracking-wider text-muted-foreground">{label}</div>
      <pre className="text-[10px] font-mono text-foreground bg-background p-1 rounded border border-border overflow-x-auto max-h-[60px] whitespace-pre-wrap break-all">
        {text}
      </pre>
    </div>
  )
}

function NodeStatusIcon({ status }: { status: string }) {
  switch (status) {
    case 'completed': return <span className="w-1.5 h-1.5 rounded-full bg-green-500" />
    case 'running': return <span className="w-1.5 h-1.5 rounded-full bg-blue-500 animate-pulse" />
    case 'failed': return <span className="w-1.5 h-1.5 rounded-full bg-red-500" />
    case 'waiting': return <span className="w-1.5 h-1.5 rounded-full bg-amber-500 animate-pulse" />
    case 'skipped': return <span className="w-1.5 h-1.5 rounded-full bg-muted-foreground/50" />
    case 'ready': return <span className="w-1.5 h-1.5 rounded-full bg-yellow-500" />
    default: return <span className="w-1.5 h-1.5 rounded-full bg-muted-foreground/30" />
  }
}

// Helper to extract node states for the canvas overlay
export function useNodeStatesForCanvas(run: FlowRun | undefined) {
  if (!run) return undefined
  return run.node_states
}
