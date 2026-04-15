import { useState, useCallback, useEffect } from "react"
import { useNavigate } from "react-router-dom"
import { Sparkles, Loader2, Check, X, Wrench, MessageSquare, ChevronRight, ArrowLeft } from "lucide-react"
import { useAgentChat } from "@/hooks/use-agent-chat"
import { useCreateApp, useAddTool, useAddPrompt, useUpdateApp } from "@/hooks/use-my-apps"
import { Button } from "@/components/ui/button"
import { Textarea } from "@/components/ui/textarea"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Separator } from "@/components/ui/separator"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { categoryLabel } from "@/lib/utils"
import type { StoreTool } from "@/lib/types"

const categories = [
  "iot-devices", "analytics", "devops", "marketing",
  "design", "utilities", "integrations", "automation",
]

// Step 1 prompt: ask AI to generate a structured plan
const PLAN_PROMPT = `You are an AI app builder for NubeIO. The user will describe an app they want. Generate a structured plan as a single JSON code block.

IMPORTANT: Respond with ONLY a \`\`\`json code block, no other text before or after it.

The JSON must have this exact structure:
\`\`\`json
{
  "name": "app-slug-name",
  "displayName": "Human Readable Name",
  "description": "A clear description of what this app does (at least 20 characters)",
  "category": "utilities",
  "tools": [
    {
      "name": "tool_name",
      "description": "What this tool does",
      "toolClass": "read-only",
      "params": {
        "param_name": { "type": "string", "required": true, "description": "What this param is" }
      }
    }
  ],
  "prompts": [
    {
      "name": "prompt_name",
      "description": "What this prompt does",
      "arguments": [
        { "name": "arg_name", "description": "What this arg is", "required": true }
      ]
    }
  ]
}
\`\`\`

Categories: iot-devices, analytics, devops, marketing, design, utilities, integrations, automation.
Tool classes: read-only, read-write, destructive.
Generate 2-5 tools and 0-2 prompts. Keep names lowercase with underscores.
The "name" field should be a lowercase slug with hyphens.

User's app idea:
`

// Step 3 prompt: ask AI to generate the actual tool scripts
const GENERATE_PROMPT = `You are an AI app builder for NubeIO. Generate the JavaScript implementation for each tool below.

IMPORTANT: Respond with ONLY JSON code blocks, one per tool, using \`\`\`json:tool markers. No other text.

For each tool, output:
\`\`\`json:tool
{
  "name": "tool_name",
  "description": "...",
  "toolClass": "...",
  "params": { ... },
  "script": "function handle(params) { ... }"
}
\`\`\`

The JS runtime APIs:
- http.get(url), http.post(url, body), http.put(url, body), http.delete(url) — returns {status, body, headers}
- secrets.get(key), config.get(key) — read user settings
- log.info(msg), log.warn(msg), log.error(msg)

Keep scripts concise and practical. Each must define a handle(params) function.

Here are the tools to implement:
`

interface ToolPlan {
  name: string
  description: string
  toolClass: string
  params: Record<string, { type: string; required: boolean; description: string }>
  enabled: boolean
}

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
  tools: ToolPlan[]
  prompts: PromptPlan[]
}

