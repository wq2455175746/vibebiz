import axios from 'axios'

import { dpHttp, request } from './client'
import type { ProjectResource } from '@/types'

export interface ResourcePresignInput {
  filename: string
  contentType: string
  size: number
}

export interface ResourcePresign {
  resourceId: string
  uploadUrl: string
  key: string
  headers: Record<string, string>
}

export interface ResourceList {
  resources: ProjectResource[]
  totalBytes: number
}

export async function presignResource(input: ResourcePresignInput): Promise<ResourcePresign> {
  return request<ResourcePresign>(dpHttp, {
    method: 'post',
    url: '/resources/presign',
    data: input,
  })
}

export async function uploadToS3(input: ResourcePresign, file: File): Promise<void> {
  await axios.put(input.uploadUrl, file, { headers: input.headers })
}

export async function uploadStaging(resourceId: string, file: File): Promise<void> {
  await request<{ uploaded: boolean }>(dpHttp, {
    method: 'post',
    url: `/resources/staging/${encodeURIComponent(resourceId)}/upload`,
    data: file,
    headers: { 'Content-Type': file.type || 'application/octet-stream' },
  })
}

export async function adoptResource(
  projectId: string,
  resourceId: string,
): Promise<ProjectResource> {
  return request<ProjectResource>(dpHttp, {
    method: 'post',
    url: `/projects/${encodeURIComponent(projectId)}/resources/adopt`,
    data: { resourceId },
  })
}

export async function listResources(projectId: string): Promise<ResourceList> {
  return request<ResourceList>(dpHttp, {
    method: 'get',
    url: `/projects/${encodeURIComponent(projectId)}/resources`,
  })
}

export async function deleteResource(projectId: string, resourceId: string): Promise<void> {
  await request<{ deleted: boolean }>(dpHttp, {
    method: 'delete',
    url: `/projects/${encodeURIComponent(projectId)}/resources/${encodeURIComponent(resourceId)}`,
  })
}

export async function deleteStaging(resourceId: string): Promise<void> {
  await request<{ deleted: boolean }>(dpHttp, {
    method: 'delete',
    url: `/resources/staging/${encodeURIComponent(resourceId)}`,
  })
}
