import { dpHttp, request } from './client'
import type { FileContent, FileEntry, WriteResult } from '@/types'

export async function listFiles(projectId: string): Promise<FileEntry[]> {
  const result = await request<{ files: FileEntry[] }>(dpHttp, {
    method: 'get',
    url: `/projects/${encodeURIComponent(projectId)}/files`,
  })
  return result.files
}

export async function readFile(projectId: string, path: string): Promise<FileContent> {
  return request<FileContent>(dpHttp, {
    method: 'get',
    url: `/projects/${encodeURIComponent(projectId)}/file`,
    params: { path },
  })
}

export async function writeFile(
  projectId: string,
  path: string,
  content: string,
  baseVersion: string,
): Promise<WriteResult> {
  return request<WriteResult>(dpHttp, {
    method: 'put',
    url: `/projects/${encodeURIComponent(projectId)}/file`,
    data: { path, content, baseVersion },
  })
}
