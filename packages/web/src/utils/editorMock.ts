import type { LogLevel, ToolState } from '@/types'

export interface MockLogStep {
  level: LogLevel
  source: string
  message: string
}

export interface MockChatStep {
  delay: number
  text?: string
  tool?: ToolState
  toolDone?: string
  log?: MockLogStep
}

const TOOL_READ = 'mock-tool-read'
const TOOL_WRITE = 'mock-tool-write'
const TOOL_BUILD = 'mock-tool-build'

export function buildMockChatSteps(prompt: string): MockChatStep[] {
  const request = prompt.trim() || '新的页面'
  return [
    {
      delay: 180,
      text: '收到。我先查看当前项目结构，确认入口文件和现有样式。',
      log: { level: 'info', source: 'agent', message: `收到需求：${request}` },
    },
    {
      delay: 260,
      tool: { id: TOOL_READ, name: 'Read', status: 'running' },
      log: { level: 'info', source: 'tool', message: 'Read index.html' },
    },
    {
      delay: 280,
      text: '现有页面是一个静态入口，我会保留可运行结构，直接更新页面内容和交互。',
      toolDone: TOOL_READ,
    },
    {
      delay: 320,
      tool: { id: TOOL_WRITE, name: 'Write', status: 'running' },
      log: { level: 'info', source: 'tool', message: 'Write index.html' },
    },
    {
      delay: 260,
      toolDone: TOOL_WRITE,
      log: { level: 'info', source: 'file', message: 'index.html · version updated' },
    },
    {
      delay: 280,
      tool: { id: TOOL_BUILD, name: 'Bash', status: 'running' },
      log: { level: 'info', source: 'bash', message: 'npm run build' },
    },
    {
      delay: 340,
      toolDone: TOOL_BUILD,
      text: '第一版已经完成，右侧预览会同步刷新。你可以继续告诉我要调整的细节。',
      log: { level: 'info', source: 'agent', message: '生成完成，预览已刷新' },
    },
  ]
}

export function buildMockLogLines(prompt: string): MockLogStep[] {
  return buildMockChatSteps(prompt)
    .map((step) => step.log)
    .filter((log): log is MockLogStep => log !== undefined)
}
