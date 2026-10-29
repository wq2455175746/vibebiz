import { describe, expect, it } from 'vitest'

import { buildModelInput, type ModelFormState } from './modelForm'

function form(overrides: Partial<ModelFormState> = {}): ModelFormState {
  return {
    modelName: 'DeepSeek Chat',
    modelCode: 'deepseek-chat',
    apiUrl: 'https://api.deepseek.com/anthropic',
    apiKey: 'sk-test',
    providerCode: 'DeepSeek',
    modelType: 'LLM',
    maxTokens: 4096,
    isDefault: true,
    status: 1,
    extraParams: '',
    ...overrides,
  }
}

describe('buildModelInput', () => {
  it('builds an LLM input', () => {
    const result = buildModelInput(form())
    expect('input' in result && result.input.modelType).toBe('LLM')
    expect('input' in result && result.input.maxTokens).toBe(4096)
  })

  it('forces non-LLM max tokens to zero', () => {
    const result = buildModelInput(form({ modelType: 'ASR', maxTokens: 4096 }))
    expect('input' in result && result.input.maxTokens).toBe(0)
  })

  it('rejects missing required fields', () => {
    expect(buildModelInput(form({ modelName: '' }))).toEqual({ error: '请填写模型名称' })
    expect(buildModelInput(form({ apiKey: '' }))).toEqual({ error: '请填写 API Key' })
  })

  it('rejects invalid code and url', () => {
    expect('error' in buildModelInput(form({ modelCode: 'bad code' }))).toBe(true)
    expect('error' in buildModelInput(form({ apiUrl: 'ftp://x' }))).toBe(true)
  })

  it('validates extra params as an object', () => {
    expect('error' in buildModelInput(form({ extraParams: '[]' }))).toBe(true)
    const result = buildModelInput(form({ extraParams: '{"voice":"alloy"}' }))
    expect('input' in result && result.input.extraParams).toEqual({ voice: 'alloy' })
  })
})
