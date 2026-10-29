<script setup lang="ts">
import { computed, watch } from 'vue'
import Dialog from 'primevue/dialog'
import Tab from 'primevue/tab'
import TabList from 'primevue/tablist'
import TabPanel from 'primevue/tabpanel'
import TabPanels from 'primevue/tabpanels'
import Tabs from 'primevue/tabs'

import { useNotificationsStore } from '@/stores/notifications'
import type { NotificationItem } from '@/types'
import { renderMarkdown } from '@/utils/markdown'
import { groupNotifications } from '@/utils/notificationGroups'
import { safeNotificationLink } from '@/utils/notificationLink'
import { formatDateTime } from '@/utils/storage'

const props = defineProps<{ visible: boolean }>()
const emit = defineEmits<{ (event: 'update:visible', value: boolean): void }>()

const store = useNotificationsStore()

const groups = computed(() => groupNotifications(store.items))

watch(
  () => props.visible,
  (visible) => {
    if (visible) {
      void store.load(store.items.length > 0)
    }
  },
)

function linkFor(item: NotificationItem): string {
  return safeNotificationLink(item.link)
}

function close(): void {
  emit('update:visible', false)
}
</script>

<template>
  <Dialog
    :visible="visible"
    modal
    header="通知"
    :style="{ width: '640px', maxWidth: 'calc(100vw - 32px)' }"
    @update:visible="close"
  >
    <div v-if="store.loading" class="notifications-state">加载中…</div>
    <div v-else-if="store.error" class="notifications-state error-text">{{ store.error }}</div>
    <Tabs v-else value="version_release">
      <TabList>
        <Tab value="version_release">版本发布</Tab>
        <Tab value="inbox">收集箱</Tab>
      </TabList>
      <TabPanels>
        <TabPanel value="version_release">
          <p v-if="groups.version_release.length === 0" class="notifications-empty">暂无消息</p>
          <article
            v-for="item in groups.version_release"
            :key="item.id"
            class="notification-item"
          >
            <div class="notification-head">
              <h3 class="notification-title">{{ item.title }}</h3>
              <span v-if="item.version" class="tag">v{{ item.version }}</span>
            </div>
            <p class="notification-time">{{ formatDateTime(item.publishedAt) }}</p>
            <div class="notification-content" v-html="renderMarkdown(item.content)" />
            <a
              v-if="linkFor(item)"
              class="notification-link"
              :href="linkFor(item)"
              target="_blank"
              rel="noopener noreferrer"
            >
              查看详情
              <i class="pi pi-external-link" />
            </a>
          </article>
        </TabPanel>
        <TabPanel value="inbox">
          <p v-if="groups.inbox.length === 0" class="notifications-empty">暂无消息</p>
          <article v-for="item in groups.inbox" :key="item.id" class="notification-item">
            <div class="notification-head">
              <h3 class="notification-title">{{ item.title }}</h3>
              <span v-if="item.version" class="tag">v{{ item.version }}</span>
            </div>
            <p class="notification-time">{{ formatDateTime(item.publishedAt) }}</p>
            <div class="notification-content" v-html="renderMarkdown(item.content)" />
            <a
              v-if="linkFor(item)"
              class="notification-link"
              :href="linkFor(item)"
              target="_blank"
              rel="noopener noreferrer"
            >
              查看详情
              <i class="pi pi-external-link" />
            </a>
          </article>
        </TabPanel>
      </TabPanels>
    </Tabs>
  </Dialog>
</template>

<style scoped>
.notifications-state {
  padding: 24px 0;
  text-align: center;
}

.notifications-empty {
  padding: 32px 0;
  color: var(--muted);
  text-align: center;
}

.notification-item {
  padding: 16px 0;
  border-bottom: 1px solid var(--line);
}

.notification-item:last-child {
  border-bottom: 0;
}

.notification-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.notification-title {
  margin: 0;
  color: var(--ink);
  font-size: 15px;
  font-weight: 600;
}

.notification-time {
  margin: 4px 0 10px;
  color: var(--muted);
  font-size: 12px;
}

.notification-content {
  color: var(--text);
  font-size: 13px;
  line-height: 1.7;
  overflow-wrap: anywhere;
}

.notification-content :deep(p) {
  margin: 0 0 8px;
}

.notification-content :deep(a) {
  color: var(--blue);
}

.notification-content :deep(pre) {
  padding: 12px;
  background: var(--soft);
  border-radius: 8px;
  overflow-x: auto;
}

.notification-content :deep(code) {
  font-size: 12px;
}

.notification-link {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin-top: 10px;
  color: var(--blue);
  font-size: 13px;
  text-decoration: none;
}

.notification-link:hover {
  text-decoration: underline;
}
</style>
