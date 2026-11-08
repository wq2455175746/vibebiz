<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'

import EvolutionChatPanel from '@/components/evolution/EvolutionChatPanel.vue'
import EvolutionPreviewPane from '@/components/evolution/EvolutionPreviewPane.vue'
import EvolutionVersionRail from '@/components/evolution/EvolutionVersionRail.vue'
import { useAuthStore } from '@/stores/auth'
import { useEvolutionStore } from '@/stores/evolution'
import {
  EVOLUTION_PREVIEW_PATH,
  EVOLUTION_PROJECT_ID,
  OFFICIAL_VERSION_LABEL,
  buildEvolutionPreviewUrl,
  statusLabel,
} from '@/utils/evolution'
import '@/styles/evolution.css'

const router = useRouter()
const auth = useAuthStore()
const store = useEvolutionStore()

const notice = ref('')
const previewNonce = ref(0)

const viewerId = computed(() => String(auth.user?.id ?? 'guest'))
const viewerName = computed(() => auth.user?.username ?? '访客')
const selectedVersion = computed(() => store.selectedVersion)
const previewUrl = computed(() => {
  if (!selectedVersion.value) {
    return ''
  }
  return buildEvolutionPreviewUrl(EVOLUTION_PREVIEW_PATH, selectedVersion.value, viewerName.value)
})

function goHome(): void {
  void router.push({ name: 'home' })
}

function refreshPreview(): void {
  previewNonce.value += 1
  notice.value = '预览已刷新'
}

function selectVersion(versionId: string): void {
  store.selectVersion(versionId)
  notice.value = ''
}

function submitProposal(content: string): void {
  const proposal = store.submitProposal(content)
  if (!proposal) {
    notice.value = '提交内容不能为空'
    return
  }
  notice.value =
    proposal.kind === 'feature'
      ? 'Agent 已给出评估意见，该提议暂未直接生成版本'
      : '已生成候选版本，请在预览中确认效果'
}

function submitChat(content: string): void {
  const ok = store.submitChat(content)
  notice.value = ok ? '对话已追加到聊天记录' : '提交内容不能为空'
}

function confirmVersion(versionId: string): void {
  const ok = store.confirmVersion(versionId)
  notice.value = ok ? '已收录你的意见版本，评估通过后随下一版本发布。' : '版本不存在'
}

onMounted(async () => {
  await auth.ensure().catch(() => undefined)
  store.init({ id: viewerId.value, name: viewerName.value })
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
          <span>{{ EVOLUTION_PROJECT_ID }} · 官方基线 {{ OFFICIAL_VERSION_LABEL }}</span>
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
        :current-version-label="selectedVersion?.label ?? OFFICIAL_VERSION_LABEL"
        :latest-preview-version="store.latestPreviewVersion"
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
