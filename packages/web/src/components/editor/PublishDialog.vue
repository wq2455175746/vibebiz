<script setup lang="ts">
import InputText from 'primevue/inputtext'
import { computed, ref, watch } from 'vue'

import { publishProject, type PublishResult } from '@/api/publish'

const props = defineProps<{
  projectId: string
  projectName: string
}>()

const emit = defineEmits<{
  (event: 'close'): void
}>()

const title = ref(props.projectName.slice(0, 80))
const description = ref('')
const tagsText = ref('')
const publishing = ref(false)
const error = ref('')
const result = ref<PublishResult | null>(null)
const copied = ref(false)

const tags = computed(() =>
  tagsText.value
    .split(/[,，、\n]/)
    .map((item) => item.trim())
    .filter(Boolean),
)

watch(
  () => props.projectName,
  (name) => {
    if (!title.value.trim()) title.value = name.slice(0, 80)
  },
)

async function publish() {
  if (!title.value.trim()) {
    error.value = '请填写作品标题'
    return
  }
  publishing.value = true
  error.value = ''
  copied.value = false
  try {
    result.value = await publishProject(props.projectId, {
      title: title.value.trim(),
      description: description.value.trim(),
      tags: tags.value,
    })
  } catch (err) {
    error.value = err instanceof Error ? err.message : '发布失败'
  } finally {
    publishing.value = false
  }
}

async function copyURL() {
  if (!result.value) return
  try {
    await navigator.clipboard.writeText(result.value.url)
    copied.value = true
    window.setTimeout(() => {
      copied.value = false
    }, 1500)
  } catch {
    error.value = '复制失败，请手动复制链接'
  }
}

function openURL() {
  if (result.value) window.open(result.value.url, '_blank', 'noopener,noreferrer')
}
</script>

<template>
  <div class="dialog-backdrop" @click.self="emit('close')">
    <div class="dialog-card publish-dialog">
      <header class="dialog-head">
        <div>
          <p class="dialog-eyebrow">发布作品</p>
          <h2>{{ result ? '发布成功' : '生成分享页' }}</h2>
        </div>
        <button class="dialog-close" type="button" aria-label="关闭" @click="emit('close')">
          <i class="pi pi-times" />
        </button>
      </header>

      <template v-if="result">
        <p class="publish-success">分享页已生成，手机打开链接即可体验。</p>
        <div class="publish-link">
          <span>{{ result.url }}</span>
          <button type="button" class="publish-copy" @click="copyURL">
            <i class="pi pi-copy" />
            {{ copied ? '已复制' : '复制' }}
          </button>
        </div>
        <div class="publish-actions">
          <button type="button" class="publish-secondary" @click="emit('close')">关闭</button>
          <button type="button" class="publish-primary" @click="openURL">
            <i class="pi pi-external-link" />
            打开分享页
          </button>
        </div>
      </template>

      <template v-else>
        <p class="publish-note">
          发布后会生成一张作品卡片，手机打开链接可以直接体验。请先确认项目中的
          <code>index.html</code> 或 <code>app.html</code> 是最新版本。
        </p>
        <label class="publish-field">
          <span>作品标题</span>
          <InputText v-model="title" maxlength="80" placeholder="例如：毕业季校园答题游戏" />
        </label>
        <label class="publish-field">
          <span>一句话介绍</span>
          <textarea
            v-model="description"
            class="publish-textarea"
            rows="3"
            maxlength="500"
            placeholder="三分钟通关的校园答题游戏"
          />
        </label>
        <label class="publish-field">
          <span>标签</span>
          <InputText v-model="tagsText" placeholder="答题、校园、小游戏" />
          <small>用逗号、顿号或换行分隔，最多 5 个。</small>
        </label>
        <p v-if="error" class="settings-error" role="alert">{{ error }}</p>
        <div class="publish-actions">
          <button type="button" class="publish-secondary" @click="emit('close')">取消</button>
          <button type="button" class="publish-primary" :disabled="publishing" @click="publish">
            <i :class="publishing ? 'pi pi-spin pi-spinner' : 'pi pi-upload'" />
            {{ publishing ? '发布中…' : '发布' }}
          </button>
        </div>
      </template>
    </div>
  </div>
</template>

<style scoped>
.publish-dialog {
  width: min(600px, 100%);
}

.publish-note,
.publish-success {
  margin: 0;
  color: var(--muted);
  font-size: 12px;
  line-height: 1.7;
}

.publish-note code {
  padding: 1px 4px;
  background: var(--soft);
  border-radius: 4px;
  color: var(--ink);
}

.publish-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  color: var(--ink);
  font-size: 12px;
  font-weight: 600;
}

.publish-field small {
  color: var(--muted);
  font-size: 11px;
  font-weight: 400;
}

.publish-field :deep(.p-inputtext) {
  width: 100%;
}

.publish-textarea {
  width: 100%;
  padding: 9px 10px;
  background: var(--surface);
  border: 1px solid var(--line-strong);
  border-radius: 8px;
  color: var(--text);
  font: inherit;
  outline: none;
  resize: vertical;
}

.publish-link {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  background: var(--soft);
  border: 1px solid var(--line);
  border-radius: 10px;
}

.publish-link span {
  flex: 1;
  overflow: hidden;
  color: var(--ink);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.publish-copy {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 5px 8px;
  background: var(--surface);
  border: 1px solid var(--line-strong);
  border-radius: 6px;
  color: var(--ink);
  font-size: 11px;
  cursor: pointer;
}

.publish-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 4px;
}

.publish-primary,
.publish-secondary {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 34px;
  padding: 0 14px;
  border-radius: 8px;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
}

.publish-primary {
  background: var(--ink);
  border: 1px solid var(--ink);
  color: var(--surface);
}

.publish-secondary {
  background: var(--surface);
  border: 1px solid var(--line-strong);
  color: var(--ink);
}

.publish-primary:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
