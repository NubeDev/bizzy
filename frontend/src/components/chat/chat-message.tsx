/**
 * Chat message bubble for the main chat page.
 * Thin wrapper around the shared ChatBubble component.
 */
import { ChatBubble } from '@/lib/chat'
import type { ChatMessage } from '@/hooks/use-agent-chat'

interface Props {
  message: ChatMessage
  isLast: boolean
  isStreaming: boolean
}

export function ChatMessageBubble({ message, isLast, isStreaming }: Props) {
  return (
    <ChatBubble
      message={message}
      isLast={isLast}
      isStreaming={isStreaming}
    />
  )
}
