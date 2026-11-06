import axios, { type AxiosInstance, type AxiosRequestConfig } from 'axios'

export interface Envelope<T> {
  code: number
  message: string
  result: T
}

export type QuotaType = 'project' | 'token' | 'upload_count' | 'upload_bytes' | 'unknown'

export interface QuotaExceededInfo {
  message: string
  code: number
  quotaType: QuotaType
  used?: number
  limit?: number
}

const QUOTA_CODES = new Set([42901, 42902, 42903, 42904])
let quotaExceededHandler: ((info: QuotaExceededInfo) => void) | null = null

export function onQuotaExceeded(handler: (info: QuotaExceededInfo) => void): () => void {
  quotaExceededHandler = handler
  return () => {
    if (quotaExceededHandler === handler) {
      quotaExceededHandler = null
    }
  }
}

export function notifyQuotaExceeded(
  message: string,
  status: number,
  code: number,
  result?: unknown,
): void {
  if (status !== 429 || !QUOTA_CODES.has(code)) {
    return
  }
  const detail = (result ?? {}) as { quotaType?: QuotaType; used?: number; limit?: number }
  quotaExceededHandler?.({
    message,
    code,
    quotaType: detail.quotaType ?? 'unknown',
    used: detail.used,
    limit: detail.limit,
  })
}

export class ApiError extends Error {
  constructor(
    message: string,
    public readonly status = -1,
    public readonly code = -1,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

const common = { timeout: 30_000, withCredentials: true }

export const cpHttp = axios.create({
  ...common,
  baseURL: import.meta.env.VITE_CP_BASE || '/cp/api/v1',
})

export const dpHttp = axios.create({
  ...common,
  baseURL: import.meta.env.VITE_DP_BASE || '/dp/api/v1',
})

type UnauthorizedHandler = () => void

let unauthorizedHandler: UnauthorizedHandler | null = null

export function onUnauthorized(handler: UnauthorizedHandler): () => void {
  unauthorizedHandler = handler
  return () => {
    if (unauthorizedHandler === handler) {
      unauthorizedHandler = null
    }
  }
}

export function unwrapEnvelope<T>(envelope: Envelope<T>): T {
  if (!envelope || envelope.code !== 0) {
    throw new ApiError(envelope?.message || 'request failed', -1, envelope?.code ?? -1)
  }
  return envelope.result
}

export async function request<T>(client: AxiosInstance, config: AxiosRequestConfig): Promise<T> {
  try {
    const response = await client.request<Envelope<T>>(config)
    return unwrapEnvelope(response.data)
  } catch (error) {
    if (error instanceof ApiError) {
      throw error
    }
    if (axios.isAxiosError(error)) {
      const data = error.response?.data as Partial<Envelope<unknown>> | undefined
      if (error.response?.status === 401) {
        unauthorizedHandler?.()
      }
      const apiError = new ApiError(
        data?.message || error.message,
        error.response?.status ?? -1,
        data?.code ?? -1,
      )
      notifyQuotaExceeded(
        apiError.message,
        apiError.status,
        apiError.code,
        (data as { result?: unknown } | undefined)?.result,
      )
      throw apiError
    }
    throw error
  }
}
