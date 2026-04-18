import { Handle, Position, type NodeProps } from '@xyflow/react'
import type { FlowPortDef } from '@/lib/types'
import { cn } from '@/lib/utils'

export interface BaseNodeData {
  label?: string
  nodeType?: string
  category?: string
  source?: string
  description?: string
  inputPorts?: FlowPortDef[]
  outputPorts?: FlowPortDef[]
  config?: Record<string, unknown>
  // Execution state (set by overlay)
  status?: string
  error?: string
  duration_ms?: number
}

const categoryColors: Record<string, string> = {
  'flow-control': 'border-blue-500/50 bg-blue-500/5',
  'tool': 'border-purple-500/50 bg-purple-500/5',
  'integration': 'border-emerald-500/50 bg-emerald-500/5',
  'data': 'border-amber-500/50 bg-amber-500/5',
  'plugin': 'border-cyan-500/50 bg-cyan-500/5',
}

const statusStyles: Record<string, string> = {
  pending: '',
  ready: 'ring-2 ring-yellow-500/50',
  running: 'ring-2 ring-blue-500 animate-pulse',
  completed: 'ring-2 ring-green-500',
  failed: 'ring-2 ring-red-500',
  skipped: 'opacity-50',
  waiting: 'ring-2 ring-amber-500 animate-pulse',
}

export function BaseNode({ data, selected }: NodeProps) {
  const d = data as BaseNodeData
  const colorClass = categoryColors[d.category || ''] || 'border-border bg-card'
  const statusClass = statusStyles[d.status || ''] || ''

  return (
    <div
      className={cn(
        'min-w-[160px] rounded border shadow-sm',
        colorClass,
        statusClass,
        selected && 'ring-2 ring-primary',
      )}
    >
      {/* Header */}
      <div className="flex items-center gap-1.5 px-3 py-1.5 border-b border-border/50">
        <span className="text-[10px] font-mono uppercase tracking-wider text-muted-foreground">
          {d.nodeType || 'node'}
        </span>
        {d.status === 'completed' && <span className="ml-auto text-green-500 text-xs">&#10003;</span>}
        {d.status === 'failed' && <span className="ml-auto text-red-500 text-xs">&#10007;</span>}
      </div>

      {/* Label */}
      <div className="px-3 py-1.5 text-xs font-medium">
        {d.label || d.nodeType}
      </div>

      {/* Input Ports */}
      {d.inputPorts?.map((port, i) => (
        <Handle
          key={`in-${port.handle}`}
          type="target"
          position={Position.Left}
          id={port.handle}
          style={{ top: `${30 + (i + 1) * 20}px` }}
          className="!w-2.5 !h-2.5 !bg-muted-foreground/50 !border-background"
        />
      ))}

      {/* Port labels */}
      {(d.inputPorts?.length || 0) > 0 && (
        <div className="px-3 pb-1.5 space-y-0.5">
          {d.inputPorts?.map((port) => (
            <div key={port.handle} className="text-[10px] text-muted-foreground flex items-center gap-1">
              <span className="w-1.5 h-1.5 rounded-full bg-muted-foreground/30" />
              {port.label || port.handle}
              {port.required && <span className="text-red-400">*</span>}
            </div>
          ))}
        </div>
      )}

      {(d.outputPorts?.length || 0) > 0 && (
        <div className="px-3 pb-1.5 space-y-0.5">
          {d.outputPorts?.map((port) => (
            <div key={port.handle} className="text-[10px] text-muted-foreground text-right flex items-center justify-end gap-1">
              {port.label || port.handle}
              <span className="w-1.5 h-1.5 rounded-full bg-muted-foreground/30" />
            </div>
          ))}
        </div>
      )}

      {/* Output Ports */}
      {d.outputPorts?.map((port, i) => (
        <Handle
          key={`out-${port.handle}`}
          type="source"
          position={Position.Right}
          id={port.handle}
          style={{ top: `${30 + (i + 1) * 20}px` }}
          className="!w-2.5 !h-2.5 !bg-muted-foreground/50 !border-background"
        />
      ))}

      {/* Error tooltip */}
      {d.error && (
        <div className="px-3 py-1 text-[10px] text-red-400 border-t border-red-500/20 truncate max-w-[200px]">
          {d.error}
        </div>
      )}

      {/* Duration */}
      {d.status === 'completed' && d.duration_ms != null && (
        <div className="px-3 pb-1 text-[10px] text-muted-foreground text-right">
          {d.duration_ms < 1000 ? `${d.duration_ms}ms` : `${(d.duration_ms / 1000).toFixed(1)}s`}
        </div>
      )}
    </div>
  )
}
