<script setup lang="ts">
import { computed, ref } from 'vue'

import type { CloneDraftAsset } from '@/types/clone'
import { CLONE_IMAGE_MAX, formatCloneSize } from '@/utils/clone'

const props = defineProps<{
  images: CloneDraftAsset[]
  coverAssetId: string | null
  unitTest: CloneDraftAsset | null
  docs: CloneDraftAsset | null
}>()

const emit = defineEmits<{
  (event: 'add-images', files: File[]): void
  (event: 'remove', id: string): void
  (event: 'set-cover', id: string): void
  (event: 'set-unit-test', file: File): void
  (event: 'set-docs', file: File): void
}>()

const imageInput = ref<HTMLInputElement | null>(null)
const zipInput = ref<HTMLInputElement | null>(null)
const docsInput = ref<HTMLInputElement | null>(null)
const dragging = ref(false)

const imageCountLabel = computed(() => `${props.images.length}/${CLONE_IMAGE_MAX}`)
const canAddImage = computed(() => props.images.length < CLONE_IMAGE_MAX)

function pickImages() {
  imageInput.value?.click()
}

function readImages(event: Event) {
  const input = event.target as HTMLInputElement
  const files = Array.from(input.files ?? [])
  input.value = ''
  if (files.length > 0) {
    emit('add-images', files)
  }
}

function dropImages(event: DragEvent) {
  dragging.value = false
  const files = Array.from(event.dataTransfer?.files ?? []).filter((file) =>
    file.type.startsWith('image/'),
  )
  if (files.length > 0) {
    emit('add-images', files)
  }
}

function readZip(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (file) {
    emit('set-unit-test', file)
  }
}

function readDocs(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (file) {
    emit('set-docs', file)
  }
}
</script>

<template>
  <section class="clone-assets">
    <div class="clone-asset-block">
      <div class="clone-asset-head">
        <strong>参考图片</strong>
        <span>{{ imageCountLabel }} · 1 张主图</span>
      </div>
      <div
        class="clone-image-grid"
        :class="{ dragging }"
        @dragover.prevent="dragging = true"
        @dragleave.prevent="dragging = false"
        @drop.prevent="dropImages"
      >
        <div
          v-for="image in images"
          :key="image.id"
          class="clone-image-tile"
          :class="{ cover: image.id === coverAssetId }"
          role="button"
          tabindex="0"
          :title="image.id === coverAssetId ? '当前主图' : '点击设为主图'"
          @click="emit('set-cover', image.id)"
          @keydown.enter="emit('set-cover', image.id)"
        >
          <img v-if="image.previewUrl" :src="image.previewUrl" :alt="image.name" />
          <span v-else class="clone-image-placeholder"><i class="pi pi-image" /></span>
          <span v-if="image.id === coverAssetId" class="clone-image-badge">主图</span>
          <button
            class="clone-image-remove"
            type="button"
            aria-label="删除图片"
            @click.stop="emit('remove', image.id)"
          >
            <i class="pi pi-times" />
          </button>
        </div>

        <button
          v-if="canAddImage"
          class="clone-image-add"
          :class="{ dragging }"
          type="button"
          @click="pickImages"
        >
          <i class="pi pi-plus" />
          <span>添加图片</span>
          <small>支持拖拽</small>
        </button>
      </div>
      <input
        ref="imageInput"
        class="clone-file-input"
        type="file"
        accept="image/jpeg,image/png,image/webp,image/gif"
        multiple
        @change="readImages"
      />
      <p v-if="images.length === 0" class="clone-asset-hint">
        可选，最多 9 张。第一张图片默认作为主图，点击缩略图可切换。
      </p>
    </div>

    <div class="clone-asset-row">
      <div class="clone-asset-row-copy">
        <strong>单元测试压缩包</strong>
        <span v-if="unitTest">{{ unitTest.name }} · {{ formatCloneSize(unitTest.size) }}</span>
        <span v-else>可选，1 个 .zip，最大 100MB</span>
      </div>
      <button v-if="!unitTest" class="clone-asset-action" type="button" @click="zipInput?.click()">
        <i class="pi pi-upload" />
        选择 zip
      </button>
      <button v-else class="clone-asset-action" type="button" @click="emit('remove', unitTest.id)">
        <i class="pi pi-trash" />
        移除
      </button>
      <input
        ref="zipInput"
        class="clone-file-input"
        type="file"
        accept=".zip,application/zip,application/x-zip-compressed"
        @change="readZip"
      />
    </div>

    <div class="clone-asset-row">
      <div class="clone-asset-row-copy">
        <strong>文档说明</strong>
        <span v-if="docs">{{ docs.name }} · {{ formatCloneSize(docs.size) }}</span>
        <span v-else>可选，1 个文件，最大 100MB</span>
      </div>
      <button v-if="!docs" class="clone-asset-action" type="button" @click="docsInput?.click()">
        <i class="pi pi-upload" />
        选择文档
      </button>
      <button v-else class="clone-asset-action" type="button" @click="emit('remove', docs.id)">
        <i class="pi pi-trash" />
        移除
      </button>
      <input ref="docsInput" class="clone-file-input" type="file" @change="readDocs" />
    </div>
  </section>
</template>
