import { Loader2 } from "lucide-react"

/** Shows live streaming text from the AI WebSocket, with a spinner and label. */
export function StreamingFeedback({ content, label }: { content: string; label: string }) {
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
