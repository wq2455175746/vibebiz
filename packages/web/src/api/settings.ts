import type { AuthUser, Member, Presign, Workspace, WorkspaceFolder } from '@/types'

import { cpHttp, request } from './client'

export { switchWorkspace } from './auth'

export interface PresignInput {
  filename: string
  contentType: string
  size: number
}

export function getProfile(): Promise<AuthUser> {
  return request<AuthUser>(cpHttp, { method: 'get', url: '/me/profile' })
}

export function updateProfile(username: string): Promise<AuthUser> {
  return request<AuthUser>(cpHttp, {
    method: 'put',
    url: '/me/profile',
    data: { username },
  })
}

export function presignAvatar(input: PresignInput): Promise<Presign> {
  return request<Presign>(cpHttp, {
    method: 'post',
    url: '/me/avatar/presign',
    data: input,
  })
}

export function confirmAvatar(key: string): Promise<AuthUser> {
  return request<AuthUser>(cpHttp, {
    method: 'put',
    url: '/me/avatar',
    data: { key },
  })
}

export function uploadAvatar(file: File): Promise<AuthUser> {
  return request<AuthUser>(cpHttp, {
    method: 'post',
    url: '/me/avatar/upload',
    data: file,
    headers: { 'Content-Type': file.type },
    timeout: 60_000,
  })
}

export function getCurrentWorkspace(): Promise<Workspace> {
  return request<Workspace>(cpHttp, { method: 'get', url: '/workspaces/current' })
}

export function updateCurrentWorkspace(name: string, description: string): Promise<Workspace> {
  return request<Workspace>(cpHttp, {
    method: 'put',
    url: '/workspaces/current',
    data: { name, description },
  })
}

export function presignWorkspaceAvatar(input: PresignInput): Promise<Presign> {
  return request<Presign>(cpHttp, {
    method: 'post',
    url: '/workspaces/current/avatar/presign',
    data: input,
  })
}

export function confirmWorkspaceAvatar(key: string): Promise<Workspace> {
  return request<Workspace>(cpHttp, {
    method: 'put',
    url: '/workspaces/current/avatar',
    data: { key },
  })
}

export function uploadWorkspaceAvatar(file: File): Promise<Workspace> {
  return request<Workspace>(cpHttp, {
    method: 'post',
    url: '/workspaces/current/avatar/upload',
    data: file,
    headers: { 'Content-Type': file.type },
    timeout: 60_000,
  })
}

export async function listMembers(): Promise<Member[]> {
  const result = await request<{ members: Member[] }>(cpHttp, {
    method: 'get',
    url: '/workspaces/current/members',
  })
  return result.members
}

export function inviteMember(email: string): Promise<Member> {
  return request<Member>(cpHttp, {
    method: 'post',
    url: '/workspaces/current/members',
    data: { email },
  })
}

export async function listFolders(): Promise<WorkspaceFolder[]> {
  const result = await request<{ folders: WorkspaceFolder[] }>(cpHttp, {
    method: 'get',
    url: '/workspaces/current/folders',
  })
  return result.folders
}

export function createFolder(name: string, parentId = 0): Promise<WorkspaceFolder> {
  return request<WorkspaceFolder>(cpHttp, {
    method: 'post',
    url: '/workspaces/current/folders',
    data: { name, parentId },
  })
}
