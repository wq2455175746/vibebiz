<script setup lang="ts">
import { computed } from 'vue'

import { useBlobUrl } from '@/composables/useBlobUrl'
import type { CardFeatured, CardProjectItem } from '@/types'
import { CARD_COVER_PLACEHOLDER } from '@/utils/card'

const props = withDefaults(
  defineProps<{ featured: CardFeatured; items: CardProjectItem[]; large?: boolean }>(),
  { large: false },
)

const resolved = computed(() => {
  const featured = props.featured
  if (featured?.kind === 'file') {
    return { kind: featured.fileType, src: featured.url, name: featured.name }
  }
  if (featured?.kind === 'project') {
    const projectId = featured.projectId
    const item = props.items.find((entry) => entry.projectId === projectId)
    if (!item) return null
    return {
      kind: 'image' as const,
      src: item.coverUrl || item.previewUrl || CARD_COVER_PLACEHOLDER,
      name: item.name,
    }
  }
  return null
})

const pdfSource = computed(() => (resolved.value?.kind === 'pdf' ? resolved.value.src : ''))
const pdfUrl = useBlobUrl(pdfSource)
</script>

<template>
  <div class="card-preview" :class="{ large }">
    <template v-if="resolved">
      <div v-if="resolved.kind === 'pdf'" class="card-preview-pdf">
        <iframe :src="pdfUrl" title="PDF 预览" referrerpolicy="no-referrer" />
        <span>{{ resolved.name }}</span>
      </div>
      <img v-else-if="resolved.kind === 'image'" :src="resolved.src" :alt="resolved.name" />
      <div v-else class="card-preview-file">
        <i :class="resolved.kind === 'pdf' ? 'pi pi-file-pdf' : 'pi pi-file'" />
        <span>{{ resolved.name }}</span>
      </div>
    </template>
    <div v-else class="card-preview-empty">
      <i class="pi pi-image" />
      <span>还没有主预览</span>
    </div>
  </div>
</template>
