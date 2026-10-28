import type { AuthUser } from '@/types'

import { cpHttp, request } from './client'

export function login(identifier: string, password: string): Promise<AuthUser> {
  return request<AuthUser>(cpHttp, {
    method: 'post',
    url: '/auth/login',
    data: { identifier, password },
  })
}

export function me(): Promise<AuthUser> {
  return request<AuthUser>(cpHttp, { method: 'get', url: '/auth/me' })
}

export async function logout(): Promise<void> {
  await request<unknown>(cpHttp, { method: 'post', url: '/auth/logout' })
}
