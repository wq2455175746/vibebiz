<script setup lang="ts">
import { computed } from 'vue'

import type { SuperStepState } from '@/types/superTeam'
import { stepDescription, stepStatusLabel, stepTitle } from '@/utils/superTeam'

const props = defineProps<{ step: SuperStepState; index: number; busy?: boolean }>()
const emit = defineEmits<{ open: [] }>()

const locked = computed(() => props.step.status === 'locked')
const actionLabel = computed(() => {
  if (props.busy) {
    return '处理中…'
  }
  if (props.step.status === 'ready') {
    return '开始'
  }
  if (props.step.status === 'active') {
    return '继续'
  }
  if (props.step.status === 'done') {
    return '查看'
  }
  return '未解锁'
})

function open(): void {
  if (!locked.value && !props.busy) {
    emit('open')
  }
}
</script>

<template>
  <article class="super-step-card" :class="`is-${step.status}`">
    <header class="super-step-head">
      <span class="super-step-index">{{ index + 1 }}</span>
      <span class="tag">{{ stepStatusLabel(step.status) }}</span>
    </header>
    <h3>{{ stepTitle(step.key) }}</h3>
    <p>{{ stepDescription(step.key) }}</p>
    <div class="super-step-progress" :aria-label="`进度 ${step.progress}%`">
      <span :style="{ width: `${step.progress}%` }" />
    </div>
    <small class="super-step-summary">{{ step.summary }}</small>
    <button class="super-step-action" type="button" :disabled="locked || busy" @click="open">
      {{ actionLabel }}
    </button>
  </article>
</template>
