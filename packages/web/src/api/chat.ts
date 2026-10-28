import { splitSSE } from '@/utils/sse'

export interface StreamEvent {
  type: string
  payload?: unknown
  message?: string
  state?: string
  commit?: string | null
}

const API_BASE = import.meta.env.VITE_DP_BASE || '/dp/api/v1'

export async function* streamChat(
  projectId: string,
  message: string,
  signal?: AbortSignal,
): AsyncGenerator<StreamEvent> {
  const response = await fetch(`${API_BASE}/projects/${encodeURIComponent(projectId)}/chat`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Accept: 'text/event-stream',
    },
    body: JSON.stringify({ message }),
    credentials: 'include',
    signal,
  })

  if (!response.ok || !response.body) {
    const text = await response.text().catch(() => '')
    throw new Error(text || `chat failed (${response.status})`)
  }

  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''

  while (true) {
    const { value, done } = await reader.read()
    if (done) {
      break
    }
    buffer += decoder.decode(value, { stream: true })
    const parsed = splitSSE(buffer)
    buffer = parsed.rest
    for (const frame of parsed.frames) {
      if (frame.data === '[DONE]') {
        return
      }
      yield JSON.parse(frame.data) as StreamEvent
    }
  }

  buffer += decoder.decode()
  const tail = splitSSE(`${buffer}\n\n`)
  for (const frame of tail.frames) {
    if (frame.data === '[DONE]') {
      return
    }
    yield JSON.parse(frame.data) as StreamEvent
  }
}
