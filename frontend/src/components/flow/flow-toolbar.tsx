import { Save, Play, CheckCircle, Undo2, Redo2, LayoutDashboard } from 'lucide-react'
import { cn } from '@/lib/utils'

interface FlowToolbarProps {
  flowName: string
  onSave: () => void
  onRun: () => void
  onValidate: () => void
  onAutoLayout: () => void
  onUndo: () => void
  onRedo: () => void
  canUndo: boolean
  canRedo: boolean
  saving: boolean
  validationErrors?: string[]
  dirty: boolean
}

export function FlowToolbar({
  flowName,
  onSave,
  onRun,
  onValidate,
  onAutoLayout,
  onUndo,
  onRedo,
  canUndo,
  canRedo,
  saving,
  validationErrors,
  dirty,
}: FlowToolbarProps) {
  return (
    <div className="flex items-center gap-1 px-3 py-1.5 border-b border-border bg-card">
      {/* Flow name */}
      <div className="flex items-center gap-2 mr-4">
        <span className="text-sm font-medium">{flowName || 'Untitled Flow'}</span>
        {dirty && <span className="w-1.5 h-1.5 rounded-full bg-amber-500" title="Unsaved changes" />}
      </div>

      <div className="flex items-center gap-0.5">
        <ToolbarButton onClick={onUndo} disabled={!canUndo} title="Undo">
          <Undo2 className="w-3.5 h-3.5" />
        </ToolbarButton>
        <ToolbarButton onClick={onRedo} disabled={!canRedo} title="Redo">
          <Redo2 className="w-3.5 h-3.5" />
        </ToolbarButton>
      </div>

      <div className="w-px h-4 bg-border mx-1" />

      <div className="flex items-center gap-0.5">
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
