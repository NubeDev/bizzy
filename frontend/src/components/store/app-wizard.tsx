import { useState, useCallback, useEffect } from "react"
import { useNavigate } from "react-router-dom"
import { Sparkles, Loader2, Check, X, MessageSquare, ChevronRight, ArrowLeft, FlaskConical, Wrench, Play, CheckCircle, Circle, AlertTriangle, Copy, ChevronDown } from "lucide-react"
import type { Spec } from "@json-render/core"
import { useAgentChat } from "@/hooks/use-agent-chat"
import { useCreateApp, useAddTool, useAddPrompt, useUpdateApp } from "@/hooks/use-my-apps"
import { useTestTool, type TestToolResponse } from "@/hooks/use-test-tool"
import { SpecRenderer } from "@/lib/json-render-registry"
import { outputToSpec } from "@/lib/output-to-spec"
import { SchemaForm } from "@/components/workshop/schema-form"
import { HttpTrace } from "@/components/workshop/http-trace"
import { Button } from "@/components/ui/button"
import { Textarea } from "@/components/ui/textarea"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Separator } from "@/components/ui/separator"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { categoryLabel } from "@/lib/utils"
import { VisualPlanCard, type VisualToolPlan, type ToolPlanStatus } from "@/components/workshop/visual-plan"
import { VISUAL_PLAN_PROMPT, REFINE_TOOL_PROMPT, GENERATE_PROMPT, ADD_TOOLS_PROMPT } from "@/lib/wizard-prompts"
import type { StoreTool } from "@/lib/types"

const categories = [
  "iot-devices", "analytics", "devops", "marketing",
  "design", "utilities", "integrations", "automation",
]

interface PromptPlan {
  name: string
  description: string
  arguments?: { name: string; description: string; required: boolean }[]
  enabled: boolean
}

interface AppPlan {
  name: string
  displayName: string
  description: string
  category: string
  tools: VisualToolPlan[]
  prompts: PromptPlan[]
}

type Step = 'describe' | 'plan' | 'generate' | 'workshop' | 'done'

const STEP_LABELS: Record<Step, string> = {
  describe: '1. Describe',
  plan: '2. Visual Plan',
  generate: '3. Generate',
  workshop: '4. Workshop',
  done: '5. Done',
}

const STEPS: Step[] = ['describe', 'plan', 'generate', 'workshop', 'done']

