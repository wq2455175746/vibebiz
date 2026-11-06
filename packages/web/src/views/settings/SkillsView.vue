<script setup lang="ts">
import { onMounted, ref } from 'vue'
import FileUpload from 'primevue/fileupload'
import type { FileUploadSelectEvent } from 'primevue/fileupload'

import { deleteMySkill, listMySkills, uploadMySkill, type SkillInfo } from '@/api/skills'
import { formatDateTime } from '@/utils/storage'

const skills = ref<SkillInfo[]>([])
const loading = ref(true)
const loadError = ref('')
const actionError = ref('')
const notice = ref('')
const selected = ref<File | null>(null)
const overwrite = ref(false)
const uploading = ref(false)
const deleting = ref('')
const fileUpload = ref<{ clear: () => void } | null>(null)

const sourceLabel: Record<SkillInfo['source'], string> = {
  user: '我的',
  project: '项目',
  plugin: '插件',
  public: '公共',
}

async function load() {
  loading.value = true
  loadError.value = ''
  try {
    skills.value = await listMySkills()
  } catch (err) {
    loadError.value = err instanceof Error ? err.message : '技能加载失败'
  } finally {
    loading.value = false
  }
}

function chooseFile(event: FileUploadSelectEvent) {
  selected.value = (event.files[0] as File | undefined) ?? null
  actionError.value = ''
  notice.value = ''
}

async function upload() {
  if (!selected.value) {
    actionError.value = '请选择 ZIP 技能包'
    return
  }
  uploading.value = true
  actionError.value = ''
  notice.value = ''
  try {
    await uploadMySkill(selected.value, overwrite.value)
    notice.value = '技能已上传，下一轮对话生效'
    selected.value = null
    overwrite.value = false
    fileUpload.value?.clear()
    await load()
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '技能上传失败'
  } finally {
    uploading.value = false
  }
}

async function remove(skill: SkillInfo) {
  if (!window.confirm(`确定删除技能「${skill.name}」吗？`)) {
    return
  }
  deleting.value = skill.name
  actionError.value = ''
  notice.value = ''
  try {
    await deleteMySkill(skill.name)
    notice.value = '技能已删除'
    await load()
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '删除技能失败'
  } finally {
    deleting.value = ''
  }
}

onMounted(load)
</script>

<template>
  <section class="settings-page">
    <header class="settings-head">
      <div>
        <h1>我的技能</h1>
        <p class="settings-desc">
          上传 ZIP 技能包到当前账号，所有项目可用；公共技能由管理员维护，只读。
        </p>
      </div>
      <span class="tag">{{ skills.length }} 个</span>
    </header>

    <div class="settings-card">
      <h2 class="settings-card-title">上传技能</h2>
      <p class="settings-note">
        ZIP 内需要包含一个顶层技能目录和 SKILL.md；技能名使用小写字母、数字、短横线或下划线。
      </p>
      <div class="skill-upload">
        <FileUpload
          ref="fileUpload"
          mode="basic"
          choose-label="选择 ZIP"
          accept=".zip,application/zip"
          :auto="false"
          :disabled="uploading"
          @select="chooseFile"
        >
          <template #filelabel="{ files }">
            {{ files?.length ? files[0].name : '未选择文件' }}
          </template>
        </FileUpload>
        <label class="skill-overwrite">
          <input v-model="overwrite" type="checkbox" />
          覆盖同名技能
        </label>
        <button
          class="settings-primary"
          type="button"
          :disabled="uploading || !selected"
          @click="upload"
        >
          {{ uploading ? '上传中…' : '上传' }}
        </button>
      </div>
    </div>

    <p v-if="actionError" class="settings-error" role="alert">{{ actionError }}</p>
    <p v-if="notice" class="settings-notice" role="status">{{ notice }}</p>
    <p v-if="loading" class="settings-state">加载中…</p>
    <p v-else-if="loadError" class="settings-error" role="alert">{{ loadError }}</p>
    <div v-else-if="skills.length === 0" class="settings-state">暂无技能</div>
    <div v-else class="settings-card settings-table-wrap">
      <table class="settings-table">
        <thead>
          <tr>
            <th>名称</th>
            <th>描述</th>
            <th>来源</th>
            <th>更新时间</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="skill in skills" :key="`${skill.source}:${skill.name}`">
            <td>{{ skill.name }}</td>
            <td>{{ skill.description || '—' }}</td>
            <td>
              <span class="tag">{{ sourceLabel[skill.source] }}</span>
            </td>
            <td>{{ skill.updatedAt ? formatDateTime(skill.updatedAt) : '—' }}</td>
            <td>
              <button
                v-if="skill.source === 'user'"
                class="settings-link danger"
                type="button"
                :disabled="deleting === skill.name"
                @click="remove(skill)"
              >
                {{ deleting === skill.name ? '删除中…' : '删除' }}
              </button>
              <span v-else class="settings-note">只读</span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>

<style scoped>
.settings-primary {
  padding: 8px 18px;
  background: var(--ink);
  border: 1px solid var(--ink);
  border-radius: 999px;
  color: var(--surface);
  cursor: pointer;
  font: inherit;
  font-size: 13px;
}

.settings-primary:disabled {
  cursor: default;
  opacity: 0.55;
}

.skill-upload {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
  margin: 12px 0;
}

.skill-overwrite {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--muted);
  font-size: 12px;
}

.settings-link {
  padding: 0;
  background: none;
  border: 0;
  color: var(--blue);
  cursor: pointer;
  font: inherit;
}

.settings-link.danger {
  color: var(--ink);
}

.settings-link:disabled {
  cursor: default;
  opacity: 0.5;
}
</style>
