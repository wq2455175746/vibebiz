<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

import { useEvolutionAdminStore } from '@/stores/evolution'
import type { EvolutionVersion } from '@/types/evolution'
import {
  EVOLUTION_PREVIEW_BASE,
  EVOLUTION_PROJECT_ID,
  OFFICIAL_USER_ID,
  OFFICIAL_VERSION_LABEL,
  buildEvolutionPreviewUrl,
  formatRelative,
  formatShortTime,
  initialOf,
  kindLabel,
  roleLabel,
  statusLabel,
} from '@/utils/evolution'

const store = useEvolutionAdminStore()
const notice = ref('')
const reply = ref('')
const previewNonce = ref(0)

const previewBase = computed(
  () => import.meta.env.VITE_EVOLUTION_PREVIEW_URL || EVOLUTION_PREVIEW_BASE,
)
const previewUrl = computed(() =>
  store.selectedVersion
    ? buildEvolutionPreviewUrl(previewBase.value, store.selectedVersion, '运营预览')
    : '',
)
const isOfficial = computed(() => store.selectedVersion?.userId === OFFICIAL_USER_ID)

function selectVersion(version: EvolutionVersion): void {
  store.selectVersion(version.id)
  notice.value = ''
  previewNonce.value += 1
}

function refreshPreview(): void {
  previewNonce.value += 1
}

function sendReply(): void {
  const ok = store.sendReply(reply.value)
  notice.value = ok ? '回复已追加到该用户聊天记录' : '请选择用户版本后再回复'
  if (ok) {
    reply.value = ''
  }
}

function submitTask(): void {
  const ok = store.submitTask()
  notice.value = ok ? '已加入自动编码任务队列，等待内部 AI 流程处理' : '该版本已在队列中或不可提交'
}

function advance(taskId: string): void {
  store.advanceTask(taskId)
  notice.value = '任务状态已推进'
}

onMounted(() => {
  store.init()
})
</script>

