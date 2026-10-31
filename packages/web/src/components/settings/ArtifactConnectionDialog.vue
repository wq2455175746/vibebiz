<script setup lang="ts">
import InputText from 'primevue/inputtext'
import { computed, reactive, watch } from 'vue'

import type {
  ArtifactConnection,
  ArtifactConnectionInput,
  ArtifactConnectionKind,
} from '@/types/enterpriseSpace'

const props = defineProps<{
  kind: ArtifactConnectionKind
  connection: ArtifactConnection | null
}>()

const emit = defineEmits<{
  (event: 'close'): void
  (event: 'save', input: ArtifactConnectionInput): void
}>()

const draft = reactive({
  name: '',
  provider: 'github' as 'github' | 'gitlab',
  url: '',
  username: '',
  secret: '',
  defaultBranch: 'main',
  registryUrl: '',
  namespace: '',
  jobsText: '',
})
const error = reactive({ message: '' })

const title = computed(() => {
  if (props.kind === 'code-repo') return '代码仓库'
  if (props.kind === 'harbor') return 'Harbor'
  return 'Jenkins'
})

const hasSavedSecret = computed(
  () => props.connection !== null && props.connection.secret !== '',
)

watch(
  () => [props.kind, props.connection] as const,
  () => {
    const item = props.connection
    draft.name = item?.name ?? ''
    draft.provider = item?.kind === 'code-repo' ? item.provider : 'github'
    draft.url = item?.kind === 'code-repo' || item?.kind === 'jenkins' ? item.url : ''
    draft.username =
      item?.kind === 'code-repo' || item?.kind === 'harbor' || item?.kind === 'jenkins'
        ? item.username
        : ''
    draft.secret = ''
    draft.defaultBranch = item?.kind === 'code-repo' ? item.defaultBranch : 'main'
    draft.registryUrl = item?.kind === 'harbor' ? item.registryUrl : ''
    draft.namespace = item?.kind === 'harbor' ? item.namespace : ''
    draft.jobsText = item?.kind === 'jenkins' ? item.jobs.join('\n') : ''
    error.message = ''
  },
  { immediate: true },
)

function selectValue(event: Event): string {
  return (event.target as HTMLSelectElement).value
}

function submit() {
  if (!draft.name.trim()) {
    error.message = '请填写名称'
    return
  }
  if (props.kind === 'code-repo') {
    if (!draft.url.trim()) {
      error.message = '请填写仓库地址'
      return
    }
    emit('save', {
      kind: 'code-repo',
      name: draft.name.trim(),
      provider: draft.provider,
      url: draft.url.trim(),
      username: draft.username.trim(),
      secret: draft.secret,
      defaultBranch: draft.defaultBranch.trim() || 'main',
    })
    return
  }
  if (props.kind === 'harbor') {
    if (!draft.registryUrl.trim() || !draft.namespace.trim()) {
      error.message = '请填写 Harbor 地址和命名空间'
      return
    }
    emit('save', {
      kind: 'harbor',
      name: draft.name.trim(),
      registryUrl: draft.registryUrl.trim(),
      namespace: draft.namespace.trim(),
      username: draft.username.trim(),
      secret: draft.secret,
    })
    return
  }
  if (!draft.url.trim()) {
    error.message = '请填写 Jenkins 地址'
    return
  }
  emit('save', {
    kind: 'jenkins',
    name: draft.name.trim(),
    url: draft.url.trim(),
    username: draft.username.trim(),
    secret: draft.secret,
    jobs: draft.jobsText
      .split('\n')
      .map((item) => item.trim())
      .filter(Boolean),
  })
}
</script>

<template>
  <div class="dialog-backdrop" @click.self="emit('close')">
    <form class="dialog-card connection-dialog" @submit.prevent="submit">
      <header class="dialog-head">
        <div>
          <p class="dialog-eyebrow">制品空间配置</p>
          <h2>{{ connection ? `编辑${title}` : `新增${title}` }}</h2>
        </div>
        <button class="dialog-close" type="button" aria-label="关闭" @click="emit('close')">
          <i class="pi pi-times" />
        </button>
      </header>

      <label class="connection-field">
        <span>名称</span>
        <InputText v-model="draft.name" maxlength="64" placeholder="例如：主仓库" />
      </label>

      <template v-if="kind === 'code-repo'">
        <label class="connection-field">
          <span>平台</span>
          <select
            :value="draft.provider"
            class="connection-select"
            @change="draft.provider = selectValue($event) as 'github' | 'gitlab'"
          >
            <option value="github">GitHub</option>
            <option value="gitlab">GitLab</option>
          </select>
        </label>
        <label class="connection-field">
          <span>仓库地址</span>
          <InputText v-model="draft.url" placeholder="https://github.com/acme/app" />
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
            :placeholder="hasSavedSecret ? '留空保持已保存的 Token' : 'ghp_...'"
          />
        </label>
        <label class="connection-field">
          <span>默认分支</span>
          <InputText v-model="draft.defaultBranch" placeholder="main" />
        </label>
      </template>

      <template v-else-if="kind === 'harbor'">
        <label class="connection-field">
          <span>Harbor 地址</span>
          <InputText v-model="draft.registryUrl" placeholder="https://harbor.example.com" />
        </label>
        <label class="connection-field">
          <span>命名空间</span>
          <InputText v-model="draft.namespace" placeholder="vibebiz" />
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
          <span>Jenkins 地址</span>
          <InputText v-model="draft.url" placeholder="https://jenkins.example.com" />
        </label>
        <label class="connection-field">
          <span>用户名</span>
          <InputText v-model="draft.username" autocomplete="off" />
        </label>
        <label class="connection-field">
          <span>API Token / 密码</span>
          <InputText
            v-model="draft.secret"
            type="password"
            autocomplete="new-password"
            :placeholder="hasSavedSecret ? '留空保持已保存的 Token' : '请输入 API Token'"
          />
        </label>
        <label class="connection-field">
          <span>任务列表</span>
          <textarea
            v-model="draft.jobsText"
            class="connection-textarea"
            rows="4"
            placeholder="build-and-test&#10;release-build"
          />
          <small>一行一个任务名。</small>
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
