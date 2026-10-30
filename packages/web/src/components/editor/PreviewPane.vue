<script setup lang="ts">
import Button from 'primevue/button'
import html2canvas from 'html2canvas'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'

import PreviewConsole from './PreviewConsole.vue'
import { uploadProjectCover } from '@/api/projects'
import { useProjectStore } from '@/stores/project'
import { usePreviewLogStore } from '@/stores/previewLogs'
import { PREVIEW_SANDBOX, buildPreviewUrl } from '@/utils/preview'

const props = defineProps<{ projectId: string; refreshToken: number; active: boolean }>()

const frameKey = ref(0)
const frame = ref<HTMLIFrameElement | null>(null)
const loading = ref(true)
const consoleOpen = ref(false)
const previewLogs = usePreviewLogStore()
const projectStore = useProjectStore()
const src = computed(() => buildPreviewUrl(props.projectId))
let captureTimer: ReturnType<typeof setTimeout> | null = null
const captureBusy = ref(false)
let captureOnLoad = false
let capturePending = false

function reload(capture = false) {
  loading.value = true
  captureOnLoad = capture
  previewLogs.clear()
  frameKey.value += 1
}

function scheduleCoverCapture() {
  if (captureTimer !== null) {
    clearTimeout(captureTimer)
  }
  captureTimer = setTimeout(() => void captureCover(), 600)
}

function isBlankCanvas(context: CanvasRenderingContext2D, width: number, height: number): boolean {
  try {
    const pixels = context.getImageData(0, 0, width, height).data
    const step = 4 * 16
    let samples = 0
    let different = 0
    const first = [pixels[0] ?? 0, pixels[1] ?? 0, pixels[2] ?? 0, pixels[3] ?? 0]
    for (let index = 0; index < pixels.length; index += step) {
      samples += 1
      if (
        Math.abs(pixels[index] - first[0]) > 8 ||
        Math.abs(pixels[index + 1] - first[1]) > 8 ||
        Math.abs(pixels[index + 2] - first[2]) > 8
      ) {
        different += 1
      }
    }
    return samples === 0 || different / samples < 0.01
  } catch {
    return false
  }
}

async function captureCover() {
  const frameElement = frame.value
  const document = frameElement?.contentDocument
  if (!document?.body || captureBusy.value) {
    return
  }
  if (!props.active) {
    capturePending = true
    return
  }
  captureBusy.value = true
  capturePending = false
  try {
    await document.fonts?.ready
    const background =
      document.defaultView?.getComputedStyle(document.body).backgroundColor || '#ffffff'
    const viewportWidth = Math.max(1, document.documentElement.clientWidth || 1200)
    const viewportHeight = Math.max(1, document.documentElement.clientHeight || 675)
    const source = await html2canvas(document.body, {
      width: viewportWidth,
      height: viewportHeight,
      windowWidth: viewportWidth,
      windowHeight: viewportHeight,
      backgroundColor: background,
      scale: 1,
      useCORS: true,
      logging: false,
      scrollX: 0,
      scrollY: 0,
    })
    const canvas = window.document.createElement('canvas')
    canvas.width = 1200
    canvas.height = 675
    const context = canvas.getContext('2d')
    if (!context) {
      return
    }
    context.drawImage(source, 0, 0, viewportWidth, viewportHeight, 0, 0, 1200, 675)
    const scaleX = 1200 / viewportWidth
    const scaleY = 675 / viewportHeight
    for (const layer of Array.from(document.querySelectorAll('canvas'))) {
      const rect = layer.getBoundingClientRect()
      if (rect.width <= 0 || rect.height <= 0 || layer.width === 0 || layer.height === 0) {
        continue
      }
      try {
        const layerImage = new Image()
        layerImage.src = layer.toDataURL('image/png')
        await layerImage.decode()
        context.drawImage(
          layerImage,
          rect.left * scaleX,
          rect.top * scaleY,
          rect.width * scaleX,
          rect.height * scaleY,
        )
      } catch {
        continue
      }
    }
    if (isBlankCanvas(context, canvas.width, canvas.height)) {
      previewLogs.append('warn', '预览画面为空，未更新简介图')
      return
    }
    const image = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, 'image/png'))
    if (!image) {
      return
    }
    const cover = await uploadProjectCover(props.projectId, image)
    projectStore.setCover(cover.coverUrl, cover.coverS3Path)
    previewLogs.append('info', '项目简介图已更新')
  } catch (error) {
    previewLogs.append('warn', `项目简介图生成失败：${(error as Error).message}`)
  } finally {
    captureBusy.value = false
  }
}

function captureNow() {
  captureOnLoad = false
  void captureCover()
}

function onFrameLoad() {
  loading.value = false
  if (captureOnLoad) {
    captureOnLoad = false
    capturePending = true
    if (props.active) {
      scheduleCoverCapture()
    }
  }
}

watch(
  () => props.active,
  (active) => {
    if (active && capturePending) {
      scheduleCoverCapture()
    }
  },
)

function onPreviewMessage(event: MessageEvent) {
  if (event.source !== frame.value?.contentWindow) {
    return
  }
  const data = event.data as { type?: unknown; level?: unknown; message?: unknown } | null
  if (data?.type !== 'vibebiz-preview-log' || typeof data.message !== 'string') {
    return
  }
  previewLogs.append(typeof data.level === 'string' ? data.level : 'info', data.message)
}

function toggleConsole() {
  consoleOpen.value = !consoleOpen.value
}

onMounted(() => {
  previewLogs.clear()
  window.addEventListener('message', onPreviewMessage)
})

onBeforeUnmount(() => {
  window.removeEventListener('message', onPreviewMessage)
  if (captureTimer !== null) {
    clearTimeout(captureTimer)
  }
})

function openInNewTab() {
  window.open(src.value, '_blank', 'noopener,noreferrer')
}

watch(
  () => props.refreshToken,
  () => reload(true),
)

watch(
  () => props.projectId,
  () => reload(false),
)
</script>

<template>
  <div class="preview-panel">
    <div class="preview-toolbar">
      <Button icon="pi pi-refresh" text rounded aria-label="刷新预览" @click="reload()" />
      <span class="preview-address">
        <i class="pi pi-lock" />
        {{ src }}
      </span>
      <span class="tag">静态预览</span>
      <Button
        :icon="consoleOpen ? 'pi pi-chevron-down' : 'pi pi-server'"
        text
        rounded
        aria-label="预览控制台"
        @click="toggleConsole"
      />
      <Button
        icon="pi pi-camera"
        text
        rounded
        aria-label="截图并更新简介图"
        :disabled="captureBusy"
        @click="captureNow"
      />
      <Button
        icon="pi pi-external-link"
        text
        rounded
        aria-label="新窗口打开"
        @click="openInNewTab"
      />
    </div>
    <div class="preview-stage">
      <div v-if="loading" class="preview-loading">
        <i class="pi pi-spin pi-spinner" />
        正在加载预览
      </div>
      <iframe
        ref="frame"
        :key="frameKey"
        :src="src"
        title="应用预览"
        :sandbox="PREVIEW_SANDBOX"
        referrerpolicy="no-referrer"
        @load="onFrameLoad"
      />
    </div>
    <PreviewConsole v-if="consoleOpen" @close="consoleOpen = false" />
  </div>
</template>
