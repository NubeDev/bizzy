/**
 * Hooks available to AI-generated live preview components.
 * These let UI components call backend tools and prompts.
 */
import { useState, useCallback, useRef, useMemo, useEffect, createContext, useContext } from "react"

// --- Tool Runner Context ---
// Allows the builder to inject a custom tool runner that uses test-tool endpoint
// while the AI Workshop version uses the agent endpoint.

type ToolRunFn = (toolName: string, params?: Record<string, unknown>) => Promise<unknown>

const ToolRunContext = createContext<ToolRunFn | null>(null)

// --- Prompt Session Context ---
// Stores the AI session ID at the provider level so it survives component
// recompilation AND page reloads (via localStorage keyed by app name).

interface PromptSessionCtx {
  ref: React.RefObject<string | null>
  storageKey: string | null
  appName: string | null
}

const PromptSessionContext = createContext<PromptSessionCtx | null>(null)

export function ToolRunProvider({ runFn, onResult, appName, children }: {
  runFn: ToolRunFn
  onResult?: (toolName: string, data: unknown) => void
  /** App name — used to persist session ID in localStorage across page reloads */
  appName?: string
  children: React.ReactNode
}) {
  // Store onResult in a ref so it doesn't affect context identity
  const onResultRef = useRef(onResult)
  onResultRef.current = onResult

  // Prompt session ref lives here — above the compiled component boundary —
  // so it persists across component remounts/recompilations.
  const storageKey = appName ? `bizzy-prompt-session-${appName}` : null
  const promptSessionRef = useRef<string | null>(null)

  // Load session on mount: try localStorage first (instant), then fetch from API (authoritative).
  const fetched = useRef(false)
  if (promptSessionRef.current === null && storageKey) {
    const stored = localStorage.getItem(storageKey)
    if (stored) promptSessionRef.current = stored
  }
  useEffect(() => {
    if (!appName || fetched.current) return
    fetched.current = true
    fetch(`/api/agents/sessions/app/${encodeURIComponent(appName)}`)
      .then(r => r.ok ? r.json() : null)
      .then(data => {
        if (data?.session_id) {
          promptSessionRef.current = data.session_id
          if (storageKey) {
            try { localStorage.setItem(storageKey, data.session_id) } catch { /* ignore */ }
          }
        }
      })
      .catch(() => {})
  }, [appName, storageKey])

  const sessionCtx = useMemo<PromptSessionCtx>(() => ({
    ref: promptSessionRef,
    storageKey,
    appName: appName || null,
  }), [storageKey, appName])

  const wrappedFn = useCallback(async (toolName: string, params?: Record<string, unknown>) => {
    const result = await runFn(toolName, params)
    // Fire onResult as a microtask so it doesn't interfere with the caller's state update
    if (onResultRef.current) {
      Promise.resolve().then(() => onResultRef.current?.(toolName, result))
    }
    return result
  }, [runFn]) // onResult is in a ref, not a dep — context value stays stable

  return (
    <ToolRunContext.Provider value={wrappedFn}>
      <PromptSessionContext.Provider value={sessionCtx}>
        {children}
      </PromptSessionContext.Provider>
    </ToolRunContext.Provider>
  )
}

interface ToolRunnerResult {
  run: (toolName: string, params?: Record<string, unknown>) => Promise<void>
  data: unknown
  loading: boolean
  error: string | null
  reset: () => void
}

/**
 * useToolRunner — calls a backend tool by name and returns the result.
 *
 * If inside a ToolRunProvider (e.g. the builder), uses the injected run function.
 * Otherwise, calls POST /api/agents/tools/:name (saved app mode).
 */
export function useToolRunner(): ToolRunnerResult {
  const [data, setData] = useState<unknown>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const contextRunFn = useContext(ToolRunContext)

  const run = useCallback(async (toolName: string, params?: Record<string, unknown>) => {
    setLoading(true)
    setError(null)
    try {
      let result: unknown

      if (contextRunFn) {
        // Builder mode — uses injected function (test-tool endpoint)
        result = await contextRunFn(toolName, params)
      } else {
        // Saved app mode — uses agent endpoint
        const res = await fetch(`/api/agents/tools/${toolName}`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(params || {}),
        })
        if (!res.ok) {
          const body = await res.json().catch(() => ({}))
          throw new Error(body.error || `Tool call failed: ${res.statusText}`)
        }
        result = await res.json()
      }

      setData(result)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setLoading(false)
    }
  }, [contextRunFn])

  const reset = useCallback(() => {
    setData(null)
    setError(null)
    setLoading(false)
  }, [])

  return { run, data, loading, error, reset }
}

