/**
 * Visual Builder for existing apps.
 *
 * Embeds the visual plan + workshop flow inside the app editor.
 * Lets users redesign tool UIs, add/remove tools, test with real data,
 * and apply changes back to the app — without starting from scratch.
 */
import { useState, useCallback, useEffect } from "react"
import { Sparkles, Loader2, Check, Play, CheckCircle, Circle, AlertTriangle, Copy, ChevronDown, ChevronRight, Wrench, FlaskConical, ArrowRight } from "lucide-react"
import type { Spec } from "@json-render/core"
import { useAgentChat } from "@/hooks/use-agent-chat"
import { useAddTool, useUpdateTool, useDeleteTool } from "@/hooks/use-my-apps"
import { useTestTool, type TestToolResponse } from "@/hooks/use-test-tool"
import { SpecRenderer } from "@/lib/json-render-registry"
import { outputToSpec } from "@/lib/output-to-spec"
import { SchemaForm } from "@/components/workshop/schema-form"
import { HttpTrace } from "@/components/workshop/http-trace"
import { VisualPlanCard, type VisualToolPlan, type ToolPlanStatus } from "@/components/workshop/visual-plan"
import { VISUAL_PLAN_PROMPT, REFINE_TOOL_PROMPT, GENERATE_PROMPT, ADD_TOOLS_PROMPT } from "@/lib/wizard-prompts"
import { Button } from "@/components/ui/button"
import { Textarea } from "@/components/ui/textarea"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Separator } from "@/components/ui/separator"
import type { StoreApp, StoreTool } from "@/lib/types"

type BuilderStep = "plan" | "generate" | "workshop" | "done"

interface Props {
  app: StoreApp
}

