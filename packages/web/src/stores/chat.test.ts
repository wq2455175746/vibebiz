import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import {
  fetchChatHistory,
  fetchChatStatus,
  streamChat,
  type StreamEvent,
} from '@/api/chat'
import { useChatStore } from './chat'
import { useLogStore } from './logs'

vi.mock('@/api/chat', () => ({
  streamChat: vi.fn(),
  fetchChatHistory: vi.fn(),
  fetchChatStatus: vi.fn(),
  stopChat: vi.fn(),
}))

const streamMock = vi.mocked(streamChat)
const historyMock = vi.mocked(fetchChatHistory)
const statusMock = vi.mocked(fetchChatStatus)

async function* events(items: StreamEvent[]): AsyncGenerator<StreamEvent> {
  for (const item of items) {
    yield item
  }
}

beforeEach(() => {
  setActivePinia(createPinia())
  streamMock.mockReset()
  historyMock.mockReset()
  statusMock.mockReset()
})

describe('chat store real stream', () => {
  it('maps assistant text, tool calls and result events', async () => {
    streamMock.mockReturnValue(
      events([
        {
          type: 'agent',
          payload: {
            type: 'assistant',
            message: {
              content: [{ type: 'text', text: '先看一下入口文件。' }],
            },
          },
        },
        {
          type: 'agent',
          payload: {
            type: 'assistant',
            message: {
              content: [
                {
                  type: 'tool_use',
                  id: 'tool-1',
                  name: 'Read',
                  input: { file_path: 'index.html' },
                },
              ],
            },
          },
        },
        {
          type: 'agent',
          payload: {
            type: 'user',
            message: {
              content: [{ type: 'tool_result', tool_use_id: 'tool-1', content: 'ok' }],
            },
          },
        },
        {
          type: 'agent',
          payload: {
            type: 'result',
            subtype: 'success',
            durationMs: 1200,
            numTurns: 2,
          },
        },
        { type: 'sandbox-status', state: 'ready' },
      ]),
    )

    const chat = useChatStore()
    const logs = useLogStore()
    chat.reset()

    await chat.send('p1', '把标题改成 Hello')

    const assistant = chat.messages.at(-1)
    expect(chat.messages.map((message) => message.role)).toEqual(['system', 'user', 'assistant'])
    expect(assistant).toMatchObject({
      role: 'assistant',
      text: '先看一下入口文件。',
      pending: false,
    })
    expect(assistant?.tools).toEqual([{ id: 'tool-1', name: 'Read', status: 'done' }])
    expect(chat.revision).toBe(1)
    expect(chat.streaming).toBe(false)
    expect(logs.entries.map((entry) => entry.message)).toEqual([
      'Agent 开始执行',
      'Read index.html',
      '执行完成 · 2 轮 · 1.2s',
      '沙箱已就绪',
    ])
  })

  it('uses result text when no assistant text was emitted', async () => {
    streamMock.mockReturnValue(
      events([
        {
          type: 'agent',
          payload: { type: 'result', subtype: 'success', result: 'OK' },
        },
      ]),
    )

    const chat = useChatStore()
    chat.reset()

    await chat.send('p1', '只回复 OK')

    expect(chat.messages.at(-1)?.text).toBe('OK')
  })

  it('ignores hook system events instead of dumping them into chat', async () => {
    streamMock.mockReturnValue(
      events([
        {
          type: 'agent',
          payload: {
            type: 'system',
            subtype: 'hook_response',
            output: 'ADHD MODE ACTIVE',
          },
        },
        {
          type: 'agent',
          payload: { type: 'result', subtype: 'success', result: 'OK' },
        },
      ]),
    )

    const chat = useChatStore()
    chat.reset()

    await chat.send('p1', '只回复 OK')

    expect(chat.messages.at(-1)?.text).toBe('OK')
    expect(chat.messages.at(-1)?.text).not.toContain('ADHD')
  })

  it('records stream errors on the assistant message and log', async () => {
    streamMock.mockImplementation(async function* () {
      yield* []
      throw new Error('upstream down')
    })

    const chat = useChatStore()
    const logs = useLogStore()
    chat.reset()

    await chat.send('p1', 'hello')

    expect(chat.messages.at(-1)).toMatchObject({
      role: 'system',
      text: '请求失败：upstream down',
      pending: false,
    })
    expect(logs.entries.at(-1)).toMatchObject({
      level: 'error',
      message: '执行失败：upstream down',
    })
  })
})

