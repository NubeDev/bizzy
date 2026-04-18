import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'
import { ChevronDown } from 'lucide-react'
import { useState, useRef, useEffect } from 'react'

interface ProviderOption {
  provider: string
  available: boolean
  type: string
  models?: string[]
}

interface Props {
  provider: string
  model: string
  onProviderChange: (provider: string) => void
  onModelChange: (model: string) => void
}

export function ProviderSelector({ provider, model, onProviderChange, onModelChange }: Props) {
  const { data: providers } = useQuery({
    queryKey: ['providers'],
    queryFn: () => api.providers(),
    staleTime: 30_000,
  })

  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [])

  const available = (providers || []).filter(p => p.available)
  const current = available.find(p => p.provider === provider)
  const currentModels = current?.models || []

  return (
    <div ref={ref} className="flex items-center gap-1.5 text-xs">
      {/* Provider dropdown */}
      <div className="relative">
        <button
          onClick={() => setOpen(!open)}
          className="flex items-center gap-1 px-2 py-1 border border-border bg-card hover:bg-accent text-muted-foreground hover:text-foreground transition-colors font-mono text-[11px]"
        >
          <span className={`w-1.5 h-1.5 rounded-full ${current?.available ? 'bg-emerald-500' : 'bg-red-500'}`} />
          {provider || 'claude'}
          <ChevronDown size={10} />
        </button>

        {open && (
          <div className="absolute bottom-full left-0 mb-1 bg-card border border-border shadow-lg z-50 min-w-[140px]">
            {available.map(p => (
              <button
                key={p.provider}
                onClick={() => {
                  onProviderChange(p.provider)
                  onModelChange('')
                  setOpen(false)
                }}
                className={`w-full text-left px-3 py-1.5 text-[11px] font-mono flex items-center gap-2 hover:bg-accent transition-colors ${
                  p.provider === provider ? 'text-foreground bg-accent/50' : 'text-muted-foreground'
                }`}
              >
                <span className="w-1.5 h-1.5 rounded-full bg-emerald-500" />
                {p.provider}
                <span className="ml-auto text-[9px] text-muted-foreground/60">{p.type}</span>
              </button>
            ))}
          </div>
        )}
      </div>

      {/* Model dropdown — only show if provider has models */}
      {currentModels.length > 0 && (
        <ModelSelector models={currentModels} model={model} onChange={onModelChange} />
      )}
    </div>
  )
}

function ModelSelector({ models, model, onChange }: { models: string[]; model: string; onChange: (m: string) => void }) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [])

  return (
    <div ref={ref} className="relative">
      <button
        onClick={() => setOpen(!open)}
        className="flex items-center gap-1 px-2 py-1 border border-border bg-card hover:bg-accent text-muted-foreground hover:text-foreground transition-colors font-mono text-[11px]"
      >
        {model || 'default'}
        <ChevronDown size={10} />
      </button>

      {open && (
        <div className="absolute bottom-full left-0 mb-1 bg-card border border-border shadow-lg z-50 min-w-[160px] max-h-[200px] overflow-y-auto">
          <button
            onClick={() => { onChange(''); setOpen(false) }}
            className={`w-full text-left px-3 py-1.5 text-[11px] font-mono hover:bg-accent transition-colors ${
              !model ? 'text-foreground bg-accent/50' : 'text-muted-foreground'
            }`}
          >
            default
          </button>
          {models.map(m => (
            <button
              key={m}
              onClick={() => { onChange(m); setOpen(false) }}
              className={`w-full text-left px-3 py-1.5 text-[11px] font-mono hover:bg-accent transition-colors ${
                m === model ? 'text-foreground bg-accent/50' : 'text-muted-foreground'
              }`}
            >
              {m}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
