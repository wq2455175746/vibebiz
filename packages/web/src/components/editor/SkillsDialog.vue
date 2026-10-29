<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'

import { listSkills, type SkillInfo } from '@/api/skills'

const props = defineProps<{ projectId: string }>()
const emit = defineEmits<{ close: [] }>()

const skills = ref<SkillInfo[]>([])
const loading = ref(true)
const error = ref('')

const sourceLabel: Record<SkillInfo['source'], string> = {
  user: '用户',
  project: '项目',
  plugin: '插件',
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    skills.value = await listSkills(props.projectId)
  } catch (err) {
    error.value = (err as Error).message
  } finally {
    loading.value = false
  }
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    emit('close')
  }
}

onMounted(() => {
  void load()
  document.addEventListener('keydown', onKeydown)
})

onBeforeUnmount(() => {
  document.removeEventListener('keydown', onKeydown)
})
</script>

<template>
  <div class="dialog-backdrop" @click.self="emit('close')">
    <section class="dialog-card skills-dialog" role="dialog" aria-modal="true" aria-label="Skills 列表">
      <header class="dialog-head">
        <div>
          <p class="dialog-eyebrow">AGENT SKILLS</p>
          <h2>当前可用 Skills</h2>
        </div>
        <button type="button" class="dialog-close" aria-label="关闭" @click="emit('close')">
          <i class="pi pi-times" />
        </button>
      </header>

      <p v-if="loading" class="dialog-state">正在读取 skills…</p>
      <div v-else-if="error" class="dialog-state">
        <p>{{ error }}</p>
        <button type="button" class="dialog-retry" @click="load">重试</button>
      </div>
      <p v-else-if="!skills.length" class="dialog-state">当前 sandbox 未发现 skills</p>
      <ul v-else class="skill-list">
        <li v-for="skill in skills" :key="skill.name" class="skill-item">
          <div class="skill-item-head">
            <strong>{{ skill.name }}</strong>
            <span class="tag">{{ sourceLabel[skill.source] }}</span>
          </div>
          <p v-if="skill.description">{{ skill.description }}</p>
        </li>
      </ul>
    </section>
  </div>
</template>
