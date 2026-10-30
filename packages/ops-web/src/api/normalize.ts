export function toList<T>(payload: T[] | Record<string, T[]>, key: string): T[] {
  if (Array.isArray(payload)) {
    return payload
  }
  const value = payload[key]
  return Array.isArray(value) ? value : []
}

export function toPage<T>(
  payload: T[] | Record<string, unknown>,
  key: string,
): {
  items: T[]
  total: number
  page: number
  pageSize: number
} {
  if (Array.isArray(payload)) {
    return { items: payload, total: payload.length, page: 1, pageSize: payload.length || 10 }
  }
  const value = payload[key]
  const items = Array.isArray(value) ? (value as T[]) : []
  return {
    items,
    total: Number(payload.total ?? items.length),
    page: Number(payload.page ?? 1),
    pageSize: Number(payload.pageSize ?? (items.length || 10)),
  }
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
