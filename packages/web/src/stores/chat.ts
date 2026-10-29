import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import {
  fetchChatHistory,
  fetchChatStatus,
  stopChat,
  streamChat,
  type ChatReferenceInput,
  type StreamEvent,
} from '@/api/chat'
import { useLogStore } from '@/stores/logs'
import { buildMockChatSteps } from '@/utils/editorMock'
import type { ChatMessage } from '@/types'

interface AgentPayload {
  type?: string
  subtype?: string
  message?: {
    content?: Array<Record<string, unknown>>
  }
  result?: unknown
  durationMs?: number
  duration_ms?: number
  numTurns?: number
  num_turns?: number
}

interface AgentBlock extends Record<string, unknown> {
  type?: string
  text?: string
  id?: string
  name?: string
  input?: unknown
  tool_use_id?: string
}

function numeric(value: unknown): number | null {
  return typeof value === 'number' && Number.isFinite(value) ? value : null
}

function toolDetail(input: unknown): string {
  if (!input || typeof input !== 'object') {
    return ''
  }
  const record = input as Record<string, unknown>
  const value = record.file_path ?? record.path ?? record.command ?? record.pattern ?? record.query
  if (typeof value !== 'string') {
    return ''
  }
  const detail = value.replace(/\s+/g, ' ').trim()
  return detail.length > 120 ? `${detail.slice(0, 117)}...` : detail
}

function resultLog(payload: AgentPayload): { level: 'info' | 'error'; message: string } {
  const success = !payload.subtype || payload.subtype === 'success'
  const turns = numeric(payload.numTurns) ?? numeric(payload.num_turns)
  const duration = numeric(payload.durationMs) ?? numeric(payload.duration_ms)
  let message = success ? '执行完成' : '执行结束'
  if (turns) {
    message += ` · ${turns} 轮`
  }
  if (duration) {
    message += ` · ${(duration / 1000).toFixed(1)}s`
  }
  return { level: success ? 'info' : 'error', message }
}

let seq = 0

function nextId() {
  seq += 1
  return `m${seq}`
}

