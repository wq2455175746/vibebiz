export interface SSEFrame {
  data: string
}

export function splitSSE(chunk: string): { frames: SSEFrame[]; rest: string } {
  const normalized = chunk.replace(/\r\n/g, '\n')
  const parts = normalized.split('\n\n')
  const rest = parts.pop() ?? ''
  const frames: SSEFrame[] = []

  for (const part of parts) {
    const data = part
      .split('\n')
      .filter((line) => line.startsWith('data:'))
      .map((line) => line.slice(5).replace(/^ /, ''))
      .join('\n')
    if (data !== '') {
      frames.push({ data })
    }
  }

  return { frames, rest }
}
