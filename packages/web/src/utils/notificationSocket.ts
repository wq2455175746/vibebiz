export function buildNotificationSocketUrl(base: string, location: Location): string {
  const url = new URL(base, `${location.protocol}//${location.host}`)
  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:'
  url.pathname = `${url.pathname.replace(/\/$/, '')}/notifications/ws`
  return url.toString()
}
