import { shadcnComponents } from "@json-render/shadcn"
import { Renderer, JSONUIProvider } from "@json-render/react"
import type { ComponentRegistry } from "@json-render/react"
import type { Spec } from "@json-render/core"
import { createElement } from "react"

// shadcnComponents is already a Record<string, ComponentRenderer> —
// pass directly to <Renderer registry={registry} />.
export const registry: ComponentRegistry = { ...shadcnComponents }

/**
 * Convenience wrapper: renders a json-render Spec inside a JSONUIProvider
 * so that VisibilityProvider and other required contexts are available.
 * Use this instead of bare <Renderer> to avoid "useVisibility must be
 * used within a VisibilityProvider" errors.
 */
export function SpecRenderer({ spec }: { spec: Spec }) {
  return createElement(
    JSONUIProvider,
    { registry },
    createElement(Renderer, { spec, registry }),
  )
}
