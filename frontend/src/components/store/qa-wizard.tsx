import { useState, useEffect, useRef } from "react"
import { Loader2, Check, ChevronRight, RotateCcw, Copy, Wrench } from "lucide-react"
import { useQaWizard } from "@/hooks/use-qa-wizard"
import type { QaQuestion, QaExchange } from "@/hooks/use-qa-wizard"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { Label } from "@/components/ui/label"

interface QaWizardProps {
  flow: string // e.g. "weather-checker.travel_quiz_qa"
  title: string
  onClose?: () => void
}

export function QaWizard({ flow, title, onClose }: QaWizardProps) {
  const { state, start, answer, restart } = useQaWizard()
  const scrollRef = useRef<HTMLDivElement>(null)
  const [copied, setCopied] = useState(false)

  // Start on mount (once only)
  const startedRef = useRef(false)
  useEffect(() => {
    if (startedRef.current) return
    startedRef.current = true
    start(flow)
  }, [flow, start])

  // Auto-scroll on state changes
  useEffect(() => {
    scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight, behavior: "smooth" })
  }, [state.exchanges, state.currentQuestion, state.resultText, state.phase])

  const handleCopy = () => {
    navigator.clipboard.writeText(state.resultText)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <div className="flex flex-col h-full max-h-[70vh]">
      {/* Header */}
      <div className="flex items-center justify-between pb-3 border-b border-border mb-4">
        <div>
          <h3 className="font-mono text-sm font-medium uppercase tracking-wider">{title}</h3>
          {state.sessionId && (
            <span className="font-mono text-[10px] text-muted-foreground">{state.sessionId}</span>
          )}
        </div>
        <div className="flex gap-1">
          {state.phase === "done" && (
            <Button variant="ghost" size="icon" className="h-7 w-7 rounded-none" onClick={restart} title="Restart">
              <RotateCcw size={14} />
            </Button>
          )}
          {onClose && (
            <Button variant="ghost" size="sm" className="rounded-none font-mono text-xs" onClick={onClose}>
              Close
            </Button>
          )}
        </div>
      </div>

      {/* Scrollable body */}
      <div ref={scrollRef} className="flex-1 overflow-y-auto space-y-3 pr-1">
        {/* Completed exchanges */}
        {state.exchanges.map((ex, i) => (
          <AnswerBubble key={i} exchange={ex} />
        ))}

        {/* Current question */}
        {state.currentQuestion && state.phase === "questioning" && (
          <QuestionCard question={state.currentQuestion} onAnswer={answer} />
        )}

        {/* Connecting / loading */}
        {state.phase === "connecting" && state.exchanges.length > 0 && (
          <div className="flex items-center gap-2 text-sm text-muted-foreground py-2">
            <Loader2 size={14} className="animate-spin" /> Loading next question...
          </div>
        )}
        {state.phase === "connecting" && state.exchanges.length === 0 && (
          <div className="flex items-center gap-2 text-sm text-muted-foreground py-4">
            <Loader2 size={14} className="animate-spin" /> Connecting...
          </div>
        )}

        {/* Generating */}
        {state.phase === "generating" && (
          <div className="border border-border bg-muted/30 p-4 space-y-2">
            <div className="flex items-center gap-2 text-sm">
              <Loader2 size={14} className="animate-spin" />
              <span>{state.generatingMessage}</span>
            </div>
          </div>
        )}

        {/* Tool calls */}
        {state.toolCalls.length > 0 && (
          <div className="space-y-1">
            {state.toolCalls.map((name, i) => (
              <div key={i} className="flex items-center gap-2 text-xs text-muted-foreground font-mono">
                <Wrench size={10} /> {name}
              </div>
            ))}
          </div>
        )}

        {/* Streaming / Done result */}
        {state.resultText && (
          <ResultView text={state.resultText} streaming={state.phase === "streaming"} copied={copied} onCopy={handleCopy} />
        )}

        {/* Done summary */}
        {state.phase === "done" && (
          <div className="flex items-center gap-4 text-xs text-muted-foreground font-mono pt-2">
            {state.durationMs != null && <span>{(state.durationMs / 1000).toFixed(1)}s</span>}
            {state.costUsd != null && <span>${state.costUsd.toFixed(4)}</span>}
            <Button variant="outline" size="sm" className="rounded-none font-mono text-xs ml-auto" onClick={restart}>
              <RotateCcw size={12} className="mr-1" /> Run Again
            </Button>
          </div>
        )}

        {/* Error */}
        {state.phase === "error" && (
          <div className="border border-destructive/30 bg-destructive/10 p-4 space-y-2">
            <p className="text-sm text-destructive font-mono">{state.error}</p>
            <Button variant="outline" size="sm" className="rounded-none font-mono text-xs" onClick={restart}>
              <RotateCcw size={12} className="mr-1" /> Try Again
            </Button>
          </div>
        )}
      </div>
    </div>
  )
}

