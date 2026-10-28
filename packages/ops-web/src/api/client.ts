import axios, { type AxiosRequestConfig } from 'axios'

export interface Envelope<T> {
  code: number
  message: string
  result: T
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

export const http = axios.create({
  baseURL: import.meta.env.VITE_OPS_API_BASE || '/cp/api/v1/admin',
  timeout: 30_000,
})

export function unwrapEnvelope<T>(envelope: Envelope<T>): T {
  if (!envelope || envelope.code !== 0) {
    throw new ApiError(envelope?.message || 'request failed', -1, envelope?.code ?? -1)
  }
  return envelope.result
}

export async function request<T>(config: AxiosRequestConfig): Promise<T> {
  try {
    const response = await http.request<Envelope<T>>(config)
    return unwrapEnvelope(response.data)
  } catch (error) {
    if (error instanceof ApiError) {
      throw error
    }
    if (axios.isAxiosError(error)) {
      const data = error.response?.data as Partial<Envelope<unknown>> | undefined
      throw new ApiError(
        data?.message || error.message,
        error.response?.status ?? -1,
        data?.code ?? -1,
      )
    }
    throw error
  }
}
