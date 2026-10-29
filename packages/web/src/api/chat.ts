import { cpHttp, dpHttp, request } from './client'
import { splitSSE } from '@/utils/sse'

export interface StreamEvent {
  type: string
  payload?: unknown
  message?: string
  state?: string
  commit?: string | null
}

export interface HistoryTool {
  id: string
  name: string
  status: 'running' | 'done' | 'error'
}

export interface HistoryReference {
  kind: 'resource' | 'project_file'
  id?: string
  name: string
  path: string
  contentType?: string
  resourceKind?: string
}

export interface HistoryMessage {
  id: string
  role: 'user' | 'assistant'
  text: string
  status: string
  tools: HistoryTool[]
  references?: HistoryReference[]
  createdAt: string
}

export interface ChatReferenceInput {
  kind: 'resource' | 'project_file'
  id?: string
  path?: string
  name?: string
  contentType?: string
  resourceKind?: string
}

export interface ChatHistory {
  messages: HistoryMessage[]
  truncated: boolean
  limit: number
}

export interface ChatStatus {
  active: boolean
  turnId: string | null
}

const API_BASE = import.meta.env.VITE_DP_BASE || '/dp/api/v1'

export async function fetchChatHistory(projectId: string, limit = 100): Promise<ChatHistory> {
  return request<ChatHistory>(cpHttp, {
    method: 'get',
    url: `/projects/${encodeURIComponent(projectId)}/chat/history`,
    params: { limit },
  })
}

export async function fetchChatStatus(projectId: string): Promise<ChatStatus> {
  return request<ChatStatus>(dpHttp, {
    method: 'get',
    url: `/projects/${encodeURIComponent(projectId)}/chat/status`,
  })
}

export async function stopChat(projectId: string): Promise<void> {
  await request<{ stopped: boolean }>(dpHttp, {
    method: 'post',
    url: `/projects/${encodeURIComponent(projectId)}/chat/stop`,
  })
}

export async function* streamChat(
  projectId: string,
  message: string,
  signal?: AbortSignal,
  references: ChatReferenceInput[] = [],
): AsyncGenerator<StreamEvent> {
  const response = await fetch(`${API_BASE}/projects/${encodeURIComponent(projectId)}/chat`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Accept: 'text/event-stream',
    },
    body: JSON.stringify({ message, references }),
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
