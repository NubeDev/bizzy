import { useCallback } from 'react'
import type { Node } from '@xyflow/react'
import type { BaseNodeData } from './custom-nodes/base-node'
import type { NodeTypeDef } from '@/lib/types'
import { SchemaForm, type JSONSchema } from './schema-form'
import { Settings2 } from 'lucide-react'

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

  const data = node ? (node.data as BaseNodeData) : null
  const nodeType = data ? (data.nodeType || node!.type || '') : ''
  const typeDef = nodeType ? nodeTypeDefs[nodeType] : undefined
  const schema = typeDef?.settings as JSONSchema | undefined
  const hasCodeEditor = nodeType === 'function' || nodeType === 'transform'

  return (
    <div className={`${hasCodeEditor ? 'w-[480px]' : 'w-72'} border-l border-border bg-card overflow-y-auto h-full flex flex-col`}>
      {/* Header */}
      <div className="flex items-center gap-2 p-3 border-b border-border shrink-0">
        <Settings2 className="w-3.5 h-3.5 text-muted-foreground" />
        <div className="flex-1 min-w-0">
          <h3 className="text-xs font-mono uppercase tracking-wider text-muted-foreground">
            {node ? 'Configure' : 'Node Config'}
          </h3>
          {data && (
            <p className="text-sm font-medium mt-0.5 truncate">{data.label || typeDef?.label || nodeType}</p>
          )}
        </div>
      </div>

      {!node ? (
        <div className="flex items-center justify-center flex-1 text-xs text-muted-foreground p-4">
          Select a node on the canvas to configure it
        </div>
      ) : (
        <div className="p-3 space-y-3 flex-1 overflow-y-auto">
          {/* Label (always shown) */}
          <div>
            <label className="text-[10px] font-mono uppercase tracking-wider text-muted-foreground mb-0.5 block">
              Label
            </label>
            <input
              type="text"
              value={data!.label || ''}
              onChange={(e) => onChange(node.id, { ...node.data, label: e.target.value })}
              className="w-full px-2 py-1 text-xs bg-background border border-border rounded"
              placeholder="Node label"
            />
          </div>

          {/* Schema-driven settings */}
          {schema?.properties ? (
            <SchemaForm
              schema={schema}
              values={(data!.config as Record<string, unknown>) || {}}
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
            {data!.description && <div className="mt-1">{data!.description}</div>}
          </div>
        </div>
      )}
    </div>
  )
}
