export function toList<T>(payload: T[] | Record<string, T[]>, key: string): T[] {
  if (Array.isArray(payload)) {
    return payload
  }
  const value = payload[key]
  return Array.isArray(value) ? value : []
}

export function toEntity<T extends object>(payload: T | Record<string, T>, key: string): T {
  if (payload && typeof payload === 'object' && key in payload) {
    const nested = (payload as Record<string, T>)[key]
    if (nested && typeof nested === 'object') {
      return nested
    }
  }
  return payload as T
}
