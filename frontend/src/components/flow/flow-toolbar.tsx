import { useState } from 'react'
import { Link } from 'react-router-dom'
import { Save, CheckCircle, LayoutDashboard, ChevronLeft, Pencil, Radio } from 'lucide-react'
import { cn } from '@/lib/utils'

export type PollInterval = 1000 | 3000 | 5000 | 10000 | 30000 | false

interface FlowToolbarProps {
  flowName: string
  onNameChange: (name: string) => void
  onSave: () => void
  onValidate: () => void
  onAutoLayout: () => void
  saving: boolean
  validationErrors?: string[]
  dirty: boolean
  pollInterval: PollInterval
  onPollIntervalChange: (interval: PollInterval) => void
  totalRuns?: number
  latestRunStatus?: string
}

const POLL_OPTIONS: { label: string; value: PollInterval }[] = [
  { label: '1s', value: 1000 },
  { label: '3s', value: 3000 },
  { label: '5s', value: 5000 },
  { label: '10s', value: 10000 },
  { label: '30s', value: 30000 },
  { label: 'Off', value: false },
]

export function FlowToolbar({
  flowName,
  onNameChange,
  onSave,
  onValidate,
  onAutoLayout,
  saving,
  validationErrors,
  dirty,
  pollInterval,
  onPollIntervalChange,
  totalRuns,
  latestRunStatus,
}: FlowToolbarProps) {
  const [editing, setEditing] = useState(false)
  const [editValue, setEditValue] = useState(flowName)

  const commitName = () => {
    const trimmed = editValue.trim()
    if (trimmed && trimmed !== flowName) {
      onNameChange(trimmed)
    }
    setEditing(false)
  }

  return (
    <div className="flex items-center gap-2 px-3 py-1.5 border-b border-border bg-background">
      {/* Back to flows list */}
      <Link
        to="/flows"
        className="flex items-center gap-0.5 text-muted-foreground hover:text-foreground transition-colors mr-1"
        title="Back to flows"
      >
        <ChevronLeft className="w-4 h-4" />
      </Link>

      {/* Editable flow name */}
      <div className="flex items-center gap-1.5 mr-3">
        {editing ? (
          <input
            autoFocus
            value={editValue}
            onChange={(e) => setEditValue(e.target.value)}
            onBlur={commitName}
            onKeyDown={(e) => { if (e.key === 'Enter') commitName(); if (e.key === 'Escape') setEditing(false) }}
            className="text-sm font-medium bg-transparent border-b border-primary outline-none px-0 py-0 w-48"
          />
        ) : (
          <button
            onClick={() => { setEditValue(flowName); setEditing(true) }}
            className="flex items-center gap-1.5 text-sm font-medium hover:text-primary transition-colors group"
          >
            {flowName || 'Untitled Flow'}
            <Pencil className="w-3 h-3 opacity-0 group-hover:opacity-50 transition-opacity" />
          </button>
        )}
        {dirty && <span className="w-1.5 h-1.5 rounded-full bg-amber-500" title="Unsaved changes" />}
      </div>

      <div className="w-px h-4 bg-border" />

      <div className="flex items-center gap-0.5 ml-1">
        <ToolbarButton onClick={onAutoLayout} title="Auto Layout">
          <LayoutDashboard className="w-3.5 h-3.5" />
        </ToolbarButton>
        <ToolbarButton onClick={onValidate} title="Validate">
          <CheckCircle className="w-3.5 h-3.5" />
        </ToolbarButton>
      </div>

      <div className="flex-1" />

      {/* Live polling status */}
      <div className="flex items-center gap-1.5 mr-2">
        {pollInterval !== false && (
          <div className="flex items-center gap-1 text-[10px] text-muted-foreground">
            <Radio className={cn('w-3 h-3', latestRunStatus === 'completed' ? 'text-green-400' : latestRunStatus === 'running' ? 'text-blue-400 animate-pulse' : 'text-muted-foreground')} />
            {totalRuns !== undefined && <span>{totalRuns} runs</span>}
          </div>
        )}
        <select
          value={pollInterval === false ? 'off' : String(pollInterval)}
          onChange={(e) => {
            const v = e.target.value
            onPollIntervalChange(v === 'off' ? false : (Number(v) as PollInterval))
          }}
          className="px-1.5 py-0.5 text-[10px] bg-background border border-border rounded text-muted-foreground"
          title="Poll interval for live node values"
        >
          {POLL_OPTIONS.map((opt) => (
            <option key={String(opt.value)} value={opt.value === false ? 'off' : String(opt.value)}>
              {opt.label}
            </option>
          ))}
        </select>
      </div>

      {/* Validation errors */}
      {validationErrors && validationErrors.length > 0 && (
        <div className="text-[10px] text-red-400 mr-2 max-w-[300px] truncate" title={validationErrors.join('\n')}>
          {validationErrors.length} error{validationErrors.length > 1 ? 's' : ''}
        </div>
      )}

      <div className="flex items-center gap-1">
        <ToolbarButton onClick={onSave} disabled={saving || !dirty} title="Save" accent>
          <Save className="w-3.5 h-3.5" />
          <span className="text-xs">{saving ? 'Saving...' : 'Save'}</span>
        </ToolbarButton>
      </div>
    </div>
  )
}

function ToolbarButton({
  onClick,
  disabled,
  title,
  accent,
  children,
}: {
  onClick: () => void
  disabled?: boolean
  title: string
  accent?: boolean
  children: React.ReactNode
}) {
  return (
    <button
      onClick={onClick}
      disabled={disabled}
      title={title}
      className={cn(
        'flex items-center gap-1 px-2 py-1 rounded text-xs transition-colors',
        'disabled:opacity-40 disabled:cursor-not-allowed',
        accent
          ? 'bg-primary/10 text-primary hover:bg-primary/20'
          : 'hover:bg-accent text-muted-foreground hover:text-foreground',
      )}
    >
      {children}
    </button>
  )
}
