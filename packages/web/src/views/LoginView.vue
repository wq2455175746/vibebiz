<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import Password from 'primevue/password'

import { sanitizeRedirect } from '@/router/redirect'
import { useAuthStore } from '@/stores/auth'
import '@/styles/home.css'

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()

const identifier = ref('')
const password = ref('')
const error = ref('')
const submitting = ref(false)

async function submit() {
  const name = identifier.value.trim()
  if (!name || !password.value || submitting.value) {
    return
  }
  submitting.value = true
  error.value = ''
  try {
    await auth.login(name, password.value)
    await router.replace(sanitizeRedirect(route.query.redirect))
  } catch (err) {
    error.value = err instanceof Error ? err.message : '登录失败，请稍后重试'
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="login-shell">
    <form class="login-card" @submit.prevent="submit">
      <div class="login-brand">
        <span class="login-logo"><i class="pi pi-box" /></span>
        <span class="login-brand-name">Vibebiz</span>
      </div>

      <div class="login-head">
        <h1 class="login-title">登录</h1>
        <p class="login-subtitle">使用用户名或邮箱继续</p>
      </div>

      <div class="login-field">
        <label for="login-identifier">用户名 / 邮箱</label>
        <InputText
          id="login-identifier"
          v-model="identifier"
          autocomplete="username"
          placeholder="用户名或邮箱"
          :disabled="submitting"
        />
      </div>

      <div class="login-field">
        <label for="login-password">密码</label>
        <Password
          id="login-password"
          v-model="password"
          :feedback="false"
          toggle-mask
          autocomplete="current-password"
          placeholder="密码"
          :disabled="submitting"
          input-class="login-password-input"
        />
      </div>

      <p v-if="error" class="login-error" role="alert">{{ error }}</p>

      <Button
        class="login-submit"
        type="submit"
        label="登录"
        :loading="submitting"
        :disabled="!identifier.trim() || !password"
      />
    </form>
  </div>
</template>
