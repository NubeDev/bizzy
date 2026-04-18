import { useState, useMemo } from "react"
import { Play, Loader2, Copy, Check, AlertTriangle, ChevronDown, ChevronRight } from "lucide-react"
import type { Spec } from "@json-render/core"
import { useTestTool, type TestToolResponse } from "@/hooks/use-test-tool"
import { SpecRenderer } from "@/lib/json-render-registry"
import { outputToSpec } from "@/lib/output-to-spec"
import { SchemaForm } from "./schema-form"
import { HttpTrace } from "./http-trace"
import { Button } from "@/components/ui/button"
import { Textarea } from "@/components/ui/textarea"
import { Label } from "@/components/ui/label"
import { Input } from "@/components/ui/input"
import type { StoreTool } from "@/lib/types"

interface Props {
  tool: StoreTool
  allowedHosts: string[]
  settings?: Record<string, string>
  secrets?: Record<string, string>
  onScriptChange?: (script: string) => void
  /** Optional json-render spec for rendering the input form instead of SchemaForm */
  inputSpec?: Spec
  /** Optional json-render spec for rendering output instead of auto-mapped spec */
  outputSpec?: Spec
}

export function ToolWorkbench({ tool, allowedHosts, settings, secrets, onScriptChange, inputSpec, outputSpec }: Props) {
  const [script, setScript] = useState(tool.script)
  const [helpers, setHelpers] = useState("")
  const [showHelpers, setShowHelpers] = useState(false)
  const [showScript, setShowScript] = useState(false)
  const [paramValues, setParamValues] = useState<Record<string, unknown>>({})
  const [timeout, setTimeout_] = useState("30s")
  const [result, setResult] = useState<TestToolResponse | null>(null)
  const [showRaw, setShowRaw] = useState(false)
  const [copied, setCopied] = useState(false)

  const testTool = useTestTool()

  const handleRun = () => {
    testTool.mutate(
      {
        script,
        helpers: helpers || undefined,
        params: paramValues,
        allowedHosts,
        settings,
        secrets,
        timeout,
      },
      {
        onSuccess: (data) => setResult(data),
      },
    )
  }

  const handleScriptChange = (val: string) => {
    setScript(val)
    onScriptChange?.(val)
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

  return (
    <div className="space-y-4">
      {/* Test form — hero section (full width, top) */}
      <div className="space-y-1.5">
        <Label className="text-xs font-mono font-medium text-muted-foreground uppercase tracking-wider">Test Params</Label>
        {inputSpec ? (
          <div className="border border-border rounded-none p-3 bg-background/60">
            <SpecRenderer spec={inputSpec} />
          </div>
        ) : (
          <SchemaForm params={tool.params} values={paramValues} onChange={setParamValues} />
        )}
      </div>

      {/* Timeout + Run button */}
      <div className="flex items-center gap-3">
        <Button
          onClick={handleRun}
          disabled={testTool.isPending || hasRequiredEmpty}
          className="rounded-none font-mono text-xs uppercase tracking-wider"
        >
          {testTool.isPending ? (
            <><Loader2 size={12} className="mr-1.5 animate-spin" /> Running</>
          ) : (
            <><Play size={12} className="mr-1.5" /> Run Test</>
          )}
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
          <span className="text-xs font-mono text-muted-foreground">
            {Math.round(result.duration_ms)}ms
          </span>
        )}
      </div>

      {/* Error from mutation (network level) */}
      {testTool.isError && (
        <div className="rounded-none border border-destructive/30 bg-destructive/10 p-3 text-xs text-destructive font-mono">
          {(testTool.error as Error).message}
        </div>
      )}

      {/* Result */}
      {result && (
        <div className="space-y-3">
          {/* Script-level error */}
          {result.error && (
            <div className="flex items-start gap-2 rounded-none border border-destructive/30 bg-destructive/10 p-3">
              <AlertTriangle size={14} className="text-destructive mt-0.5 shrink-0" />
              <pre className="text-xs font-mono text-destructive whitespace-pre-wrap">{result.error}</pre>
            </div>
          )}

          {/* Output */}
          {result.output && (
            <div className="space-y-1.5">
              <div className="flex items-center justify-between">
                <Label className="text-xs font-mono font-medium text-muted-foreground uppercase tracking-wider">Output</Label>
                <div className="flex items-center gap-2">
                  <button
                    onClick={() => setShowRaw(!showRaw)}
                    className="text-[11px] font-mono text-muted-foreground hover:text-foreground transition-colors"
                  >
                    {showRaw ? "Formatted" : "Raw JSON"}
                  </button>
                  <Button variant="ghost" size="icon" className="h-6 w-6 rounded-none" onClick={handleCopy}>
                    {copied ? <Check size={12} /> : <Copy size={12} />}
                  </Button>
                </div>
              </div>
              {showRaw ? (
                <pre className="text-xs font-mono bg-muted/50 border border-border p-3 overflow-auto max-h-80 whitespace-pre-wrap">
                  {JSON.stringify(result.output, null, 2)}
                </pre>
              ) : (
                <RenderedOutput data={result.output} outputSpec={outputSpec} />
              )}
            </div>
          )}

          {/* HTTP Trace */}
          <HttpTrace entries={result.http_log || []} />
        </div>
      )}

      {/* Script editor — collapsed at bottom */}
      <div className="border border-border rounded-none">
        <button
          onClick={() => setShowScript(!showScript)}
          className="w-full flex items-center gap-2 px-3 py-2 text-left hover:bg-accent/50 transition-colors"
        >
          {showScript ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
          <Label className="text-xs font-mono font-medium text-muted-foreground uppercase tracking-wider cursor-pointer">
            Script
          </Label>
          <span className="text-[11px] font-mono text-muted-foreground ml-auto">
            {script.split('\n').length} lines
          </span>
        </button>
        {showScript && (
          <div className="px-3 pb-3 space-y-1.5">
            <div className="flex justify-end">
              <button
                onClick={() => setShowHelpers(!showHelpers)}
                className="text-[11px] font-mono text-muted-foreground hover:text-foreground transition-colors"
              >
                {showHelpers ? "Hide helpers" : "Show helpers"}
              </button>
            </div>
            {showHelpers && (
              <Textarea
                value={helpers}
                onChange={(e) => setHelpers(e.target.value)}
                rows={5}
                placeholder="// Shared helper functions (optional)&#10;function apiGet(path) { ... }"
                className="font-mono text-xs rounded-none bg-background/60 border-border"
              />
            )}
            <Textarea
              value={script}
              onChange={(e) => handleScriptChange(e.target.value)}
              rows={12}
              className="font-mono text-xs rounded-none bg-background/60 border-border"
            />
          </div>
        )}
      </div>
    </div>
  )
}

/** Renders tool output using json-render with shadcn components. */
function RenderedOutput({ data, outputSpec }: { data: unknown; outputSpec?: Spec }) {
  const spec = useMemo(() => outputSpec ?? outputToSpec(data), [data, outputSpec])
  return (
    <div className="rounded-none border border-border p-3 overflow-auto max-h-80">
      <SpecRenderer spec={spec} />
    </div>
  )
}
