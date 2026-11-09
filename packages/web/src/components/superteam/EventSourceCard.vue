<script setup lang="ts">
import { computed } from 'vue'

import type { EventSourceConfig } from '@/types/superTeam'
import { sourceMeta, sourceStatusLabel } from '@/utils/superTeam'

const props = defineProps<{ source: EventSourceConfig }>()
const emit = defineEmits<{ configure: [] }>()

const meta = computed(() => sourceMeta(props.source.kind))
const disabled = computed(() => props.source.status === 'coming_soon')
const statusLabel = computed(() => sourceStatusLabel(props.source.status))
</script>

<template>
  <button
    type="button"
    class="super-source-card"
    :class="{ 'is-disabled': disabled, 'is-connected': source.status === 'connected' }"
    :disabled="disabled"
    :aria-label="`${meta.label}事件入口`"
    @click="emit('configure')"
  >
    <span class="super-source-icon"><i :class="meta.icon" /></span>
    <span class="super-source-body">
      <strong>{{ meta.label }}</strong>
      <small>{{ meta.description }}</small>
    </span>
    <span class="super-source-foot">
      <span class="tag" :class="{ 'tag-accent': source.status === 'connected' }">
        {{ statusLabel }}
      </span>
      <span class="super-source-count">{{ source.eventCount }} 条事件</span>
    </span>
  </button>
</template>
