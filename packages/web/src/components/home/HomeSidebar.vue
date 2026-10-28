<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import Popover from 'primevue/popover'

import { useAuthStore } from '@/stores/auth'
import type { Project } from '@/types'

defineProps<{ recentProjects: Project[] }>()

const auth = useAuthStore()
const router = useRouter()

const account = computed(() => {
  const name = auth.user?.username ?? ''
  return {
    name,
    email: auth.user?.email ?? '',
    initial: name ? name.charAt(0).toUpperCase() : '?',
  }
})

const accountMenu = ref<InstanceType<typeof Popover> | null>(null)

function toggleAccount(event: Event) {
  accountMenu.value?.toggle(event)
}

function closeAccount() {
  accountMenu.value?.hide()
}

async function logout() {
  closeAccount()
  await auth.logout()
  await router.push({ name: 'login' })
}

const navigation = [
  { label: '首页', icon: 'pi pi-home', active: true },
  { label: '资源', icon: 'pi pi-book', active: false },
  { label: '我的项目', icon: 'pi pi-folder', active: false },
  { label: '定时任务', icon: 'pi pi-clock', active: false },
]
</script>

<template>
  <aside class="app-sidebar">
    <div class="sidebar-brand">
      <span class="sidebar-logo"><i class="pi pi-box" /></span>
      <span class="sidebar-brand-name">Vibebiz</span>
      <button class="sidebar-icon-btn sidebar-collapse" type="button" aria-label="收起侧栏">
        <i class="pi pi-angle-double-left" />
      </button>
    </div>

    <div class="sidebar-scroll">
      <div class="sidebar-user">
        <span class="sidebar-avatar">{{ account.initial }}</span>
        <span class="sidebar-user-name">{{ account.name }}</span>
      </div>

      <nav class="sidebar-nav" aria-label="主导航">
        <button
          v-for="item in navigation"
          :key="item.label"
          class="sidebar-nav-item"
          :class="{ active: item.active }"
          type="button"
        >
          <i :class="item.icon" />
          <span>{{ item.label }}</span>
        </button>
      </nav>

      <section class="sidebar-group">
        <p class="sidebar-group-title">文件夹</p>
        <button class="sidebar-nav-item" type="button">
          <i class="pi pi-folder" />
          <span>test</span>
        </button>
      </section>

      <section class="sidebar-group">
        <p class="sidebar-group-title">最近</p>
        <p v-if="recentProjects.length === 0" class="sidebar-empty">暂无对话记录</p>
        <button
          v-for="project in recentProjects"
          :key="project.id"
          class="sidebar-recent"
          type="button"
        >
          {{ project.name }}
        </button>
      </section>
    </div>

    <div class="sidebar-footer">
      <div class="sidebar-tools">
        <button class="sidebar-icon-btn" type="button" aria-label="帮助">
          <i class="pi pi-question-circle" />
        </button>
        <button class="sidebar-icon-btn" type="button" aria-label="通知">
          <i class="pi pi-bell" />
        </button>
        <button
          class="sidebar-account-trigger"
          type="button"
          aria-label="账户菜单"
          aria-haspopup="dialog"
          @click="toggleAccount"
        >
          <span class="sidebar-avatar">{{ account.initial }}</span>
        </button>
      </div>

      <Popover ref="accountMenu">
        <div class="account-card">
          <div class="account-head">
            <span class="account-avatar">{{ account.initial }}</span>
            <div class="account-identity">
              <strong>{{ account.name }}</strong>
              <span>{{ account.email }}</span>
            </div>
          </div>

          <div class="account-actions">
            <button class="account-action" type="button" @click="closeAccount">
              <i class="pi pi-user" />
              个人主页
            </button>
            <button class="account-action" type="button" @click="closeAccount">
              <i class="pi pi-cog" />
              设置
            </button>
            <button class="account-action danger" type="button" @click="logout">
              <i class="pi pi-sign-out" />
              退出登录
            </button>
          </div>
        </div>
      </Popover>
    </div>
  </aside>
</template>
