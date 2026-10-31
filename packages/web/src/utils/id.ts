export function randomId(): string {
  const source = globalThis.crypto as Crypto | undefined
  if (source && typeof source.randomUUID === 'function') {
    return source.randomUUID()
  }

  const bytes = new Uint8Array(16)
  if (source && typeof source.getRandomValues === 'function') {
    source.getRandomValues(bytes)
  } else {
    for (let index = 0; index < bytes.length; index += 1) {
      bytes[index] = Math.floor(Math.random() * 256)
    }
  }

  bytes[6] = (bytes[6] & 0x0f) | 0x40
  bytes[8] = (bytes[8] & 0x3f) | 0x80

  return [...bytes]
    .map((byte) => byte.toString(16).padStart(2, '0'))
    .join('')
    .replace(/^(.{8})(.{4})(.{4})(.{4})(.{12})$/, '$1-$2-$3-$4-$5')
}
