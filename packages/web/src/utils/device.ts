const DEVICE_KEY = 'vibebiz.deviceId'

function randomId(): string {
  if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) {
    return crypto.randomUUID()
  }
  return `dev-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`
}

export function getDeviceId(): string {
  const stored = localStorage.getItem(DEVICE_KEY)
  if (stored) {
    return stored
  }
  const id = randomId()
  localStorage.setItem(DEVICE_KEY, id)
  return id
}

export function deviceHeader(): Record<string, string> {
  return { 'X-Device-Id': getDeviceId() }
}
