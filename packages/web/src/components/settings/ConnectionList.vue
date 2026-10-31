<script setup lang="ts">
import type { EnterpriseConnection } from '@/types/enterpriseSpace'
import { connectionDescription, maskedCredential } from '@/utils/enterpriseConnections'

defineProps<{
  items: EnterpriseConnection[]
  emptyTitle: string
  emptyText: string
}>()

const emit = defineEmits<{
  (event: 'edit', item: EnterpriseConnection): void
  (event: 'remove', item: EnterpriseConnection): void
}>()

function credential(connection: EnterpriseConnection): string {
  if ('secret' in connection) return maskedCredential(connection.secret)
  if ('kubeconfig' in connection) return maskedCredential(connection.kubeconfig)
  return ''
}
</script>

<template>
  <div v-if="items.length === 0" class="connection-empty">
    <i class="pi pi-box" />
    <strong>{{ emptyTitle }}</strong>
    <span>{{ emptyText }}</span>
  </div>
  <div v-else class="connection-list">
    <article v-for="item in items" :key="item.id" class="connection-card">
      <div class="connection-main">
        <strong>{{ item.name }}</strong>
        <span>{{ connectionDescription(item) }}</span>
      </div>
      <span v-if="credential(item)" class="connection-credential">
        {{ credential(item) }}
      </span>
      <div class="connection-actions">
        <button type="button" class="connection-action" @click="emit('edit', item)">编辑</button>
        <button type="button" class="connection-action danger" @click="emit('remove', item)">
          删除
        </button>
      </div>
    </article>
  </div>
</template>
