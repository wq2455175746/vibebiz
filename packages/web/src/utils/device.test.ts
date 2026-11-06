import { beforeEach, describe, expect, it, vi } from 'vitest'

function memoryStorage() {
  const values = new Map<string, string>()
  return {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => void values.set(key, value),
    removeItem: (key: string) => void values.delete(key),
    clear: () => values.clear(),
  }
}

describe('getDeviceId', () => {
  beforeEach(() => {
    vi.resetModules()
    vi.stubGlobal('localStorage', memoryStorage())
  })

  it('generates and persists an id', async () => {
    const { getDeviceId } = await import('./device')
    const first = getDeviceId()
    expect(first.length).toBeGreaterThan(8)
    expect(localStorage.getItem('vibebiz.deviceId')).toBe(first)
    expect(getDeviceId()).toBe(first)
  })

  it('reuses an existing id', async () => {
    localStorage.setItem('vibebiz.deviceId', 'device-123')
    const { getDeviceId } = await import('./device')
    expect(getDeviceId()).toBe('device-123')
  })

  it('exposes the request header', async () => {
    localStorage.setItem('vibebiz.deviceId', 'device-123')
    const { deviceHeader } = await import('./device')
    expect(deviceHeader()).toEqual({ 'X-Device-Id': 'device-123' })
  })

  it('keeps an in-memory id when storage throws', async () => {
    vi.stubGlobal('localStorage', {
      getItem: () => {
        throw new Error('storage blocked')
      },
      setItem: () => {
        throw new Error('storage blocked')
      },
      removeItem: () => {},
      clear: () => {},
    })
    const { deviceHeader, getDeviceId } = await import('./device')
    const first = getDeviceId()
    expect(first.length).toBeGreaterThan(8)
    expect(getDeviceId()).toBe(first)
    expect(deviceHeader()).toEqual({ 'X-Device-Id': first })
  })
})