export function AppWizard() {
  const navigate = useNavigate()
  const { messages, isStreaming, send, clear } = useAgentChat()
  const createAppMutation = useCreateApp()
  const updateAppMutation = useUpdateApp()
  const addToolMutation = useAddTool()
  const addPromptMutation = useAddPrompt()

  const [step, setStep] = useState<Step>('describe')
  const [idea, setIdea] = useState("")
  const [plan, setPlan] = useState<AppPlan | null>(null)
  const [planError, setPlanError] = useState("")
  const [generatedTools, setGeneratedTools] = useState<StoreTool[]>([])
  const [createdAppId, setCreatedAppId] = useState<string | null>(null)
  const [buildProgress, setBuildProgress] = useState("")
  const [buildDone, setBuildDone] = useState(false)

  // Track approval status per tool (visual plan step)
  const [toolStatus, setToolStatus] = useState<Record<string, ToolPlanStatus>>({})
  // Track which tool is being refined
  const [refiningTool, setRefiningTool] = useState<string | null>(null)
  // Track if we're adding new tools
  const [addingTools, setAddingTools] = useState(false)
  // "Add more tools" chat input
  const [addToolsInput, setAddToolsInput] = useState("")

  // Workshop step state
  const [workshopToolName, setWorkshopToolName] = useState<string | null>(null)
  const [testedTools, setTestedTools] = useState<Set<string>>(new Set())

  // Regenerate feedback input
  const [regenFeedback, setRegenFeedback] = useState("")

  // Step 1 → 2: Generate plan from description
  const handleGeneratePlan = useCallback(() => {
    if (!idea.trim()) return
    setPlanError("")
    const prompt = VISUAL_PLAN_PROMPT + idea.trim()
    send(prompt, idea.trim())
    setStep('plan')
  }, [idea, send])

  // Regenerate plan keeping context — clears current plan and re-prompts AI
  const handleRegeneratePlan = useCallback((feedback?: string) => {
    if (!idea.trim()) return
    clear()
    setPlan(null)
    setPlanError("")
    setToolStatus({})
    setRefiningTool(null)
    setAddingTools(false)
    setRegenFeedback("")

    const extra = feedback?.trim()
      ? `\n\nThe user was not happy with the previous plan. Their feedback: "${feedback.trim()}"\nGenerate a different/better plan based on this feedback.`
      : `\n\nThe user wants a fresh take. Generate a different plan for the same idea.`

    const prompt = VISUAL_PLAN_PROMPT + idea.trim() + extra
    send(prompt, `Regenerating: ${idea.trim()}`)
  }, [idea, send, clear])

  // Parse the plan from AI response (now with specs)
  const parsePlan = useCallback((content: string): AppPlan | null => {
    const jsonMatch = content.match(/```(?:json)?\s*\n([\s\S]*?)```/)
    if (!jsonMatch) return null
    try {
      const raw = JSON.parse(jsonMatch[1])
      const tools: VisualToolPlan[] = (raw.tools || []).map((t: Record<string, unknown>) => ({
        name: t.name as string,
        description: t.description as string,
        toolClass: t.toolClass as string,
        params: (t.params || {}) as VisualToolPlan["params"],
        inputSpec: t.inputSpec as Spec | undefined,
        outputSpec: t.outputSpec as Spec | undefined,
        sampleOutput: t.sampleOutput,
        enabled: true,
      }))
      return {
        name: raw.name || "my-app",
        displayName: raw.displayName || raw.name || "My App",
        description: raw.description || "",
        category: raw.category || "utilities",
        tools,
        prompts: (raw.prompts || []).map((p: Record<string, unknown>) => ({ ...p, enabled: true })),
      }
    } catch {
      return null
    }
  }, [])

  // Parse a refined tool from AI response
  const parseRefinedTool = useCallback((content: string): VisualToolPlan | null => {
    const jsonMatch = content.match(/```(?:json)?\s*\n([\s\S]*?)```/)
    if (!jsonMatch) return null
    try {
      const t = JSON.parse(jsonMatch[1])
      return {
        name: t.name,
        description: t.description,
        toolClass: t.toolClass,
        params: t.params || {},
        inputSpec: t.inputSpec,
        outputSpec: t.outputSpec,
        sampleOutput: t.sampleOutput,
        enabled: true,
      }
    } catch {
      return null
    }
  }, [])

  // Build the app (called from workshop step when user finalizes)
  const buildApp = useCallback(async () => {
    if (!plan || generatedTools.length === 0) return
    try {
      setBuildProgress("Creating app...")
      const app = await createAppMutation.mutateAsync({
        name: plan.name,
        displayName: plan.displayName,
        description: plan.description,
        category: plan.category,
      })
      setCreatedAppId(app.id)

      for (const tool of generatedTools) {
        setBuildProgress(`Adding tool: ${tool.name}...`)
        await addToolMutation.mutateAsync({ appId: app.id, tool })
      }

      const enabledPrompts = plan.prompts.filter(p => p.enabled)
      for (const p of enabledPrompts) {
        setBuildProgress(`Adding prompt: ${p.name}...`)
        await addPromptMutation.mutateAsync({
          appId: app.id,
          prompt: {
            name: p.name,
            description: p.description,
            arguments: p.arguments,
            body: `{{${(p.arguments || []).map(a => a.name).join('}}\n\n{{')}}}`,
          }
        })
      }

      if (plan.description.length >= 20) {
        await updateAppMutation.mutateAsync({
          id: app.id,
          data: { description: plan.description, longDescription: plan.description },
        })
      }

      setBuildProgress("")
      setBuildDone(true)
      setStep('done')
    } catch (err) {
      setBuildProgress(`Error: ${err instanceof Error ? err.message : 'Unknown error'}`)
    }
  }, [plan, generatedTools, createAppMutation, addToolMutation, addPromptMutation, updateAppMutation])

  // Watch for plan completion
  const lastAssistantMsg = messages.filter(m => m.role === 'assistant').pop()
  const lastContent = lastAssistantMsg?.content || ''

  useEffect(() => {
    if (step !== 'plan' || isStreaming || !lastContent) return

    // If we're refining a tool, parse the refined version
    if (refiningTool && plan) {
      const refined = parseRefinedTool(lastContent)
      if (refined) {
        const newTools = plan.tools.map(t =>
          t.name === refiningTool ? { ...refined, enabled: t.enabled } : t
        )
        setPlan({ ...plan, tools: newTools })
        setToolStatus(prev => ({ ...prev, [refiningTool]: "pending" }))
        setRefiningTool(null)
      }
      return
    }

    // If we're adding new tools to an existing plan
    if (addingTools && plan) {
      const jsonMatch = lastContent.match(/```(?:json)?\s*\n([\s\S]*?)```/)
      if (jsonMatch) {
        try {
          const raw = JSON.parse(jsonMatch[1])
          const newToolsArr: VisualToolPlan[] = (Array.isArray(raw) ? raw : [raw]).map((t: Record<string, unknown>) => ({
            name: t.name as string,
            description: t.description as string,
            toolClass: t.toolClass as string,
            params: (t.params || {}) as VisualToolPlan["params"],
            inputSpec: t.inputSpec as Spec | undefined,
            outputSpec: t.outputSpec as Spec | undefined,
            sampleOutput: t.sampleOutput,
            enabled: true,
          }))
          if (newToolsArr.length > 0) {
            setPlan({ ...plan, tools: [...plan.tools, ...newToolsArr] })
            setToolStatus(prev => {
              const next = { ...prev }
              for (const t of newToolsArr) next[t.name] = "pending"
              return next
            })
          }
        } catch { /* skip */ }
      }
      setAddingTools(false)
      return
    }

    // Otherwise, parse initial plan
    if (!plan) {
      const parsed = parsePlan(lastContent)
      if (parsed) {
        setPlan(parsed)
        const status: Record<string, ToolPlanStatus> = {}
        for (const t of parsed.tools) {
          status[t.name] = "pending"
        }
        setToolStatus(status)
      } else {
        setPlanError("AI didn't return a valid plan. Try again with a clearer description.")
      }
    }
  }, [step, isStreaming, lastContent, plan, refiningTool, addingTools, parsePlan, parseRefinedTool])

  // Watch for generated tools → transition to workshop step (not straight to build)
  useEffect(() => {
    if (step !== 'generate' || isStreaming || generatedTools.length > 0 || !lastContent) return
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
      // Go to workshop step — NOT straight to build
      setWorkshopToolName(tools[0].name)
      setStep('workshop')
    }
  }, [step, isStreaming, lastContent, generatedTools.length])

  // Approve a tool
  const handleApproveTool = (toolName: string) => {
    setToolStatus(prev => ({ ...prev, [toolName]: "approved" }))
  }

  // Reject a tool
  const handleRejectTool = (toolName: string) => {
    setToolStatus(prev => ({ ...prev, [toolName]: "rejected" }))
  }

  // Restore a rejected or approved tool back to pending
  const handleRestoreTool = (toolName: string) => {
    setToolStatus(prev => ({ ...prev, [toolName]: "pending" }))
  }

  // Request edit for a tool via AI
  const handleRequestEdit = (toolName: string, feedback: string) => {
    if (!plan) return
    const tool = plan.tools.find(t => t.name === toolName)
    if (!tool) return

    setRefiningTool(toolName)
    setToolStatus(prev => ({ ...prev, [toolName]: "editing" }))

    const toolJson = JSON.stringify({
      name: tool.name,
      description: tool.description,
      toolClass: tool.toolClass,
      params: tool.params,
      inputSpec: tool.inputSpec,
      outputSpec: tool.outputSpec,
      sampleOutput: tool.sampleOutput,
    }, null, 2)

    const prompt = REFINE_TOOL_PROMPT + toolJson + "\n\nUser's requested change:\n" + feedback
    send(prompt, `Refining ${toolName}: ${feedback}`)
  }

  // Request new tools to be added
  const handleAddTools = (request: string) => {
    if (!plan || !request.trim()) return
    setAddingTools(true)
    const existingNames = plan.tools.map(t => `- ${t.name}: ${t.description}`).join('\n')
    const prompt = ADD_TOOLS_PROMPT + existingNames + "\n\nUser's request:\n" + request.trim()
    send(prompt, `Adding tools: ${request.trim()}`)
    setAddToolsInput("")
  }

  // Tools that are not rejected count as "active"
  const activeTools = plan?.tools.filter(t => toolStatus[t.name] !== "rejected") || []

  // All active tools must be approved to proceed
  const allApproved = activeTools.length > 0 && activeTools.every(t => toolStatus[t.name] === "approved")

  // Step 2 → 3: Generate tool implementations (only non-rejected tools)
  const handleGenerate = useCallback(() => {
    if (!plan) return
    clear()
    const toolsToGenerate = plan.tools.filter(t => toolStatus[t.name] !== "rejected")
    const toolsDesc = toolsToGenerate.map(t =>
      `- ${t.name}: ${t.description} (class: ${t.toolClass}, params: ${JSON.stringify(t.params)})`
    ).join('\n')
    const prompt = GENERATE_PROMPT + toolsDesc
    send(prompt, `Generating ${toolsToGenerate.length} tools...`)
    setStep('generate')
  }, [plan, toolStatus, send, clear])

  // Workshop: finalize and build the app
  const handleFinalize = useCallback(() => {
    buildApp()
  }, [buildApp])

  // Reset wizard
  const handleReset = () => {
    clear()
    setStep('describe')
    setIdea("")
    setPlan(null)
    setPlanError("")
    setGeneratedTools([])
    setCreatedAppId(null)
    setBuildProgress("")
    setBuildDone(false)
    setToolStatus({})
    setRefiningTool(null)
    setAddingTools(false)
    setAddToolsInput("")
    setRegenFeedback("")
    setWorkshopToolName(null)
    setTestedTools(new Set())
  }

  // Get the visual plan spec for a generated tool (matched by name)
  const getSpecsForTool = (toolName: string) => {
    const planTool = plan?.tools.find(t => t.name === toolName)
    return {
      inputSpec: planTool?.inputSpec,
      outputSpec: planTool?.outputSpec,
    }
  }

  return (
    <div className="max-w-3xl mx-auto">
      {/* Progress indicator */}
      <div className="flex items-center gap-2 mb-6 text-xs text-muted-foreground">
        {STEPS.map((s, i) => (
          <div key={s} className="flex items-center gap-2">
            {i > 0 && <ChevronRight size={12} />}
            <span className={step === s ? 'text-foreground font-medium' : STEPS.indexOf(s) < STEPS.indexOf(step) ? 'text-foreground' : ''}>
              {STEP_LABELS[s]}
            </span>
          </div>
        ))}
      </div>

      {/* Step 1: Describe */}
      {step === 'describe' && (
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Sparkles size={18} /> What do you want to build?
            </CardTitle>
            <CardDescription>
              Describe your app idea in plain English. The AI will create tools with live UI previews for you to review.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <Textarea
              value={idea}
              onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) => setIdea(e.target.value)}
              placeholder="Example: I want a currency converter that gets live exchange rates and supports multiple currencies"
              rows={5}
              className="text-sm"
              autoFocus
            />
            <div className="flex flex-wrap gap-2">
              <span className="text-xs text-muted-foreground">Try:</span>
              {[
                "A weather dashboard that checks conditions for multiple cities",
                "A URL health checker that monitors uptime of websites",
                "A currency converter with live exchange rates",
              ].map(s => (
                <Badge key={s} variant="outline" className="cursor-pointer text-xs" onClick={() => setIdea(s)}>
                  {s}
                </Badge>
              ))}
            </div>
            <div className="flex justify-end">
              <Button onClick={handleGeneratePlan} disabled={!idea.trim() || isStreaming}>
                {isStreaming ? <Loader2 size={14} className="mr-1 animate-spin" /> : <Sparkles size={14} className="mr-1" />}
                Generate Plan
              </Button>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Step 2: Visual Plan */}
      {step === 'plan' && (
        <Card>
          <CardHeader>
            <CardTitle>Visual Plan</CardTitle>
            <CardDescription>
              {isStreaming && refiningTool
                ? `Refining ${refiningTool}...`
                : isStreaming && addingTools
                  ? "AI is adding new tools to your plan..."
                  : isStreaming
                    ? "AI is creating your app plan with live previews..."
                    : plan
                      ? "Approve, reject, or tweak each tool. Add more if you need them."
                      : "Waiting for AI..."
              }
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            {isStreaming && !plan && (
              <StreamingFeedback content={lastContent} label="Generating visual plan..." />
            )}

            {planError && (
              <div className="text-sm text-destructive bg-destructive/10 rounded-none p-3">
                {planError}
                <Button variant="ghost" size="sm" className="ml-2" onClick={handleReset}>Try again</Button>
              </div>
            )}

            {plan && (
              <>
                {/* App metadata */}
                <div className="grid grid-cols-2 gap-3">
                  <div className="space-y-1">
                    <Label className="text-xs">App Name</Label>
                    <Input
                      value={plan.displayName}
                      onChange={(e: React.ChangeEvent<HTMLInputElement>) => setPlan({ ...plan, displayName: e.target.value })}
                      className="text-sm"
                    />
                  </div>
                  <div className="space-y-1">
                    <Label className="text-xs">Category</Label>
                    <Select value={plan.category} onValueChange={(v) => setPlan({ ...plan, category: v })}>
                      <SelectTrigger className="text-sm"><SelectValue /></SelectTrigger>
                      <SelectContent>
                        {categories.map(c => <SelectItem key={c} value={c}>{categoryLabel(c)}</SelectItem>)}
                      </SelectContent>
                    </Select>
                  </div>
                </div>
                <div className="space-y-1">
                  <Label className="text-xs">Description</Label>
                  <Textarea
                    value={plan.description}
                    onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) => setPlan({ ...plan, description: e.target.value })}
                    rows={2}
                    className="text-sm"
                  />
                </div>

                <Separator />

                {/* Visual tool cards */}
                <div>
                  <div className="flex items-center justify-between mb-3">
                    <Label className="text-xs">
                      Tools — {activeTools.filter(t => toolStatus[t.name] === "approved").length}/{activeTools.length} approved
                      {allApproved && activeTools.length > 0 && (
                        <span className="text-emerald-500 ml-2">Ready to generate</span>
                      )}
                    </Label>
                  </div>
                  <div className="space-y-3">
                    {plan.tools.map((tool) => (
                      <div key={tool.name}>
                        {isStreaming && refiningTool === tool.name ? (
                          <div className="border border-border rounded-none p-4">
                            <StreamingFeedback content={lastContent} label={`Refining ${tool.name}...`} />
                          </div>
                        ) : (
                          <VisualPlanCard
                            tool={tool}
                            status={toolStatus[tool.name] || "pending"}
                            onApprove={() => handleApproveTool(tool.name)}
                            onReject={() => handleRejectTool(tool.name)}
                            onRestore={() => handleRestoreTool(tool.name)}
                            onRequestEdit={(feedback) => handleRequestEdit(tool.name, feedback)}
                          />
                        )}
                      </div>
                    ))}
                  </div>

                  {/* Add more tools */}
                  {isStreaming && addingTools ? (
                    <div className="mt-4 border border-border rounded-none p-4">
                      <StreamingFeedback content={lastContent} label="Adding new tools..." />
                    </div>
                  ) : (
                    <div className="mt-4 border border-dashed border-border rounded-none p-3 space-y-2">
                      <Label className="text-xs text-muted-foreground">Want more tools? Describe what you need:</Label>
                      <div className="flex gap-2">
                        <Textarea
                          value={addToolsInput}
                          onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) => setAddToolsInput(e.target.value)}
                          placeholder='e.g. "add a tool that checks DNS records" or "I also need a response time history tracker"'
                          rows={1}
                          className="text-xs rounded-none flex-1"
                        />
                        <Button
                          size="sm"
                          variant="outline"
                          className="rounded-none text-xs shrink-0"
                          onClick={() => handleAddTools(addToolsInput)}
                          disabled={!addToolsInput.trim() || isStreaming}
                        >
                          <Sparkles size={12} className="mr-1" /> Add
                        </Button>
                      </div>
                    </div>
                  )}
                </div>

                {plan.prompts.length > 0 && (
                  <div>
                    <Label className="text-xs mb-2 block">Prompts ({plan.prompts.filter(p => p.enabled).length}/{plan.prompts.length})</Label>
                    <div className="space-y-2">
                      {plan.prompts.map((prompt, i) => (
                        <div key={i} className="flex items-start gap-3 p-2 rounded-none bg-muted/50">
                          <button
                            onClick={() => {
                              const prompts = [...plan.prompts]
                              prompts[i] = { ...prompts[i], enabled: !prompts[i].enabled }
                              setPlan({ ...plan, prompts })
                            }}
                            className="mt-0.5 shrink-0"
                          >
                            {prompt.enabled
                              ? <Check size={16} className="text-foreground" />
                              : <X size={16} className="text-muted-foreground" />
                            }
                          </button>
                          <div className={prompt.enabled ? "" : "opacity-40"}>
                            <div className="flex items-center gap-2">
                              <MessageSquare size={12} className="text-muted-foreground" />
                              <code className="text-sm">{prompt.name}</code>
                            </div>
                            <p className="text-xs text-muted-foreground mt-0.5">{prompt.description}</p>
                          </div>
                        </div>
                      ))}
                    </div>
                  </div>
                )}

                <Separator />

                {/* Regenerate plan — keep context, get a fresh plan */}
                <div className="border border-dashed border-border rounded-none p-3 space-y-2">
                  <Label className="text-xs text-muted-foreground">Not happy with the plan? Regenerate with feedback:</Label>
                  <div className="flex gap-2">
                    <Textarea
                      value={regenFeedback}
                      onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) => setRegenFeedback(e.target.value)}
                      placeholder='e.g. "fewer tools, focus on just the core converter" or "make it more like a monitoring dashboard"'
                      rows={1}
                      className="text-xs rounded-none flex-1"
                    />
                    <Button
                      size="sm"
                      variant="outline"
                      className="rounded-none text-xs shrink-0"
                      onClick={() => handleRegeneratePlan(regenFeedback)}
                      disabled={isStreaming}
                    >
                      <Sparkles size={12} className="mr-1" /> Regenerate
                    </Button>
                  </div>
                </div>

                <div className="flex justify-between">
                  <Button variant="ghost" onClick={handleReset}>
                    <ArrowLeft size={14} className="mr-1" /> Start Over
                  </Button>
                  <Button
                    onClick={handleGenerate}
                    disabled={!allApproved || activeTools.length === 0 || isStreaming}
                  >
                    <Sparkles size={14} className="mr-1" />
                    {allApproved ? `Generate ${activeTools.length} Tools` : `Approve all tools first (${activeTools.filter(t => toolStatus[t.name] === "approved").length}/${activeTools.length})`}
                  </Button>
                </div>
              </>
            )}
          </CardContent>
        </Card>
      )}

      {/* Step 3: Generating (show streaming AI output) */}
      {step === 'generate' && (
        <Card>
          <CardHeader>
            <CardTitle>Generating Code</CardTitle>
            <CardDescription>
              AI is writing the JavaScript implementations for your {plan?.tools.filter(t => t.enabled).length || 0} tools...
            </CardDescription>
          </CardHeader>
          <CardContent>
            <StreamingFeedback content={lastContent} label="Writing tool implementations..." />
          </CardContent>
        </Card>
      )}

      {/* Step 4: Workshop — test each tool with real data */}
      {step === 'workshop' && plan && (
        <div className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <FlaskConical size={18} /> Workshop — Test with Real Data
              </CardTitle>
              <CardDescription>
                Your tools have been generated. Test each one with real inputs to make sure they work.
                The forms below use the UI you approved in the Visual Plan.
              </CardDescription>
            </CardHeader>
          </Card>

          {/* Tool tabs */}
          <div className="flex flex-wrap gap-1 border-b border-border pb-2">
            {generatedTools.map((t) => {
              const tested = testedTools.has(t.name)
              const isActive = workshopToolName === t.name
              return (
                <button
                  key={t.name}
                  onClick={() => setWorkshopToolName(t.name)}
                  className={`flex items-center gap-1.5 px-3 py-1.5 text-xs font-mono rounded-none transition-colors ${
                    isActive
                      ? "bg-accent text-foreground border border-border"
                      : "text-muted-foreground hover:bg-accent/50 hover:text-foreground"
                  }`}
                >
                  {tested ? (
                    <CheckCircle size={12} className="text-emerald-400 shrink-0" />
                  ) : (
                    <Circle size={12} className="text-muted-foreground/40 shrink-0" />
                  )}
                  {t.name}
                </button>
              )
            })}
          </div>

          {/* Active tool workbench */}
          {generatedTools.map((tool) => {
            if (tool.name !== workshopToolName) return null
            const specs = getSpecsForTool(tool.name)
            return (
              <WizardToolWorkbench
                key={tool.name}
                tool={tool}
                inputSpec={specs.inputSpec}
                outputSpec={specs.outputSpec}
                onTested={() => setTestedTools(prev => new Set(prev).add(tool.name))}
              />
            )
          })}

          {/* Finalize */}
          <Separator />
          <div className="flex justify-between items-center">
            <div className="text-xs text-muted-foreground">
              {testedTools.size}/{generatedTools.length} tools tested
            </div>
            <div className="flex gap-2">
              <Button variant="ghost" onClick={() => setStep('plan')}>
                <ArrowLeft size={14} className="mr-1" /> Back to Plan
              </Button>
              <Button onClick={handleFinalize} disabled={buildProgress !== ""}>
                {buildProgress ? (
                  <><Loader2 size={14} className="mr-1 animate-spin" /> {buildProgress}</>
                ) : (
                  <><Check size={14} className="mr-1" /> Create App</>
                )}
              </Button>
            </div>
          </div>
        </div>
      )}

      {/* Step 5: Done */}
      {step === 'done' && (
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Check size={18} className="text-green-500" /> App Created!
            </CardTitle>
            <CardDescription>
              Your app "{plan?.displayName}" has been created with {generatedTools.length} tools
              {plan?.prompts.filter(p => p.enabled).length ? ` and ${plan.prompts.filter(p => p.enabled).length} prompts` : ''}.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              {generatedTools.map(t => (
                <div key={t.name} className="flex items-center gap-2 text-sm">
                  <Check size={14} className="text-green-500" />
                  <Wrench size={12} className="text-muted-foreground" />
                  <code>{t.name}</code>
                  <Badge variant="outline" className="text-[10px]">{t.toolClass}</Badge>
                </div>
              ))}
              {plan?.prompts.filter(p => p.enabled).map(p => (
                <div key={p.name} className="flex items-center gap-2 text-sm">
                  <Check size={14} className="text-green-500" />
                  <MessageSquare size={12} className="text-muted-foreground" />
                  <code>{p.name}</code>
                </div>
              ))}
            </div>

            <Separator />

            <div className="flex gap-2">
              <Button onClick={() => createdAppId && navigate(`/my-apps/${createdAppId}/workshop`)}>
                <FlaskConical size={14} className="mr-1" /> Open in Workshop
              </Button>
              <Button variant="outline" onClick={() => createdAppId && navigate(`/my-apps/${createdAppId}/edit`)}>
                Open in Editor
              </Button>
              <Button variant="outline" onClick={handleReset}>
                Create Another
              </Button>
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  )
}