<template>
  <div class="evo-admin">
    <header class="evo-admin-head">
      <div>
        <h2>集思进化</h2>
        <p>特殊项目 {{ EVOLUTION_PROJECT_ID }} · 官方基线 {{ OFFICIAL_VERSION_LABEL }}</p>
      </div>
      <div class="evo-admin-stats">
        <span
          ><strong>{{ store.versionGroups.length }}</strong> 位用户</span
        >
        <span
          ><strong>{{ store.versions.length }}</strong> 个版本</span
        >
        <span
          ><strong>{{ store.tasks.length }}</strong> 个编码任务</span
        >
      </div>
    </header>

    <p v-if="notice" class="evo-notice">{{ notice }}</p>

    <div class="evo-admin-grid">
      <aside class="evo-panel evo-versions">
        <header class="evo-panel-head">
          <strong>用户版本</strong>
          <span>{{ store.versionGroups.length }} 位用户</span>
        </header>
        <div class="evo-version-list">
          <section
            v-for="group in store.versionGroups"
            :key="group.userId"
            class="evo-version-group"
            :class="{
              active: store.activeUserId === group.userId,
              official: group.userId === OFFICIAL_USER_ID,
            }"
          >
            <button type="button" class="evo-version-user" @click="selectVersion(group.latest)">
              <span class="evo-avatar">{{ initialOf(group.userName) }}</span>
              <span class="evo-version-info">
                <strong>
                  {{ group.userName }}
                  <em v-if="group.userId === OFFICIAL_USER_ID">官方</em>
                </strong>
                <span>{{ group.latest.label }} · {{ formatRelative(group.latest.updatedAt) }}</span>
              </span>
              <span class="evo-status" :class="group.latest.status">
                {{ statusLabel(group.latest.status) }}
              </span>
            </button>
            <div
              v-if="group.versions.length > 1 && group.userId !== OFFICIAL_USER_ID"
              class="evo-version-history"
            >
              <button
                v-for="version in group.versions"
                :key="version.id"
                type="button"
                :class="{ active: version.id === store.selectedVersion?.id }"
                @click="selectVersion(version)"
              >
                <span>{{ version.label }}</span>
                <time>{{ formatShortTime(version.createdAt) }}</time>
              </button>
            </div>
          </section>
        </div>
      </aside>

      <main class="evo-panel evo-preview">
        <header class="evo-panel-head">
          <div class="evo-preview-title">
            <strong>{{ store.selectedVersion?.label || '未选择版本' }}</strong>
            <span>
              {{ store.selectedVersion?.userName || '—' }} ·
              {{ store.selectedVersion ? statusLabel(store.selectedVersion.status) : '—' }}
            </span>
          </div>
          <div class="evo-preview-actions">
            <span class="evo-badge">origin · {{ OFFICIAL_VERSION_LABEL }}</span>
            <button type="button" class="evo-ghost" @click="refreshPreview">刷新预览</button>
          </div>
        </header>

        <div class="evo-preview-body">
          <iframe
            v-if="previewUrl"
            :key="`${previewUrl}-${previewNonce}`"
            :src="previewUrl"
            title="集思进化版本预览"
          />
          <p v-else class="evo-empty">选择一个用户版本开始预览</p>
        </div>

        <footer class="evo-preview-foot">
          <span>{{ store.selectedVersion?.summary || '官方基线只读，不参与编码任务。' }}</span>
          <button
            type="button"
            class="evo-primary"
            :disabled="!store.canSubmit"
            @click="submitTask"
          >
            提交自动编码任务
          </button>
        </footer>
      </main>

      <aside class="evo-panel evo-chat">
        <header class="evo-panel-head">
          <strong>对话与提议</strong>
          <span>{{ store.selectedVersion?.userName || '—' }}</span>
        </header>

        <div class="evo-chat-body">
          <article
            v-for="message in store.activeChat"
            :key="message.id"
            class="evo-message"
            :class="message.role"
          >
            <header>
              <span>{{ roleLabel(message.role) }}</span>
              <time>{{ formatShortTime(message.createdAt) }}</time>
            </header>
            <p>{{ message.content }}</p>
          </article>

          <article
            v-for="proposal in store.activeProposals"
            :key="proposal.id"
            class="evo-proposal"
            :class="proposal.kind"
          >
            <header>
              <span class="evo-kind">{{ kindLabel(proposal.kind) }}</span>
              <span class="evo-status" :class="proposal.status">
                {{ statusLabel(proposal.status) }}
              </span>
              <time>{{ formatShortTime(proposal.createdAt) }}</time>
            </header>
            <p>{{ proposal.content }}</p>
            <p class="evo-proposal-reply">{{ proposal.agentReply }}</p>
          </article>

          <p v-if="!store.activeChat.length && !store.activeProposals.length" class="evo-empty">
            该用户还没有对话与提议。
          </p>
        </div>

        <footer class="evo-chat-foot">
          <textarea
            v-model="reply"
            rows="3"
            :disabled="isOfficial"
            placeholder="以运营身份回复，回复会追加到该用户聊天记录…"
          />
          <button
            type="button"
            class="evo-primary"
            :disabled="!reply.trim() || isOfficial"
            @click="sendReply"
          >
            追加回复
          </button>
        </footer>
      </aside>
    </div>

    <section class="evo-panel evo-tasks">
      <header class="evo-panel-head">
        <strong>自动编码任务队列</strong>
        <span>{{ store.tasks.length }} 个任务 · 人工确认后进入内部 CI/CD</span>
      </header>
      <div class="evo-task-list">
        <article v-for="task in store.tasks" :key="task.id" class="evo-task">
          <div class="evo-task-main">
            <strong>{{ task.versionLabel }}</strong>
            <span>{{ task.userName }} · {{ task.originVersion }}</span>
            <p>{{ task.chatSummary }}</p>
          </div>
          <div class="evo-task-side">
            <span class="evo-status" :class="task.status">{{ statusLabel(task.status) }}</span>
            <time>{{ formatRelative(task.createdAt) }}</time>
            <button
              v-if="task.status !== 'released' && task.status !== 'failed'"
              type="button"
              class="evo-ghost"
              @click="advance(task.id)"
            >
              模拟推进
            </button>
          </div>
        </article>
        <p v-if="!store.tasks.length" class="evo-empty">还没有自动编码任务。</p>
      </div>
    </section>
  </div>
</template>

<style scoped>
.evo-admin {
  display: flex;
  min-height: 100%;
  flex-direction: column;
  gap: 14px;
  padding: 22px 26px 30px;
}

.evo-admin-head {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 16px;
}

.evo-admin-head h2 {
  color: var(--ink);
  font-size: 18px;
  font-weight: 600;
}

.evo-admin-head p {
  margin-top: 4px;
  color: var(--muted);
  font-size: 11px;
  letter-spacing: 0.03em;
}

.evo-admin-stats {
  display: flex;
  gap: 18px;
}

.evo-admin-stats span {
  display: flex;
  align-items: baseline;
  gap: 5px;
  color: var(--muted);
  font-size: 10px;
}

.evo-admin-stats strong {
  color: var(--ink);
  font-size: 18px;
  font-weight: 600;
}

