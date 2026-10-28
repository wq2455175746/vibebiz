<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import Password from 'primevue/password'

import { ApiError } from '@/api/client'
import { useOpsAuthStore } from '@/stores/auth'

const route = useRoute()
const router = useRouter()
const auth = useOpsAuthStore()

const identifier = ref('')
const password = ref('')
const error = ref('')

const redirect = computed(() => {
  const raw = route.query.redirect
  const target = Array.isArray(raw) ? raw[0] : raw
  if (typeof target === 'string' && target.startsWith('/') && !target.startsWith('//')) {
    return target
  }
  return '/'
})

async function submit(): Promise<void> {
  error.value = ''
  const account = identifier.value.trim()
  if (!account || !password.value) {
    error.value = '请输入账号和密码'
    return
  }
  try {
    await auth.login(account, password.value)
    await router.replace(redirect.value)
  } catch (err) {
    error.value = err instanceof ApiError ? err.message : '登录失败，请重试'
  }
}
</script>

<template>
  <div class="login-shell">
    <section class="login-panel">
      <div class="login-brand">
        <span class="login-logo"><i class="pi pi-box" /></span>
        <div>
          <strong>Vibebiz Ops</strong>
          <p>运营后台</p>
        </div>
      </div>

      <div class="login-head">
        <h1>管理员登录</h1>
        <p>使用运营账号访问本地管理后台。</p>
      </div>

      <form class="login-form" @submit.prevent="submit">
        <label class="login-field">
          <span>账号或邮箱</span>
          <InputText
            v-model="identifier"
            autocomplete="username"
            placeholder="admin 或 admin_vibebiz@qq.com"
            :disabled="auth.loading"
          />
        </label>

        <label class="login-field">
          <span>密码</span>
          <Password
            v-model="password"
            :feedback="false"
            toggle-mask
            autocomplete="current-password"
            placeholder="请输入密码"
            :disabled="auth.loading"
            input-class="login-password"
          />
        </label>

        <p v-if="error" class="login-error">{{ error }}</p>

        <Button
          type="submit"
          label="登录"
          icon="pi pi-sign-in"
          severity="contrast"
          :loading="auth.loading"
          :disabled="auth.loading"
        />
      </form>

      <p class="login-foot">仅限本地运营使用，生产环境必须接入 SSO + RBAC。</p>
    </section>
  </div>
</template>

<style scoped>
.login-shell {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100%;
  padding: 24px;
  background: var(--canvas);
}

.login-panel {
  width: min(420px, 100%);
  padding: 32px;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 16px;
}

.login-brand {
  display: flex;
  align-items: center;
  gap: 12px;
}

.login-logo {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 38px;
  height: 38px;
  background: var(--ink);
  border-radius: 10px;
  color: var(--surface);
  font-size: 16px;
}

.login-brand strong {
  color: var(--ink);
  font-size: 15px;
  font-weight: 600;
}

.login-brand p {
  margin-top: 2px;
  color: var(--muted);
  font-size: 11px;
  letter-spacing: 0.08em;
}

.login-head {
  margin-top: 30px;
}

.login-head h1 {
  color: var(--ink);
  font-size: 22px;
  font-weight: 650;
  letter-spacing: -0.01em;
}

.login-head p {
  margin-top: 8px;
  color: var(--muted);
  font-size: 13px;
}

.login-form {
  display: flex;
  flex-direction: column;
  gap: 16px;
  margin-top: 26px;
}

.login-field {
  display: flex;
  flex-direction: column;
  gap: 7px;
}

.login-field span {
  color: var(--text);
  font-size: 12px;
  font-weight: 500;
}

.login-field :deep(.p-inputtext),
.login-field :deep(.login-password) {
  width: 100%;
}

.login-field :deep(.p-password) {
  width: 100%;
}

.login-field :deep(.p-inputtext:enabled:focus),
.login-field :deep(.p-password input:enabled:focus) {
  border-color: var(--blue);
  box-shadow: 0 0 0 2px var(--blue-soft);
}

.login-error {
  padding: 10px 12px;
  background: var(--soft);
  border: 1px solid var(--line-strong);
  border-radius: 9px;
  color: var(--ink);
  font-size: 12px;
}

.login-form :deep(.p-button) {
  justify-content: center;
}

.login-foot {
  margin-top: 22px;
  color: var(--muted);
  font-size: 11px;
  line-height: 1.6;
}
</style>