export const useChatStore = defineStore('chat', () => {
  const messages = ref<ChatMessage[]>([])
  const streaming = ref(false)
  const agentRunning = ref(false)
  const busy = computed(() => streaming.value || agentRunning.value)
  const revision = ref(0)
  let controller: AbortController | null = null
  let mockRun = 0
  let mockTimers: ReturnType<typeof setTimeout>[] = []

  function clearMockTimers() {
    mockTimers.forEach((timer) => clearTimeout(timer))
    mockTimers = []
    mockRun += 1
  }

  function systemMessage(): ChatMessage {
    return {
      id: nextId(),
      role: 'system',
      text: '描述你的需求，我会直接修改代码并刷新右侧预览。',
      tools: [],
      pending: false,
    }
  }

  function reset() {
    disconnect()
    agentRunning.value = false
    revision.value = 0
    messages.value = [systemMessage()]
  }

  async function syncStatus(projectId: string): Promise<boolean> {
    const status = await fetchChatStatus(projectId)
    agentRunning.value = status.active
    return status.active
  }

  async function loadHistory(projectId: string): Promise<boolean> {
    const history = await fetchChatHistory(projectId)
    if (history.messages.length === 0) {
      return false
    }
    messages.value = [
      systemMessage(),
      ...history.messages.map((message) => ({
        id: message.id,
        role: message.role,
        text: message.text,
        tools: message.tools.map((tool) => ({
          id: tool.id,
          name: tool.name,
          status: tool.status,
        })),
        references: message.references ?? [],
        pending: false,
      })),
    ]
    revision.value += 1
    return true
  }

  function sendMock(projectId: string, text: string) {
    void projectId
    if (streaming.value) {
      return
    }
    clearMockTimers()
    messages.value.push({
      id: nextId(),
      role: 'user',
      text,
      tools: [],
      pending: false,
    })
    messages.value.push({
      id: nextId(),
      role: 'assistant',
      text: '',
      tools: [],
      pending: true,
    })

    const assistant = messages.value[messages.value.length - 1]
    const run = mockRun
    const logs = useLogStore()
    const steps = buildMockChatSteps(text)
    let elapsed = 0
    streaming.value = true

    steps.forEach((step, index) => {
      elapsed += step.delay
      mockTimers.push(
        setTimeout(() => {
          if (run !== mockRun) {
            return
          }
          if (step.text) {
            assistant.text = assistant.text ? `${assistant.text}\n\n${step.text}` : step.text
          }
          if (step.tool) {
            assistant.tools.push({ ...step.tool })
          }
          if (step.toolDone) {
            const tool = assistant.tools.find((item) => item.id === step.toolDone)
            if (tool) {
              tool.status = 'done'
            }
          }
          if (step.log) {
            logs.append(step.log)
          }
          if (index === steps.length - 1) {
            assistant.pending = false
            streaming.value = false
            revision.value += 1
          }
        }, elapsed),
      )
    })
  }

  async function send(projectId: string, text: string, references: ChatReferenceInput[] = []) {
    if (streaming.value || agentRunning.value) {
      return
    }
    const logs = useLogStore()
    messages.value.push({
      id: nextId(),
      role: 'user',
      text,
      tools: [],
      references: references.map((item) => ({
        kind: item.kind,
        id: item.id,
        name: item.name ?? item.id ?? item.path ?? '',
        path: item.path ?? '',
        contentType: item.contentType,
        resourceKind: item.resourceKind,
      })),
      pending: false,
    })
    messages.value.push({
      id: nextId(),
      role: 'assistant',
      text: '',
      tools: [],
      pending: true,
    })

    const assistant = messages.value[messages.value.length - 1]
    streaming.value = true
    agentRunning.value = true
    controller = new AbortController()
    logs.append({ level: 'info', source: 'agent', message: 'Agent 开始执行' })

    try {
      for await (const event of streamChat(projectId, text, controller.signal, references)) {
        applyEvent(assistant, event, logs)
      }
    } catch (error) {
      if ((error as Error).name !== 'AbortError') {
        const message = (error as Error).message
        assistant.text = assistant.text || `请求失败：${message}`
        assistant.role = 'system'
        logs.append({ level: 'error', source: 'agent', message: `执行失败：${message}` })
      }
    } finally {
      assistant.pending = false
      streaming.value = false
      agentRunning.value = false
      revision.value += 1
      controller = null
    }
  }

  function applyEvent(
    target: ChatMessage,
    event: StreamEvent,
    logs: ReturnType<typeof useLogStore>,
  ) {
    if (event.type === 'error') {
      const message = String(event.message ?? 'agent error')
      target.text = target.text || message
      logs.append({ level: 'error', source: 'agent', message: `执行失败：${message}` })
      return
    }
    if (event.type === 'sandbox-status') {
      target.pending = false
      if (event.state === 'ready') {
        logs.append({ level: 'info', source: 'sandbox', message: '沙箱已就绪' })
      }
      return
    }
    if (event.type !== 'agent') {
      return
    }

    const payload = event.payload as AgentPayload | undefined
    if (!payload || payload.type === 'system') {
      return
    }
    if (payload.type === 'result') {
      if (!target.text && typeof payload.result === 'string') {
        target.text = payload.result
      }
      target.pending = false
      logs.append({ source: 'agent', ...resultLog(payload) })
      return
    }

    const blocks = (payload.message?.content ?? []) as AgentBlock[]
    for (const block of blocks) {
      if (block.type === 'text' && typeof block.text === 'string') {
        if (block.text) {
          target.text = target.text ? `${target.text}\n\n${block.text}` : block.text
        }
      }
      if (
        payload.type === 'assistant' &&
        block.type === 'tool_use' &&
        typeof block.name === 'string'
      ) {
        const id = String(block.id ?? nextId())
        if (!target.tools.some((item) => item.id === id)) {
          target.tools.push({ id, name: block.name, status: 'running' })
          const detail = toolDetail(block.input)
          logs.append({
            level: 'info',
            source: 'tool',
            message: detail ? `${block.name} ${detail}` : block.name,
          })
        }
      }
      if (block.type === 'tool_result' && typeof block.tool_use_id === 'string') {
        const tool = target.tools.find((item) => item.id === block.tool_use_id)
        if (tool) {
          tool.status = 'done'
        }
      }
    }
  }

  function disconnect() {
    controller?.abort()
    controller = null
    clearMockTimers()
    const pending = messages.value.find((message) => message.pending)
    if (pending) {
      pending.pending = false
    }
    streaming.value = false
    revision.value += 1
  }

  async function stop(projectId: string) {
    disconnect()
    try {
      await stopChat(projectId)
    } finally {
      await syncStatus(projectId).catch(() => undefined)
    }
  }

  return {
    messages,
    streaming,
    agentRunning,
    busy,
    revision,
    syncStatus,
    reset,
    loadHistory,
    send,
    sendMock,
    disconnect,
    stop,
  }
})
