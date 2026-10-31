<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import Textarea from 'primevue/textarea'

import type { CloneDraft, CloneDraftAsset, CloneDraftField } from '@/types/clone'
import {
  addCloneAsset,
  canSubmitClone,
  createCloneAsset,
  nextCoverAfterRemoval,
  removeCloneAsset,
  validateCloneAsset,
  validateCloneDraft,
} from '@/utils/clone'
import CloneAssetField from './CloneAssetField.vue'

export interface CloneCreatePayload {
  draft: CloneDraft
  assets: CloneDraftAsset[]
  coverAssetId: string | null
}

const props = defineProps<{ visible: boolean; submitting?: boolean; submitError?: string }>()

const emit = defineEmits<{
  (event: 'update:visible', value: boolean): void
  (event: 'submit', payload: CloneCreatePayload): void
}>()

const draft = ref<CloneDraft>({ name: '', description: '', sourceUrl: '' })
const assets = ref<CloneDraftAsset[]>([])
const coverAssetId = ref<string | null>(null)
const touched = ref<Record<CloneDraftField, boolean>>({
  name: false,
  description: false,
  sourceUrl: false,
})
const assetError = ref('')
const submitted = ref(false)

const open = computed({
  get: () => props.visible,
  set: (value: boolean) => emit('update:visible', value),
})
const errors = computed(() => validateCloneDraft(draft.value))
const submittable = computed(() => canSubmitClone(draft.value))
const images = computed(() => assets.value.filter((asset) => asset.role === 'image'))
const unitTest = computed(() => assets.value.find((asset) => asset.role === 'unit_test') ?? null)
const docs = computed(() => assets.value.find((asset) => asset.role === 'docs') ?? null)

function newId(): string {
  return globalThis.crypto?.randomUUID?.() ?? `clone-asset-${Date.now()}-${Math.random()}`
}

function fieldError(field: CloneDraftField): string {
  if (!touched.value[field] && !draft.value[field].trim()) {
    return ''
  }
  if (field === 'sourceUrl' && !touched.value[field] && !draft.value[field]) {
    return ''
  }
  return errors.value[field] ?? ''
}

function touch(field: CloneDraftField) {
  touched.value[field] = true
}

function revoke(asset: CloneDraftAsset) {
  if (asset.previewUrl) {
    URL.revokeObjectURL(asset.previewUrl)
  }
}

function reset(dispose: boolean) {
  if (dispose) {
    assets.value.forEach(revoke)
  }
  draft.value = { name: '', description: '', sourceUrl: '' }
  assets.value = []
  coverAssetId.value = null
  touched.value = { name: false, description: false, sourceUrl: false }
  assetError.value = ''
  submitted.value = false
}

function addImages(files: File[]) {
  assetError.value = ''
  for (const file of files) {
    const error = validateCloneAsset(file, assets.value, 'image')
    if (error) {
      assetError.value = error
      break
    }
    const asset = createCloneAsset(
      newId(),
      file,
      'image',
      URL.createObjectURL(file),
      file,
    )
    assets.value = addCloneAsset(assets.value, asset)
    if (!coverAssetId.value) {
      coverAssetId.value = asset.id
    }
  }
}

function setUnitTest(file: File) {
  assetError.value = ''
  const error = validateCloneAsset(file, assets.value, 'unit_test')
  if (error) {
    assetError.value = error
    return
  }
  assets.value = removeCloneAsset(assets.value, unitTest.value?.id ?? '')
  assets.value = addCloneAsset(assets.value, createCloneAsset(newId(), file, 'unit_test', undefined, file))
}

function setDocs(file: File) {
  assetError.value = ''
  const error = validateCloneAsset(file, assets.value, 'docs')
  if (error) {
    assetError.value = error
    return
  }
  if (docs.value) {
    revoke(docs.value)
    assets.value = removeCloneAsset(assets.value, docs.value.id)
  }
  assets.value = addCloneAsset(assets.value, createCloneAsset(newId(), file, 'docs', undefined, file))
}

function removeAsset(id: string) {
  const asset = assets.value.find((item) => item.id === id)
  if (asset) {
    revoke(asset)
  }
  coverAssetId.value = nextCoverAfterRemoval(
    assets.value,
    coverAssetId.value,
    id,
  )
  assets.value = removeCloneAsset(assets.value, id)
}

function setCover(id: string) {
  const asset = assets.value.find((item) => item.id === id)
  if (asset?.role === 'image') {
    coverAssetId.value = id
  }
}

function submit() {
  touched.value = { name: true, description: true, sourceUrl: true }
  if (!submittable.value) {
    return
  }
  submitted.value = true
  emit('submit', {
    draft: {
      name: draft.value.name.trim(),
      description: draft.value.description.trim(),
      sourceUrl: draft.value.sourceUrl.trim(),
    },
    assets: assets.value,
    coverAssetId: coverAssetId.value,
  })
}

watch(
  () => props.visible,
  (visible) => {
    if (visible) {
      if (!submitted.value) {
        reset(true)
      } else {
        reset(false)
      }
    } else if (!submitted.value) {
      reset(true)
    }
  },
)

onBeforeUnmount(() => {
  if (!submitted.value) {
    assets.value.forEach(revoke)
  }
})
</script>

<template>
  <Dialog
    v-model:visible="open"
    header="创建克隆"
    modal
    :draggable="false"
    :style="{ width: 'min(720px, calc(100vw - 32px))' }"
    :pt="{ content: { style: 'max-height: 68vh; overflow-y: auto' } }"
  >
    <div class="clone-form">
      <label class="clone-form-field">
        <span>名称 <em>*</em></span>
        <InputText
          v-model="draft.name"
          placeholder="例如：某 SaaS 控制台"
          :maxlength="80"
          @blur="touch('name')"
        />
        <small v-if="fieldError('name')" class="clone-form-error">{{ fieldError('name') }}</small>
      </label>

      <label class="clone-form-field">
        <span>描述 <em>*</em></span>
        <Textarea
          v-model="draft.description"
          rows="4"
          placeholder="说明目标软件是什么、核心功能、希望复刻的范围"
          :maxlength="2000"
          @blur="touch('description')"
        />
        <small v-if="fieldError('description')" class="clone-form-error">
          {{ fieldError('description') }}
        </small>
      </label>

      <label class="clone-form-field">
        <span>目标 URL</span>
        <InputText
          v-model="draft.sourceUrl"
          placeholder="https://example.com（可选，仅作为参考）"
          @blur="touch('sourceUrl')"
        />
        <small v-if="fieldError('sourceUrl')" class="clone-form-error">
          {{ fieldError('sourceUrl') }}
        </small>
      </label>

      <CloneAssetField
        :images="images"
        :cover-asset-id="coverAssetId"
        :unit-test="unitTest"
        :docs="docs"
        @add-images="addImages"
        @remove="removeAsset"
        @set-cover="setCover"
        @set-unit-test="setUnitTest"
        @set-docs="setDocs"
      />

      <p v-if="assetError" class="clone-form-error">{{ assetError }}</p>
      <p v-if="submitError" class="clone-form-error">{{ submitError }}</p>
    </div>

    <template #footer>
      <Button label="取消" text :disabled="submitting" @click="open = false" />
      <Button
        :label="submitting ? '正在创建…' : 'Clone'"
        :disabled="!submittable || submitting"
        @click="submit"
      />
    </template>
  </Dialog>
</template>
