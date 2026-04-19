import { useState, useEffect, useRef } from 'react'
import type { DebugEntry } from '@/lib/types'
import { cn } from '@/lib/utils'
import { Bug, Trash2, ChevronRight, Copy, Check } from 'lucide-react'

interface DebugPanelProps {
  entries: DebugEntry[]
  onClear?: () => void
}

// Stable color palette — each node_id gets a consistent color.
const NODE_COLORS = [
  { bg: 'bg-blue-500/10', border: 'border-l-blue-500', text: 'text-blue-400', chip: 'bg-blue-500/15 text-blue-400' },
  { bg: 'bg-emerald-500/10', border: 'border-l-emerald-500', text: 'text-emerald-400', chip: 'bg-emerald-500/15 text-emerald-400' },
  { bg: 'bg-purple-500/10', border: 'border-l-purple-500', text: 'text-purple-400', chip: 'bg-purple-500/15 text-purple-400' },
  { bg: 'bg-amber-500/10', border: 'border-l-amber-500', text: 'text-amber-400', chip: 'bg-amber-500/15 text-amber-400' },
  { bg: 'bg-rose-500/10', border: 'border-l-rose-500', text: 'text-rose-400', chip: 'bg-rose-500/15 text-rose-400' },
  { bg: 'bg-cyan-500/10', border: 'border-l-cyan-500', text: 'text-cyan-400', chip: 'bg-cyan-500/15 text-cyan-400' },
  { bg: 'bg-pink-500/10', border: 'border-l-pink-500', text: 'text-pink-400', chip: 'bg-pink-500/15 text-pink-400' },
  { bg: 'bg-indigo-500/10', border: 'border-l-indigo-500', text: 'text-indigo-400', chip: 'bg-indigo-500/15 text-indigo-400' },
]

function getNodeColor(nodeId: string, allNodeIds: string[]) {
  const idx = allNodeIds.indexOf(nodeId)
  return NODE_COLORS[(idx >= 0 ? idx : 0) % NODE_COLORS.length]
}

export function DebugPanel({ entries, onClear }: DebugPanelProps) {
  const [filter, setFilter] = useState<string | null>(null)
  const scrollRef = useRef<HTMLDivElement>(null)

  // Auto-scroll to latest entry.
  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight
    }
  }, [entries.length])

  const filtered = filter
    ? entries.filter((e) => e.node_id === filter)
    : entries

  // Unique debug node IDs for filter chips and color assignment.
  const sources = Array.from(new Set(entries.map((e) => e.node_id)))

  return (
    <div className="w-80 border-l border-border bg-card flex flex-col h-full">
      {/* Header */}
      <div className="flex items-center gap-2 p-3 border-b border-border shrink-0">
        <Bug className="w-3.5 h-3.5 text-amber-400" />
        <h3 className="text-xs font-mono uppercase tracking-wider text-muted-foreground flex-1">
          Debug
        </h3>
        {entries.length > 0 && (
          <>
            <span className="text-[10px] text-muted-foreground bg-muted px-1.5 py-0.5 rounded-full font-mono">
              {filtered.length}{filter ? ` / ${entries.length}` : ''}
            </span>
            {onClear && (
              <button
                onClick={onClear}
                className="p-1 hover:bg-accent rounded text-muted-foreground hover:text-foreground"
                title="Clear debug log"
              >
                <Trash2 className="w-3 h-3" />
              </button>
            )}
          </>
        )}
      </div>

      {/* Filter chips */}
      {sources.length > 1 && (
        <div className="flex items-center gap-1 px-3 py-1.5 border-b border-border overflow-x-auto shrink-0">
          <FilterChip active={filter === null} onClick={() => setFilter(null)} className="text-muted-foreground">
            All
          </FilterChip>
          {sources.map((nodeId) => {
            const label = entries.find((e) => e.node_id === nodeId)?.label || nodeId
            const color = getNodeColor(nodeId, sources)
            return (
              <FilterChip
                key={nodeId}
                active={filter === nodeId}
                onClick={() => setFilter(filter === nodeId ? null : nodeId)}
                className={filter === nodeId ? color.chip : 'text-muted-foreground'}
              >
                <span className={cn('inline-block w-1.5 h-1.5 rounded-full mr-1', color.text, 'bg-current')} />
                {label}
              </FilterChip>
            )
          })}
        </div>
      )}

      {/* Entries */}
      <div ref={scrollRef} className="flex-1 overflow-y-auto">
        {filtered.length === 0 ? (
          <div className="flex items-center justify-center h-32 text-xs text-muted-foreground">
            {entries.length === 0
              ? 'Add a debug node to see messages here'
              : 'No entries match filter'}
          </div>
        ) : (
          filtered.map((entry, i) => (
            <DebugRow
              key={`${entry.msg_id}-${i}`}
              entry={entry}
              color={getNodeColor(entry.node_id, sources)}
            />
          ))
        )}
      </div>
    </div>
  )
}

