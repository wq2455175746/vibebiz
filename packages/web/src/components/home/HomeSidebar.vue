<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import Popover from 'primevue/popover'

import { createFolder, listFolders } from '@/api/settings'
import { useAuthStore } from '@/stores/auth'
import type { Project, WorkspaceFolder } from '@/types'

defineProps<{ recentProjects: Project[]; collapsed?: boolean }>()

const emit = defineEmits<{
  (event: 'workspace-changed'): void
  (event: 'toggle-collapse'): void
}>()

const auth = useAuthStore()
const router = useRouter()

const account = computed(() => {
  const name = auth.user?.username ?? ''
  return {
    name,
    email: auth.user?.email ?? '',
    avatarUrl: auth.user?.avatarUrl ?? null,
    initial: name ? name.charAt(0).toUpperCase() : '?',
  }
})

const workspaces = computed(() => auth.user?.workspaces ?? [])

const currentWorkspace = computed(() => {
  const current = auth.user
  if (!current) {
    return null
  }
  return workspaces.value.find((workspace) => workspace.id === current.currentWorkspaceId) ?? null
})

const accountMenu = ref<InstanceType<typeof Popover> | null>(null)
const workspaceMenu = ref<InstanceType<typeof Popover> | null>(null)
const switching = ref(false)
const workspaceError = ref('')

const folders = ref<WorkspaceFolder[]>([])
const folderFormOpen = ref(false)
const folderName = ref('')
const folderInput = ref<HTMLInputElement | null>(null)
const folderSaving = ref(false)
const folderError = ref('')

const rootFolders = computed(() => folders.value.filter((folder) => folder.parentId === 0))

async function loadFolders() {
  try {
    folders.value = await listFolders()
  } catch {
    folders.value = []
  }
}

watch(
  () => auth.user?.currentWorkspaceId,
  () => {
    closeFolderForm()
    void loadFolders()
  },
  { immediate: true },
)

async function toggleFolderForm() {
  if (folderFormOpen.value) {
    closeFolderForm()
    return
  }
  folderFormOpen.value = true
  folderError.value = ''
  await nextTick()
  folderInput.value?.focus()
}

function closeFolderForm() {
  folderFormOpen.value = false
  folderName.value = ''
  folderError.value = ''
}

async function submitFolder() {
  const name = folderName.value.trim()
  if (!name || folderSaving.value) {
    return
  }
  folderSaving.value = true
  folderError.value = ''
  try {
    const folder = await createFolder(name)
    folders.value = [...folders.value, folder]
    closeFolderForm()
  } catch (err) {
    folderError.value = err instanceof Error ? err.message : '文件夹创建失败'
  } finally {
    folderSaving.value = false
  }
}

function toggleAccount(event: Event) {
  accountMenu.value?.toggle(event)
}

function closeAccount() {
  accountMenu.value?.hide()
}

function toggleWorkspace(event: Event) {
  workspaceError.value = ''
  workspaceMenu.value?.toggle(event)
}

function closeWorkspace() {
  workspaceMenu.value?.hide()
}

async function changeWorkspace(workspaceId: number) {
  if (switching.value) {
    return
  }
  if (workspaceId === auth.user?.currentWorkspaceId) {
    closeWorkspace()
    return
  }
  switching.value = true
  workspaceError.value = ''
  try {
    await auth.switchWorkspace(workspaceId)
    emit('workspace-changed')
    closeWorkspace()
  } catch (err) {
    workspaceError.value = err instanceof Error ? err.message : '工作区切换失败'
  } finally {
    switching.value = false
  }
}

