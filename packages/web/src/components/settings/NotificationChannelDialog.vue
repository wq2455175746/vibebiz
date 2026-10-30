<script setup lang="ts">
import InputText from 'primevue/inputtext'
import { computed, ref, watch } from 'vue'

import type {
  NotificationChannel,
  NotificationChannelInput,
  NotificationChannelType,
} from '@/types'
import {
  draftToInput,
  emptyChannelDraft,
  notificationChannelTypes,
  validateChannelDraft,
  type ChannelDraft,
} from '@/utils/notificationChannels'

const props = defineProps<{
  channel?: NotificationChannel | null
  saving: boolean
  error: string
}>()

const emit = defineEmits<{
  (event: 'close'): void
  (event: 'submit', input: NotificationChannelInput): void
}>()

const draft = ref<ChannelDraft>(emptyChannelDraft())
const localError = ref('')
const description = computed(
  () => notificationChannelTypes.find((item) => item.value === draft.value.type)?.description ?? '',
)
const needsURL = computed(() => draft.value.type !== 'email')
const needsSecret = computed(() => draft.value.type === 'feishu' || draft.value.type === 'dingtalk')

watch(
  () => props.channel,
  (channel) => {
    localError.value = ''
    if (!channel) {
      draft.value = emptyChannelDraft()
      return
    }
    draft.value = {
      id: channel.id,
      name: channel.name,
      type: channel.type,
      url: channel.config.url ?? '',
      secret: channel.config.secret ?? '',
      to: channel.config.to ?? '',
      enabled: channel.enabled,
    }
  },
  { immediate: true },
)

function changeType(event: Event) {
  draft.value.type = (event.target as HTMLSelectElement).value as NotificationChannelType
  localError.value = ''
}

function submit() {
  const error = validateChannelDraft(draft.value)
  if (error) {
    localError.value = error
    return
  }
  localError.value = ''
  emit('submit', draftToInput(draft.value))
}
</script>

<template>
  <div class="dialog-backdrop" @click.self="emit('close')">
    <form class="dialog-card channel-dialog" @submit.prevent="submit">
      <header class="dialog-head">
        <div>
          <p class="dialog-eyebrow">通知渠道</p>
          <h2>{{ channel ? '编辑渠道' : '添加渠道' }}</h2>
        </div>
        <button class="dialog-close" type="button" aria-label="关闭" @click="emit('close')">
          <i class="pi pi-times" />
        </button>
      </header>

      <label class="channel-field">
        <span>渠道名称</span>
        <InputText
          v-model="draft.name"
          maxlength="64"
          placeholder="例如：研发群"
          :disabled="saving"
        />
      </label>

      <label class="channel-field">
        <span>渠道类型</span>
        <select class="channel-select" :value="draft.type" :disabled="saving" @change="changeType">
          <option v-for="item in notificationChannelTypes" :key="item.value" :value="item.value">
            {{ item.label }}
          </option>
        </select>
        <small>{{ description }}</small>
      </label>

      <label v-if="needsURL" class="channel-field">
        <span>Webhook 地址</span>
        <InputText
          v-model="draft.url"
          placeholder="https://..."
          :disabled="saving"
          autocomplete="off"
        />
      </label>

      <label v-if="needsSecret" class="channel-field">
        <span>签名密钥（可选）</span>
        <InputText
          v-model="draft.secret"
          type="password"
          placeholder="机器人加签密钥"
          :disabled="saving"
          autocomplete="new-password"
        />
      </label>

      <label v-if="draft.type === 'email'" class="channel-field">
        <span>收件邮箱</span>
        <InputText
          v-model="draft.to"
          type="email"
          placeholder="you@example.com"
          :disabled="saving"
          autocomplete="email"
        />
      </label>

      <label class="channel-toggle">
        <input v-model="draft.enabled" type="checkbox" :disabled="saving" />
        <span>启用此渠道</span>
      </label>

      <p v-if="localError || error" class="settings-error" role="alert">
        {{ localError || error }}
      </p>

      <div class="channel-dialog-actions">
        <button class="channel-secondary" type="button" :disabled="saving" @click="emit('close')">
          取消
        </button>
        <button class="channel-primary" type="submit" :disabled="saving">
          <i :class="saving ? 'pi pi-spin pi-spinner' : 'pi pi-check'" />
          {{ saving ? '保存中…' : '保存' }}
        </button>
      </div>
    </form>
  </div>
</template>

<style scoped>
.channel-dialog {
  width: min(560px, 100%);
}

.channel-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  color: var(--ink);
  font-size: 12px;
  font-weight: 600;
}

.channel-field small {
  color: var(--muted);
  font-size: 11px;
  font-weight: 400;
  line-height: 1.5;
}

.channel-field :deep(.p-inputtext) {
  width: 100%;
}

.channel-select {
  width: 100%;
  height: 38px;
  padding: 0 10px;
  background: var(--surface);
  border: 1px solid var(--line-strong);
  border-radius: 8px;
  color: var(--text);
  outline: none;
}

.channel-select:focus {
  border-color: var(--ink);
}

.channel-toggle {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--text);
  font-size: 12px;
}

.channel-toggle input {
  width: 15px;
  height: 15px;
  accent-color: var(--ink);
}

.channel-dialog-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 4px;
}

.channel-primary,
.channel-secondary {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 34px;
  padding: 0 14px;
  border-radius: 8px;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
}

.channel-primary {
  background: var(--ink);
  border: 1px solid var(--ink);
  color: var(--surface);
}

.channel-secondary {
  background: var(--surface);
  border: 1px solid var(--line-strong);
  color: var(--ink);
}

.channel-primary:disabled,
.channel-secondary:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
