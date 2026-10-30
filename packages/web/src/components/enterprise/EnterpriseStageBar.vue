<script setup lang="ts">
import type { EnterpriseStageKey, EnterpriseStageStateMap } from '@/utils/enterprise'
import { ENTERPRISE_STAGES } from '@/utils/enterprise'

const props = defineProps<{
  states: EnterpriseStageStateMap
  activeKey: EnterpriseStageKey | null
  viewingKey: EnterpriseStageKey | null
}>()

const emit = defineEmits<{
  (event: 'select', key: EnterpriseStageKey): void
}>()

function selectable(key: EnterpriseStageKey): boolean {
  const status = props.states[key].status
  return status === 'done' || status === 'active'
}

function select(key: EnterpriseStageKey): void {
  if (selectable(key)) {
    emit('select', key)
  }
}
</script>

<template>
  <nav class="enterprise-stage-bar" aria-label="企业工程阶段进度">
    <button
      v-for="stage in ENTERPRISE_STAGES"
      :key="stage.key"
      class="enterprise-stage-step"
      :class="[
        `status-${states[stage.key].status}`,
        {
          viewed: viewingKey ? viewingKey === stage.key : activeKey === stage.key,
        },
      ]"
      type="button"
      :disabled="!selectable(stage.key)"
      :aria-current="activeKey === stage.key ? 'step' : undefined"
      @click="select(stage.key)"
    >
      <span class="enterprise-stage-index">
        <i v-if="states[stage.key].status === 'done'" class="pi pi-check" />
        <i v-else-if="states[stage.key].status === 'placeholder'" class="pi pi-lock" />
        <span v-else>{{ stage.seq }}</span>
      </span>
      <span class="enterprise-stage-copy">
        <strong>{{ stage.title }}</strong>
        <small>{{ stage.roleLabel }}</small>
      </span>
      <span v-if="stage.key === activeKey" class="enterprise-stage-pulse">进行中</span>
    </button>
  </nav>
</template>
