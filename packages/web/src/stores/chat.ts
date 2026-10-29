import { defineStore } from 'pinia'
import { ref } from 'vue'

import { streamChat, type StreamEvent } from '@/api/chat'
import { useLogStore } from '@/stores/logs'
import { buildMockChatSteps } from '@/utils/editorMock'
import type { ChatMessage } from '@/types'

interface AgentPayload {
  type?: string
  message?: {
    content?: Array<Record<string, unknown>>
  }
}

let seq = 0

function nextId() {
  seq += 1
  return `m${seq}`
}

export const useChatStore = defineStore('chat', () => {
  const messages = ref<ChatMessage[]>([])
  const streaming = ref(false)
  const revision = ref(0)
  let controller: AbortController | null = null
  let mockRun = 0
  let mockTimers: ReturnType<typeof setTimeout>[] = []

  function clearMockTimers() {
    mockTimers.forEach((timer) => clearTimeout(timer))
    mockTimers = []
    mockRun += 1
  }

  function reset() {
    controller?.abort()
    controller = null
    clearMockTimers()
    messages.value = [
      {
        id: nextId(),
        role: 'system',
        text: '描述你的需求，我会直接修改代码并刷新右侧预览。',
        tools: [],
        pending: false,
      },
    ]
    streaming.value = false
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

  async function send(projectId: string, text: string) {
    if (streaming.value) {
      return
    }
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
    streaming.value = true
    controller = new AbortController()

    try {
      for await (const event of streamChat(projectId, text, controller.signal)) {
        applyEvent(assistant, event)
      }
    } catch (error) {
      if ((error as Error).name !== 'AbortError') {
        assistant.text = assistant.text || `请求失败：${(error as Error).message}`
        assistant.role = 'system'
      }
    } finally {
      assistant.pending = false
      streaming.value = false
      revision.value += 1
      controller = null
    }
  }

  function applyEvent(target: ChatMessage, event: StreamEvent) {
    if (event.type === 'error') {
      target.text = target.text || String(event.message ?? 'agent error')
      return
    }
    if (event.type === 'sandbox-status') {
      target.pending = false
      return
    }
    if (event.type !== 'agent') {
      return
    }

    const payload = event.payload as AgentPayload | undefined
    if (!payload) {
      return
    }
    if (payload.type === 'result') {
      target.pending = false
      return
    }

    for (const block of payload.message?.content ?? []) {
      const type = block.type
      if (type === 'text' && typeof block.text === 'string') {
        target.text += block.text
      }
      if (type === 'tool_use' && typeof block.name === 'string') {
        target.tools.push({
          id: String(block.id ?? nextId()),
          name: block.name,
          status: 'running',
        })
      }
      if (type === 'tool_result' && typeof block.tool_use_id === 'string') {
        const tool = target.tools.find((item) => item.id === block.tool_use_id)
        if (tool) {
          tool.status = 'done'
        }
      }
    }
  }

  function stop() {
    controller?.abort()
    clearMockTimers()
    const pending = messages.value.find((message) => message.pending)
    if (pending) {
      pending.pending = false
    }
    streaming.value = false
    revision.value += 1
  }

  return { messages, streaming, revision, reset, send, sendMock, stop }
})
