<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'

import { useCloneProjectsStore } from '@/stores/cloneProjects'
import type { CloneAssetView, CloneDraftAsset } from '@/types/clone'
import { validateCloneAsset } from '@/utils/clone'
import CloneAssetField from './CloneAssetField.vue'

const props = defineProps<{ visible: boolean; projectId: string }>()

const emit = defineEmits<{
  (event: 'update:visible', value: boolean): void
}>()

const store = useCloneProjectsStore()
const error = ref('')

const open = computed({
  get: () => props.visible,
  set: (value: boolean) => emit('update:visible', value),
})
const project = computed(() => store.current)
const imageAssets = computed<CloneAssetView[]>(
  () =>
    project.value?.assets.filter(
      (asset) => asset.role === 'cover' || asset.role === 'gallery',
    ) ?? [],
)
const unitTestAsset = computed(
  () => project.value?.assets.find((asset) => asset.role === 'unit_test') ?? null,
)
const allDocs = computed(
  () => project.value?.assets.filter((asset) => asset.role === 'docs') ?? [],
)
const firstDocs = computed(() => allDocs.value[0] ?? null)
const coverAssetId = computed(
  () => project.value?.assets.find((asset) => asset.role === 'cover')?.resourceId ?? null,
)

function toDraftImage(asset: CloneAssetView): CloneDraftAsset {
  return {
    id: asset.resourceId,
    name: asset.displayName || asset.name,
    size: asset.size,
    mime: asset.contentType,
    role: 'image',
  }
}

function toDraftAsset(asset: CloneAssetView | null, role: 'unit_test' | 'docs'): CloneDraftAsset | null {
  if (!asset) {
    return null
  }
  return {
    id: asset.resourceId,
    name: asset.displayName || asset.name,
    size: asset.size,
    mime: asset.contentType,
    role,
  }
}

const images = computed(() => imageAssets.value.map(toDraftImage))
const unitTest = computed(() => toDraftAsset(unitTestAsset.value, 'unit_test'))
const docs = computed(() => toDraftAsset(firstDocs.value, 'docs'))

async function addImages(files: File[]) {
  error.value = ''
  for (const file of files) {
    const message = validateCloneAsset(file, images.value, 'image')
    if (message) {
      error.value = message
      break
    }
    try {
      await store.addAsset(props.projectId, file, 'image')
    } catch (err) {
      error.value = (err as Error).message
      break
    }
  }
}

async function setUnitTest(file: File) {
  error.value = ''
  const message = validateCloneAsset(file, unitTest.value ? [unitTest.value] : [], 'unit_test')
  if (message) {
    error.value = message
    return
  }
  try {
    await store.addAsset(props.projectId, file, 'unit_test')
  } catch (err) {
    error.value = (err as Error).message
  }
}

async function setDocs(file: File) {
  error.value = ''
  const message = validateCloneAsset(file, [], 'docs')
  if (message) {
    error.value = message
    return
  }
  try {
    await store.addAsset(props.projectId, file, 'docs')
  } catch (err) {
    error.value = (err as Error).message
  }
}

async function remove(assetId: string) {
  error.value = ''
  try {
    await store.removeAsset(props.projectId, assetId)
  } catch (err) {
    error.value = (err as Error).message
  }
}

async function setCover(assetId: string) {
  error.value = ''
  try {
    await store.setCover(props.projectId, assetId)
  } catch (err) {
    error.value = (err as Error).message
  }
}

watch(
  () => props.visible,
  (visible) => {
    if (visible) {
      error.value = ''
      if (store.current?.id !== props.projectId) {
        void store.loadOne(props.projectId)
      }
    }
  },
)
</script>

<template>
  <Dialog
    v-model:visible="open"
    header="补充附件"
    modal
    :draggable="false"
    :style="{ width: 'min(680px, calc(100vw - 32px))' }"
    :pt="{ content: { style: 'max-height: 66vh; overflow-y: auto' } }"
  >
    <CloneAssetField
      :images="images"
      :cover-asset-id="coverAssetId"
      :unit-test="unitTest"
      :docs="docs"
      @add-images="addImages"
      @remove="remove"
      @set-cover="setCover"
      @set-unit-test="setUnitTest"
      @set-docs="setDocs"
    />

    <div v-if="allDocs.length > 0" class="clone-doc-list">
      <strong>已上传文档</strong>
      <div v-for="doc in allDocs" :key="doc.id" class="clone-doc-item">
        <span>{{ doc.displayName || doc.name }}</span>
        <button type="button" aria-label="删除文档" @click="remove(doc.resourceId)">
          <i class="pi pi-times" />
        </button>
      </div>
    </div>

    <p v-if="error" class="clone-form-error">{{ error }}</p>
    <p v-if="store.uploading" class="clone-asset-hint">正在上传并同步资料…</p>

    <template #footer>
      <Button label="完成" :disabled="store.uploading" @click="open = false" />
    </template>
  </Dialog>
</template>