// --- Answer Bubble (completed Q&A pair) ---

function AnswerBubble({ exchange }: { exchange: QaExchange }) {
  const displayAnswer = Array.isArray(exchange.answer)
    ? exchange.answer.join(", ")
    : String(exchange.answer)

  // If the answer was a select option, show the label
  let label = displayAnswer
  if (exchange.question.options) {
    const opt = exchange.question.options.find(o => o.value === displayAnswer)
    if (opt) label = opt.label
  }

  return (
    <div className="flex items-start gap-2 text-sm">
      <Check size={14} className="text-primary mt-0.5 shrink-0" />
      <div className="min-w-0">
        <span className="text-muted-foreground">{exchange.question.label}</span>
        <span className="mx-2 text-muted-foreground">→</span>
        <span className="font-medium">{label}</span>
      </div>
    </div>
  )
}

// --- Question Card (active question) ---

function QuestionCard({ question, onAnswer }: { question: QaQuestion; onAnswer: (v: string | string[]) => void }) {
  // Show weather context if present
  const context = question.context as Record<string, unknown> | undefined
  const weather = context?.weather as Record<string, unknown> | undefined

  return (
    <div className="border border-primary/30 bg-primary/5 p-4 space-y-3">
      <Label className="text-sm font-medium block">{question.label}</Label>

      {weather && (
        <div className="flex gap-3 text-xs font-mono text-muted-foreground bg-muted/50 border border-border p-2">
          <span>{weather.temperature_c as number}°C</span>
          <span>{weather.conditions as string}</span>
          <span>Wind {weather.wind_kmh as number} km/h</span>
          <span>{weather.humidity_pct as number}% humidity</span>
        </div>
      )}

      {question.input === "select" && question.options ? (
        <SelectInput question={question} onAnswer={onAnswer} />
      ) : question.input === "multi_select" && question.options ? (
        <MultiSelectInput question={question} onAnswer={onAnswer} />
      ) : question.input === "textarea" ? (
        <TextareaInput question={question} onAnswer={onAnswer} />
      ) : question.input === "number" ? (
        <NumberInput question={question} onAnswer={onAnswer} />
      ) : (
        <TextInput question={question} onAnswer={onAnswer} />
      )}
    </div>
  )
}

// --- Input Types ---

function TextInput({ question, onAnswer }: { question: QaQuestion; onAnswer: (v: string) => void }) {
  const [value, setValue] = useState(question.default || "")
  const handleSubmit = () => {
    const v = value.trim()
    if (question.required && !v) return
    if (question.min_length && v.length < question.min_length) return
    onAnswer(v || question.default || "")
  }
  return (
    <div className="flex gap-2">
      <Input
        className="rounded-none bg-transparent border-border font-mono text-sm h-9 flex-1"
        placeholder={question.placeholder}
        value={value}
        onChange={(e: React.ChangeEvent<HTMLInputElement>) => setValue(e.target.value)}
        onKeyDown={(e: React.KeyboardEvent) => e.key === "Enter" && handleSubmit()}
        autoFocus
      />
      <Button size="sm" className="rounded-none font-mono text-xs h-9" onClick={handleSubmit}>
        <ChevronRight size={14} />
      </Button>
    </div>
  )
}

function TextareaInput({ question, onAnswer }: { question: QaQuestion; onAnswer: (v: string) => void }) {
  const [value, setValue] = useState(question.default || "")
  const handleSubmit = () => {
    onAnswer(value.trim() || question.default || "")
  }
  return (
    <div className="space-y-2">
      <Textarea
        className="rounded-none bg-transparent border-border font-mono text-sm min-h-[60px]"
        placeholder={question.placeholder}
        value={value}
        onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) => setValue(e.target.value)}
        autoFocus
      />
      <Button size="sm" className="rounded-none font-mono text-xs" onClick={handleSubmit}>
        {question.required ? "Submit" : "Skip / Submit"} <ChevronRight size={14} className="ml-1" />
      </Button>
    </div>
  )
}

