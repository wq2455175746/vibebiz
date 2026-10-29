import type { ModelInput } from '@/types'

export const MODEL_TYPES = [
  { label: 'LLM 对话', value: 'LLM' },
  { label: 'Embedding', value: 'EMBEDDING' },
  { label: 'Rerank', value: 'RERANK' },
  { label: 'OCR', value: 'OCR' },
  { label: 'ASR 语音识别', value: 'ASR' },
  { label: 'TTS 语音合成', value: 'TTS' },
  { label: 'Other', value: 'OTHER' },
]

export const MODEL_PROVIDERS = [
  { label: 'OpenAI', value: 'OpenAI' },
  { label: 'DeepSeek', value: 'DeepSeek' },
  { label: 'Anthropic', value: 'Anthropic' },
  { label: 'Qwen', value: 'Qwen' },
  { label: 'Other', value: 'Other' },
]

export interface ModelFormState {
  modelName: string
  modelCode: string
  apiUrl: string
  apiKey: string
  providerCode: string
  modelType: string
  maxTokens: number
  isDefault: boolean
  status: number
  extraParams: string
}

const codePattern = /^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$/

export function buildModelInput(
  form: ModelFormState,
): { input: ModelInput } | { error: string } {
  const modelName = form.modelName.trim()
  const modelCode = form.modelCode.trim()
  const apiUrl = form.apiUrl.trim()
  const apiKey = form.apiKey.trim()

  if (!modelName) return { error: '请填写模型名称' }
  if (!modelCode) return { error: '请填写模型编码' }
  if (!codePattern.test(modelCode)) {
    return { error: '模型编码只能包含字母、数字、点、下划线和中划线' }
  }
  if (!apiUrl) return { error: '请填写 API URL' }
  if (!apiUrl.startsWith('http://') && !apiUrl.startsWith('https://')) {
    return { error: 'API URL 必须以 http:// 或 https:// 开头' }
  }
  if (!apiKey) return { error: '请填写 API Key' }

  const maxTokens = form.modelType === 'LLM' ? Number(form.maxTokens) : 0
  if (form.modelType === 'LLM' && (!Number.isFinite(maxTokens) || maxTokens <= 0)) {
    return { error: '最大 tokens 必须大于 0' }
  }

  let extraParams: Record<string, unknown> | undefined
  if (form.extraParams.trim()) {
    try {
      const parsed = JSON.parse(form.extraParams)
      if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
        return { error: 'extra_params 必须是 JSON 对象' }
      }
      extraParams = parsed as Record<string, unknown>
    } catch {
      return { error: 'extra_params 不是合法 JSON' }
    }
  }

  return {
    input: {
      modelName,
      modelCode,
      apiUrl,
      apiKey,
      providerCode: form.providerCode,
      modelType: form.modelType,
      maxTokens,
      isDefault: form.isDefault,
      status: form.status,
      extraParams,
    },
  }
}
