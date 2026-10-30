<script setup lang="ts">
import { onMounted, ref } from 'vue'

import {
  createNotificationChannel,
  deleteNotificationChannel,
  listNotificationChannels,
  testNotificationChannel,
  updateNotificationChannel,
} from '@/api/settings'
import NotificationChannelDialog from '@/components/settings/NotificationChannelDialog.vue'
import type { NotificationChannel, NotificationChannelInput } from '@/types'
import { formatDateTime } from '@/utils/storage'
import { typeIcon, typeLabel } from '@/utils/notificationChannels'

const channels = ref<NotificationChannel[]>([])
const loading = ref(true)
const loadError = ref('')
const notice = ref('')
const actionError = ref('')
const dialogOpen = ref(false)
const editing = ref<NotificationChannel | null>(null)
const saving = ref(false)
const dialogError = ref('')
const testingId = ref<number | null>(null)
const togglingId = ref<number | null>(null)

async function load() {
  loading.value = true
  loadError.value = ''
  try {
    channels.value = await listNotificationChannels()
  } catch (err) {
    loadError.value = err instanceof Error ? err.message : '通知渠道加载失败'
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editing.value = null
  dialogError.value = ''
  dialogOpen.value = true
}

function openEdit(channel: NotificationChannel) {
  editing.value = channel
  dialogError.value = ''
  dialogOpen.value = true
}

function closeDialog() {
  if (saving.value) {
    return
  }
  dialogOpen.value = false
  editing.value = null
  dialogError.value = ''
}

async function save(input: NotificationChannelInput) {
  saving.value = true
  dialogError.value = ''
  notice.value = ''
  try {
    if (editing.value) {
      await updateNotificationChannel(editing.value.id, input)
      notice.value = '通知渠道已更新'
    } else {
      await createNotificationChannel(input)
      notice.value = '通知渠道已添加'
    }
    dialogOpen.value = false
    editing.value = null
    await load()
  } catch (err) {
    dialogError.value = err instanceof Error ? err.message : '通知渠道保存失败'
  } finally {
    saving.value = false
  }
}

async function toggle(channel: NotificationChannel) {
  if (togglingId.value !== null) {
    return
  }
  togglingId.value = channel.id
  actionError.value = ''
  notice.value = ''
  try {
    await updateNotificationChannel(channel.id, {
      name: channel.name,
      type: channel.type,
      config: channel.config,
      enabled: !channel.enabled,
    })
    await load()
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '通知渠道更新失败'
  } finally {
    togglingId.value = null
  }
}

async function sendTest(channel: NotificationChannel) {
  if (testingId.value !== null) {
    return
  }
  testingId.value = channel.id
  actionError.value = ''
  notice.value = ''
  try {
    await testNotificationChannel(channel.id)
    notice.value = `测试消息已发送到「${channel.name}」`
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '测试发送失败'
  } finally {
    testingId.value = null
    await load()
  }
}

async function remove(channel: NotificationChannel) {
  if (!window.confirm(`确定删除通知渠道「${channel.name}」吗？`)) {
    return
  }
  actionError.value = ''
  notice.value = ''
  try {
    await deleteNotificationChannel(channel.id)
    notice.value = '通知渠道已删除'
    await load()
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '通知渠道删除失败'
  }
}

onMounted(load)
</script>

<template>
  <section class="settings-page">
    <header class="settings-head">
      <div>
        <h1>通知渠道</h1>
        <p>配置属于你的 Webhook、飞书、企微、钉钉机器人或邮箱，并发送测试消息。</p>
      </div>
      <button class="channel-add" type="button" @click="openCreate">
        <i class="pi pi-plus" />
        添加渠道
      </button>
    </header>

    <p v-if="notice" class="settings-notice" role="status">{{ notice }}</p>
    <p v-if="actionError" class="settings-error" role="alert">{{ actionError }}</p>
    <p v-if="loading" class="settings-state">加载中…</p>
    <p v-else-if="loadError" class="settings-error" role="alert">{{ loadError }}</p>
    <div v-else-if="channels.length === 0" class="settings-state channel-empty">
      <i class="pi pi-bell" />
      <strong>暂无通知渠道</strong>
      <span>添加一个渠道后，可以在这里发送测试消息。</span>
    </div>

    <div v-else class="channel-list">
      <article v-for="channel in channels" :key="channel.id" class="channel-card">
        <span class="channel-icon"><i :class="typeIcon(channel.type)" /></span>
        <div class="channel-body">
          <div class="channel-title">
            <strong>{{ channel.name }}</strong>
            <span class="channel-type">{{ typeLabel(channel.type) }}</span>
            <span class="channel-state" :class="{ disabled: !channel.enabled }">
              {{ channel.enabled ? '已启用' : '已停用' }}
            </span>
          </div>
          <div class="channel-meta">
            <span v-if="channel.type === 'email'">{{ channel.config.to }}</span>
            <span v-else class="channel-url">{{ channel.config.url }}</span>
          </div>
          <div class="channel-test" :class="channel.lastTestStatus">
            <template v-if="channel.lastTestStatus">
              最近测试：{{ channel.lastTestStatus === 'success' ? '成功' : '失败' }} ·
              {{ formatDateTime(channel.lastTestAt) }}
              <span v-if="channel.lastTestError"> · {{ channel.lastTestError }}</span>
            </template>
            <template v-else>尚未测试</template>
          </div>
        </div>
        <div class="channel-actions">
          <button class="channel-action" type="button" @click="openEdit(channel)">编辑</button>
          <button
            class="channel-action"
            type="button"
            :disabled="testingId === channel.id"
            @click="sendTest(channel)"
          >
            <i :class="testingId === channel.id ? 'pi pi-spin pi-spinner' : 'pi pi-send'" />
            测试
          </button>
          <button
            class="channel-action"
            type="button"
            :disabled="togglingId === channel.id"
            @click="toggle(channel)"
          >
            {{ channel.enabled ? '停用' : '启用' }}
          </button>
          <button class="channel-action danger" type="button" @click="remove(channel)">删除</button>
        </div>
      </article>
    </div>

    <NotificationChannelDialog
      v-if="dialogOpen"
      :channel="editing"
      :saving="saving"
      :error="dialogError"
      @close="closeDialog"
      @submit="save"
    />
  </section>
</template>

<style scoped>
.channel-add {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 36px;
  padding: 0 14px;
  background: var(--ink);
  border: 1px solid var(--ink);
  border-radius: 8px;
  color: var(--surface);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
}

.channel-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 7px;
}

