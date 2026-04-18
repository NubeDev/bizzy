import { memo } from 'react'
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
  inputValue?: unknown
  outputValue?: unknown
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
  completed: 'ring-2 ring-green-500/50',
  failed: 'ring-2 ring-red-500',
  skipped: 'opacity-50',
  waiting: 'ring-2 ring-amber-500 animate-pulse',
}

export const BaseNode = memo(function BaseNode({ data, selected }: NodeProps) {
  const d = data as BaseNodeData
  const colorClass = categoryColors[d.category || ''] || 'border-border bg-card'
  const statusClass = statusStyles[d.status || ''] || ''

  return (
    <div
      className={cn(
        'min-w-[140px] rounded border shadow-sm',
        colorClass,
        statusClass,
        selected && 'ring-2 ring-primary',
      )}
    >
      {/* Header */}
      <div className="flex items-center gap-1.5 px-2.5 py-1 border-b border-border/50">
        <span className="text-[10px] font-mono uppercase tracking-wider text-muted-foreground">
          {d.nodeType || 'node'}
        </span>
        <span className="ml-auto flex items-center gap-1">
          {d.status === 'completed' && d.duration_ms != null && (
            <span className="text-[9px] text-muted-foreground/70">
              {d.duration_ms < 1000 ? `${d.duration_ms}ms` : `${(d.duration_ms / 1000).toFixed(1)}s`}
            </span>
          )}
          {d.status === 'completed' && <span className="text-green-500 text-[10px]">&#10003;</span>}
          {d.status === 'failed' && <span className="text-red-500 text-[10px]">&#10007;</span>}
        </span>
      </div>

      {/* Label */}
      <div className="px-2.5 py-1 text-xs font-medium">
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
        <div className="px-2.5 pb-1 space-y-0.5">
          {d.inputPorts?.map((port) => (
            <PortLabel
              key={port.handle}
              port={port}
              side="input"
              value={d.inputValue}
            />
          ))}
        </div>
      )}

      {(d.outputPorts?.length || 0) > 0 && (
        <div className="px-2.5 pb-1 space-y-0.5">
          {d.outputPorts?.map((port) => (
            <PortLabel
              key={port.handle}
              port={port}
              side="output"
              value={d.outputValue}
            />
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

      {/* Error */}
      {d.error && (
        <div className="px-2.5 py-1 text-[10px] text-red-400 border-t border-red-500/20 truncate max-w-[200px]">
          {d.error}
        </div>
      )}
    </div>
  )
})

function PortLabel({ port, side, value }: { port: FlowPortDef; side: 'input' | 'output'; value?: unknown }) {
  const hasValue = value != null
  const isInput = side === 'input'

  return (
    <div className={cn('group relative text-[10px] text-muted-foreground flex items-center gap-1', !isInput && 'justify-end')}>
      {isInput && (
        <span className={cn('w-1.5 h-1.5 rounded-full shrink-0', hasValue ? 'bg-green-500' : 'bg-muted-foreground/30')} />
      )}
      {port.label || port.handle}
      {isInput && port.required && <span className="text-red-400">*</span>}
      {!isInput && (
        <span className={cn('w-1.5 h-1.5 rounded-full shrink-0', hasValue ? 'bg-green-500' : 'bg-muted-foreground/30')} />
      )}

      {/* Hover tooltip */}
      {hasValue && (
        <div className={cn(
          'absolute z-50 hidden group-hover:block',
          'bg-popover border border-border rounded shadow-lg p-2',
          'min-w-[180px] max-w-[320px]',
          isInput ? 'left-0 top-full mt-1' : 'right-0 top-full mt-1',
        )}>
          <div className="text-[9px] font-mono uppercase tracking-wider text-muted-foreground mb-1">
            {side}
          </div>
          <pre className="text-[10px] font-mono text-foreground whitespace-pre-wrap break-all max-h-[200px] overflow-y-auto">
            {formatValue(value)}
          </pre>
        </div>
      )}
    </div>
  )
}

function formatValue(value: unknown): string {
  if (value == null) return 'null'
  if (typeof value === 'string') return value
  if (typeof value === 'number' || typeof value === 'boolean') return String(value)
  return JSON.stringify(value, null, 2)
}
