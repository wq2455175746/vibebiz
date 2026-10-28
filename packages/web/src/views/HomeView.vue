<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'

import { createProject, listProjects } from '@/api/projects'
import HomeDiscovery from '@/components/home/HomeDiscovery.vue'
import HomeHero from '@/components/home/HomeHero.vue'
import HomeSidebar from '@/components/home/HomeSidebar.vue'
import '@/styles/home.css'
import type { Project } from '@/types'

const router = useRouter()
const prompt = ref('')
const creating = ref(false)
const error = ref('')
const projects = ref<Project[]>([])

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

onMounted(async () => {
  try {
    projects.value = (await listProjects()).slice(0, 4)
  } catch {
    projects.value = []
  }
})
</script>

<template>
  <div class="home-shell">
    <HomeSidebar :recent-projects="projects" />

    <main class="home-main">
      <div class="home-main-inner">
        <HomeHero v-model="prompt" :creating="creating" :error="error" @submit="create" />
        <HomeDiscovery />
      </div>
    </main>
  </div>
</template>
