import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { History, Plus, Loader2 } from 'lucide-react'
import { AgentChat } from '@/components/chat/agent-chat'
import { api } from '@/lib/api'

export function ChatPage() {
  const [claudeSessionId, setClaudeSessionId] = useState<string | null>(null)
  const [showSessions, setShowSessions] = useState(false)

  return (
    <div className="h-[calc(100vh-64px)] flex">
      {/* Session sidebar */}
      {showSessions && (
        <SessionSidebar
          onResume={(id) => { setClaudeSessionId(id); setShowSessions(false) }}
          onNew={() => { setClaudeSessionId(null); setShowSessions(false) }}
          onClose={() => setShowSessions(false)}
        />
      )}

      {/* Main chat */}
      <div className="flex-1 relative">
        <button
          onClick={() => setShowSessions(!showSessions)}
          className="absolute top-3 left-3 z-10 w-8 h-8 flex items-center justify-center text-muted-foreground hover:text-foreground border border-border bg-background transition-colors"
          title="Sessions"
        >
          <History size={14} />
        </button>

        <AgentChat
          key={claudeSessionId || 'new'}
          resumeSessionId={claudeSessionId}
          systemPrompt="You are a helpful AI assistant with access to the user's installed apps, tools, and prompts via MCP. Help them accomplish tasks using the available tools."
          placeholder="Ask anything... (/ for prompts)"
          suggestions={[
            { label: 'Show system status', prompt: 'Show me the Rubix runtime status' },
            { label: 'Query nodes', prompt: 'List all nodes of type rubix.device' },
            { label: 'Available tools', prompt: 'What tools do I have access to?' },
            { label: 'Node types', prompt: 'Show me all available node types' },
          ]}
        />
      </div>
    </div>
  )
}

function SessionSidebar({ onResume, onNew, onClose }: {
  onResume: (claudeSessionId: string) => void
  onNew: () => void
  onClose: () => void
}) {
  const { data: sessions, isLoading } = useQuery({
    queryKey: ['sessions'],
    queryFn: () => api.listSessions(),
    staleTime: 10_000,
  })

  // Only show sessions that have a Claude session ID (resumable)
  const resumable = (sessions || [])
    .filter(s => s.claude_session_id)
    .sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())

  return (
    <div className="w-72 border-r border-border bg-background flex flex-col shrink-0">
      <div className="flex items-center justify-between px-3 py-3 border-b border-border">
        <span className="text-xs font-mono uppercase tracking-wider text-muted-foreground">Sessions</span>
        <button
          onClick={onNew}
          className="flex items-center gap-1 text-xs font-mono text-primary hover:opacity-70 transition-opacity"
        >
          <Plus size={12} /> New
        </button>
      </div>

      <div className="flex-1 overflow-y-auto">
        {isLoading ? (
          <div className="flex justify-center py-8">
            <Loader2 size={16} className="animate-spin text-muted-foreground" />
          </div>
        ) : resumable.length === 0 ? (
          <p className="text-xs text-muted-foreground text-center py-8 px-3">
            No sessions yet. Start a chat to create one.
          </p>
        ) : (
          resumable.map(s => (
            <button
              key={s.id}
              onClick={() => onResume(s.claude_session_id!)}
              className="w-full text-left px-3 py-2.5 border-b border-border/50 hover:bg-accent transition-colors"
            >
              <p className="text-xs text-foreground truncate">{s.prompt.slice(0, 80)}</p>
              <p className="text-[10px] text-muted-foreground mt-0.5">
                {new Date(s.created_at).toLocaleString()} &middot; {(s.cost_usd * 100).toFixed(1)}&cent;
              </p>
            </button>
          ))
        )}
      </div>
    </div>
  )
}
