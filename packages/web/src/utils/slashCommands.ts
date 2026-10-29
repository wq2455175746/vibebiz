export type SlashContext = 'home' | 'editor'

export interface SlashCommand {
  name: string
  title: string
  description: string
  disabled?: boolean
}

const COMMANDS: Record<SlashContext, SlashCommand[]> = {
  home: [{ name: 'project', title: '/project', description: '选择已有项目并追加对话' }],
  editor: [
    { name: 'skills', title: '/skills', description: '查看当前可用的 skills' },
    { name: 'model', title: '/model', description: '模型选择即将支持', disabled: true },
  ],
}

export function commandQuery(input: string): string | null {
  if (!input.startsWith('/')) {
    return null
  }
  const query = input.slice(1)
  return /\s/.test(query) ? null : query
}

export function matchCommands(input: string, context: SlashContext): SlashCommand[] {
  const query = commandQuery(input)
  if (query === null) {
    return []
  }
  return COMMANDS[context].filter((command) => command.name.startsWith(query))
}

export function resolveCommand(input: string, context: SlashContext): SlashCommand | null {
  const query = commandQuery(input)
  if (query === null) {
    return null
  }
  return COMMANDS[context].find((command) => command.name === query) ?? null
}

export function commandContextError(input: string, context: SlashContext): string | null {
  const name = commandQuery(input)
  if (name === null) {
    return null
  }
  const known = [...COMMANDS.home, ...COMMANDS.editor].find((command) => command.name === name)
  if (!known) {
    return context === 'home' ? '未知命令，输入 / 查看可用命令' : null
  }
  if (context === 'home' && name !== 'project') {
    return '该命令仅编辑器可用'
  }
  if (context === 'editor' && name === 'project') {
    return '该命令仅首页可用'
  }
  return null
}

export function stripCommand(input: string, name: string): string {
  const prefix = `/${name}`
  return input.startsWith(prefix) ? input.slice(prefix.length).trimStart() : input
}
