<script setup lang="ts">
import { PPTXViewer } from 'pptx-viewer'
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'

const props = defineProps<{ src: string }>()

const host = ref<HTMLDivElement | null>(null)
const loading = ref(true)
const error = ref('')
const slideCount = ref(0)
const currentSlide = ref(0)
let viewer: PPTXViewer | null = null
let requestId = 0

function clearViewer() {
  viewer?.destroy()
  viewer = null
  if (host.value) host.value.replaceChildren()
}

async function load() {
  const id = ++requestId
  clearViewer()
  loading.value = true
  error.value = ''
  slideCount.value = 0
  currentSlide.value = 0
  try {
    const response = await fetch(props.src, { credentials: 'include', cache: 'no-store' })
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}`)
    }
    const data = await response.arrayBuffer()
    if (id !== requestId || !host.value) return
    const instance = new PPTXViewer(host.value, {
      keyboardNavigation: true,
      showControls: true,
      onSlideChange: (index) => {
        currentSlide.value = index
      },
    })
    viewer = instance
    await instance.load(data)
    if (id !== requestId) {
      instance.destroy()
      return
    }
    slideCount.value = instance.getSlideCount()
    currentSlide.value = instance.getCurrentSlide()
  } catch (err) {
    if (id !== requestId) return
    clearViewer()
    error.value = err instanceof Error ? err.message : '未知错误'
  } finally {
    if (id === requestId) {
      loading.value = false
    }
  }
}

onMounted(() => void load())
watch(
  () => props.src,
  () => void load(),
)
onBeforeUnmount(() => {
  requestId += 1
  clearViewer()
})
</script>

<template>
  <div class="pptx-preview">
    <div ref="host" class="pptx-viewer" />
    <div v-if="loading" class="pptx-state">
      <i class="pi pi-spin pi-spinner" />
      正在加载幻灯片…
    </div>
    <div v-else-if="error" class="pptx-state error-text">
      <i class="pi pi-exclamation-circle" />
      PPTX 预览加载失败：{{ error }}
    </div>
    <div v-else class="pptx-meta">
      <span>{{ slideCount }} 页</span>
      <span>{{ currentSlide + 1 }} / {{ slideCount }}</span>
    </div>
  </div>
</template>

<style scoped>
.pptx-preview {
  position: relative;
  display: flex;
  height: 100%;
  min-height: 0;
  flex-direction: column;
  background: var(--surface);
}

.pptx-viewer {
  min-height: 0;
  flex: 1;
  overflow: auto;
}

.pptx-state {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 32px;
  background: var(--surface);
  color: var(--muted);
  font-size: 12px;
  text-align: center;
}

.pptx-meta {
  display: flex;
  justify-content: space-between;
  padding: 7px 12px;
  border-top: 1px solid var(--line);
  color: var(--muted);
  font-size: 11px;
}
</style>