// ---------------------------------------------------------------------------
// Embedded tool workbench for the wizard's workshop step.
// Uses the approved inputSpec/outputSpec from the visual plan.
// ---------------------------------------------------------------------------

interface WizardToolWorkbenchProps {
  tool: StoreTool
  inputSpec?: Spec
  outputSpec?: Spec
  onTested: () => void
}

function WizardToolWorkbench({ tool, inputSpec, outputSpec, onTested }: WizardToolWorkbenchProps) {
  const [paramValues, setParamValues] = useState<Record<string, unknown>>({})
  const [timeout, setTimeout_] = useState("30s")
  const [result, setResult] = useState<TestToolResponse | null>(null)
  const [showRaw, setShowRaw] = useState(false)
  const [showScript, setShowScript] = useState(false)
  const [copied, setCopied] = useState(false)

  const testTool = useTestTool()

  const handleRun = () => {
    testTool.mutate(
      {
        script: tool.script,
        params: paramValues,
        allowedHosts: ["*"],
        timeout,
      },
      {
        onSuccess: (data) => {
          setResult(data)
          onTested()
        },
      },
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

  return (
    <Card>
      <CardHeader className="pb-3">
        <div className="flex items-center gap-2">
          <Wrench size={14} className="text-muted-foreground" />
          <code className="text-sm font-semibold">{tool.name}</code>
          <Badge variant="outline" className="text-[10px]">{tool.toolClass}</Badge>
        </div>
        {tool.description && (
          <p className="text-xs text-muted-foreground">{tool.description}</p>
        )}
      </CardHeader>
      <CardContent className="space-y-4">
        {/* Input form — uses approved inputSpec or falls back to SchemaForm */}
        <div className="space-y-1.5">
          <Label className="text-xs font-mono font-medium text-muted-foreground uppercase tracking-wider">Input</Label>
          {inputSpec ? (
            <div className="border border-border rounded-none p-3 bg-background/60">
              <SpecRenderer spec={inputSpec} />
            </div>
          ) : (
            <SchemaForm params={tool.params} values={paramValues} onChange={setParamValues} />
          )}
        </div>

        {/* Run */}
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

        {/* Mutation error */}
        {testTool.isError && (
          <div className="rounded-none border border-destructive/30 bg-destructive/10 p-3 text-xs text-destructive font-mono">
            {(testTool.error as Error).message}
          </div>
        )}

        {/* Result */}
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

            <HttpTrace entries={result.http_log || []} />
          </div>
        )}

        {/* Script — collapsed */}
        <div className="border border-border rounded-none">
          <button
            onClick={() => setShowScript(!showScript)}
            className="w-full flex items-center gap-2 px-3 py-2 text-left hover:bg-accent/50 transition-colors"
          >
            {showScript ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
            <span className="text-xs font-mono font-medium text-muted-foreground uppercase tracking-wider">
              Script
            </span>
            <span className="text-[11px] font-mono text-muted-foreground ml-auto">
              {tool.script.split('\n').length} lines
            </span>
          </button>
          {showScript && (
            <pre className="px-3 pb-3 text-xs font-mono text-muted-foreground whitespace-pre-wrap overflow-auto max-h-60">
              {tool.script}
            </pre>
          )}
        </div>
      </CardContent>
    </Card>
  )
}

/** Renders tool output using json-render with the approved outputSpec, or auto-maps. */
function RenderedOutput({ data, outputSpec }: { data: unknown; outputSpec?: Spec }) {
  const spec = outputSpec ?? outputToSpec(data)
  return (
    <div className="rounded-none border border-border p-3 overflow-auto max-h-80">
      <SpecRenderer spec={spec} />
    </div>
  )
}

/** Shows live streaming text from the AI WebSocket, with a spinner and label. */
function StreamingFeedback({ content, label }: { content: string; label: string }) {
  // Extract a readable preview: strip markdown fences, show last meaningful lines
  const preview = content
    ? content
        .replace(/```[\s\S]*?```/g, '[code block]')  // collapse code blocks
        .split('\n')
        .filter(l => l.trim())
        .slice(-6)  // last 6 non-empty lines
        .join('\n')
    : ''

  return (
    <div className="space-y-3">
      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        <Loader2 size={14} className="animate-spin shrink-0" />
        <span>{label}</span>
      </div>
      {preview && (
        <pre className="text-xs font-mono text-muted-foreground bg-muted/50 border border-border rounded-none p-3 overflow-auto max-h-40 whitespace-pre-wrap">
          {preview}
          <span className="inline-block w-1.5 h-3.5 bg-foreground/60 ml-0.5 animate-pulse" />
        </pre>
      )}
    </div>
  )
}
