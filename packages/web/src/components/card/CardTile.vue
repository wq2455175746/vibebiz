<script setup lang="ts">
import CardPreview from './CardPreview.vue'
import type { BusinessCard } from '@/types'

const props = defineProps<{ card: BusinessCard }>()
const emit = defineEmits<{ open: [id: string] }>()
const initial = (props.card.ownerName || '?').charAt(0).toUpperCase()
</script>

<template>
  <article class="card-tile" @click="emit('open', card.id)">
    <div class="card-tile-preview">
      <CardPreview :featured="card.featured" :items="card.items" />
    </div>
    <div class="card-tile-body">
      <div class="card-tile-person">
        <img
          v-if="card.avatarUrl"
          class="card-tile-avatar"
          :src="card.avatarUrl"
          :alt="card.ownerName"
        />
        <span v-else class="card-tile-avatar">{{ initial }}</span>
        <div>
          <strong>{{ card.ownerName }}</strong>
          <p>{{ card.headline || '这个人还没有写简介' }}</p>
        </div>
      </div>
      <div class="card-tile-tags">
        <span v-for="tag in card.tags.slice(0, 3)" :key="tag" class="tag">{{ tag }}</span>
        <span v-if="card.tags.length > 3" class="tag">+{{ card.tags.length - 3 }}</span>
      </div>
      <footer class="card-tile-foot">
        <span>{{ card.items.length }} 个项目</span>
        <span><i class="pi pi-comment" /> {{ card.commentCount }}</span>
      </footer>
    </div>
  </article>
</template>
