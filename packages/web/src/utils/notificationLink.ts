export function safeNotificationLink(link: string): string {
  const value = link.trim()
  if (!value) {
    return ''
  }
  if (value.startsWith('/') && !value.startsWith('//')) {
    return value
  }
  try {
    const url = new URL(value)
    if (url.protocol === 'http:' || url.protocol === 'https:') {
      return value
    }
  } catch {
    return ''
  }
  return ''
}
