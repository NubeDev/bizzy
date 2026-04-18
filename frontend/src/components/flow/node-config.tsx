import { useCallback } from 'react'
import type { Node } from '@xyflow/react'
import type { BaseNodeData } from './custom-nodes/base-node'
import type { NodeTypeDef } from '@/lib/types'
import { SchemaForm, type JSONSchema } from './schema-form'
import { X } from 'lucide-react'

interface NodeConfigProps {
  node: Node | null
  nodeTypeDefs: Record<string, NodeTypeDef>
  onChange: (...args: any[]) => void
  onClose: () => void
}

export function NodeConfig({ node, nodeTypeDefs, onChange, onClose }: NodeConfigProps) {
  const handleDataChange = useCallback(
    (key: string, value: unknown) => {
      if (!node) return
      const config = { ...((node.data as BaseNodeData).config || {}), [key]: value }
      onChange(node.id, { ...node.data, config })
    },
    [node, onChange],
  )

  if (!node) return null

  const data = node.data as BaseNodeData
  const nodeType = data.nodeType || node.type || ''
  const typeDef = nodeTypeDefs[nodeType]
  const schema = typeDef?.settings as JSONSchema | undefined

  return (
    <div className="w-72 border-l border-border bg-card overflow-y-auto">
      <div className="flex items-center justify-between p-3 border-b border-border">
        <div>
          <h3 className="text-xs font-mono uppercase tracking-wider text-muted-foreground">
            Configure
          </h3>
          <p className="text-sm font-medium mt-0.5">{data.label || typeDef?.label || nodeType}</p>
        </div>
        <button onClick={onClose} className="p-1 hover:bg-accent rounded">
          <X className="w-3.5 h-3.5" />
        </button>
      </div>

      <div className="p-3 space-y-3">
        {/* Label (always shown) */}
        <div>
          <label className="text-[10px] font-mono uppercase tracking-wider text-muted-foreground mb-0.5 block">
            Label
          </label>
          <input
            type="text"
            value={data.label || ''}
            onChange={(e) => onChange(node.id, { ...node.data, label: e.target.value })}
            className="w-full px-2 py-1 text-xs bg-background border border-border rounded"
            placeholder="Node label"
          />
        </div>

        {/* Schema-driven settings */}
        {schema?.properties ? (
          <SchemaForm
            schema={schema}
            values={(data.config as Record<string, unknown>) || {}}
            onChange={handleDataChange}
          />
        ) : (
          <p className="text-[10px] text-muted-foreground italic">
            No configurable settings for this node type.
          </p>
        )}

        {/* Node info */}
        <div className="pt-2 border-t border-border text-[10px] text-muted-foreground space-y-0.5">
          <div>ID: <span className="font-mono">{node.id}</span></div>
          <div>Type: <span className="font-mono">{nodeType}</span></div>
          {typeDef?.source && typeDef.source !== 'builtin' && (
            <div>Source: <span className="font-mono">{typeDef.source}</span></div>
          )}
          {data.description && <div className="mt-1">{data.description}</div>}
        </div>
      </div>
    </div>
  )
}
