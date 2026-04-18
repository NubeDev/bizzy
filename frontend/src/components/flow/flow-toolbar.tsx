import { useState } from 'react'
import { Link } from 'react-router-dom'
import { Save, Play, CheckCircle, LayoutDashboard, ChevronLeft, Pencil } from 'lucide-react'
import { cn } from '@/lib/utils'

interface FlowToolbarProps {
  flowName: string
  onNameChange: (name: string) => void
  onSave: () => void
  onRun: () => void
  onValidate: () => void
  onAutoLayout: () => void
  saving: boolean
  validationErrors?: string[]
  dirty: boolean
}

export function FlowToolbar({
  flowName,
  onNameChange,
  onSave,
  onRun,
  onValidate,
  onAutoLayout,
  saving,
  validationErrors,
  dirty,
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
        <ToolbarButton onClick={onRun} title="Run Flow" accent>
          <Play className="w-3.5 h-3.5" />
          <span className="text-xs">Run</span>
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
