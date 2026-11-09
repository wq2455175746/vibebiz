<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { useSuperTeamStore } from '@/stores/superTeam'
import type { SuperStepKey } from '@/types/superTeam'
import { stepTitle } from '@/utils/superTeam'

const props = defineProps<{ step: SuperStepKey }>()
const route = useRoute()
const router = useRouter()
const store = useSuperTeamStore()

const visible = computed(() => route.query.from === 'super-team')

function back(): void {
  void router.push({ name: 'super-team' })
}

function complete(): void {
  store.completeStep(props.step)
  back()
}
</script>

<template>
  <div v-if="visible" class="superteam-context-bar">
    <span class="superteam-context-title">
      <i class="pi pi-users" />
      超级团队 · {{ stepTitle(step) }}
    </span>
    <span class="superteam-context-actions">
      <button type="button" class="superteam-context-btn" @click="back">返回超级团队</button>
      <button type="button" class="superteam-context-btn primary" @click="complete">
        完成本阶段
      </button>
    </span>
  </div>
</template>
