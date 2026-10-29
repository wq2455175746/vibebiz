import { describe, expect, it } from 'vitest'

import {
  commandContextError,
  commandQuery,
  matchCommands,
  resolveCommand,
  stripCommand,
} from './slashCommands'

describe('slashCommands', () => {
  it('returns all context commands for a bare slash', () => {
    expect(matchCommands('/', 'home').map((item) => item.name)).toEqual(['project'])
    expect(matchCommands('/', 'editor').map((item) => item.name)).toEqual(['skills', 'model'])
  })

  it('filters by prefix', () => {
    expect(matchCommands('/s', 'editor').map((item) => item.name)).toEqual(['skills'])
    expect(matchCommands('/x', 'editor')).toEqual([])
  })

  it('resolves exact commands', () => {
    expect(resolveCommand('/skills', 'editor')?.name).toBe('skills')
    expect(resolveCommand('/project', 'home')?.name).toBe('project')
    expect(resolveCommand('/skills', 'home')).toBeNull()
  })

  it('ignores inputs with whitespace', () => {
    expect(commandQuery('/skills now')).toBeNull()
    expect(matchCommands('/skills now', 'editor')).toEqual([])
  })

  it('reports wrong-context and unknown commands', () => {
    expect(commandContextError('/project', 'editor')).toBe('该命令仅首页可用')
    expect(commandContextError('/skills', 'home')).toBe('该命令仅编辑器可用')
    expect(commandContextError('/nope', 'home')).toBe('未知命令，输入 / 查看可用命令')
    expect(commandContextError('/nope', 'editor')).toBeNull()
  })

  it('strips the command token', () => {
    expect(stripCommand('/project', 'project')).toBe('')
    expect(stripCommand('/project hello', 'project')).toBe('hello')
  })
})
