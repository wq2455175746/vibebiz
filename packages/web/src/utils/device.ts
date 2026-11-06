const DEVICE_KEY = 'vibebiz.deviceId'

let cachedId: string | null = null

function randomId(): string {
  if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) {
    return crypto.randomUUID()
  }
  return `dev-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`
}

function readStoredId(): string | null {
  try {
    return localStorage.getItem(DEVICE_KEY)
  } catch {
    return null
  }
}

function persistId(id: string): void {
  try {
    localStorage.setItem(DEVICE_KEY, id)
  } catch {
    return
  }
}

export function getDeviceId(): string {
  if (cachedId) {
    return cachedId
  }
  const stored = readStoredId()
  if (stored) {
    cachedId = stored
    return cachedId
  }
  const id = randomId()
  cachedId = id
  persistId(id)
  return id
}

export function deviceHeader(): Record<string, string> {
  return { 'X-Device-Id': getDeviceId() }
}
