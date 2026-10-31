import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import {
  addCloneAsset,
  createClone,
  deleteClone,
  deleteCloneAsset,
  getClone,
  listClones,
  setCloneCover,
  stageCloneFile,
} from '@/api/clones'
import type { CloneDraft, CloneDraftAsset, CloneProject } from '@/types/clone'
import { createCloneAsset } from '@/utils/clone'

function newId(): string {
  return globalThis.crypto?.randomUUID?.() ?? `clone-${Date.now()}-${Math.random()}`
}

export const useCloneProjectsStore = defineStore('cloneProjects', () => {
  const projects = ref<CloneProject[]>([])
  const current = ref<CloneProject | null>(null)
  const loading = ref(false)
  const creating = ref(false)
  const uploading = ref(false)
  const error = ref('')

  const currentId = computed(() => current.value?.id ?? null)

  function upsert(project: CloneProject) {
    const index = projects.value.findIndex((item) => item.id === project.id)
    if (index >= 0) {
      projects.value = projects.value.map((item) => (item.id === project.id ? project : item))
    } else {
      projects.value = [project, ...projects.value]
    }
    if (current.value?.id === project.id || !current.value) {
      current.value = project
    }
  }

  async function load() {
    loading.value = true
    error.value = ''
    try {
      projects.value = await listClones()
      if (current.value && !projects.value.some((item) => item.id === current.value?.id)) {
        current.value = null
      }
    } catch (err) {
      error.value = (err as Error).message
      projects.value = []
    } finally {
      loading.value = false
    }
  }

  async function loadOne(id: string): Promise<CloneProject | null> {
    loading.value = true
    error.value = ''
    try {
      const project = await getClone(id)
      upsert(project)
      current.value = project
      return project
    } catch (err) {
      error.value = (err as Error).message
      current.value = null
      return null
    } finally {
      loading.value = false
    }
  }

  async function create(
    draft: CloneDraft,
    assets: CloneDraftAsset[],
    coverAssetId: string | null,
  ): Promise<CloneProject> {
    creating.value = true
    error.value = ''
    try {
      const resourceByLocal = new Map<string, string>()
      for (const asset of assets) {
        resourceByLocal.set(asset.id, await stageCloneFile(asset))
      }
      const galleryResourceIds = assets
        .filter((asset) => asset.role === 'image' && asset.id !== coverAssetId)
        .map((asset) => resourceByLocal.get(asset.id))
        .filter((id): id is string => Boolean(id))
      const unitTest = assets.find((asset) => asset.role === 'unit_test')
      const docs = assets.find((asset) => asset.role === 'docs')
      const project = await createClone({
        name: draft.name.trim(),
        description: draft.description.trim(),
        sourceUrl: draft.sourceUrl.trim(),
        assets: {
          coverResourceId: coverAssetId
            ? resourceByLocal.get(coverAssetId)
            : undefined,
          galleryResourceIds,
          unitTestResourceId: unitTest ? resourceByLocal.get(unitTest.id) : undefined,
          docsResourceId: docs ? resourceByLocal.get(docs.id) : undefined,
        },
      })
      upsert(project)
      current.value = project
      return project
    } catch (err) {
      error.value = (err as Error).message
      throw err
    } finally {
      creating.value = false
    }
  }

  async function remove(id: string) {
    error.value = ''
    try {
      await deleteClone(id)
      projects.value = projects.value.filter((project) => project.id !== id)
      if (current.value?.id === id) {
        current.value = null
      }
    } catch (err) {
      error.value = (err as Error).message
      throw err
    }
  }

  async function refresh(id: string) {
    const project = await getClone(id)
    upsert(project)
    if (current.value?.id === id) {
      current.value = project
    }
    return project
  }

  async function addAsset(
    projectId: string,
    file: File,
    role: CloneDraftAsset['role'],
  ): Promise<void> {
    uploading.value = true
    error.value = ''
    try {
      const existing = current.value?.id === projectId ? current.value : await getClone(projectId)
      const asset = createCloneAsset(newId(), file, role, undefined, file)
      const resourceId = await stageCloneFile(asset)
      const backendRole = role === 'image' ? 'gallery' : role
      await addCloneAsset(projectId, resourceId, backendRole)
      if (role === 'image' && !existing.coverUrl) {
        await setCloneCover(projectId, resourceId)
      }
      await refresh(projectId)
    } catch (err) {
      error.value = (err as Error).message
      throw err
    } finally {
      uploading.value = false
    }
  }

  async function removeAsset(projectId: string, resourceId: string) {
    uploading.value = true
    error.value = ''
    try {
      await deleteCloneAsset(projectId, resourceId)
      await refresh(projectId)
    } catch (err) {
      error.value = (err as Error).message
      throw err
    } finally {
      uploading.value = false
    }
  }

  async function setCover(projectId: string, resourceId: string) {
    uploading.value = true
    error.value = ''
    try {
      await setCloneCover(projectId, resourceId)
      await refresh(projectId)
    } catch (err) {
      error.value = (err as Error).message
      throw err
    } finally {
      uploading.value = false
    }
  }

  function reset() {
    projects.value = []
    current.value = null
    loading.value = false
    creating.value = false
    uploading.value = false
    error.value = ''
  }

  return {
    projects,
    current,
    currentId,
    loading,
    creating,
    uploading,
    error,
    load,
    loadOne,
    create,
    remove,
    refresh,
    addAsset,
    removeAsset,
    setCover,
    reset,
  }
})