function FilterChip({ active, onClick, className, children }: {
  active: boolean; onClick: () => void; className?: string; children: React.ReactNode
}) {
  return (
    <button
      onClick={onClick}
      className={cn(
        'flex items-center px-1.5 py-0.5 text-[10px] rounded transition-colors whitespace-nowrap',
        active ? className : 'text-muted-foreground hover:text-foreground',
      )}
    >
      {children}
    </button>
  )
}

function DebugRow({ entry, color }: { entry: DebugEntry; color: typeof NODE_COLORS[0] }) {
  const [expanded, setExpanded] = useState(false)
  const [copied, setCopied] = useState(false)
  const time = formatTime(entry.ts)

  const handleCopy = (e: React.MouseEvent) => {
    e.stopPropagation()
    const text = typeof entry.value === 'string' ? entry.value : JSON.stringify(entry.value, null, 2)
    navigator.clipboard.writeText(text).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 1500)
    })
  }

  return (
    <div className={cn('border-b border-border/40 border-l-2', color.border)}>
      {/* Collapsed row */}
      <div
        className={cn('flex items-start gap-2 px-3 py-2 cursor-pointer group', color.bg, 'hover:brightness-110')}
        onClick={() => setExpanded(!expanded)}
      >
        <ChevronRight className={cn('w-3 h-3 mt-0.5 text-muted-foreground shrink-0 transition-transform', expanded && 'rotate-90')} />
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2 mb-0.5">
            <span className="text-[10px] text-muted-foreground font-mono tabular-nums">{time}</span>
            <span className={cn('text-[10px] font-medium', color.text)}>{entry.label}</span>
            <span className="flex-1" />
            {/* Copy button */}
            <button
              onClick={handleCopy}
              className="opacity-0 group-hover:opacity-100 p-0.5 hover:bg-accent rounded transition-opacity"
              title="Copy value"
            >
              {copied ? (
                <Check className="w-3 h-3 text-green-400" />
              ) : (
                <Copy className="w-3 h-3 text-muted-foreground" />
              )}
            </button>
          </div>
          {!expanded && (
            <div className="text-[11px] font-mono text-foreground/80 truncate">
              {formatPreview(entry.value)}
            </div>
          )}
        </div>
      </div>

      {/* Expanded: interactive JSON tree */}
      {expanded && (
        <div className="px-3 pb-3 pl-8">
          <div className="bg-background rounded border border-border p-2 overflow-x-auto max-h-[300px] overflow-y-auto">
            <JsonTree value={entry.value} />
          </div>
          <div className="flex items-center gap-3 mt-1.5 text-[9px] text-muted-foreground font-mono">
            <span>msgid: {entry.msg_id || '---'}</span>
            <span>node: {entry.node_id}</span>
          </div>
        </div>
      )}
    </div>
  )
}

// --- Interactive JSON Tree ---

