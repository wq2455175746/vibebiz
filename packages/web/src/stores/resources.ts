import { defineStore } from 'pinia'
import { ref } from 'vue'

import {
  adoptResource,
  deleteResource,
  deleteStaging,
  listResources,
  presignResource,
  uploadStaging,
  uploadToS3,
} from '@/api/resources'
import type { ProjectResource } from '@/types'

const MAX_FILE_BYTES = 20 * 1024 * 1024
const MAX_PROJECT_BYTES = 200 * 1024 * 1024

export const useResourceStore = defineStore('resources', () => {
  const items = ref<ProjectResource[]>([])
  const totalBytes = ref(0)
  const loading = ref(false)
  const uploading = ref(false)
  const error = ref('')

  async function load(projectId: string) {
    loading.value = true
    error.value = ''
    try {
      const result = await listResources(projectId)
      items.value = result.resources
      totalBytes.value = result.totalBytes
    } catch (err) {
      error.value = (err as Error).message
    } finally {
      loading.value = false
    }
  }

  async function stage(file: File): Promise<string> {
    if (file.size <= 0 || file.size > MAX_FILE_BYTES) {
      throw new Error('单个文件不能超过 20MB')
    }
    const presign = await presignResource({
      filename: file.name,
      contentType: file.type || '',
      size: file.size,
    })
    try {
      try {
        await uploadToS3(presign, file)
      } catch {
        await uploadStaging(presign.resourceId, file)
      }
      return presign.resourceId
    } catch (err) {
      await deleteStaging(presign.resourceId).catch(() => undefined)
      throw err
    }
  }

  async function adopt(projectId: string, resourceId: string): Promise<ProjectResource> {
    const item = await adoptResource(projectId, resourceId)
    items.value = [...items.value.filter((entry) => entry.id !== item.id), item]
    totalBytes.value = items.value.reduce((sum, entry) => sum + entry.size, 0)
    return item
  }

  async function upload(projectId: string, file: File): Promise<ProjectResource> {
    if (uploading.value) {
      throw new Error('正在上传，请稍后')
    }
    if (totalBytes.value + file.size > MAX_PROJECT_BYTES) {
      throw new Error('项目资料总量不能超过 200MB')
    }
    uploading.value = true
    error.value = ''
    try {
      const resourceId = await stage(file)
      return await adopt(projectId, resourceId)
    } catch (err) {
      error.value = (err as Error).message
      throw err
    } finally {
      uploading.value = false
    }
  }

  async function remove(projectId: string, resourceId: string) {
    await deleteResource(projectId, resourceId)
    items.value = items.value.filter((item) => item.id !== resourceId)
    totalBytes.value = items.value.reduce((sum, item) => sum + item.size, 0)
  }

  function reset() {
    items.value = []
    totalBytes.value = 0
    loading.value = false
    uploading.value = false
    error.value = ''
  }

  return { items, totalBytes, loading, uploading, error, load, stage, adopt, upload, remove, reset }
})