interface PromptRunnerResult {
  run: (promptOrName: string, args?: Record<string, string>) => Promise<void>
  text: string | null
  loading: boolean
  error: string | null
  reset: () => void
}

/**
 * usePromptRunner — sends a prompt to the AI and returns the response.
 * Supports multi-turn session resume: after the first call, subsequent calls
 * automatically send the session ID so the AI remembers previous context.
 *
 * Session tracking is lifted to ToolRunProvider (via PromptSessionContext)
 * so it survives component recompilation in the app builder.
 */
export function usePromptRunner(): PromptRunnerResult {
  const [text, setText] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  // Use the context if inside a ToolRunProvider (builder mode) — survives remounts
  // and page reloads (via localStorage). Falls back to a local ref for saved app mode.
  const sessionCtx = useContext(PromptSessionContext)
  const localSessionRef = useRef<string | null>(null)
  const sessionRef = sessionCtx?.ref || localSessionRef
  // Keep a ref to sessionCtx so the memoized run() callback always sees the latest storageKey,
  // even though useCallback has [] deps (otherwise storageKey is stale from first render).
  const sessionCtxRef = useRef(sessionCtx)
  sessionCtxRef.current = sessionCtx

  const run = useCallback(async (promptOrName: string, args?: Record<string, unknown>) => {
    setLoading(true)
    setError(null)
    setText(null)
    try {
      let finalPrompt = promptOrName

      // If args provided, try named prompt template first, then fall back to appending context
      if (args && Object.keys(args).length > 0) {
        let usedTemplate = false
        try {
          const res = await fetch(`/my/prompts/${promptOrName}`)
          if (res.ok) {
            const promptData = await res.json()
            if (promptData.body) {
              finalPrompt = promptData.body
              for (const [key, value] of Object.entries(args)) {
                const strVal = typeof value === "string" ? value : JSON.stringify(value)
                finalPrompt = finalPrompt.replace(new RegExp(`\\{\\{${key}\\}\\}`, 'g'), strVal)
              }
              usedTemplate = true
            }
          }
        } catch { /* not a named prompt, that's fine */ }

        if (!usedTemplate) {
          // Append args as context to the raw prompt
          const contextLines = Object.entries(args).map(([k, v]) =>
            `${k}: ${typeof v === "string" ? v : JSON.stringify(v)}`
          ).join("\n")
          finalPrompt = promptOrName + "\n\nContext:\n" + contextLines
        }
      }

      const controller = new AbortController()
      const timeoutId = setTimeout(() => controller.abort(), 60000) // 60s timeout

      const body: Record<string, string> = { prompt: finalPrompt }
      if (sessionRef.current) {
        body.session_id = sessionRef.current
      }
      const currentAppName = sessionCtxRef.current?.appName
      if (currentAppName) {
        body.app = currentAppName
      }

      const res = await fetch('/api/agents/run/sync', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
        signal: controller.signal,
      })
      clearTimeout(timeoutId)

      if (!res.ok) {
        const errBody = await res.json().catch(() => ({}))
        throw new Error(errBody.error || `Prompt failed: ${res.statusText}`)
      }
      const result = await res.json()
      // Store session ID for multi-turn resume.
      // Claude returns claude_session_id; other providers use the bizzy session_id.
      const resumeId = result.claude_session_id || result.session_id || null
      if (resumeId) {
        sessionRef.current = resumeId
        // Persist to localStorage so it survives page reloads.
        // Use the ref (not the closure-captured value) to get the latest storageKey.
        const key = sessionCtxRef.current?.storageKey
        if (key) {
          try { localStorage.setItem(key, resumeId) } catch { /* ignore */ }
        }
      }
      setText(result.text || '')
    } catch (err) {
      if (err instanceof DOMException && err.name === 'AbortError') {
        setError('Request timed out (60s). Try a shorter prompt.')
      } else {
        setError(err instanceof Error ? err.message : String(err))
      }
    } finally {
      setLoading(false)
    }
  }, [])

  const reset = useCallback(() => {
    setText(null)
    setError(null)
    setLoading(false)
    sessionRef.current = null
  }, [])

  return { run, text, loading, error, reset }
}
