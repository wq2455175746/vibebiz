<script setup lang="ts">
import type { EvolutionVersion } from '@/types/evolution'

defineProps<{
  url: string
  version: EvolutionVersion | null
  refreshKey?: number
}>()
</script>

<template>
  <div class="evolution-preview">
    <div class="evolution-preview-bar">
      <span class="evolution-browser-dots"><i /><i /><i /></span>
      <span class="evolution-preview-address">
        {{ version ? `${version.userName} · ${version.label}` : 'origin_version' }}
      </span>
      <span class="evolution-preview-scenario">{{ version?.scenario ?? 'baseline' }}</span>
    </div>

    <div class="evolution-preview-frame">
      <iframe
        v-if="url"
        :key="`${url}-${refreshKey ?? 0}`"
        :src="url"
        :title="version ? `预览 ${version.label}` : '集思进化预览'"
      />
      <div v-else class="evolution-preview-empty">选择一个版本开始预览</div>
    </div>

    <footer class="evolution-preview-meta">
      <span>{{ version?.summary || '当前线上版本的交互副本。' }}</span>
      <span class="evolution-preview-origin">origin_version · 只读</span>
    </footer>
  </div>
</template>