function NumberInput({ question, onAnswer }: { question: QaQuestion; onAnswer: (v: string) => void }) {
  const [value, setValue] = useState(question.default || "")
  const handleSubmit = () => {
    if (question.required && !value.trim()) return
    onAnswer(value.trim())
  }
  return (
    <div className="flex gap-2">
      <Input
        type="number"
        className="rounded-none bg-transparent border-border font-mono text-sm h-9 flex-1"
        placeholder={question.placeholder}
        value={value}
        onChange={(e: React.ChangeEvent<HTMLInputElement>) => setValue(e.target.value)}
        onKeyDown={(e: React.KeyboardEvent) => e.key === "Enter" && handleSubmit()}
        autoFocus
      />
      <Button size="sm" className="rounded-none font-mono text-xs h-9" onClick={handleSubmit}>
        <ChevronRight size={14} />
      </Button>
    </div>
  )
}

function SelectInput({ question, onAnswer }: { question: QaQuestion; onAnswer: (v: string) => void }) {
  const [selected, setSelected] = useState<string | null>(null)
  return (
    <div className="space-y-2">
      {question.options?.map(opt => (
        <button
          key={opt.value}
          className={`w-full text-left px-3 py-2 border text-sm font-mono transition-colors ${
            selected === opt.value
              ? "border-primary bg-primary/10 text-foreground"
              : "border-border bg-transparent text-muted-foreground hover:border-foreground/30"
          }`}
          onClick={() => setSelected(opt.value)}
        >
          {opt.label}
        </button>
      ))}
      <Button
        size="sm"
        className="rounded-none font-mono text-xs"
        disabled={question.required && !selected}
        onClick={() => onAnswer(selected || "")}
      >
        Confirm <ChevronRight size={14} className="ml-1" />
      </Button>
    </div>
  )
}

function MultiSelectInput({ question, onAnswer }: { question: QaQuestion; onAnswer: (v: string) => void }) {
  const [selected, setSelected] = useState<string[]>([])
  const toggle = (val: string) => {
    setSelected(prev => prev.includes(val) ? prev.filter(v => v !== val) : [...prev, val])
  }
  return (
    <div className="space-y-2">
      <div className="flex flex-wrap gap-2">
        {question.options?.map(opt => (
          <button
            key={opt.value}
            className={`px-3 py-1.5 border text-xs font-mono transition-colors ${
              selected.includes(opt.value)
                ? "border-primary bg-primary/10 text-foreground"
                : "border-border bg-transparent text-muted-foreground hover:border-foreground/30"
            }`}
            onClick={() => toggle(opt.value)}
          >
            {opt.label}
          </button>
        ))}
      </div>
      <Button
        size="sm"
        className="rounded-none font-mono text-xs"
        onClick={() => onAnswer(selected.join(","))}
      >
        {question.required && selected.length === 0 ? "Select at least one" : "Confirm"} <ChevronRight size={14} className="ml-1" />
      </Button>
    </div>
  )
}

// --- Result View: smart rendering for structured JSON results ---

