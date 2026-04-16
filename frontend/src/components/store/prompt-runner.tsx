import { useState } from "react"
import { Play, Loader2, ChevronDown, ChevronRight, Copy, Check } from "lucide-react"
import { api } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import type { StorePrompt } from "@/lib/types"

interface PromptRunnerProps {
  appName: string
  prompt: StorePrompt
  installed: boolean
}

export function PromptRunner({ appName, prompt, installed }: PromptRunnerProps) {
  const [expanded, setExpanded] = useState(false)
  const [args, setArgs] = useState<Record<string, string>>({})
  const [running, setRunning] = useState(false)
  const [result, setResult] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)

  const argDefs = prompt.arguments || []

  const handleRun = async () => {
    setRunning(true)
    setResult(null)
    setError(null)
    try {
      // Interpolate arguments into the prompt body
      let body = prompt.body || ""
      for (const [k, v] of Object.entries(args)) {
        body = body.replaceAll(`{{${k}}}`, v)
      }
      const res = await api.runPrompt(body)
      setResult(res.text)
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

  const canRun = installed && argDefs.every(a =>
    !a.required || (args[a.name] && args[a.name].trim() !== "")
  )

  return (
    <div className="rounded-none border border-border bg-card">
      <button
        className="w-full flex items-center gap-2 p-4 text-left hover:bg-accent/50 transition-colors"
        onClick={() => setExpanded(!expanded)}
      >
        {expanded ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
        <code className="text-sm font-medium font-mono flex-1">{prompt.name}</code>
      </button>

      {expanded && (
        <div className="border-t border-border p-4 space-y-3">
          <p className="text-xs text-muted-foreground">{prompt.description}</p>

          {argDefs.length > 0 && (
            <div className="space-y-2">
              {argDefs.map(a => (
                <div key={a.name} className="space-y-1">
                  <Label className="text-xs font-mono">
                    {a.name}
                    {a.required && <span className="text-destructive ml-0.5">*</span>}
                  </Label>
                  <Input
                    className="rounded-none bg-transparent border-border font-mono text-sm h-8"
                    placeholder={a.description}
                    value={args[a.name] || ""}
                    onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
                      setArgs({ ...args, [a.name]: e.target.value })
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
              <span className="text-xs text-muted-foreground">Install the app first to run prompts</span>
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
