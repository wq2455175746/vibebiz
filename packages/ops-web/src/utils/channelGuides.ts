export function splitLines(value: string): string[] {
  return value
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean)
}

export function isMarkdownFile(name: string): boolean {
  return /\.(md|markdown)$/i.test(name)
}
