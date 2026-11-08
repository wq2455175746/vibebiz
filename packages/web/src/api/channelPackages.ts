import { cpHttp, request } from './client'
import type {
  ChannelGuide,
  ChannelPackage,
  ChannelPackageChannel,
  ChannelPackageSourceType,
} from '@/types'

export interface GenerateChannelPackageInput {
  channel: ChannelPackageChannel
  sourceType: ChannelPackageSourceType
  resourceId?: string
}

export interface DownloadedPackage {
  blob: Blob
  filename: string
}

export async function generateChannelPackage(
  projectId: string,
  input: GenerateChannelPackageInput,
): Promise<ChannelPackage> {
  const result = await request<{ package: ChannelPackage }>(cpHttp, {
    method: 'post',
    url: `/projects/${encodeURIComponent(projectId)}/channel-packages`,
    data: input,
  })
  return result.package
}

export async function listChannelPackages(projectId: string): Promise<ChannelPackage[]> {
  const result = await request<{ packages: ChannelPackage[] }>(cpHttp, {
    method: 'get',
    url: `/projects/${encodeURIComponent(projectId)}/channel-packages`,
  })
  return result.packages
}

export async function deleteChannelPackage(projectId: string, packageId: string): Promise<void> {
  await request<{ deleted: boolean }>(cpHttp, {
    method: 'delete',
    url: `/projects/${encodeURIComponent(projectId)}/channel-packages/${encodeURIComponent(packageId)}`,
  })
}

export async function listChannelGuides(): Promise<ChannelGuide[]> {
  const result = await request<{ guides: ChannelGuide[] }>(cpHttp, {
    method: 'get',
    url: '/channel-guides',
  })
  return result.guides
}

export async function downloadChannelPackage(
  projectId: string,
  packageId: string,
): Promise<DownloadedPackage> {
  const base = import.meta.env.VITE_CP_BASE || '/cp/api/v1'
  const response = await fetch(
    `${base}/projects/${encodeURIComponent(projectId)}/channel-packages/${encodeURIComponent(packageId)}/download`,
    { credentials: 'include' },
  )
  if (!response.ok) {
    throw new Error(`下载失败（${response.status}）`)
  }
  const disposition = response.headers.get('content-disposition') || ''
  const match = /filename="?([^";]+)"?/.exec(disposition)
  return {
    blob: await response.blob(),
    filename: match?.[1] || `${projectId}-channel.zip`,
  }
}
