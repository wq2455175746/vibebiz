<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import Toast from 'primevue/toast'
import { useToast } from 'primevue/usetoast'

import { onQuotaExceeded, type QuotaExceededInfo } from '@/api/client'
import QuotaExceededDialog from '@/components/QuotaExceededDialog.vue'
import { useNotificationSocket } from '@/composables/useNotificationSocket'
import { useAuthStore } from '@/stores/auth'
import { useNotificationsStore } from '@/stores/notifications'
import { newNotifications } from '@/utils/newNotifications'

const auth = useAuthStore()
const notifications = useNotificationsStore()
const toast = useToast()
let baselineReady = false

const { connect, disconnect } = useNotificationSocket(async () => {
  const previous = notifications.items
  await notifications.load(true)
  if (!baselineReady) {
    baselineReady = true
    return
  }
  for (const item of newNotifications(previous, notifications.items)) {
    const severity: 'error' | 'info' = item.title.includes('禁用') ? 'error' : 'info'
    toast.add({
      severity,
      summary: item.title,
      detail: item.content,
      life: 10_000,
    })
  }
})

watch(
  () => auth.isAuthenticated,
  async (authenticated) => {
    if (authenticated) {
      await notifications.load(true)
      baselineReady = true
      connect()
      return
    }
    baselineReady = false
    disconnect()
    notifications.clear()
  },
  { immediate: true },
)

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