function ResultView({ text, streaming, copied, onCopy }: { text: string; streaming: boolean; copied: boolean; onCopy: () => void }) {
  const [showRaw, setShowRaw] = useState(false)

  // Try to parse as JSON for structured display
  let parsed: Record<string, unknown> | null = null
  try {
    parsed = JSON.parse(text)
  } catch {
    // Not JSON — render as plain text
  }

  // Plain text / streaming result
  if (!parsed || streaming) {
    return (
      <div className="relative border border-border bg-card p-4">
        <Button variant="ghost" size="icon" className="absolute top-2 right-2 h-6 w-6 rounded-none" onClick={onCopy}>
          {copied ? <Check size={12} /> : <Copy size={12} />}
        </Button>
        <pre className="text-sm font-mono whitespace-pre-wrap leading-relaxed pr-8">{text}</pre>
        {streaming && <span className="inline-block w-2 h-4 bg-primary animate-pulse ml-0.5" />}
      </div>
    )
  }

  // Structured JSON result
  const title = parsed.title as string | undefined
  const isCorrect = parsed.is_correct as boolean | undefined
  const explanation = parsed.explanation as string | undefined
  const yourAnswer = parsed.your_answer as string | undefined
  const correctAnswer = parsed.correct_answer as string | undefined
  const weather = parsed.weather as Record<string, unknown> | undefined
  const city = parsed.city as string | undefined
  const country = parsed.country as string | undefined
  const scenario = parsed.scenario as string | undefined

  // Generic key-value pairs for unknown structures
  const knownKeys = new Set(["type", "title", "is_correct", "explanation", "your_answer", "correct_answer", "weather", "city", "country", "scenario"])
  const extraKeys = Object.keys(parsed).filter(k => !knownKeys.has(k))

  return (
    <div className="border border-border bg-card overflow-hidden">
      {/* Title bar */}
      {title && (
        <div className={`px-4 py-3 font-mono text-sm font-medium uppercase tracking-wider ${
          isCorrect === true ? "bg-emerald-500/15 text-emerald-400" :
          isCorrect === false ? "bg-orange-500/15 text-orange-400" :
          "bg-muted/50"
        }`}>
          {title}
        </div>
      )}

      <div className="p-4 space-y-3">
        {/* Location + scenario */}
        {(city || scenario) && (
          <div className="flex items-center gap-2 text-xs text-muted-foreground font-mono">
            {city && <span>{city}{country ? `, ${country}` : ""}</span>}
            {scenario && <><span className="text-border">|</span><span>{scenario}</span></>}
          </div>
        )}

        {/* Weather context */}
        {weather && (
          <div className="flex flex-wrap gap-3 text-xs font-mono bg-muted/30 border border-border p-2.5">
            {weather.temperature_c != null && <span>{weather.temperature_c as number}°C</span>}
            {weather.feels_like_c != null && <span className="text-muted-foreground">(feels {weather.feels_like_c as number}°C)</span>}
            {weather.conditions && <span>{weather.conditions as string}</span>}
            {weather.wind_kmh != null && <span>Wind {weather.wind_kmh as number} km/h</span>}
            {weather.humidity_pct != null && <span>{weather.humidity_pct as number}%</span>}
          </div>
        )}

        {/* Your answer vs correct */}
        {yourAnswer && (
          <div className="space-y-1.5">
            <div className="flex items-start gap-2 text-sm">
              <span className="text-muted-foreground shrink-0">Your answer:</span>
              <span className={`font-medium ${isCorrect ? "text-emerald-400" : "text-orange-400"}`}>{yourAnswer}</span>
            </div>
            {correctAnswer && !isCorrect && (
              <div className="flex items-start gap-2 text-sm">
                <span className="text-muted-foreground shrink-0">Correct:</span>
                <span className="font-medium text-emerald-400">{correctAnswer}</span>
              </div>
            )}
          </div>
        )}

        {/* Explanation */}
        {explanation && (
          <p className="text-sm leading-relaxed border-l-2 border-primary/40 pl-3">{explanation}</p>
        )}

        {/* Extra fields (generic key-value for any result structure) */}
        {extraKeys.length > 0 && (
          <div className="space-y-1.5">
            {extraKeys.map(key => {
              const val = parsed![key]
              if (val == null) return null
              if (typeof val === "object") {
                return (
                  <div key={key}>
                    <span className="text-xs text-muted-foreground font-mono">{key}:</span>
                    <pre className="text-xs font-mono bg-muted/30 border border-border p-2 mt-1 overflow-auto max-h-40 whitespace-pre-wrap">
                      {JSON.stringify(val, null, 2)}
                    </pre>
                  </div>
                )
              }
              return (
                <div key={key} className="flex items-start gap-2 text-sm">
                  <span className="text-muted-foreground font-mono text-xs shrink-0">{key}:</span>
                  <span className="text-sm">{String(val)}</span>
                </div>
              )
            })}
          </div>
        )}

        {/* Raw JSON toggle */}
        <div className="pt-1 flex items-center gap-2">
          <button
            className="text-[10px] font-mono text-muted-foreground hover:text-foreground uppercase tracking-wider"
            onClick={() => setShowRaw(!showRaw)}
          >
            {showRaw ? "Hide" : "Show"} raw JSON
          </button>
          <Button variant="ghost" size="icon" className="h-5 w-5 rounded-none" onClick={onCopy}>
            {copied ? <Check size={10} /> : <Copy size={10} />}
          </Button>
        </div>
        {showRaw && (
          <pre className="text-[11px] font-mono bg-muted/30 border border-border p-3 overflow-auto max-h-60 whitespace-pre-wrap">
            {JSON.stringify(parsed, null, 2)}
          </pre>
        )}
      </div>
    </div>
  )
}
