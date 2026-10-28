function pad(value: number): string {
  return String(value).padStart(2, '0')
}

export function formatDateTime(value: unknown): string {
  if (typeof value !== 'string' || value.trim() === '') {
    return '—'
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }
  return [
    `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`,
    `${pad(date.getHours())}:${pad(date.getMinutes())}`,
  ].join(' ')
}

export function formatNumber(value: unknown): string {
  if (typeof value === 'number' && Number.isFinite(value)) {
    return value.toLocaleString('zh-CN')
  }
  if (typeof value === 'string' && value.trim() !== '') {
    return value
  }
  return '—'
}
