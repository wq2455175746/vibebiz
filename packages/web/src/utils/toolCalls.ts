import type { ToolState } from '@/types'

export interface ToolSummary {
  total: number
  running: number
  collapsible: boolean
  detail: string
}

export function summarizeTools(tools: ToolState[], threshold = 3): ToolSummary {
  const counts = new Map<string, number>()
  for (const tool of tools) {
    counts.set(tool.name, (counts.get(tool.name) ?? 0) + 1)
  }
  return {
    total: tools.length,
    running: tools.filter((tool) => tool.status === 'running').length,
    collapsible: tools.length > threshold,
    detail: [...counts.entries()].map(([name, count]) => `${name} ×${count}`).join(' · '),
  }
}