type Step = 'describe' | 'plan' | 'generate' | 'done'

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

  // Step 1 → 2: Generate plan from description
  const handleGeneratePlan = useCallback(() => {
    if (!idea.trim()) return
    setPlanError("")
    const prompt = PLAN_PROMPT + idea.trim()
    send(prompt, idea.trim())
    setStep('plan')
  }, [idea, send])

  // Parse the plan from AI response
  const parsePlan = useCallback((content: string): AppPlan | null => {
    const jsonMatch = content.match(/```(?:json)?\s*\n([\s\S]*?)```/)
    if (!jsonMatch) return null
    try {
      const raw = JSON.parse(jsonMatch[1])
      return {
        name: raw.name || "my-app",
        displayName: raw.displayName || raw.name || "My App",
        description: raw.description || "",
        category: raw.category || "utilities",
        tools: (raw.tools || []).map((t: Record<string, unknown>) => ({ ...t, enabled: true })),
        prompts: (raw.prompts || []).map((p: Record<string, unknown>) => ({ ...p, enabled: true })),
      }
    } catch {
      return null
    }
  }, [])

  // Step 3 → 4: Create app and add everything
  const buildApp = useCallback(async (tools: StoreTool[]) => {
    if (!plan) return
    try {
      setBuildProgress("Creating app...")
      const app = await createAppMutation.mutateAsync({
        name: plan.name,
        displayName: plan.displayName,
        description: plan.description,
        category: plan.category,
      })
      setCreatedAppId(app.id)

      for (const tool of tools) {
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
  }, [plan, createAppMutation, addToolMutation, addPromptMutation, updateAppMutation])

  // Watch for plan completion
  const lastAssistantMsg = messages.filter(m => m.role === 'assistant').pop()
  const lastContent = lastAssistantMsg?.content || ''

  useEffect(() => {
    if (step !== 'plan' || isStreaming || plan || !lastContent) return
    const parsed = parsePlan(lastContent)
    if (parsed) {
      setPlan(parsed)
    } else {
      setPlanError("AI didn't return a valid plan. Try again with a clearer description.")
    }
  }, [step, isStreaming, lastContent, plan, parsePlan])

  // Watch for generated tools
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
      buildApp(tools)
    }
  }, [step, isStreaming, lastContent, generatedTools.length, buildApp])

  // Step 2 → 3: Generate tool implementations
  const handleGenerate = useCallback(() => {
    if (!plan) return
    clear()
    const enabledTools = plan.tools.filter(t => t.enabled)
    const toolsDesc = enabledTools.map(t =>
      `- ${t.name}: ${t.description} (class: ${t.toolClass}, params: ${JSON.stringify(t.params)})`
    ).join('\n')
    const prompt = GENERATE_PROMPT + toolsDesc
    send(prompt, `Generating ${enabledTools.length} tools...`)
    setStep('generate')
  }, [plan, send, clear])

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
  }

  return (
    <div className="max-w-2xl mx-auto">
      {/* Progress indicator */}
      <div className="flex items-center gap-2 mb-6 text-xs text-muted-foreground">
        {(['describe', 'plan', 'generate', 'done'] as Step[]).map((s, i) => (
          <div key={s} className="flex items-center gap-2">
            {i > 0 && <ChevronRight size={12} />}
            <span className={step === s ? 'text-foreground font-medium' : s < step ? 'text-foreground' : ''}>
              {s === 'describe' ? '1. Describe' : s === 'plan' ? '2. Review Plan' : s === 'generate' ? '3. Generate' : '4. Done'}
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
              Describe your app idea in plain English. The AI will create the tools and prompts for you.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <Textarea
              value={idea}
              onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) => setIdea(e.target.value)}
              placeholder="Example: I want a shopping list app. Users should be able to add items with quantities, remove items, check items off as bought, view the full list, and clear all checked items. Also include a prompt that generates a shopping list from a meal plan."
              rows={5}
              className="text-sm"
              autoFocus
            />
            <div className="flex flex-wrap gap-2">
              <span className="text-xs text-muted-foreground">Try:</span>
              {[
                "A weather dashboard that checks conditions for multiple cities",
                "A URL health checker that monitors uptime of websites",
                "A notes app with create, search, tag, and archive",
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

      {/* Step 2: Review Plan */}
      {step === 'plan' && (
        <Card>
          <CardHeader>
            <CardTitle>Review Your App</CardTitle>
            <CardDescription>
              {isStreaming ? "AI is creating your app plan..." : plan ? "Review and customize before generating." : "Waiting for AI..."}
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            {isStreaming && (
              <div className="flex items-center gap-2 text-sm text-muted-foreground py-8 justify-center">
                <Loader2 size={16} className="animate-spin" /> Thinking...
              </div>
            )}

            {planError && (
              <div className="text-sm text-destructive bg-destructive/10 rounded-none p-3">
                {planError}
                <Button variant="ghost" size="sm" className="ml-2" onClick={handleReset}>Try again</Button>
              </div>
            )}

            {plan && !isStreaming && (
              <>
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

                <div>
                  <Label className="text-xs mb-2 block">Tools ({plan.tools.filter(t => t.enabled).length}/{plan.tools.length})</Label>
                  <div className="space-y-2">
                    {plan.tools.map((tool, i) => (
                      <div key={i} className="flex items-start gap-3 p-2 rounded-none bg-muted/50">
                        <button
                          onClick={() => {
                            const tools = [...plan.tools]
                            tools[i] = { ...tools[i], enabled: !tools[i].enabled }
                            setPlan({ ...plan, tools })
                          }}
                          className="mt-0.5 shrink-0"
                        >
                          {tool.enabled
                            ? <Check size={16} className="text-foreground" />
                            : <X size={16} className="text-muted-foreground" />
                          }
                        </button>
                        <div className={tool.enabled ? "" : "opacity-40"}>
                          <div className="flex items-center gap-2">
                            <Wrench size={12} className="text-muted-foreground" />
                            <code className="text-sm">{tool.name}</code>
                            <Badge variant="outline" className="text-[10px]">{tool.toolClass}</Badge>
                          </div>
                          <p className="text-xs text-muted-foreground mt-0.5">{tool.description}</p>
                        </div>
                      </div>
                    ))}
                  </div>
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

                <div className="flex justify-between">
                  <Button variant="ghost" onClick={handleReset}>
                    <ArrowLeft size={14} className="mr-1" /> Start Over
                  </Button>
                  <Button onClick={handleGenerate} disabled={plan.tools.filter(t => t.enabled).length === 0}>
                    <Sparkles size={14} className="mr-1" /> Generate App
                  </Button>
                </div>
              </>
            )}
          </CardContent>
        </Card>
      )}

      {/* Step 3: Generating */}
      {step === 'generate' && !buildDone && (
        <Card>
          <CardHeader>
            <CardTitle>Building Your App</CardTitle>
            <CardDescription>
              {isStreaming ? "AI is writing the code..." : buildProgress ? buildProgress : "Creating app..."}
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className="flex flex-col items-center gap-3 py-8">
              <Loader2 size={32} className="animate-spin text-muted-foreground" />
              <p className="text-sm text-muted-foreground">
                {isStreaming
                  ? `Generating ${plan?.tools.filter(t => t.enabled).length || 0} tools...`
                  : buildProgress || "Processing..."
                }
              </p>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Step 4: Done */}
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
              <Button onClick={() => createdAppId && navigate(`/my-apps/${createdAppId}/edit`)}>
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
