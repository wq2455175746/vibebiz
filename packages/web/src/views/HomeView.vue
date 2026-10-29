<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'

import { createProject, listProjects } from '@/api/projects'
import HomeDiscovery from '@/components/home/HomeDiscovery.vue'
import HomeHero from '@/components/home/HomeHero.vue'
import HomeSidebar from '@/components/home/HomeSidebar.vue'
import { useNotificationSocket } from '@/composables/useNotificationSocket'
import { useAuthStore } from '@/stores/auth'
import { useNotificationsStore } from '@/stores/notifications'
import '@/styles/home.css'
import type { Project } from '@/types'

const router = useRouter()
const auth = useAuthStore()
const notifications = useNotificationsStore()
const prompt = ref('')
const creating = ref(false)
const error = ref('')
const projects = ref<Project[]>([])
const sidebarCollapsed = ref(localStorage.getItem('vibebiz:sidebar-collapsed') === '1')

const { connect, disconnect } = useNotificationSocket(() => {
  void notifications.load(true)
})

watch(
  () => auth.isAuthenticated,
  (authenticated) => {
    if (authenticated) {
      connect()
    } else {
      disconnect()
      notifications.clear()
    }
  },
)

function toggleSidebar() {
  sidebarCollapsed.value = !sidebarCollapsed.value
  localStorage.setItem('vibebiz:sidebar-collapsed', sidebarCollapsed.value ? '1' : '0')
}

async function create() {
  const text = prompt.value.trim()
  if (!text || creating.value) {
    return
  }
  creating.value = true
  error.value = ''
  try {
    const project = await createProject(text)
    router.push({ name: 'editor', params: { id: project.id } })
  } catch (err) {
    error.value = (err as Error).message
  } finally {
    creating.value = false
  }
}

async function loadProjects() {
  try {
    projects.value = await listProjects()
  } catch {
    projects.value = []
  }
}

onMounted(async () => {
  await auth.ensure()
  if (auth.isAuthenticated) {
    connect()
  }
  await loadProjects()
})
</script>

<template>
  <div class="home-shell" :class="{ 'sidebar-collapsed': sidebarCollapsed }">
    <HomeSidebar
      :recent-projects="projects.slice(0, 4)"
      :collapsed="sidebarCollapsed"
      @toggle-collapse="toggleSidebar"
      @workspace-changed="loadProjects"
    />

    <main class="home-main">
      <div class="home-main-inner">
        <HomeHero v-model="prompt" :creating="creating" :error="error" @submit="create" />
        <HomeDiscovery :projects="projects" />
      </div>
    </main>
  </div>
</template>
