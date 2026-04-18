import { useState, useEffect, useCallback } from "react"
import { api } from "@/lib/api"
import type { BootstrapPrompt } from "@/lib/types"

/** Cache so we only fetch once per page load. */
let cache: BootstrapPrompt[] | null = null
let inflight: Promise<BootstrapPrompt[]> | null = null

/**
 * Fetches all bootstrap prompts from the backend. Results are cached
 * in-memory so subsequent calls are instant.
 */
export function useBootstrapPrompts() {
  const [prompts, setPrompts] = useState<BootstrapPrompt[]>(cache || [])
  const [loading, setLoading] = useState(!cache)

  useEffect(() => {
    if (cache) { setPrompts(cache); setLoading(false); return }
    if (!inflight) {
      inflight = api.bootstrapPrompts().then(data => { cache = data; return data })
    }
    inflight.then(data => { setPrompts(data); setLoading(false) })
  }, [])

  /** Look up a single prompt by name. */
  const get = useCallback((name: string) => prompts.find(p => p.name === name), [prompts])

  /** Get the body text of a prompt by name (empty string if not found). */
  const body = useCallback((name: string) => get(name)?.body ?? "", [get])

  /** Compose multiple prompt bodies by name into a single string. */
  const compose = useCallback(
    (names: string[]) => names.map(n => body(n)).filter(Boolean).join("\n\n---\n\n"),
    [body],
  )

  return { prompts, loading, get, body, compose }
}
