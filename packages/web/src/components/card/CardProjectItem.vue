<script setup lang="ts">
import { CARD_TYPE_LABELS } from '@/utils/card'
import type { CardProjectItem } from '@/types'

const props = defineProps<{ item: CardProjectItem }>()
const emit = defineEmits<{
  open: [item: CardProjectItem]
  remove: [projectId: string]
}>()
</script>

<template>
  <article class="card-project-item" @click="emit('open', props.item)">
    <div class="card-project-cover">
      <img v-if="item.coverUrl" :src="item.coverUrl" :alt="item.name" />
      <span v-else class="card-project-placeholder"><i class="pi pi-image" /></span>
    </div>
    <div class="card-project-body">
      <div class="card-project-title">
        <strong>{{ item.name }}</strong>
        <span class="tag">{{ CARD_TYPE_LABELS[item.type] }}</span>
      </div>
      <p>{{ item.description || '暂无简介' }}</p>
    </div>
    <button
      class="card-project-remove"
      type="button"
      aria-label="移除项目"
      @click.stop="emit('remove', item.projectId)"
    >
      <i class="pi pi-times" />
    </button>
  </article>
</template>
