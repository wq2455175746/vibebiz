<script setup lang="ts">
import InputText from 'primevue/inputtext'
import { computed, reactive, watch } from 'vue'

import type {
  DeliveryConnection,
  DeliveryConnectionInput,
  DeliveryConnectionKind,
} from '@/types/enterpriseSpace'

const props = defineProps<{
  kind: DeliveryConnectionKind
  connection: DeliveryConnection | null
}>()

const emit = defineEmits<{
  (event: 'close'): void
  (event: 'save', input: DeliveryConnectionInput): void
}>()

const draft = reactive({
  name: '',
  apiServer: '',
  kubeconfig: '',
  defaultNamespace: 'default',
  host: '',
  port: 22,
  username: '',
  secret: '',
  url: '',
})
const error = reactive({ message: '' })

const title = computed(() => {
  if (props.kind === 'k8s') return 'K8s 集群'
  if (props.kind === 'cloud') return '云主机'
  return 'Argo'
})

const hasSavedSecret = computed(
  () => props.connection !== null && 'secret' in props.connection && props.connection.secret !== '',
)
const hasSavedKubeconfig = computed(
  () =>
    props.connection !== null &&
    props.connection.kind === 'k8s' &&
    props.connection.kubeconfig !== '',
)

watch(
  () => [props.kind, props.connection] as const,
  () => {
    const item = props.connection
    draft.name = item?.name ?? ''
    draft.apiServer = item?.kind === 'k8s' ? item.apiServer : ''
    draft.kubeconfig = ''
    draft.defaultNamespace = item?.kind === 'k8s' ? item.defaultNamespace : 'default'
    draft.host = item?.kind === 'cloud' ? item.host : ''
    draft.port = item?.kind === 'cloud' ? item.port : 22
    draft.username = item?.kind === 'cloud' || item?.kind === 'argo' ? item.username : ''
    draft.secret = ''
    draft.url = item?.kind === 'argo' ? item.url : ''
    error.message = ''
  },
  { immediate: true },
)

function submit() {
  if (!draft.name.trim()) {
    error.message = '请填写名称'
    return
  }
  if (props.kind === 'k8s') {
    if (!draft.apiServer.trim()) {
      error.message = '请填写 apiserver 地址'
      return
    }
    emit('save', {
      kind: 'k8s',
      name: draft.name.trim(),
      apiServer: draft.apiServer.trim(),
      kubeconfig: draft.kubeconfig,
      defaultNamespace: draft.defaultNamespace.trim() || 'default',
    })
    return
  }
  if (props.kind === 'cloud') {
    if (!draft.host.trim() || !draft.username.trim()) {
      error.message = '请填写主机地址和用户名'
      return
    }
    emit('save', {
      kind: 'cloud',
      name: draft.name.trim(),
      host: draft.host.trim(),
      port: Number(draft.port) || 22,
      username: draft.username.trim(),
      secret: draft.secret,
    })
    return
  }
  if (!draft.url.trim()) {
    error.message = '请填写 Argo 地址'
    return
  }
  emit('save', {
    kind: 'argo',
    name: draft.name.trim(),
    url: draft.url.trim(),
    username: draft.username.trim(),
    secret: draft.secret,
  })
}
</script>

<template>
  <div class="dialog-backdrop" @click.self="emit('close')">
    <form class="dialog-card connection-dialog" @submit.prevent="submit">
      <header class="dialog-head">
        <div>
          <p class="dialog-eyebrow">交付空间配置</p>
          <h2>{{ connection ? `编辑${title}` : `新增${title}` }}</h2>
        </div>
        <button class="dialog-close" type="button" aria-label="关闭" @click="emit('close')">
          <i class="pi pi-times" />
        </button>
      </header>

      <label class="connection-field">
        <span>名称</span>
        <InputText v-model="draft.name" maxlength="64" placeholder="例如：生产集群" />
      </label>

      <template v-if="kind === 'k8s'">
        <label class="connection-field">
          <span>apiserver 地址</span>
          <InputText v-model="draft.apiServer" placeholder="https://kubernetes.example.com:6443" />
        </label>
        <label class="connection-field">
          <span>kubeconfig</span>
          <textarea
            v-model="draft.kubeconfig"
            class="connection-textarea"
            rows="6"
            :placeholder="hasSavedKubeconfig ? '留空保持已保存的 kubeconfig' : '粘贴 kubeconfig 内容'"
          />
        </label>
        <label class="connection-field">
          <span>默认命名空间</span>
          <InputText v-model="draft.defaultNamespace" placeholder="default" />
        </label>
      </template>

      <template v-else-if="kind === 'cloud'">
        <label class="connection-field">
          <span>主机地址</span>
          <InputText v-model="draft.host" placeholder="10.0.0.10" />
        </label>
        <label class="connection-field">
          <span>SSH 端口</span>
          <input v-model.number="draft.port" class="connection-select" type="number" min="1" />
        </label>
        <label class="connection-field">
          <span>用户名</span>
          <InputText v-model="draft.username" autocomplete="off" />
        </label>
        <label class="connection-field">
          <span>密码</span>
          <InputText
            v-model="draft.secret"
            type="password"
            autocomplete="new-password"
            :placeholder="hasSavedSecret ? '留空保持已保存的密码' : '请输入密码'"
          />
        </label>
      </template>

      <template v-else>
        <label class="connection-field">
          <span>Argo 地址</span>
          <InputText v-model="draft.url" placeholder="https://argo.example.com" />
        </label>
        <label class="connection-field">
          <span>用户名</span>
          <InputText v-model="draft.username" autocomplete="off" />
        </label>
        <label class="connection-field">
          <span>密码 / Token</span>
          <InputText
            v-model="draft.secret"
            type="password"
            autocomplete="new-password"
            :placeholder="hasSavedSecret ? '留空保持已保存的凭据' : '请输入密码或 Token'"
          />
        </label>
      </template>

      <p v-if="error.message" class="settings-error" role="alert">{{ error.message }}</p>

      <div class="connection-dialog-actions">
        <button class="connection-secondary" type="button" @click="emit('close')">取消</button>
        <button class="connection-primary" type="submit">
          <i class="pi pi-check" />
          保存
        </button>
      </div>
    </form>
  </div>
</template>
