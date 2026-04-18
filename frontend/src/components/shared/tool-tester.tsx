/**
 * Shared tool test component — used in app editor, builder, and workshop.
 * One component, used everywhere. Handles params form, run, output, HTTP trace.
 */
import { useState, useMemo } from "react"
import { Play, Loader2, Check, Copy, AlertTriangle, ChevronDown, ChevronRight } from "lucide-react"
import type { Spec } from "@json-render/core"
import { useTestTool, type TestToolResponse } from "@/hooks/use-test-tool"
import { SpecRenderer } from "@/lib/json-render-registry"
import { outputToSpec } from "@/lib/output-to-spec"
import { SchemaForm } from "@/components/workshop/schema-form"
import { HttpTrace } from "@/components/workshop/http-trace"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import type { StoreTool } from "@/lib/types"

interface Props {
  tool: StoreTool
  allowedHosts?: string[]
  settings?: Record<string, string>
  secrets?: Record<string, string>
  /** If true, show the script in a collapsible section */
  showScript?: boolean
  /** Compact mode — less padding, smaller text */
  compact?: boolean
}

export function ToolTester({ tool, allowedHosts = ["*"], settings, secrets, showScript, compact }: Props) {
  const [paramValues, setParamValues] = useState<Record<string, unknown>>({})
  const [timeout, setTimeout_] = useState("30s")
  const [result, setResult] = useState<TestToolResponse | null>(null)
  const [showRaw, setShowRaw] = useState(false)
  const [scriptOpen, setScriptOpen] = useState(false)
  const [copied, setCopied] = useState(false)

  const testTool = useTestTool()

  const handleRun = () => {
    testTool.mutate(
      { script: tool.script, params: paramValues, allowedHosts, settings, secrets, timeout },
      { onSuccess: (data) => setResult(data) },
    )
  }

  const handleCopy = () => {
    if (result) {
      navigator.clipboard.writeText(JSON.stringify(result.output, null, 2))
      setCopied(true)
      window.setTimeout(() => setCopied(false), 2000)
    }
  }

  const hasRequiredEmpty = Object.entries(tool.params).some(
    ([name, def]) => def.required && (paramValues[name] === undefined || paramValues[name] === ""),
  )

  const p = compact ? "p-0" : ""

  return (
    <div className={`space-y-3 ${p}`}>
      {/* Params form */}
      <SchemaForm params={tool.params} values={paramValues} onChange={setParamValues} />

      {/* Run + timeout */}
      <div className="flex items-center gap-3">
        <Button
          onClick={handleRun}
          disabled={testTool.isPending || hasRequiredEmpty}
          size={compact ? "sm" : "default"}
          className="rounded-none font-mono text-xs uppercase tracking-wider"
        >
          {testTool.isPending
            ? <><Loader2 size={12} className="mr-1.5 animate-spin" /> Running</>
            : <><Play size={12} className="mr-1.5" /> Run Test</>
          }
        </Button>
        <div className="flex items-center gap-2">
          <Label className="text-xs font-mono text-muted-foreground">Timeout</Label>
          <Input
            value={timeout}
            onChange={(e) => setTimeout_(e.target.value)}
            className="w-20 rounded-none bg-background/60 border-border font-mono text-xs h-7"
          />
        </div>
        {result && (
          <span className="text-xs font-mono text-muted-foreground">{Math.round(result.duration_ms)}ms</span>
        )}
      </div>

      {/* Network error */}
      {testTool.isError && (
        <div className="rounded-none border border-destructive/30 bg-destructive/10 p-2 text-xs text-destructive font-mono">
          {(testTool.error as Error).message}
        </div>
      )}

      {/* Result */}
      {result && (
        <div className="space-y-2">
          {result.error && (
            <div className="flex items-start gap-2 rounded-none border border-destructive/30 bg-destructive/10 p-2">
              <AlertTriangle size={12} className="text-destructive mt-0.5 shrink-0" />
              <pre className="text-xs font-mono text-destructive whitespace-pre-wrap">{result.error}</pre>
            </div>
          )}

          {result.output && (
            <div className="space-y-1">
              <div className="flex items-center justify-between">
                <Label className="text-[11px] font-mono text-muted-foreground uppercase tracking-wider">Output</Label>
                <div className="flex items-center gap-2">
                  <button onClick={() => setShowRaw(!showRaw)} className="text-[11px] font-mono text-muted-foreground hover:text-foreground">
                    {showRaw ? "Formatted" : "Raw"}
                  </button>
                  <Button variant="ghost" size="icon" className="h-5 w-5 rounded-none" onClick={handleCopy}>
                    {copied ? <Check size={10} /> : <Copy size={10} />}
                  </Button>
                </div>
              </div>
              {showRaw ? (
                <pre className="text-[11px] font-mono bg-muted/50 border border-border p-2 overflow-auto max-h-60 whitespace-pre-wrap">
                  {JSON.stringify(result.output, null, 2)}
                </pre>
              ) : (
                <div className="rounded-none border border-border p-2 overflow-auto max-h-60">
                  <SpecRenderer spec={outputToSpec(result.output)} />
                </div>
              )}
            </div>
          )}

          {result.http_log?.length > 0 && <HttpTrace entries={result.http_log} />}
        </div>
      )}

      {/* Script (collapsible) */}
      {showScript && (
        <div className="border border-border rounded-none">
          <button onClick={() => setScriptOpen(!scriptOpen)}
            className="w-full flex items-center gap-2 px-3 py-2 text-left hover:bg-accent/50 transition-colors">
            {scriptOpen ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
            <span className="text-xs font-mono font-medium text-muted-foreground uppercase tracking-wider">Script</span>
            <span className="text-[11px] font-mono text-muted-foreground ml-auto">{tool.script.split('\n').length} lines</span>
          </button>
          {scriptOpen && (
            <pre className="px-3 pb-3 text-xs font-mono text-muted-foreground whitespace-pre-wrap overflow-auto max-h-60">{tool.script}</pre>
          )}
        </div>
      )}
    </div>
  )
}