.evo-notice {
  margin: 0;
  padding: 9px 12px;
  background: var(--blue-soft);
  border: 1px solid var(--line);
  border-radius: 9px;
  color: var(--blue);
  font-size: 12px;
}

.evo-admin-grid {
  display: grid;
  grid-template-columns: 268px minmax(0, 1fr) 340px;
  gap: 14px;
  min-height: 520px;
}

.evo-panel {
  display: flex;
  min-width: 0;
  min-height: 0;
  flex-direction: column;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 12px;
}

.evo-panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 13px 14px;
  border-bottom: 1px solid var(--line);
}

.evo-panel-head strong {
  color: var(--ink);
  font-size: 12px;
  font-weight: 600;
}

.evo-panel-head > span {
  overflow: hidden;
  color: var(--muted);
  font-size: 10px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.evo-version-list {
  display: flex;
  flex: 1;
  min-height: 0;
  flex-direction: column;
  gap: 6px;
  padding: 9px;
  overflow-y: auto;
}

.evo-version-group {
  display: flex;
  flex-direction: column;
  gap: 3px;
  padding: 5px;
  border: 1px solid transparent;
  border-radius: 9px;
}

.evo-version-group.active {
  background: var(--blue-soft);
  border-color: var(--line);
}

.evo-version-group.official {
  background: var(--canvas);
}

.evo-version-user {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px;
  background: transparent;
  border: 1px solid transparent;
  border-radius: 8px;
  color: var(--text);
  text-align: left;
  cursor: pointer;
}

.evo-version-user:hover {
  background: var(--soft);
}

.evo-version-group.active .evo-version-user {
  background: var(--surface);
  border-color: var(--line);
}

.evo-avatar {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  width: 27px;
  height: 27px;
  background: var(--ink);
  border-radius: 999px;
  color: var(--surface);
  font-size: 10px;
  font-weight: 600;
}

.evo-version-info {
  display: flex;
  flex: 1;
  min-width: 0;
  flex-direction: column;
  gap: 1px;
}

.evo-version-info strong {
  display: flex;
  align-items: center;
  gap: 4px;
  overflow: hidden;
  color: var(--ink);
  font-size: 11px;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.evo-version-info strong em {
  padding: 1px 5px;
  background: var(--muted);
  border-radius: 999px;
  color: var(--surface);
  font-size: 8px;
  font-style: normal;
}

.evo-version-info > span {
  overflow: hidden;
  color: var(--muted);
  font-size: 9px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.evo-version-history {
  display: flex;
  flex-direction: column;
  gap: 1px;
  padding-left: 35px;
}

.evo-version-history button {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 3px 6px;
  background: transparent;
  border: 1px solid transparent;
  border-radius: 6px;
  color: var(--muted);
  font-size: 9px;
  cursor: pointer;
}

.evo-version-history button:hover {
  background: var(--surface);
  color: var(--ink);
}

.evo-version-history button.active {
  background: var(--surface);
  border-color: var(--line);
  color: var(--blue);
}

.evo-status {
  display: inline-flex;
  align-items: center;
  padding: 2px 7px;
  background: var(--soft);
  border: 1px solid var(--line);
  border-radius: 999px;
  color: var(--muted);
  font-size: 9px;
  white-space: nowrap;
}

.evo-status.preview,
.evo-status.queued,
.evo-status.submitted,
.evo-status.coding,
.evo-status.ci {
  background: var(--blue-soft);
  color: var(--blue);
}

.evo-status.reviewing {
  background: var(--ink);
  border-color: var(--ink);
  color: var(--surface);
}

.evo-status.accepted,
.evo-status.released {
  background: var(--soft);
  color: var(--ink);
}

.evo-status.failed,
.evo-status.rejected {
  border-style: dashed;
  color: var(--ink);
}

.evo-preview-title {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.evo-preview-title span {
  color: var(--muted);
  font-size: 9px;
}

.evo-preview-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.evo-badge {
  padding: 3px 8px;
  background: var(--soft);
  border: 1px solid var(--line);
  border-radius: 999px;
  color: var(--muted);
  font-size: 9px;
  white-space: nowrap;
}

.evo-ghost {
  padding: 5px 9px;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 7px;
  color: var(--muted);
  font-size: 10px;
  cursor: pointer;
}

.evo-ghost:hover {
  background: var(--soft);
  color: var(--ink);
}

.evo-primary {
  padding: 8px 13px;
  background: var(--ink);
  border: 1px solid var(--ink);
  border-radius: 8px;
  color: var(--surface);
  font-size: 11px;
  font-weight: 500;
  cursor: pointer;
}

.evo-primary:disabled {
  background: var(--line);
  border-color: var(--line);
  color: var(--muted);
  cursor: not-allowed;
}

.evo-preview-body {
  position: relative;
  flex: 1;
  min-height: 0;
  padding: 12px;
}

.evo-preview-body iframe {
  width: 100%;
  height: 100%;
  min-height: 380px;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 9px;
}

.evo-preview-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 11px 14px;
  border-top: 1px solid var(--line);
}

.evo-preview-foot span {
  overflow: hidden;
  color: var(--muted);
  font-size: 10px;
  line-height: 1.5;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.evo-chat-body {
  display: flex;
  flex: 1;
  min-height: 0;
  flex-direction: column;
  gap: 9px;
  padding: 12px;
  overflow-y: auto;
}

.evo-message {
  display: flex;
  max-width: 94%;
  flex-direction: column;
  gap: 4px;
  padding: 9px 10px;
  background: var(--soft);
  border: 1px solid var(--line);
  border-radius: 10px;
}

.evo-message.user {
  align-self: flex-end;
  background: var(--ink);
  border-color: var(--ink);
  color: var(--surface);
}

.evo-message.admin {
  align-self: flex-end;
  background: var(--blue-soft);
  color: var(--blue);
}

.evo-message header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  font-size: 8px;
  opacity: 0.68;
}

.evo-message p {
  font-size: 11px;
  line-height: 1.6;
  white-space: pre-wrap;
}

.evo-proposal {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 10px;
  background: var(--surface);
  border: 1px solid var(--line);
  border-left: 3px solid var(--line-strong);
  border-radius: 9px;
}

.evo-proposal.bug {
  border-left-color: var(--ink);
}

.evo-proposal.visual {
  border-left-color: var(--blue);
}

.evo-proposal.feature {
  border-left-style: dashed;
  border-left-color: var(--muted);
}

.evo-proposal header {
  display: flex;
  align-items: center;
  gap: 6px;
}

.evo-proposal header time {
  margin-left: auto;
  color: var(--muted);
  font-size: 8px;
}

.evo-kind {
  padding: 2px 6px;
  background: var(--soft);
  border-radius: 999px;
  color: var(--ink);
  font-size: 9px;
  font-weight: 600;
}

.evo-proposal > p {
  color: var(--ink);
  font-size: 11px;
  line-height: 1.55;
}

.evo-proposal-reply {
  padding: 7px 8px;
  background: var(--soft);
  border-radius: 7px;
  color: var(--muted) !important;
  font-size: 10px !important;
}

.evo-chat-foot {
  display: flex;
  flex-direction: column;
  gap: 7px;
  padding: 11px 12px 12px;
  border-top: 1px solid var(--line);
}

.evo-chat-foot textarea {
  width: 100%;
  padding: 8px 9px;
  background: var(--canvas);
  border: 1px solid var(--line);
  border-radius: 8px;
  color: var(--ink);
  font-size: 11px;
  line-height: 1.55;
  resize: none;
  outline: none;
}

.evo-chat-foot textarea:focus {
  background: var(--surface);
  border-color: var(--blue);
}

.evo-chat-foot button {
  align-self: flex-end;
}

.evo-tasks {
  min-height: 180px;
}

.evo-task-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 12px;
}

.evo-task {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 14px;
  padding: 11px 12px;
  background: var(--canvas);
  border: 1px solid var(--line);
  border-radius: 9px;
}

.evo-task-main {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 3px;
}

.evo-task-main strong {
  color: var(--ink);
  font-size: 11px;
  font-weight: 600;
}

.evo-task-main > span {
  color: var(--muted);
  font-size: 9px;
}

.evo-task-main p {
  margin-top: 2px;
  color: var(--text);
  font-size: 10px;
  line-height: 1.5;
}

.evo-task-side {
  display: flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 8px;
}

.evo-task-side time {
  color: var(--muted);
  font-size: 9px;
}

.evo-empty {
  margin: 0;
  padding: 26px 12px;
  color: var(--muted);
  font-size: 11px;
  text-align: center;
}

@media (max-width: 1280px) {
  .evo-admin-grid {
    grid-template-columns: 240px minmax(0, 1fr) 300px;
  }
}

@media (max-width: 1080px) {
  .evo-admin-grid {
    grid-template-columns: 240px minmax(0, 1fr);
  }

  .evo-chat {
    grid-column: 1 / -1;
    max-height: 420px;
  }
}
</style>
