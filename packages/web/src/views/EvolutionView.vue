<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'

import { streamChat } from '@/api/chat'
import EvolutionChatPanel from '@/components/evolution/EvolutionChatPanel.vue'
import EvolutionPreviewPane from '@/components/evolution/EvolutionPreviewPane.vue'
import EvolutionVersionRail from '@/components/evolution/EvolutionVersionRail.vue'
import { useAuthStore } from '@/stores/auth'
import { useEvolutionStore } from '@/stores/evolution'
import {
  EVOLUTION_PROJECT_ID,
  OFFICIAL_VERSION_LABEL,
  buildVersionPreviewUrl,
  evolutionProjectID,
  statusLabel,
} from '@/utils/evolution'
import '@/styles/evolution.css'

const EVOLUTION_AGENT_PROMPT =
  '你是 Vibebiz 集思进化助手。当前工作区是 Vibebiz 平台的交互副本，入口是 index.html（内含 CSS 与 JS）。' +
  '请根据用户建议直接修改当前工作区文件，保持单页静态可预览，不引入构建步骤、外部依赖或后端服务。' +
  '只修改当前项目目录，颜色只用黑、白、灰、蓝。修改完成后用一句话说明改了什么。'

const router = useRouter()
const auth = useAuthStore()
const store = useEvolutionStore()

const notice = ref('')
const previewNonce = ref(0)
const agentRunning = ref(false)

const viewerId = computed(() => String(auth.user?.id ?? 'guest'))
const viewerName = computed(() => auth.user?.username ?? '访客')
const selectedVersion = computed(() => store.selectedVersion)
const previewUrl = computed(() => {
  const version = selectedVersion.value
  if (!version) {
    return ''
  }
  return buildVersionPreviewUrl(version, viewerName.value)
})

function goHome(): void {
  void router.push({ name: 'home' })
}

function refreshPreview(): void {
  previewNonce.value += 1
  notice.value = '预览已刷新'
}

async function selectVersion(versionId: string): Promise<void> {
  notice.value = ''
  try {
    await store.selectVersion(versionId)
  } catch {
    notice.value = store.error || '加载对话失败'
  }
}

async function runSandboxAgent(content: string): Promise<void> {
  agentRunning.value = true
  notice.value = 'Agent 正在修改你的 sandbox 版本…'
  try {
    for await (const event of streamChat(
      evolutionProjectID(viewerId.value),
      content,
      undefined,
      [],
      EVOLUTION_AGENT_PROMPT,
    )) {
      if (event.type === 'error') {
        throw new Error(event.message || 'Agent 修改失败')
      }
    }
    notice.value = 'Agent 已修改完成，预览已刷新'
  } catch (error) {
    notice.value = error instanceof Error ? error.message : 'Agent 修改失败'
  } finally {
    agentRunning.value = false
    previewNonce.value += 1
    await store.refresh().catch(() => undefined)
  }
}

async function submitProposal(content: string): Promise<void> {
  try {
    const proposal = await store.submitProposal(content)
    if (proposal.kind === 'feature') {
      notice.value = 'Agent 已给出评估意见，该提议暂未直接生成版本'
      return
    }
    if (!proposal.versionId) {
      notice.value = '已记录提议，但没有生成预览版本'
      return
    }
    await runSandboxAgent(content)
  } catch {
    notice.value = store.error || '提交提议失败'
  }
}

async function submitChat(content: string): Promise<void> {
  try {
    await store.submitChat(content)
    notice.value = '对话已追加到聊天记录'
  } catch {
    notice.value = store.error || '发送对话失败'
  }
}

async function confirmVersion(versionId: string): Promise<void> {
  try {
    await store.confirmVersion(versionId)
    notice.value = '已收录你的意见版本，评估通过后随下一版本发布。'
  } catch {
    notice.value = store.error || '确认版本失败'
  }
}

onMounted(async () => {
  await auth.ensure().catch(() => undefined)
  try {
    await store.init({ id: viewerId.value, name: viewerName.value })
  } catch {
    notice.value = store.error || '加载集思进化数据失败'
  }
})
</script>

<template>
  <div class="evolution-shell">
    <header class="evolution-top">
      <button type="button" class="evolution-back" @click="goHome">
        <i class="pi pi-arrow-left" />
        <span>返回</span>
      </button>

      <div class="evolution-title">
        <span class="evolution-mark"><i class="pi pi-sparkles" /></span>
        <div>
          <strong>集思进化计划</strong>
          <span>
            {{ store.projectId || EVOLUTION_PROJECT_ID }} · 官方基线
            {{ store.originVersion || OFFICIAL_VERSION_LABEL }}
          </span>
        </div>
      </div>

      <div class="evolution-top-meta">
        <span class="tag">多用户共创</span>
        <span class="evolution-top-user">{{ viewerName }}</span>
      </div>
    </header>

    <p v-if="notice" class="evolution-notice">{{ notice }}</p>

    <div class="evolution-body">
      <EvolutionChatPanel
        :messages="store.activeChat"
        :proposals="store.activeProposals"
        :is-own="store.isOwnVersion"
        :selected-user-name="store.activeUserName"
        :current-version-label="selectedVersion?.label ?? store.originVersion"
        :latest-preview-version="store.latestPreviewVersion"
        :busy="agentRunning"
        @submit-proposal="submitProposal"
        @submit-chat="submitChat"
        @confirm-version="confirmVersion"
      />

      <main class="evolution-stage">
        <header class="evolution-stage-head">
          <div class="evolution-stage-title">
            <strong>{{ selectedVersion?.label ?? '未选择版本' }}</strong>
            <span>
              {{ selectedVersion?.userName ?? '—' }} ·
              {{ selectedVersion ? statusLabel(selectedVersion.status) : '—' }}
            </span>
          </div>
          <div class="evolution-stage-actions">
            <span class="evolution-stage-origin">origin_version 只读</span>
            <button type="button" @click="refreshPreview">
              <i class="pi pi-refresh" />
              刷新预览
            </button>
          </div>
        </header>

        <EvolutionPreviewPane
          :url="previewUrl"
          :version="selectedVersion"
          :refresh-key="previewNonce"
        />
      </main>

      <EvolutionVersionRail
        :groups="store.versionGroups"
        :selected-version-id="store.selectedVersionId"
        :current-user-id="viewerId"
        @select="selectVersion"
      />
    </div>
  </div>
</template>