describe('chat store history', () => {
  it('loads persisted history and marks it complete', async () => {
    historyMock.mockResolvedValue({
      messages: [
        {
          id: 'h1',
          role: 'user',
          text: '把标题改一下',
          status: 'completed',
          tools: [],
          createdAt: '2026-10-29T10:00:00Z',
        },
        {
          id: 'h2',
          role: 'assistant',
          text: '改好了',
          status: 'completed',
          tools: [{ id: 't1', name: 'Edit', status: 'done' }],
          createdAt: '2026-10-29T10:00:01Z',
        },
      ],
      truncated: false,
      limit: 100,
    })

    const chat = useChatStore()
    chat.reset()

    await expect(chat.loadHistory('p1')).resolves.toBe(true)

    expect(chat.messages.map((message) => message.role)).toEqual(['system', 'user', 'assistant'])
    expect(chat.messages.at(-1)).toMatchObject({
      id: 'h2',
      text: '改好了',
      createdAt: '2026-10-29T10:00:01Z',
      pending: false,
      tools: [{ id: 't1', name: 'Edit', status: 'done' }],
    })
  })

  it('returns false when history is empty', async () => {
    historyMock.mockResolvedValue({ messages: [], truncated: false, limit: 100 })

    const chat = useChatStore()
    chat.reset()

    await expect(chat.loadHistory('p1')).resolves.toBe(false)
    expect(chat.messages.map((message) => message.role)).toEqual(['system'])
  })
})


describe('chat store agent status', () => {
  it('tracks a server-side active turn as busy', async () => {
    statusMock.mockResolvedValue({ active: true, turnId: 'turn-1' })

    const chat = useChatStore()
    chat.reset()

    await expect(chat.syncStatus('p1')).resolves.toBe(true)
    expect(chat.agentRunning).toBe(true)
    expect(chat.busy).toBe(true)
  })

  it('does not send a second prompt while the server reports an active turn', async () => {
    statusMock.mockResolvedValue({ active: true, turnId: 'turn-1' })

    const chat = useChatStore()
    chat.reset()
    await chat.syncStatus('p1')

    await chat.send('p1', 'second task')

    expect(streamMock).not.toHaveBeenCalled()
    expect(chat.messages.map((message) => message.role)).toEqual(['system'])
  })
})

describe('chat store background turn recovery', () => {
  it('keeps a running turn in the background without reloading history', async () => {
    statusMock.mockResolvedValue({ active: true, turnId: 'turn-1' })

    const chat = useChatStore()
    chat.reset()

    await expect(chat.resume('p1')).resolves.toBe(true)

    expect(chat.agentRunning).toBe(true)
    expect(historyMock).not.toHaveBeenCalled()
  })

  it('reloads history when the background turn has completed', async () => {
    statusMock.mockResolvedValue({ active: false, turnId: null })
    historyMock.mockResolvedValue({
      messages: [
        {
          id: 'h1',
          role: 'user',
          text: '继续执行',
          status: 'completed',
          tools: [],
          createdAt: '2026-10-30T01:00:00Z',
        },
        {
          id: 'h2',
          role: 'assistant',
          text: '后台已完成',
          status: 'completed',
          tools: [],
          createdAt: '2026-10-30T01:00:01Z',
        },
      ],
      truncated: false,
      limit: 100,
    })

    const chat = useChatStore()
    chat.reset()

    await expect(chat.resume('p1')).resolves.toBe(false)

    expect(chat.agentRunning).toBe(false)
    expect(chat.messages.map((message) => message.text)).toEqual([
      '描述你的需求，我会直接修改代码并刷新右侧预览。',
      '继续执行',
      '后台已完成',
    ])
  })
})
