import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { streamChat } from './chat'

function memoryStorage(initial: Record<string, string> = {}) {
  const values = new Map<string, string>(Object.entries(initial))
  return {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => void values.set(key, value),
    removeItem: (key: string) => void values.delete(key),
    clear: () => values.clear(),
  }
}

describe('streamChat', () => {
  beforeEach(() => {
    vi.stubGlobal('localStorage', memoryStorage({ 'vibebiz.deviceId': 'device-123' }))
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('sends the device id header', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response('data: [DONE]\n\n', {
        status: 200,
        headers: { 'Content-Type': 'text/event-stream' },
      }),
    )
    vi.stubGlobal('fetch', fetchMock)

    await streamChat('p1', 'hello').next()

    const init = fetchMock.mock.calls[0][1] as RequestInit
    expect(new Headers(init.headers).get('X-Device-Id')).toBe('device-123')
    expect(new Headers(init.headers).get('Content-Type')).toBe('application/json')
    expect(new Headers(init.headers).get('Accept')).toBe('text/event-stream')
  })
})
