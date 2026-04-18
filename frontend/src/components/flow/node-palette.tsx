import { useNodeTypes } from '@/hooks/use-flows'
import type { NodeTypeDef } from '@/lib/types'
import { cn } from '@/lib/utils'

const categoryLabels: Record<string, string> = {
  'flow-control': 'Flow Control',
  'tool': 'Tools',
  'integration': 'Integrations',
  'data': 'Data',
  'plugin': 'Plugins',
}

const categoryOrder = ['flow-control', 'data', 'integration', 'tool', 'plugin']

export function NodePalette() {
  const { data } = useNodeTypes()

  if (!data) return null

  const grouped = data.grouped
  const sortedCategories = categoryOrder.filter((c) => grouped[c]?.length)

  return (
    <div className="w-56 border-r border-border bg-card overflow-y-auto">
      <div className="p-3 border-b border-border">
        <h3 className="text-xs font-mono uppercase tracking-wider text-muted-foreground">
          Node Palette
        </h3>
      </div>
      <div className="p-2 space-y-3">
        {sortedCategories.map((category) => (
          <div key={category}>
            <div className="text-[10px] font-mono uppercase tracking-wider text-muted-foreground px-1 mb-1">
              {categoryLabels[category] || category}
            </div>
            <div className="space-y-0.5">
              {grouped[category].map((nodeDef) => (
                <DraggableNode key={nodeDef.type} nodeDef={nodeDef} />
              ))}
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}

function DraggableNode({ nodeDef }: { nodeDef: NodeTypeDef }) {
  const onDragStart = (e: React.DragEvent) => {
    e.dataTransfer.setData('application/flow-node-type', JSON.stringify(nodeDef))
    e.dataTransfer.effectAllowed = 'move'
  }

  const categoryColor: Record<string, string> = {
    'flow-control': 'border-l-blue-500',
    'tool': 'border-l-purple-500',
    'integration': 'border-l-emerald-500',
    'data': 'border-l-amber-500',
    'plugin': 'border-l-cyan-500',
  }

  return (
    <div
      draggable
      onDragStart={onDragStart}
      className={cn(
        'flex items-center gap-2 px-2 py-1.5 rounded text-xs cursor-grab',
        'hover:bg-accent border-l-2 border-transparent',
        'active:cursor-grabbing transition-colors',
        categoryColor[nodeDef.category] || '',
      )}
      title={nodeDef.description}
    >
      <span className="truncate">{nodeDef.label}</span>
      <span className="ml-auto text-[9px] text-muted-foreground">
        {nodeDef.source === 'builtin' ? '' : nodeDef.source}
      </span>
    </div>
  )
}
