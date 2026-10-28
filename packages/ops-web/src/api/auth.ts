import { request } from './client'

import type { OpsUser } from '@/types'

export function login(identifier: string, password: string): Promise<OpsUser> {
  return request<OpsUser>({
    method: 'post',
    url: '/auth/login',
    data: { identifier, password },
  })
}

export function me(): Promise<OpsUser> {
  return request<OpsUser>({
    method: 'get',
    url: '/auth/me',
  })
}

export async function logout(): Promise<void> {
  await request<null>({
    method: 'post',
    url: '/auth/logout',
  })
}
