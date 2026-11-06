<script setup lang="ts">
import { onBeforeUnmount, ref } from 'vue'
import Toast from 'primevue/toast'

import { onQuotaExceeded, type QuotaExceededInfo } from '@/api/client'
import QuotaExceededDialog from '@/components/QuotaExceededDialog.vue'

const quotaVisible = ref(false)
const quotaInfo = ref<QuotaExceededInfo | null>(null)
const disposeQuota = onQuotaExceeded((info) => {
  quotaInfo.value = info
  quotaVisible.value = true
})
onBeforeUnmount(disposeQuota)
</script>

<template>
  <router-view />
  <Toast />
  <QuotaExceededDialog v-model:visible="quotaVisible" :info="quotaInfo" />
</template>