export function AppVisualBuilder({ app }: Props) {
  const { messages, isStreaming, send, clear } = useAgentChat()
  const addToolMutation = useAddTool()
  const updateToolMutation = useUpdateTool()
  const deleteToolMutation = useDeleteTool()

  const [step, setStep] = useState<BuilderStep>("plan")
  const [plan, setPlan] = useState<VisualToolPlan[] | null>(null)
  const [generatedTools, setGeneratedTools] = useState<StoreTool[]>([])
  const [toolStatus, setToolStatus] = useState<Record<string, ToolPlanStatus>>({})
  const [refiningTool, setRefiningTool] = useState<string | null>(null)
  const [addingTools, setAddingTools] = useState(false)
  const [addToolsInput, setAddToolsInput] = useState("")
  const [regenInput, setRegenInput] = useState("")
  const [workshopToolName, setWorkshopToolName] = useState<string | null>(null)
  const [testedTools, setTestedTools] = useState<Set<string>>(new Set())
  const [applyProgress, setApplyProgress] = useState("")
  const [generated, setGenerated] = useState(false)

  // Seed plan from existing app tools on first render
  useEffect(() => {
    if (plan || !app.tools?.length) return
    const seeded: VisualToolPlan[] = app.tools.map(t => ({
      name: t.name,
      description: t.description,
      toolClass: t.toolClass,
      params: t.params as VisualToolPlan["params"],
      enabled: true,
    }))
    setPlan(seeded)
    const status: Record<string, ToolPlanStatus> = {}
    for (const t of seeded) status[t.name] = "pending"
    setToolStatus(status)
  }, [app.tools, plan])

  // Generate visual specs for existing tools via AI
  const handleGenerateSpecs = useCallback(() => {
    if (!app.tools?.length) return
    clear()
    const desc = app.tools.map(t =>
      `- ${t.name}: ${t.description} (class: ${t.toolClass}, params: ${JSON.stringify(t.params)})`
    ).join('\n')
    const prompt = VISUAL_PLAN_PROMPT + `Existing app "${app.displayName}": ${app.description}\n\nExisting tools:\n${desc}\n\nGenerate visual specs (inputSpec, outputSpec, sampleOutput) for these existing tools. Keep the same tool names, params, and descriptions.`
    send(prompt, `Generating visual specs for ${app.displayName}...`)
  }, [app, send, clear])

  // Regenerate with feedback
  const handleRegenerate = useCallback((feedback?: string) => {
    clear()
    setPlan(null)
    setToolStatus({})
    setRefiningTool(null)
    setAddingTools(false)
    setRegenInput("")

    const desc = app.tools?.map(t =>
      `- ${t.name}: ${t.description} (class: ${t.toolClass}, params: ${JSON.stringify(t.params)})`
    ).join('\n') || ''

    const extra = feedback?.trim()
      ? `\n\nUser feedback on the previous plan: "${feedback.trim()}"`
      : `\n\nGenerate a different visual design.`

    const prompt = VISUAL_PLAN_PROMPT + `Existing app "${app.displayName}": ${app.description}\n\nExisting tools:\n${desc}${extra}`
    send(prompt, `Regenerating specs...`)
  }, [app, send, clear])

  // Parse plan from AI response
  const lastAssistantMsg = messages.filter(m => m.role === 'assistant').pop()
  const lastContent = lastAssistantMsg?.content || ''

  useEffect(() => {
    if (step !== 'plan' || isStreaming || !lastContent) return

    // Refining a tool
    if (refiningTool && plan) {
      const jsonMatch = lastContent.match(/```(?:json)?\s*\n([\s\S]*?)```/)
      if (jsonMatch) {
        try {
          const t = JSON.parse(jsonMatch[1])
          const refined: VisualToolPlan = {
            name: t.name, description: t.description, toolClass: t.toolClass,
            params: t.params || {}, inputSpec: t.inputSpec, outputSpec: t.outputSpec,
            sampleOutput: t.sampleOutput, enabled: true,
          }
          setPlan(plan.map(p => p.name === refiningTool ? { ...refined, enabled: p.enabled } : p))
          setToolStatus(prev => ({ ...prev, [refiningTool]: "pending" }))
          setRefiningTool(null)
        } catch { /* skip */ }
      }
      return
    }

    // Adding tools
    if (addingTools && plan) {
      const jsonMatch = lastContent.match(/```(?:json)?\s*\n([\s\S]*?)```/)
      if (jsonMatch) {
        try {
          const raw = JSON.parse(jsonMatch[1])
          const newTools: VisualToolPlan[] = (Array.isArray(raw) ? raw : [raw]).map((t: Record<string, unknown>) => ({
            name: t.name as string, description: t.description as string, toolClass: t.toolClass as string,
            params: (t.params || {}) as VisualToolPlan["params"],
            inputSpec: t.inputSpec as Spec | undefined, outputSpec: t.outputSpec as Spec | undefined,
            sampleOutput: t.sampleOutput, enabled: true,
          }))
          if (newTools.length > 0) {
            setPlan([...plan, ...newTools])
            setToolStatus(prev => {
              const next = { ...prev }
              for (const t of newTools) next[t.name] = "pending"
              return next
            })
          }
        } catch { /* skip */ }
      }
      setAddingTools(false)
      return
    }

    // Initial plan parse
    if (plan?.every(t => !t.inputSpec && !t.outputSpec)) {
      const jsonMatch = lastContent.match(/```(?:json)?\s*\n([\s\S]*?)```/)
      if (jsonMatch) {
        try {
          const raw = JSON.parse(jsonMatch[1])
          const tools: VisualToolPlan[] = (raw.tools || []).map((t: Record<string, unknown>) => ({
            name: t.name as string, description: t.description as string, toolClass: t.toolClass as string,
            params: (t.params || {}) as VisualToolPlan["params"],
            inputSpec: t.inputSpec as Spec | undefined, outputSpec: t.outputSpec as Spec | undefined,
            sampleOutput: t.sampleOutput, enabled: true,
          }))
          if (tools.length > 0) {
            setPlan(tools)
            const status: Record<string, ToolPlanStatus> = {}
            for (const t of tools) status[t.name] = "pending"
            setToolStatus(status)
          }
        } catch { /* skip */ }
      }
    }
  }, [step, isStreaming, lastContent, plan, refiningTool, addingTools])

  // Watch for generated tools → workshop
  useEffect(() => {
    if (step !== 'generate' || isStreaming || generated || !lastContent) return
    const tools: StoreTool[] = []
    const toolRegex = /```json:tool\s*\n([\s\S]*?)```/g
    let match
    while ((match = toolRegex.exec(lastContent)) !== null) {
      try {
        const t = JSON.parse(match[1])
        if (t.name && t.script) {
          tools.push({ name: t.name, description: t.description || '', toolClass: t.toolClass || 'read-only', mode: '', params: t.params || {}, script: t.script })
        }
      } catch { /* skip */ }
    }
    if (tools.length === 0) {
      const fallbackRegex = /```json\s*\n([\s\S]*?)```/g
      while ((match = fallbackRegex.exec(lastContent)) !== null) {
        try {
          const t = JSON.parse(match[1])
          if (t.name && t.script) {
            tools.push({ name: t.name, description: t.description || '', toolClass: t.toolClass || 'read-only', mode: '', params: t.params || {}, script: t.script })
          }
        } catch { /* skip */ }
      }
    }
    if (tools.length > 0) {
      setGeneratedTools(tools)
      setWorkshopToolName(tools[0].name)
      setGenerated(true)
      setStep('workshop')
    }
  }, [step, isStreaming, lastContent, generated])

  const activeTools = plan?.filter(t => toolStatus[t.name] !== "rejected") || []
  const allApproved = activeTools.length > 0 && activeTools.every(t => toolStatus[t.name] === "approved")
  const hasSpecs = plan?.some(t => t.inputSpec || t.outputSpec) ?? false

  const handleGenerate = useCallback(() => {
    if (!plan) return
    clear()
    setGenerated(false)
    const toolsToGen = plan.filter(t => toolStatus[t.name] !== "rejected")
    const toolsDesc = toolsToGen.map(t =>
      `- ${t.name}: ${t.description} (class: ${t.toolClass}, params: ${JSON.stringify(t.params)})`
    ).join('\n')
    send(GENERATE_PROMPT + toolsDesc, `Generating ${toolsToGen.length} tools...`)
    setStep('generate')
  }, [plan, toolStatus, send, clear])

  const handleRequestEdit = (toolName: string, feedback: string) => {
    if (!plan) return
    const tool = plan.find(t => t.name === toolName)
    if (!tool) return
    setRefiningTool(toolName)
    setToolStatus(prev => ({ ...prev, [toolName]: "editing" }))
    const toolJson = JSON.stringify({
      name: tool.name, description: tool.description, toolClass: tool.toolClass,
      params: tool.params, inputSpec: tool.inputSpec, outputSpec: tool.outputSpec, sampleOutput: tool.sampleOutput,
    }, null, 2)
    send(REFINE_TOOL_PROMPT + toolJson + "\n\nUser's requested change:\n" + feedback, `Refining ${toolName}: ${feedback}`)
  }

  const handleAddTools = (request: string) => {
    if (!plan || !request.trim()) return
    setAddingTools(true)
    const existing = plan.map(t => `- ${t.name}: ${t.description}`).join('\n')
    send(ADD_TOOLS_PROMPT + existing + "\n\nUser's request:\n" + request.trim(), `Adding tools: ${request.trim()}`)
    setAddToolsInput("")
  }

  // Apply changes back to the actual app
  const handleApplyToApp = async () => {
    if (generatedTools.length === 0) return
    const existingNames = new Set(app.tools?.map(t => t.name) || [])
    try {
      for (const tool of generatedTools) {
        if (existingNames.has(tool.name)) {
          setApplyProgress(`Updating ${tool.name}...`)
          await updateToolMutation.mutateAsync({ appId: app.id, name: tool.name, tool })
        } else {
          setApplyProgress(`Adding ${tool.name}...`)
          await addToolMutation.mutateAsync({ appId: app.id, tool })
        }
      }
      // Delete tools that were rejected
      const generatedNames = new Set(generatedTools.map(t => t.name))
      for (const name of existingNames) {
        if (!generatedNames.has(name)) {
          setApplyProgress(`Removing ${name}...`)
          await deleteToolMutation.mutateAsync({ appId: app.id, name })
        }
      }
      setApplyProgress("")
      setStep('done')
    } catch (err) {
      setApplyProgress(`Error: ${err instanceof Error ? err.message : 'Unknown error'}`)
    }
  }

  const getSpecsForTool = (toolName: string) => {
    const planTool = plan?.find(t => t.name === toolName)
    return { inputSpec: planTool?.inputSpec, outputSpec: planTool?.outputSpec }
  }

  return (
    <div className="space-y-4">
      {/* Step indicator */}
      <div className="flex items-center gap-2 text-xs text-muted-foreground">
        {(["plan", "generate", "workshop", "done"] as BuilderStep[]).map((s, i) => (
          <div key={s} className="flex items-center gap-2">
            {i > 0 && <ChevronRight size={12} />}
            <span className={step === s ? 'text-foreground font-medium' : ''}>
              {s === 'plan' ? 'Visual Plan' : s === 'generate' ? 'Generate' : s === 'workshop' ? 'Workshop' : 'Applied'}
            </span>
          </div>
        ))}
      </div>

      {/* Plan step */}
      {step === 'plan' && (
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Visual Builder</CardTitle>
            <CardDescription>
              {!hasSpecs
                ? "Generate visual UI specs for your tools, then test and apply changes."
                : "Approve, reject, or tweak each tool's UI. Add more if you need them."
              }
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            {/* Generate specs button (if no specs yet) */}
            {!hasSpecs && !isStreaming && (
              <Button onClick={handleGenerateSpecs} className="rounded-none text-xs">
                <Sparkles size={12} className="mr-1" /> Generate Visual Specs for {app.tools?.length || 0} Tools
              </Button>
            )}

            {/* Streaming feedback */}
            {isStreaming && (
              <StreamingFeedback content={lastContent} label={refiningTool ? `Refining ${refiningTool}...` : addingTools ? "Adding new tools..." : "Generating visual specs..."} />
            )}

            {/* Tool cards */}
            {plan && hasSpecs && (
              <>
                <div className="flex items-center justify-between">
                  <Label className="text-xs">
                    {activeTools.filter(t => toolStatus[t.name] === "approved").length}/{activeTools.length} approved
                    {allApproved && <span className="text-emerald-500 ml-2">Ready to generate</span>}
                  </Label>
                </div>
                <div className="space-y-3">
                  {plan.map(tool => (
                    <div key={tool.name}>
                      {isStreaming && refiningTool === tool.name ? (
                        <div className="border border-border rounded-none p-4">
                          <StreamingFeedback content={lastContent} label={`Refining ${tool.name}...`} />
                        </div>
                      ) : (
                        <VisualPlanCard
                          tool={tool}
                          status={toolStatus[tool.name] || "pending"}
                          onApprove={() => setToolStatus(prev => ({ ...prev, [tool.name]: "approved" }))}
                          onReject={() => setToolStatus(prev => ({ ...prev, [tool.name]: "rejected" }))}
                          onRestore={() => setToolStatus(prev => ({ ...prev, [tool.name]: "pending" }))}
                          onRequestEdit={(fb) => handleRequestEdit(tool.name, fb)}
                        />
                      )}
                    </div>
                  ))}
                </div>

                {/* Add more tools */}
                {!isStreaming && (
                  <div className="border border-dashed border-border rounded-none p-3 space-y-2">
                    <Label className="text-xs text-muted-foreground">Want more tools?</Label>
                    <div className="flex gap-2">
                      <Textarea
                        value={addToolsInput}
                        onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) => setAddToolsInput(e.target.value)}
                        placeholder='e.g. "add a tool that checks DNS records"'
                        rows={1}
                        className="text-xs rounded-none flex-1"
                      />
                      <Button size="sm" variant="outline" className="rounded-none text-xs shrink-0"
                        onClick={() => handleAddTools(addToolsInput)} disabled={!addToolsInput.trim() || isStreaming}>
                        <Sparkles size={12} className="mr-1" /> Add
                      </Button>
                    </div>
                  </div>
                )}

                {/* Regenerate */}
                {!isStreaming && (
                  <div className="border border-dashed border-border rounded-none p-3 space-y-2">
                    <Label className="text-xs text-muted-foreground">Not happy? Regenerate with feedback:</Label>
                    <div className="flex gap-2">
                      <Textarea
                        value={regenInput}
                        onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) => setRegenInput(e.target.value)}
                        placeholder='e.g. "use card layouts instead of tables"'
                        rows={1}
                        className="text-xs rounded-none flex-1"
                      />
                      <Button size="sm" variant="outline" className="rounded-none text-xs shrink-0"
                        onClick={() => handleRegenerate(regenInput)} disabled={isStreaming}>
                        <Sparkles size={12} className="mr-1" /> Regenerate
                      </Button>
                    </div>
                  </div>
                )}

                <Separator />
                <div className="flex justify-end">
                  <Button onClick={handleGenerate} disabled={!allApproved || activeTools.length === 0 || isStreaming}>
                    <Sparkles size={14} className="mr-1" />
                    {allApproved ? `Generate ${activeTools.length} Tools` : `Approve all first (${activeTools.filter(t => toolStatus[t.name] === "approved").length}/${activeTools.length})`}
                  </Button>
                </div>
              </>
            )}
          </CardContent>
        </Card>
      )}

      {/* Generate step */}
      {step === 'generate' && (
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Generating Code</CardTitle>
          </CardHeader>
          <CardContent>
            <StreamingFeedback content={lastContent} label="Writing tool implementations..." />
          </CardContent>
        </Card>
      )}

      {/* Workshop step */}
      {step === 'workshop' && (
        <>
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <FlaskConical size={16} /> Workshop — Test with Real Data
              </CardTitle>
              <CardDescription>
                Test each tool, then apply changes to your app.
              </CardDescription>
            </CardHeader>
          </Card>

          <div className="flex flex-wrap gap-1 border-b border-border pb-2">
            {generatedTools.map(t => {
              const tested = testedTools.has(t.name)
              const isActive = workshopToolName === t.name
              return (
                <button key={t.name} onClick={() => setWorkshopToolName(t.name)}
                  className={`flex items-center gap-1.5 px-3 py-1.5 text-xs font-mono rounded-none transition-colors ${isActive ? "bg-accent text-foreground border border-border" : "text-muted-foreground hover:bg-accent/50"}`}>
                  {tested ? <CheckCircle size={12} className="text-emerald-400" /> : <Circle size={12} className="text-muted-foreground/40" />}
                  {t.name}
                </button>
              )
            })}
          </div>

          {generatedTools.map(tool => {
            if (tool.name !== workshopToolName) return null
            const specs = getSpecsForTool(tool.name)
            return <EmbeddedWorkbench key={tool.name} tool={tool} inputSpec={specs.inputSpec} outputSpec={specs.outputSpec} onTested={() => setTestedTools(prev => new Set(prev).add(tool.name))} />
          })}

          <Separator />
          <div className="flex justify-between items-center">
            <span className="text-xs text-muted-foreground">{testedTools.size}/{generatedTools.length} tested</span>
            <div className="flex gap-2">
              <Button variant="ghost" onClick={() => { setStep('plan'); setGenerated(false); setGeneratedTools([]) }}>
                Back to Plan
              </Button>
              <Button onClick={handleApplyToApp} disabled={!!applyProgress}>
                {applyProgress ? <><Loader2 size={14} className="mr-1 animate-spin" /> {applyProgress}</> : <><Check size={14} className="mr-1" /> Apply to App</>}
              </Button>
            </div>
          </div>
        </>
      )}

      {/* Done */}
      {step === 'done' && (
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-base">
              <Check size={16} className="text-green-500" /> Changes Applied
            </CardTitle>
            <CardDescription>
              {generatedTools.length} tools have been updated in {app.displayName}.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-2 mb-4">
              {generatedTools.map(t => (
                <div key={t.name} className="flex items-center gap-2 text-sm">
                  <Check size={14} className="text-green-500" />
                  <Wrench size={12} className="text-muted-foreground" />
                  <code>{t.name}</code>
                  <Badge variant="outline" className="text-[10px]">{t.toolClass}</Badge>
                </div>
              ))}
            </div>
            <Button variant="outline" onClick={() => { setStep('plan'); setPlan(null); setGeneratedTools([]); setGenerated(false); setTestedTools(new Set()); setToolStatus({}) }}>
              Run Visual Builder Again
            </Button>
          </CardContent>
        </Card>
      )}
    </div>
  )
}