function openSettings() {
  closeAccount()
  void router.push({ name: 'settings-profile' })
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
  <aside class="app-sidebar" :class="{ collapsed }">
    <div class="sidebar-brand">
      <span class="sidebar-logo"><i class="pi pi-box" /></span>
      <span class="sidebar-brand-name">Vibebiz</span>
      <button
        class="sidebar-icon-btn sidebar-collapse"
        type="button"
        :aria-label="collapsed ? '展开侧栏' : '收起侧栏'"
        :aria-expanded="!collapsed"
        :title="collapsed ? '展开侧栏' : '收起侧栏'"
        @click="emit('toggle-collapse')"
      >
        <i :class="collapsed ? 'pi pi-angle-double-right' : 'pi pi-angle-double-left'" />
      </button>
    </div>

    <div class="sidebar-scroll">
      <div class="sidebar-user" :title="collapsed ? account.name : undefined">
        <img
          v-if="account.avatarUrl"
          class="sidebar-avatar avatar-photo"
          :src="account.avatarUrl"
          :alt="account.name"
        />
        <span v-else class="sidebar-avatar">{{ account.initial }}</span>
        <div class="sidebar-user-info">
          <span class="sidebar-user-name">{{ account.name }}</span>
          <button
            class="sidebar-workspace-trigger"
            type="button"
            aria-haspopup="dialog"
            :disabled="switching"
            @click="toggleWorkspace"
          >
            <i class="pi pi-building" />
            <span>{{ currentWorkspace?.name ?? '选择工作区' }}</span>
            <i class="pi pi-angle-down" />
          </button>
        </div>
      </div>

      <nav class="sidebar-nav" aria-label="主导航">
        <button
          v-for="item in navigation"
          :key="item.label"
          class="sidebar-nav-item"
          :class="{ active: item.active }"
          type="button"
          :aria-label="item.label"
          :title="collapsed ? item.label : undefined"
        >
          <i :class="item.icon" />
          <span>{{ item.label }}</span>
        </button>
      </nav>

      <section class="sidebar-group">
        <div class="sidebar-group-head">
          <p class="sidebar-group-title">文件夹</p>
          <button
            class="sidebar-icon-btn sidebar-group-add"
            type="button"
            :aria-label="folderFormOpen ? '取消新建文件夹' : '新建文件夹'"
            :title="folderFormOpen ? '取消新建文件夹' : '新建文件夹'"
            @click="toggleFolderForm"
          >
            <i :class="folderFormOpen ? 'pi pi-times' : 'pi pi-plus'" />
          </button>
        </div>
        <form v-if="folderFormOpen" class="sidebar-folder-form" @submit.prevent="submitFolder">
          <input
            ref="folderInput"
            v-model="folderName"
            class="sidebar-folder-input"
            maxlength="64"
            placeholder="文件夹名称"
            aria-label="文件夹名称"
            :disabled="folderSaving"
            @keydown.esc="closeFolderForm"
          />
          <button
            class="sidebar-folder-submit"
            type="submit"
            aria-label="创建文件夹"
            :disabled="folderSaving || !folderName.trim()"
          >
            <i :class="folderSaving ? 'pi pi-spin pi-spinner' : 'pi pi-check'" />
          </button>
        </form>
        <p v-if="folderError" class="sidebar-folder-error" role="alert">{{ folderError }}</p>
        <p v-if="rootFolders.length === 0 && !folderFormOpen" class="sidebar-empty">暂无文件夹</p>
        <div v-for="folder in rootFolders" :key="folder.id" class="sidebar-folder">
          <i class="pi pi-folder" />
          <span>{{ folder.name }}</span>
        </div>
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
        <button class="sidebar-icon-btn" type="button" aria-label="帮助" title="帮助">
          <i class="pi pi-question-circle" />
        </button>
        <button class="sidebar-icon-btn" type="button" aria-label="通知" title="通知">
          <i class="pi pi-bell" />
        </button>
        <button
          class="sidebar-account-trigger"
          type="button"
          aria-label="账户菜单"
          aria-haspopup="dialog"
          title="账户菜单"
          @click="toggleAccount"
        >
          <img
            v-if="account.avatarUrl"
            class="sidebar-avatar avatar-photo"
            :src="account.avatarUrl"
            :alt="account.name"
          />
          <span v-else class="sidebar-avatar">{{ account.initial }}</span>
        </button>
      </div>

      <Popover ref="accountMenu">
        <div class="account-card">
          <div class="account-head">
            <img
              v-if="account.avatarUrl"
              class="account-avatar avatar-photo"
              :src="account.avatarUrl"
              :alt="account.name"
            />
            <span v-else class="account-avatar">{{ account.initial }}</span>
            <div class="account-identity">
              <strong>{{ account.name }}</strong>
              <span>{{ account.email }}</span>
            </div>
          </div>

          <div class="account-actions">
            <button class="account-action" type="button" @click="openSettings">
              <i class="pi pi-cog" />
              账户设置
            </button>
            <button class="account-action danger" type="button" @click="logout">
              <i class="pi pi-sign-out" />
              退出登录
            </button>
          </div>
        </div>
      </Popover>

      <Popover ref="workspaceMenu">
        <div class="workspace-card">
          <p class="workspace-card-title">切换工作区</p>
          <button
            v-for="workspace in workspaces"
            :key="workspace.id"
            class="workspace-option"
            :class="{ active: workspace.id === auth.user?.currentWorkspaceId }"
            type="button"
            :disabled="switching"
            @click="changeWorkspace(workspace.id)"
          >
            <span class="workspace-option-name">{{ workspace.name }}</span>
            <span class="workspace-option-role">{{ workspace.isOwner ? '所有者' : '编辑者' }}</span>
            <i
              v-if="workspace.id === auth.user?.currentWorkspaceId"
              class="pi pi-check workspace-option-check"
            />
          </button>
          <p v-if="workspaces.length === 0" class="sidebar-empty">暂无工作区</p>
          <p v-if="workspaceError" class="workspace-error" role="alert">{{ workspaceError }}</p>
        </div>
      </Popover>
    </div>
  </aside>
</template>
