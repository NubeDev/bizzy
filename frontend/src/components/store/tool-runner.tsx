import { useState } from "react"
import { Play, Loader2, ChevronDown, ChevronRight, Copy, Check } from "lucide-react"
import { api } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import type { StoreTool } from "@/lib/types"

interface ToolRunnerProps {
  appName: string
  tool: StoreTool
  installed: boolean
}

export function ToolRunner({ appName, tool, installed }: ToolRunnerProps) {
  const [expanded, setExpanded] = useState(false)
  const [params, setParams] = useState<Record<string, string>>({})
  const [running, setRunning] = useState(false)
  const [result, setResult] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)

  const paramEntries = Object.entries(tool.params || {})

  const handleRun = async () => {
    setRunning(true)
    setResult(null)
    setError(null)
    try {
      const toolName = `${appName}.${tool.name}`
      const castParams: Record<string, unknown> = {}
      for (const [k, v] of Object.entries(params)) {
        const def = tool.params[k]
        if (def?.type === "number") {
          castParams[k] = Number(v)
        } else if (def?.type === "boolean") {
          castParams[k] = v === "true"
        } else {
          castParams[k] = v
        }
      }
      const res = await api.callTool(toolName, castParams)
      setResult(JSON.stringify(res, null, 2))
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unknown error")
    } finally {
      setRunning(false)
    }
  }

  const handleCopy = () => {
    if (result) {
      navigator.clipboard.writeText(result)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    }
  }

  const canRun = installed && paramEntries.every(([k, def]) =>
    !def.required || (params[k] && params[k].trim() !== "")
  )

  return (
    <div className="rounded-none border border-border bg-card">
      <button
        className="w-full flex items-center gap-2 p-4 text-left hover:bg-accent/50 transition-colors"
        onClick={() => setExpanded(!expanded)}
      >
        {expanded ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
        <code className="text-sm font-medium font-mono flex-1">{tool.name}</code>
        <span className="font-mono text-[10px] border border-border px-1.5 py-0.5 text-muted-foreground">{tool.toolClass}</span>
      </button>

      {expanded && (
        <div className="border-t border-border p-4 space-y-3">
          <p className="text-xs text-muted-foreground">{tool.description}</p>

          {paramEntries.length > 0 && (
            <div className="space-y-2">
              {paramEntries.map(([name, def]) => (
                <div key={name} className="space-y-1">
                  <Label className="text-xs font-mono">
                    {name}
                    {def.required && <span className="text-destructive ml-0.5">*</span>}
                    <span className="text-muted-foreground font-normal ml-2">{def.type}</span>
                  </Label>
                  <Input
                    className="rounded-none bg-transparent border-border font-mono text-sm h-8"
                    placeholder={def.description}
                    value={params[name] || ""}
                    onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
                      setParams({ ...params, [name]: e.target.value })
                    }
                  />
                </div>
              ))}
            </div>
          )}

          <div className="flex items-center gap-2">
            <Button
              size="sm"
              className="rounded-none font-mono text-xs uppercase tracking-wider"
              onClick={handleRun}
              disabled={!canRun || running}
            >
              {running ? (
                <><Loader2 size={12} className="mr-1 animate-spin" /> Running</>
              ) : (
                <><Play size={12} className="mr-1" /> Run</>
              )}
            </Button>
            {!installed && (
              <span className="text-xs text-muted-foreground">Install the app first to run tools</span>
            )}
          </div>

          {result && (
            <div className="relative">
              <Button
                variant="ghost"
                size="icon"
                className="absolute top-2 right-2 h-6 w-6 rounded-none"
                onClick={handleCopy}
              >
                {copied ? <Check size={12} /> : <Copy size={12} />}
              </Button>
              <pre className="text-xs font-mono bg-muted/50 border border-border p-3 overflow-auto max-h-64 whitespace-pre-wrap">{result}</pre>
            </div>
          )}

          {error && (
            <pre className="text-xs font-mono text-destructive bg-destructive/10 border border-destructive/20 p-3">{error}</pre>
          )}
        </div>
      )}
    </div>
  )
}
