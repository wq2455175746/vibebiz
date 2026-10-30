<script setup lang="ts">
import type { AgentRole } from '@/types'

defineProps<{
  roles: AgentRole[]
  selected: number[]
  activeIndex: number
  disabled?: boolean
}>()

const emit = defineEmits<{
  toggle: [role: AgentRole]
}>()
</script>

<template>
  <div class="agent-role-menu" role="listbox" aria-label="选择开发者角色" @mousedown.prevent>
    <button
      v-for="(role, index) in roles"
      :key="role.id"
      type="button"
      class="agent-role-item"
      :class="{ active: activeIndex === index, selected: selected.includes(role.id) }"
      role="option"
      :aria-selected="selected.includes(role.id)"
      :disabled="disabled"
      @click="emit('toggle', role)"
    >
      <span class="agent-role-check">{{ selected.includes(role.id) ? '✓' : '' }}</span>
      <span class="agent-role-name"
        >{{ role.roleNameZh }} <small>{{ role.roleNameEn }}</small></span
      >
      <small class="agent-role-desc">{{ role.description }}</small>
    </button>
  </div>
</template>
