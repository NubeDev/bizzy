import { useCallback } from 'react'
import type { Node } from '@xyflow/react'
import type { BaseNodeData } from './custom-nodes/base-node'
import { X } from 'lucide-react'

interface NodeConfigProps {
  node: Node | null
  onChange: (...args: any[]) => void
  onClose: () => void
}

export function NodeConfig({ node, onChange, onClose }: NodeConfigProps) {
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

  return (
    <div className="w-72 border-l border-border bg-card overflow-y-auto">
      <div className="flex items-center justify-between p-3 border-b border-border">
        <div>
          <h3 className="text-xs font-mono uppercase tracking-wider text-muted-foreground">
            Configure
          </h3>
          <p className="text-sm font-medium mt-0.5">{data.label || nodeType}</p>
        </div>
        <button onClick={onClose} className="p-1 hover:bg-accent rounded">
          <X className="w-3.5 h-3.5" />
        </button>
      </div>

      <div className="p-3 space-y-3">
        {/* Label */}
        <Field label="Label">
          <input
            type="text"
            value={data.label || ''}
            onChange={(e) => onChange(node.id, { ...node.data, label: e.target.value })}
            className="w-full px-2 py-1 text-xs bg-background border border-border rounded"
            placeholder="Node label"
          />
        </Field>

        {/* Trigger config */}
        {nodeType === 'trigger' && (
          <>
            <Field label="Trigger Type">
              <select
                value={(data.config?.type as string) || 'manual'}
                onChange={(e) => handleDataChange('type', e.target.value)}
                className="w-full px-2 py-1 text-xs bg-background border border-border rounded"
              >
                <option value="manual">Manual (API only)</option>
                <option value="cron">Cron Schedule</option>
                <option value="interval">Interval</option>
              </select>
            </Field>
            {((data.config?.type as string) === 'cron' || (data.config?.type as string) === 'interval') && (
              <Field label={(data.config?.type as string) === 'cron' ? 'Cron Expression' : 'Interval'}>
                <input
                  type="text"
                  value={(data.config?.schedule as string) || ''}
                  onChange={(e) => handleDataChange('schedule', e.target.value)}
                  className="w-full px-2 py-1 text-xs bg-background border border-border rounded font-mono"
                  placeholder={(data.config?.type as string) === 'cron' ? '*/5 * * * *' : '10s, 5m, 1h'}
                />
              </Field>
            )}
          </>
        )}

        {/* Type-specific config fields */}
        {(nodeType === 'condition' || nodeType === 'switch' || nodeType === 'transform') && (
          <Field label="Expression">
            <textarea
              value={(data.config?.expression as string) || ''}
              onChange={(e) => handleDataChange('expression', e.target.value)}
              className="w-full px-2 py-1 text-xs bg-background border border-border rounded font-mono resize-y min-h-[60px]"
              placeholder={nodeType === 'condition' ? 'input > 100' : 'input'}
            />
          </Field>
        )}

        {nodeType === 'delay' && (
          <Field label="Duration">
            <input
              type="text"
              value={(data.config?.duration as string) || '1s'}
              onChange={(e) => handleDataChange('duration', e.target.value)}
              className="w-full px-2 py-1 text-xs bg-background border border-border rounded"
              placeholder="1s, 5m, 1h"
            />
          </Field>
        )}

        {nodeType === 'value' && (
          <Field label="Value (JSON)">
            <textarea
              value={(data.config?.value as string) != null ? JSON.stringify(data.config?.value, null, 2) : ''}
              onChange={(e) => {
                try {
                  handleDataChange('value', JSON.parse(e.target.value))
                } catch {
                  handleDataChange('value', e.target.value)
                }
              }}
              className="w-full px-2 py-1 text-xs bg-background border border-border rounded font-mono resize-y min-h-[60px]"
              placeholder={'{\n  "hello": "world"\n}'}
            />
          </Field>
        )}

        {nodeType === 'template' && (
          <Field label="Template">
            <textarea
              value={(data.config?.template as string) || ''}
              onChange={(e) => handleDataChange('template', e.target.value)}
              className="w-full px-2 py-1 text-xs bg-background border border-border rounded font-mono resize-y min-h-[60px]"
              placeholder={'Hello {{.input}}!'}
            />
          </Field>
        )}

        {nodeType === 'http-request' && (
          <>
            <Field label="URL">
              <input
                type="text"
                value={(data.config?.url as string) || ''}
                onChange={(e) => handleDataChange('url', e.target.value)}
                className="w-full px-2 py-1 text-xs bg-background border border-border rounded font-mono"
                placeholder="https://api.example.com/data"
              />
            </Field>
            <Field label="Method">
              <select
                value={(data.config?.method as string) || 'GET'}
                onChange={(e) => handleDataChange('method', e.target.value)}
                className="w-full px-2 py-1 text-xs bg-background border border-border rounded"
              >
                <option value="GET">GET</option>
                <option value="POST">POST</option>
                <option value="PUT">PUT</option>
                <option value="DELETE">DELETE</option>
              </select>
            </Field>
          </>
        )}

        {nodeType === 'set-variable' && (
          <Field label="Variable Name">
            <input
              type="text"
              value={(data.config?.variable as string) || ''}
              onChange={(e) => handleDataChange('variable', e.target.value)}
              className="w-full px-2 py-1 text-xs bg-background border border-border rounded"
              placeholder="myVar"
            />
          </Field>
        )}

        {nodeType === 'approval' && (
          <Field label="Approval Message">
            <textarea
              value={(data.config?.message as string) || ''}
              onChange={(e) => handleDataChange('message', e.target.value)}
              className="w-full px-2 py-1 text-xs bg-background border border-border rounded resize-y min-h-[40px]"
              placeholder="Please approve to continue..."
            />
          </Field>
        )}

        {(nodeType === 'ai-prompt' || nodeType === 'ai-runner') && (
          <>
            <Field label="Provider">
              <input
                type="text"
                value={(data.config?.provider as string) || ''}
                onChange={(e) => handleDataChange('provider', e.target.value)}
                className="w-full px-2 py-1 text-xs bg-background border border-border rounded"
                placeholder="claude, opencode, ollama"
              />
            </Field>
            <Field label="Model">
              <input
                type="text"
                value={(data.config?.model as string) || ''}
                onChange={(e) => handleDataChange('model', e.target.value)}
                className="w-full px-2 py-1 text-xs bg-background border border-border rounded"
                placeholder="Optional model override"
              />
            </Field>
          </>
        )}

        {nodeType === 'ai-runner' && (
          <>
            <Field label="Work Directory">
              <input
                type="text"
                value={(data.config?.work_dir as string) || ''}
                onChange={(e) => handleDataChange('work_dir', e.target.value)}
                className="w-full px-2 py-1 text-xs bg-background border border-border rounded font-mono"
                placeholder="/home/user/code/project"
              />
            </Field>
            <Field label="Thinking Budget">
              <select
                value={(data.config?.thinking_budget as string) || 'medium'}
                onChange={(e) => handleDataChange('thinking_budget', e.target.value)}
                className="w-full px-2 py-1 text-xs bg-background border border-border rounded"
              >
                <option value="low">Low</option>
                <option value="medium">Medium</option>
                <option value="high">High</option>
              </select>
            </Field>
          </>
        )}

        {/* Error handling */}
        <Field label="On Error">
          <select
            value={(data.config?.on_error as string) || 'stop'}
            onChange={(e) => handleDataChange('on_error', e.target.value)}
            className="w-full px-2 py-1 text-xs bg-background border border-border rounded"
          >
            <option value="stop">Stop flow</option>
            <option value="skip">Skip node</option>
            <option value="retry">Retry</option>
            <option value="fallback">Fallback to error port</option>
          </select>
        </Field>

        {(data.config?.on_error === 'retry') && (
          <Field label="Max Retries">
            <input
              type="number"
              value={(data.config?.max_retries as number) || 3}
              onChange={(e) => handleDataChange('max_retries', parseInt(e.target.value))}
              className="w-full px-2 py-1 text-xs bg-background border border-border rounded"
              min={1}
              max={10}
            />
          </Field>
        )}

        {/* Node info */}
        <div className="pt-2 border-t border-border text-[10px] text-muted-foreground space-y-0.5">
          <div>ID: <span className="font-mono">{node.id}</span></div>
          <div>Type: <span className="font-mono">{nodeType}</span></div>
          {data.description && <div className="mt-1">{data.description}</div>}
        </div>
      </div>
    </div>
  )
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <label className="text-[10px] font-mono uppercase tracking-wider text-muted-foreground mb-0.5 block">
        {label}
      </label>
      {children}
    </div>
  )
}