function JsonTree({ value, depth = 0 }: { value: unknown; depth?: number }) {
  if (value === null) return <JsonPrimitive value="null" color="text-orange-400" />
  if (value === undefined) return <JsonPrimitive value="undefined" color="text-muted-foreground" />
  if (typeof value === 'boolean') return <JsonPrimitive value={String(value)} color="text-orange-400" />
  if (typeof value === 'number') return <JsonPrimitive value={String(value)} color="text-blue-400" />
  if (typeof value === 'string') return <JsonString value={value} />

  if (Array.isArray(value)) {
    if (value.length === 0) return <span className="text-[11px] font-mono text-muted-foreground">[]</span>
    return <JsonArray items={value} depth={depth} />
  }

  if (typeof value === 'object') {
    const entries = Object.entries(value as Record<string, unknown>)
    if (entries.length === 0) return <span className="text-[11px] font-mono text-muted-foreground">{'{}'}</span>
    return <JsonObject entries={entries} depth={depth} />
  }

  return <span className="text-[11px] font-mono">{String(value)}</span>
}

function JsonPrimitive({ value, color }: { value: string; color: string }) {
  return <span className={cn('text-[11px] font-mono', color)}>{value}</span>
}

function JsonString({ value }: { value: string }) {
  if (value.length > 80) {
    return <span className="text-[11px] font-mono text-green-400 break-all">&quot;{value}&quot;</span>
  }
  return <span className="text-[11px] font-mono text-green-400">&quot;{value}&quot;</span>
}

function JsonObject({ entries, depth }: { entries: [string, unknown][]; depth: number }) {
  const [open, setOpen] = useState(depth < 2)
  const count = entries.length

  if (!open) {
    return (
      <span
        className="text-[11px] font-mono text-muted-foreground cursor-pointer hover:text-foreground"
        onClick={() => setOpen(true)}
      >
        {'{'} <span className="text-[10px]">{count} {count === 1 ? 'key' : 'keys'}</span> {'}'}
      </span>
    )
  }

  return (
    <div>
      <span className="text-[11px] font-mono text-muted-foreground cursor-pointer hover:text-foreground" onClick={() => setOpen(false)}>
        {'{'}
      </span>
      <div className="ml-3 border-l border-border/30 pl-2">
        {entries.map(([key, val]) => (
          <div key={key} className="flex items-start gap-1">
            <span className="text-[11px] font-mono text-purple-400 shrink-0">{key}</span>
            <span className="text-[11px] font-mono text-muted-foreground shrink-0">:</span>
            <JsonTree value={val} depth={depth + 1} />
          </div>
        ))}
      </div>
      <span className="text-[11px] font-mono text-muted-foreground">{'}'}</span>
    </div>
  )
}

function JsonArray({ items, depth }: { items: unknown[]; depth: number }) {
  const [open, setOpen] = useState(depth < 2)

  if (!open) {
    return (
      <span
        className="text-[11px] font-mono text-muted-foreground cursor-pointer hover:text-foreground"
        onClick={() => setOpen(true)}
      >
        {'['} <span className="text-[10px]">{items.length} items</span> {']'}
      </span>
    )
  }

  return (
    <div>
      <span className="text-[11px] font-mono text-muted-foreground cursor-pointer hover:text-foreground" onClick={() => setOpen(false)}>
        {'['}
      </span>
      <div className="ml-3 border-l border-border/30 pl-2">
        {items.map((item, i) => (
          <div key={i} className="flex items-start gap-1">
            <span className="text-[10px] font-mono text-muted-foreground/50 shrink-0 w-4 text-right">{i}</span>
            <JsonTree value={item} depth={depth + 1} />
          </div>
        ))}
      </div>
      <span className="text-[11px] font-mono text-muted-foreground">{']'}</span>
    </div>
  )
}

// --- Helpers ---

function formatTime(ts: string): string {
  try {
    const d = new Date(ts)
    return d.toLocaleTimeString('en-US', { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' })
  } catch {
    return ts.slice(11, 19)
  }
}

function formatPreview(value: unknown): string {
  if (value === null || value === undefined) return 'null'
  if (typeof value === 'string') return value.length > 60 ? value.slice(0, 60) + '...' : value
  if (typeof value === 'number' || typeof value === 'boolean') return String(value)
  if (Array.isArray(value)) return `[${value.length} items]`
  if (typeof value === 'object') {
    const keys = Object.keys(value)
    if (keys.length <= 3) return `{ ${keys.join(', ')} }`
    return `{ ${keys.slice(0, 3).join(', ')}, ... (${keys.length} keys) }`
  }
  return String(value)
}