function StreamingFeedback({ content, label }: { content: string; label: string }) {
  const preview = content
    ? content.replace(/```[\s\S]*?```/g, '[code block]').split('\n').filter(l => l.trim()).slice(-6).join('\n')
    : ''
  return (
    <div className="space-y-3">
      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        <Loader2 size={14} className="animate-spin shrink-0" /> <span>{label}</span>
      </div>
      {preview && (
        <pre className="text-xs font-mono text-muted-foreground bg-muted/50 border border-border rounded-none p-3 overflow-auto max-h-40 whitespace-pre-wrap">
          {preview}<span className="inline-block w-1.5 h-3.5 bg-foreground/60 ml-0.5 animate-pulse" />
        </pre>
      )}
    </div>
  )
}

function EmbeddedWorkbench({ tool, inputSpec, outputSpec, onTested }: { tool: StoreTool; inputSpec?: Spec; outputSpec?: Spec; onTested: () => void }) {
  const [paramValues, setParamValues] = useState<Record<string, unknown>>({})
  const [timeout, setTimeout_] = useState("30s")
  const [result, setResult] = useState<TestToolResponse | null>(null)
  const [showRaw, setShowRaw] = useState(false)
  const [showScript, setShowScript] = useState(false)
  const [copied, setCopied] = useState(false)
  const testTool = useTestTool()

  const handleRun = () => {
    testTool.mutate({ script: tool.script, params: paramValues, allowedHosts: ["*"], timeout }, {
      onSuccess: (data) => { setResult(data); onTested() },
    })
  }

  const handleCopy = () => {
    if (result) { navigator.clipboard.writeText(JSON.stringify(result.output, null, 2)); setCopied(true); window.setTimeout(() => setCopied(false), 2000) }
  }

  const hasRequiredEmpty = Object.entries(tool.params).some(([name, def]) => def.required && (paramValues[name] === undefined || paramValues[name] === ""))

  return (
    <Card>
      <CardHeader className="pb-3">
        <div className="flex items-center gap-2">
          <Wrench size={14} className="text-muted-foreground" />
          <code className="text-sm font-semibold">{tool.name}</code>
          <Badge variant="outline" className="text-[10px]">{tool.toolClass}</Badge>
        </div>
        {tool.description && <p className="text-xs text-muted-foreground">{tool.description}</p>}
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="space-y-1.5">
          <Label className="text-xs font-mono font-medium text-muted-foreground uppercase tracking-wider">Input</Label>
          {inputSpec ? (
            <div className="border border-border rounded-none p-3 bg-background/60"><SpecRenderer spec={inputSpec} /></div>
          ) : (
            <SchemaForm params={tool.params} values={paramValues} onChange={setParamValues} />
          )}
        </div>
        <div className="flex items-center gap-3">
          <Button onClick={handleRun} disabled={testTool.isPending || hasRequiredEmpty} className="rounded-none font-mono text-xs uppercase tracking-wider">
            {testTool.isPending ? <><Loader2 size={12} className="mr-1.5 animate-spin" /> Running</> : <><Play size={12} className="mr-1.5" /> Run Test</>}
          </Button>
          <div className="flex items-center gap-2">
            <Label className="text-xs font-mono text-muted-foreground">Timeout</Label>
            <Input value={timeout} onChange={(e) => setTimeout_(e.target.value)} className="w-20 rounded-none bg-background/60 border-border font-mono text-xs h-7" />
          </div>
          {result && <span className="text-xs font-mono text-muted-foreground">{Math.round(result.duration_ms)}ms</span>}
        </div>
        {testTool.isError && <div className="rounded-none border border-destructive/30 bg-destructive/10 p-3 text-xs text-destructive font-mono">{(testTool.error as Error).message}</div>}
        {result && (
          <div className="space-y-3">
            {result.error && (
              <div className="flex items-start gap-2 rounded-none border border-destructive/30 bg-destructive/10 p-3">
                <AlertTriangle size={14} className="text-destructive mt-0.5 shrink-0" />
                <pre className="text-xs font-mono text-destructive whitespace-pre-wrap">{result.error}</pre>
              </div>
            )}
            {result.output && (
              <div className="space-y-1.5">
                <div className="flex items-center justify-between">
                  <Label className="text-xs font-mono font-medium text-muted-foreground uppercase tracking-wider">Output</Label>
                  <div className="flex items-center gap-2">
                    <button onClick={() => setShowRaw(!showRaw)} className="text-[11px] font-mono text-muted-foreground hover:text-foreground transition-colors">{showRaw ? "Formatted" : "Raw JSON"}</button>
                    <Button variant="ghost" size="icon" className="h-6 w-6 rounded-none" onClick={handleCopy}>{copied ? <Check size={12} /> : <Copy size={12} />}</Button>
                  </div>
                </div>
                {showRaw ? (
                  <pre className="text-xs font-mono bg-muted/50 border border-border p-3 overflow-auto max-h-80 whitespace-pre-wrap">{JSON.stringify(result.output, null, 2)}</pre>
                ) : (
                  <div className="rounded-none border border-border p-3 overflow-auto max-h-80"><SpecRenderer spec={outputSpec ?? outputToSpec(result.output)} /></div>
                )}
              </div>
            )}
            <HttpTrace entries={result.http_log || []} />
          </div>
        )}
        <div className="border border-border rounded-none">
          <button onClick={() => setShowScript(!showScript)} className="w-full flex items-center gap-2 px-3 py-2 text-left hover:bg-accent/50 transition-colors">
            {showScript ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
            <span className="text-xs font-mono font-medium text-muted-foreground uppercase tracking-wider">Script</span>
            <span className="text-[11px] font-mono text-muted-foreground ml-auto">{tool.script.split('\n').length} lines</span>
          </button>
          {showScript && <pre className="px-3 pb-3 text-xs font-mono text-muted-foreground whitespace-pre-wrap overflow-auto max-h-60">{tool.script}</pre>}
        </div>
      </CardContent>
    </Card>
  )
}
