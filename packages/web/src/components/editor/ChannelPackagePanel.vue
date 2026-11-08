<script setup lang="ts">
import { onMounted, ref } from 'vue'

import {
  downloadChannelPackage,
  generateChannelPackage,
  listChannelPackages,
} from '@/api/channelPackages'
import { adoptResource, presignResource, uploadStaging, uploadToS3 } from '@/api/resources'
import type { ChannelPackage, ChannelPackageChannel, ChannelPackageSourceType } from '@/types'
import { channelLabel, channelOptions, levelLabel } from '@/utils/channelPackages'
import { formatBytes } from '@/utils/storage'

const props = defineProps<{ projectId: string }>()

const sourceType = ref<ChannelPackageSourceType>('html5')
const artifactResourceId = ref('')
const artifactName = ref('')
const uploading = ref(false)
const channel = ref<ChannelPackageChannel>('wechat_minigame')
const packages = ref<ChannelPackage[]>([])
const generating = ref(false)
const error = ref('')

function errorMessage(err: unknown, fallback: string): string {
  return err instanceof Error ? err.message : fallback
}

async function load() {
  try {
    packages.value = await listChannelPackages(props.projectId)
  } catch (err) {
    error.value = errorMessage(err, '加载渠道包失败')
  }
}

async function onArtifactChange(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (file) await uploadArtifact(file)
}

async function uploadArtifact(file: File) {
  uploading.value = true
  error.value = ''
  try {
    const presign = await presignResource({
      filename: file.name,
      contentType: file.type || 'application/zip',
      size: file.size,
      purpose: 'channel_build',
    })
    try {
      await uploadToS3(presign, file)
    } catch {
      await uploadStaging(presign.resourceId, file)
    }
    const adopted = await adoptResource(props.projectId, presign.resourceId)
    artifactResourceId.value = adopted.id
    artifactName.value = adopted.displayName || file.name
  } catch (err) {
    artifactResourceId.value = ''
    artifactName.value = ''
    error.value = errorMessage(err, '上传失败')
  } finally {
    uploading.value = false
  }
}

async function generate() {
  if (sourceType.value === 'engine_artifact' && !artifactResourceId.value) {
    error.value = '请先上传引擎成品 zip'
    return
  }
  generating.value = true
  error.value = ''
  try {
    const result = await generateChannelPackage(props.projectId, {
      channel: channel.value,
      sourceType: sourceType.value,
      resourceId: sourceType.value === 'engine_artifact' ? artifactResourceId.value : undefined,
    })
    packages.value = [result, ...packages.value.filter((item) => item.id !== result.id)]
  } catch (err) {
    error.value = errorMessage(err, '生成失败')
  } finally {
    generating.value = false
  }
}

async function download(item: ChannelPackage) {
  try {
    const result = await downloadChannelPackage(props.projectId, item.id)
    const url = URL.createObjectURL(result.blob)
    try {
      const link = document.createElement('a')
      link.href = url
      link.download = result.filename
      link.rel = 'noopener'
      link.click()
    } finally {
      URL.revokeObjectURL(url)
    }
  } catch (err) {
    error.value = errorMessage(err, '下载失败')
  }
}

function openGuide(item: ChannelPackage) {
  if (!item.guideUrl) return
  window.open(item.guideUrl, '_blank', 'noopener,noreferrer')
}

onMounted(load)
</script>

<template>
  <div class="channel-package-panel">
    <section class="channel-package-section">
      <p class="channel-package-label">发布渠道</p>
      <div class="channel-package-channels">
        <button
          v-for="item in channelOptions"
          :key="item.value"
          type="button"
          class="channel-package-chip"
          :class="{ active: channel === item.value }"
          @click="channel = item.value"
        >
          {{ item.label }}
        </button>
      </div>
    </section>

    <section class="channel-package-section">
      <p class="channel-package-label">来源</p>
      <div class="channel-package-sources">
        <button
          type="button"
          class="channel-package-chip"
          :class="{ active: sourceType === 'html5' }"
          @click="sourceType = 'html5'"
        >
          当前工作区
        </button>
        <button
          type="button"
          class="channel-package-chip"
          :class="{ active: sourceType === 'engine_artifact' }"
          @click="sourceType = 'engine_artifact'"
        >
          上传引擎成品
        </button>
      </div>
      <div v-if="sourceType === 'engine_artifact'" class="channel-package-upload">
        <label class="channel-package-file">
          <input
            type="file"
            accept=".zip,application/zip"
            :disabled="uploading"
            @change="onArtifactChange"
          />
          <span>
            <i :class="uploading ? 'pi pi-spin pi-spinner' : 'pi pi-upload'" />
            {{ uploading ? '上传中…' : artifactResourceId ? '重新上传 zip' : '选择 zip 文件' }}
          </span>
        </label>
        <small v-if="artifactName">{{ artifactName }} 已上传并关联</small>
        <small v-else>先上传引擎成品 zip，生成时会自动关联。</small>
      </div>
    </section>

    <p v-if="error" class="channel-package-error" role="alert">{{ error }}</p>

    <button
      type="button"
      class="channel-package-generate"
      :disabled="generating || uploading"
      @click="generate"
    >
      <i :class="generating ? 'pi pi-spin pi-spinner' : 'pi pi-box'" />
      {{ generating ? '生成中…' : '生成渠道包' }}
    </button>

    <section class="channel-package-results">
      <p class="channel-package-label">已生成的渠道包</p>
      <p v-if="!packages.length" class="channel-package-empty">还没有渠道包，选择渠道后生成。</p>
      <article v-for="item in packages" :key="item.id" class="channel-package-item">
        <header class="channel-package-item-head">
          <div>
            <strong>{{ channelLabel(item.channel) }}</strong>
            <small>
              {{ item.sourceType === 'html5' ? '当前工作区' : '引擎成品' }} · v{{ item.version }}
            </small>
          </div>
          <span class="channel-package-level" :class="`level-${item.level.toLowerCase()}`">
            {{ levelLabel(item.level) }}
          </span>
        </header>
        <p v-if="item.status === 'failed'" class="channel-package-error">
          {{ item.errorMessage || '生成失败' }}
        </p>
        <ul v-if="item.warnings.length" class="channel-package-warnings">
          <li v-for="(warning, index) in item.warnings" :key="index">{{ warning }}</li>
        </ul>
        <footer class="channel-package-item-foot">
          <span class="channel-package-size">{{ formatBytes(item.fileSizeBytes) }}</span>
          <button
            type="button"
            class="channel-package-link"
            :disabled="item.status !== 'ready'"
            @click="download(item)"
          >
            <i class="pi pi-download" />
            下载
          </button>
          <button type="button" class="channel-package-link" @click="openGuide(item)">
            <i class="pi pi-book" />
            查看指引
          </button>
        </footer>
      </article>
    </section>
  </div>
</template>
