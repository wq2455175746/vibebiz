import axios from 'axios'

import { cpHttp, dpHttp, request } from './client'
import { deleteStaging, presignResource, uploadStaging, uploadToS3 } from './resources'
import type {
  CloneAssetRole,
  CloneAssetView,
  CloneDraft,
  CloneDraftAsset,
  CloneProject,
  CloneTestResult,
} from '@/types/clone'

export interface CreateCloneAssetsInput {
  coverResourceId?: string
  galleryResourceIds?: string[]
  unitTestResourceId?: string
  docsResourceId?: string
}

export interface CreateCloneInput extends CloneDraft {
  assets: CreateCloneAssetsInput
}

const PURPOSE_BY_ROLE: Record<CloneDraftAsset['role'], string> = {
  image: 'clone_image',
  unit_test: 'clone_unit_test',
  docs: 'clone_docs',
}

export async function listClones(): Promise<CloneProject[]> {
  const result = await request<{ clones: CloneProject[] }>(cpHttp, {
    method: 'get',
    url: '/clones',
  })
  return result.clones ?? []
}

export function getClone(id: string): Promise<CloneProject> {
  return request<CloneProject>(cpHttp, {
    method: 'get',
    url: `/clones/${encodeURIComponent(id)}`,
  })
}

export function createClone(input: CreateCloneInput): Promise<CloneProject> {
  return request<CloneProject>(cpHttp, {
    method: 'post',
    url: '/clones',
    data: input,
  })
}

export async function deleteClone(id: string): Promise<void> {
  await request<{ deleted: boolean }>(cpHttp, {
    method: 'delete',
    url: `/clones/${encodeURIComponent(id)}`,
  })
}

export async function listCloneAssets(projectId: string): Promise<CloneAssetView[]> {
  const result = await request<{ assets: CloneAssetView[] }>(dpHttp, {
    method: 'get',
    url: `/projects/${encodeURIComponent(projectId)}/clone/assets`,
  })
  return result.assets ?? []
}

export function addCloneAsset(
  projectId: string,
  resourceId: string,
  role: Exclude<CloneAssetRole, 'cover'>,
): Promise<CloneAssetView> {
  return request<CloneAssetView>(dpHttp, {
    method: 'post',
    url: `/projects/${encodeURIComponent(projectId)}/clone/assets`,
    data: { resourceId, role },
  })
}

export async function deleteCloneAsset(projectId: string, resourceId: string): Promise<void> {
  await request<{ deleted: boolean }>(dpHttp, {
    method: 'delete',
    url: `/projects/${encodeURIComponent(projectId)}/clone/assets/${encodeURIComponent(resourceId)}`,
  })
}

export function setCloneCover(
  projectId: string,
  resourceId: string,
): Promise<CloneAssetView> {
  return request<CloneAssetView>(dpHttp, {
    method: 'put',
    url: `/projects/${encodeURIComponent(projectId)}/clone/assets/${encodeURIComponent(resourceId)}/cover`,
  })
}

export function runCloneTests(projectId: string): Promise<CloneTestResult> {
  return request<CloneTestResult>(dpHttp, {
    method: 'post',
    url: `/projects/${encodeURIComponent(projectId)}/clone/tests/run`,
  })
}

export async function stageCloneFile(asset: CloneDraftAsset): Promise<string> {
  if (!asset.file) {
    throw new Error(`缺少文件：${asset.name}`)
  }
  const purpose = PURPOSE_BY_ROLE[asset.role]
  const presign = await presignResource({
    filename: asset.name,
    contentType: asset.mime || asset.file.type || '',
    size: asset.size,
    purpose,
  })
  try {
    try {
      await uploadToS3(presign, asset.file)
    } catch {
      await uploadStaging(presign.resourceId, asset.file)
    }
    return presign.resourceId
  } catch (error) {
    await deleteStaging(presign.resourceId).catch(() => undefined)
    if (axios.isAxiosError(error)) {
      throw new Error(error.message)
    }
    throw error
  }
}
