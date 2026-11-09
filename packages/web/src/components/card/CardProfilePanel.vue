<script setup lang="ts">
import { computed, ref } from 'vue'

import CardPreview from './CardPreview.vue'
import type { BusinessCard, CardFeatured } from '@/types'
import { CARD_MAX_TAGS, validateCardUpload } from '@/utils/card'

const props = defineProps<{ card: BusinessCard; uploadingFeatured?: boolean }>()
const emit = defineEmits<{
  'update:headline': [value: string]
  'add-tag': [raw: string]
  'remove-tag': [tag: string]
  'set-featured': [featured: CardFeatured]
  'upload-featured': [file: File]
  'clear-featured': []
}>()

const tagInput = ref('')
const tagError = ref('')
const uploadError = ref('')
const fileInput = ref<HTMLInputElement | null>(null)

const initial = computed(() => (props.card.ownerName || '?').charAt(0).toUpperCase())
const tagsFull = computed(() => props.card.tags.length >= CARD_MAX_TAGS)
const selectedProjectId = computed(() =>
  props.card.featured?.kind === 'project' ? props.card.featured.projectId : '',
)

function submitTag() {
  const value = tagInput.value.trim()
  if (!value) return
  if (props.card.tags.includes(value)) {
    tagError.value = '标签已存在'
    return
  }
  if (props.card.tags.length >= CARD_MAX_TAGS) {
    tagError.value = '最多 5 个标签'
    return
  }
  tagError.value = ''
  emit('add-tag', value)
  tagInput.value = ''
}

function onTagKeydown(event: KeyboardEvent) {
  if (event.key === 'Enter' || event.key === ',' || event.key === '，') {
    event.preventDefault()
    submitTag()
  }
}

function selectProject(event: Event) {
  const projectId = (event.target as HTMLSelectElement).value
  if (projectId) emit('set-featured', { kind: 'project', projectId })
}

function openUpload() {
  uploadError.value = ''
  fileInput.value?.click()
}

function onFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  const message = validateCardUpload(file)
  if (message) {
    uploadError.value = message
    return
  }
  uploadError.value = ''
  emit('upload-featured', file)
}
</script>

<template>
  <section class="card-profile">
    <header class="card-profile-head">
      <img v-if="card.avatarUrl" class="card-profile-avatar" :src="card.avatarUrl" :alt="card.ownerName" />
      <span v-else class="card-profile-avatar">{{ initial }}</span>
      <div>
        <strong>{{ card.ownerName || '未命名开发者' }}</strong>
        <p>{{ card.headline || '个人信息' }}</p>
      </div>
    </header>

    <label class="card-field">
      <span>一句话简介</span>
      <input
        :value="card.headline"
        maxlength="80"
        placeholder="例如：前端工程师，喜欢做小游戏"
        @input="emit('update:headline', ($event.target as HTMLInputElement).value)"
      />
    </label>

    <div class="card-field">
      <span>主预览</span>
      <CardPreview :featured="card.featured" :items="card.items" large />
      <div class="card-featured-actions">
        <select
          v-if="card.items.length > 0"
          class="card-select"
          :value="selectedProjectId"
          @change="selectProject"
        >
          <option value="">从已添加项目选择</option>
          <option v-for="item in card.items" :key="item.projectId" :value="item.projectId">
            {{ item.name }}
          </option>
        </select>
        <button type="button" class="card-secondary" :disabled="uploadingFeatured" @click="openUpload">
          <i :class="uploadingFeatured ? 'pi pi-spin pi-spinner' : 'pi pi-upload'" />
          {{ uploadingFeatured ? '上传中…' : '上传 PDF / 图片' }}
        </button>
        <button v-if="card.featured" type="button" class="card-text-btn" @click="emit('clear-featured')">
          移除主预览
        </button>
      </div>
      <input ref="fileInput" class="card-file-input" type="file" accept="application/pdf,image/*" @change="onFileChange" />
      <p v-if="uploadError" class="error-text" role="alert">{{ uploadError }}</p>
      <small>只能从已添加项目中选择，或上传 2MB 以内的 PDF / 图片。</small>
    </div>

    <div class="card-field">
      <span>标签</span>
      <div class="card-tags">
        <span v-for="tag in card.tags" :key="tag" class="card-tag">
          {{ tag }}
          <button type="button" :aria-label="`删除标签 ${tag}`" @click="emit('remove-tag', tag)">
            <i class="pi pi-times" />
          </button>
        </span>
      </div>
      <input
        v-model="tagInput"
        :disabled="tagsFull"
        maxlength="12"
        :placeholder="tagsFull ? '最多 5 个标签' : '输入标签后回车'"
        @keydown="onTagKeydown"
        @blur="submitTag"
      />
      <p v-if="tagError" class="error-text" role="alert">{{ tagError }}</p>
      <small>最多 5 个标签，每个不超过 12 个字。</small>
    </div>
  </section>
</template>
