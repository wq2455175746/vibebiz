import { beforeEach, describe, expect, it, vi } from 'vitest'

import { deviceHeader, getDeviceId } from './device'

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
    vi.stubGlobal('localStorage', memoryStorage())
  })

  it('generates and persists an id', () => {
    const first = getDeviceId()
    expect(first.length).toBeGreaterThan(8)
    expect(localStorage.getItem('vibebiz.deviceId')).toBe(first)
    expect(getDeviceId()).toBe(first)
  })

  it('reuses an existing id', () => {
    localStorage.setItem('vibebiz.deviceId', 'device-123')
    expect(getDeviceId()).toBe('device-123')
  })

  it('exposes the request header', () => {
    localStorage.setItem('vibebiz.deviceId', 'device-123')
    expect(deviceHeader()).toEqual({ 'X-Device-Id': 'device-123' })
  })
})
