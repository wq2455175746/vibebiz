<script setup lang="ts">
import type { EvolutionVersionGroup } from '@/types/evolution'
import {
  OFFICIAL_USER_ID,
  formatRelative,
  formatShortTime,
  initialOf,
  statusLabel,
} from '@/utils/evolution'

defineProps<{
  groups: EvolutionVersionGroup[]
  selectedVersionId: string | null
  currentUserId: string
}>()

const emit = defineEmits<{
  (event: 'select', versionId: string): void
}>()

function select(versionId: string): void {
  emit('select', versionId)
}
</script>

<template>
  <aside class="evolution-rail">
    <header class="evolution-rail-head">
      <strong>用户版本</strong>
      <span>{{ groups.length }} 位用户</span>
    </header>

    <div class="evolution-rail-list">
      <section
        v-for="group in groups"
        :key="group.userId"
        class="evolution-rail-group"
        :class="{
          'is-current': group.userId === currentUserId,
          'is-official': group.userId === OFFICIAL_USER_ID,
        }"
      >
        <button
          type="button"
          class="evolution-rail-user"
          :class="{ active: group.latest.id === selectedVersionId }"
          @click="select(group.latest.id)"
        >
          <span class="evolution-avatar">{{ initialOf(group.userName) }}</span>
          <span class="evolution-rail-user-info">
            <strong>
              {{ group.userName }}
              <em v-if="group.userId === currentUserId">你</em>
              <em v-else-if="group.userId === OFFICIAL_USER_ID">官方</em>
            </strong>
            <span>{{ group.latest.label }} · {{ formatRelative(group.latest.updatedAt) }}</span>
          </span>
          <span class="evolution-status" :class="group.latest.status">
            {{ statusLabel(group.latest.status) }}
          </span>
        </button>

        <div
          v-if="group.versions.length > 1 && group.userId !== OFFICIAL_USER_ID"
          class="evolution-rail-history"
        >
          <button
            v-for="version in group.versions"
            :key="version.id"
            type="button"
            :class="{ active: version.id === selectedVersionId }"
            @click="select(version.id)"
          >
            <span>{{ version.label }}</span>
            <time>{{ formatShortTime(version.createdAt) }}</time>
          </button>
        </div>

        <p v-if="group.userId === OFFICIAL_USER_ID" class="evolution-rail-note">
          CI 构建产物，只读基线
        </p>
      </section>
    </div>
  </aside>
</template>
