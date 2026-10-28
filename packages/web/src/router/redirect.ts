export function sanitizeRedirect(raw: unknown): string {
  if (typeof raw !== 'string') {
    return '/'
  }
  if (!raw.startsWith('/') || raw.startsWith('//') || raw.startsWith('/\\')) {
    return '/'
  }
  return raw
}