.channel-empty > i {
  color: var(--muted);
  font-size: 22px;
}

.channel-empty strong {
  color: var(--ink);
}

.channel-empty span {
  color: var(--muted);
  font-size: 12px;
}

.channel-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.channel-card {
  display: flex;
  align-items: flex-start;
  gap: 14px;
  padding: 18px 20px;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 14px;
}

.channel-icon {
  display: grid;
  place-items: center;
  width: 34px;
  height: 34px;
  flex: none;
  background: var(--soft);
  border: 1px solid var(--line);
  border-radius: 9px;
  color: var(--ink);
  font-size: 14px;
}

.channel-body {
  min-width: 0;
  flex: 1;
}

.channel-title {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}

.channel-title strong {
  color: var(--ink);
  font-size: 14px;
  font-weight: 650;
}

.channel-type,
.channel-state {
  padding: 2px 7px;
  border: 1px solid var(--line);
  border-radius: 999px;
  color: var(--muted);
  font-size: 10px;
}

.channel-state {
  background: var(--blue-soft);
  border-color: var(--line);
  color: var(--blue);
}

.channel-state.disabled {
  background: var(--soft);
  color: var(--muted);
}

.channel-meta {
  margin-top: 7px;
  color: var(--muted);
  font-size: 12px;
}

.channel-url {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.channel-test {
  margin-top: 5px;
  color: var(--muted);
  font-size: 11px;
}

.channel-test.failed {
  color: var(--ink);
}

.channel-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 5px;
  max-width: 300px;
}

.channel-action {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 5px 8px;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 7px;
  color: var(--muted);
  font-size: 11px;
  cursor: pointer;
}

.channel-action:hover {
  background: var(--soft);
  color: var(--ink);
}

.channel-action.danger {
  color: var(--ink);
}

.channel-action:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

@media (max-width: 760px) {
  .channel-card {
    flex-direction: column;
  }

  .channel-actions {
    width: 100%;
    max-width: none;
    justify-content: flex-start;
  }
}
</style>
