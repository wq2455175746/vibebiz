import { createApp, defineComponent, h } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { useNotificationSocket } from './useNotificationSocket'

class FakeWebSocket {
  static instances: FakeWebSocket[] = []

  url: string
  closed = false
  onopen: (() => void) | null = null
  onmessage: ((event: MessageEvent) => void) | null = null
  onclose: (() => void) | null = null
  onerror: (() => void) | null = null

  constructor(url: string | URL) {
    this.url = String(url)
    FakeWebSocket.instances.push(this)
  }

  close(): void {
    this.closed = true
    this.onclose?.()
  }

  serverClose(): void {
    this.onclose?.()
  }

  open(): void {
    this.onopen?.()
  }
}

function setup(onChange: () => void = () => {}) {
  let api!: ReturnType<typeof useNotificationSocket>
  const host = document.createElement('div')
  const app = createApp(
    defineComponent({
      setup() {
        api = useNotificationSocket(onChange)
        return () => h('div')
      },
    }),
  )
  app.mount(host)
  return { api, unmount: () => app.unmount() }
}

beforeEach(() => {
  FakeWebSocket.instances = []
  vi.stubGlobal('WebSocket', FakeWebSocket)
  vi.useFakeTimers()
})

afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
})

describe('useNotificationSocket', () => {
  it('does not open a duplicate socket while connected', () => {
    const { api, unmount } = setup()

    api.connect()
    api.connect()

    expect(FakeWebSocket.instances).toHaveLength(1)
    unmount()
  })

  it('reconnects with exponential backoff', () => {
    const { api, unmount } = setup()
    api.connect()

    FakeWebSocket.instances[0].serverClose()
    vi.advanceTimersByTime(999)
    expect(FakeWebSocket.instances).toHaveLength(1)
    vi.advanceTimersByTime(1)
    expect(FakeWebSocket.instances).toHaveLength(2)

    FakeWebSocket.instances[1].serverClose()
    vi.advanceTimersByTime(1999)
    expect(FakeWebSocket.instances).toHaveLength(2)
    vi.advanceTimersByTime(1)
    expect(FakeWebSocket.instances).toHaveLength(3)

    FakeWebSocket.instances[2].serverClose()
    vi.advanceTimersByTime(3999)
    expect(FakeWebSocket.instances).toHaveLength(3)
    vi.advanceTimersByTime(1)
    expect(FakeWebSocket.instances).toHaveLength(4)

    unmount()
  })

  it('ignores close events from a stale socket', () => {
    const { api, unmount } = setup()
    api.connect()
    const stale = FakeWebSocket.instances[0]

    api.disconnect()
    api.connect()
    const live = FakeWebSocket.instances[1]

    stale.serverClose()
    vi.advanceTimersByTime(30_000)

    expect(FakeWebSocket.instances).toHaveLength(2)
    expect(live.closed).toBe(false)
    unmount()
  })

  it('ignores error events from a stale socket', () => {
    const { api, unmount } = setup()
    api.connect()
    const stale = FakeWebSocket.instances[0]

    api.disconnect()
    api.connect()
    const live = FakeWebSocket.instances[1]

    stale.onerror?.()
    vi.advanceTimersByTime(30_000)

    expect(FakeWebSocket.instances).toHaveLength(2)
    expect(live.closed).toBe(false)
    unmount()
  })

  it('does not open immediately while a retry is pending', () => {
    const { api, unmount } = setup()
    api.connect()
    FakeWebSocket.instances[0].serverClose()

    api.connect()
    expect(FakeWebSocket.instances).toHaveLength(1)

    vi.advanceTimersByTime(1000)
    expect(FakeWebSocket.instances).toHaveLength(2)
    unmount()
  })

  it('stops reconnecting after disconnect', () => {
    const { api, unmount } = setup()
    api.connect()
    FakeWebSocket.instances[0].serverClose()

    api.disconnect()
    vi.advanceTimersByTime(30_000)

    expect(FakeWebSocket.instances).toHaveLength(1)
    unmount()
  })

  it('invokes onChange only for notifications.changed events', () => {
    const onChange = vi.fn()
    const { api, unmount } = setup(onChange)
    api.connect()
    const socket = FakeWebSocket.instances[0]

    socket.onmessage?.({ data: JSON.stringify({ type: 'notifications.changed' }) } as MessageEvent)
    socket.onmessage?.({ data: JSON.stringify({ type: 'other' }) } as MessageEvent)
    socket.onmessage?.({ data: 'not-json' } as MessageEvent)

    expect(onChange).toHaveBeenCalledTimes(1)
    unmount()
  })
})
