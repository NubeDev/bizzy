import { useMutation } from '@tanstack/react-query'

export interface HTTPLogEntry {
  method: string
  url: string
  status: number
  duration_ms: number
  redirected_from: string | null
}

export interface TestToolRequest {
  script: string
  helpers?: string
  params: Record<string, unknown>
  allowedHosts: string[]
  settings?: Record<string, string>
  secrets?: Record<string, string>
  timeout?: string
}

export interface TestToolResponse {
  output: unknown
  error: string | null
  duration_ms: number
  http_log: HTTPLogEntry[]
}

async function callTestTool(req: TestToolRequest): Promise<TestToolResponse> {
  const res = await fetch('/api/apps/test-tool', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body.error || res.statusText)
  }
  return res.json()
}

export function useTestTool() {
  return useMutation({
    mutationFn: callTestTool,
  })
}
