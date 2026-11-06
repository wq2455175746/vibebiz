<script setup lang="ts">
import { computed } from 'vue'
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'

import type { QuotaExceededInfo } from '@/api/client'
import { quotaDescription, quotaUsage } from '@/utils/quota'

const props = defineProps<{ visible: boolean; info: QuotaExceededInfo | null }>()
const emit = defineEmits<{ (event: 'update:visible', value: boolean): void }>()

const description = computed(() => quotaDescription(props.info))
const usage = computed(() => quotaUsage(props.info))
</script>

<template>
  <Dialog
    :visible="props.visible"
    header="额度已用完"
    modal
    :style="{ width: '420px' }"
    @update:visible="emit('update:visible', $event)"
  >
    <div class="quota-dialog">
      <p class="quota-dialog__desc">{{ description }}</p>
      <p v-if="usage" class="quota-dialog__usage">{{ usage }}</p>
      <p class="quota-dialog__hint">请联系管理员调整配额后再试。</p>
    </div>
    <template #footer>
      <Button label="我知道了" @click="emit('update:visible', false)" />
    </template>
  </Dialog>
</template>

<style scoped>
.quota-dialog {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.quota-dialog__desc {
  margin: 0;
  color: var(--text);
  font-size: 14px;
}

.quota-dialog__usage {
  margin: 0;
  color: var(--muted);
  font-size: 13px;
}

.quota-dialog__hint {
  margin: 0;
  color: var(--muted);
  font-size: 13px;
}
</style>
