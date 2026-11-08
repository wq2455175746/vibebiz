<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import Dialog from 'primevue/dialog'

import { useEvolutionAdminStore } from '@/stores/evolution'
import type { EvolutionRow, EvolutionTask, EvolutionVersion } from '@/types/evolution'
import {
  EVOLUTION_PREVIEW_BASE,
  buildEvolutionPreviewUrl,
  formatRelative,
  formatShortTime,
  initialOf,
  kindLabel,
  nextTaskStatus,
  roleLabel,
  statusLabel,
} from '@/utils/evolution'

const store = useEvolutionAdminStore()
const notice = ref('')
const keyword = ref('')
const statusFilter = ref('all')
const previewVisible = ref(false)
const previewVersion = ref<EvolutionVersion | null>(null)
const previewNonce = ref(0)
const conversationVisible = ref(false)
const conversationVersion = ref<EvolutionVersion | null>(null)
const replyText = ref('')
const sending = ref(false)
const actionTaskId = ref('')

const previewBase = computed(
  () => import.meta.env.VITE_EVOLUTION_PREVIEW_URL || EVOLUTION_PREVIEW_BASE,
)
const previewUrl = computed(() =>
  previewVersion.value
    ? buildEvolutionPreviewUrl(previewBase.value, previewVersion.value, '运营预览')
    : '',
)
const filteredRows = computed(() => {
  const text = keyword.value.trim().toLowerCase()
  return store.rows.filter((row) => {
    if (statusFilter.value !== 'all' && row.version.status !== statusFilter.value) {
      return false
    }
    if (!text) {
      return true
    }
    return [row.version.userName, row.version.label, row.version.summary].some((value) =>
      value.toLowerCase().includes(text),
    )
  })
})
const conversationTitle = computed(() =>
  conversationVersion.value
    ? `${conversationVersion.value.userName} · ${conversationVersion.value.label}`
    : '对话与提议',
)

function openPreview(row: EvolutionRow): void {
  previewVersion.value = row.version
  previewNonce.value += 1
  previewVisible.value = true
}

async function openConversation(row: EvolutionRow): Promise<void> {
  notice.value = ''
  try {
    await store.selectVersion(row.version.id)
    conversationVersion.value = row.version
    replyText.value = ''
    conversationVisible.value = true
  } catch {
    notice.value = store.error || '加载对话失败'
  }
}

async function sendReply(): Promise<void> {
  const version = conversationVersion.value
  if (!version || !replyText.value.trim()) {
    return
  }
  sending.value = true
  try {
    await store.reply(version.id, replyText.value)
    replyText.value = ''
    notice.value = '回复已追加到该用户聊天记录'
  } catch {
    notice.value = store.error || '回复失败'
  } finally {
    sending.value = false
  }
}

async function addToPlan(row: EvolutionRow): Promise<void> {
  actionTaskId.value = row.version.id
  notice.value = ''
  try {
    await store.submitTask(row.version.id)
    notice.value = `已将 ${row.version.userName} 的 ${row.version.label} 加入编码计划`
  } catch {
    notice.value = store.error || '加入编码计划失败'
  } finally {
    actionTaskId.value = ''
  }
}

async function advance(task: EvolutionTask): Promise<void> {
  actionTaskId.value = task.id
  notice.value = ''
  try {
    await store.advanceTask(task.id, nextTaskStatus(task.status))
    notice.value = '任务状态已推进'
  } catch {
    notice.value = store.error || '推进任务失败'
  } finally {
    actionTaskId.value = ''
  }
}

function canAdvance(task: EvolutionTask): boolean {
  return task.status !== 'released' && task.status !== 'failed'
}

onMounted(async () => {
  try {
    await store.init()
  } catch {
    notice.value = store.error || '加载集思进化数据失败'
  }
})
</script>

<template>
  <div class="evo-admin">
    <header class="evo-head">
      <div class="evo-head-title">
        <h2>集思进化</h2>
        <p>{{ store.projectId }} · 官方基线 {{ store.originVersion }}</p>
      </div>
      <div class="evo-stats">
        <div class="evo-stat">
          <strong>{{ store.stats.users }}</strong>
          <span>参与用户</span>
        </div>
        <div class="evo-stat">
          <strong>{{ store.stats.versions }}</strong>
          <span>提交变更</span>
        </div>
        <div class="evo-stat">
          <strong>{{ store.stats.pendingProposals }}</strong>
          <span>待处理提议</span>
        </div>
        <div class="evo-stat">
          <strong>{{ store.stats.tasks }}</strong>
          <span>编码计划</span>
        </div>
      </div>
    </header>

    <p v-if="notice" class="evo-notice">{{ notice }}</p>
    <p v-if="store.error" class="evo-error">{{ store.error }}</p>

    <section class="evo-panel">
      <header class="evo-panel-head">
        <div class="evo-panel-title">
          <strong>提交变更</strong>
          <span>{{ filteredRows.length }} 条 · 默认不加载预览，点击右侧操作查看</span>
        </div>
        <div class="evo-filters">
          <input v-model="keyword" type="search" placeholder="搜索用户 / 版本 / 摘要" />
          <select v-model="statusFilter">
            <option value="all">全部状态</option>
            <option value="preview">待确认</option>
            <option value="accepted">已采纳</option>
            <option value="submitted">已提交</option>
            <option value="released">已发布</option>
          </select>
        </div>
      </header>

      <div class="evo-table-wrap">
        <table class="evo-table">
          <thead>
            <tr>
              <th>用户</th>
              <th>版本</th>
              <th>变更摘要</th>
              <th>提议</th>
              <th>状态</th>
              <th>更新时间</th>
              <th class="right">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in filteredRows" :key="row.version.id">
              <td>
                <div class="evo-user">
                  <span class="evo-avatar">{{ initialOf(row.version.userName) }}</span>
                  <div class="evo-user-text">
                    <strong>{{ row.version.userName }}</strong>
                    <span>{{ row.version.userId }}</span>
                  </div>
                </div>
              </td>
              <td>
                <strong class="evo-version-label">{{ row.version.label }}</strong>
              </td>
              <td>
                <p class="evo-summary">{{ row.version.summary || '暂无摘要' }}</p>
              </td>
              <td>
                <span class="evo-proposal-count">{{ row.proposalCount }} 条</span>
                <span v-if="row.latestProposal" class="evo-summary-meta">
                  最新 {{ kindLabel(row.latestProposal.kind) }}
                </span>
              </td>
              <td>
                <span class="evo-status" :class="row.version.status">
                  {{ statusLabel(row.version.status) }}
                </span>
              </td>
              <td>
                <time class="evo-time">{{ formatRelative(row.version.updatedAt) }}</time>
              </td>
              <td class="right">
                <div class="evo-actions">
                  <button type="button" class="evo-ghost" @click="openPreview(row)">预览</button>
                  <button type="button" class="evo-ghost" @click="openConversation(row)">
                    对话
                  </button>
                  <button
                    type="button"
                    class="evo-primary"
                    :disabled="Boolean(row.task) || actionTaskId === row.version.id"
                    @click="addToPlan(row)"
                  >
                    {{ row.task ? '已在编码计划' : '添加到编码计划' }}
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
        <p v-if="!filteredRows.length && !store.loading" class="evo-empty">
          暂无提交变更。用户提交提议后会出现在这里。
        </p>
        <p v-if="store.loading" class="evo-empty">加载中...</p>
      </div>
    </section>

    <section class="evo-panel">
      <header class="evo-panel-head">
        <div class="evo-panel-title">
          <strong>编码计划</strong>
          <span>{{ store.sortedTasks.length }} 个任务 · CI/CD 暂未接入，状态由运营手动推进</span>
        </div>
      </header>
      <div class="evo-table-wrap">
        <table class="evo-table">
          <thead>
            <tr>
              <th>版本</th>
              <th>用户</th>
              <th>对话摘要</th>
              <th>状态</th>
              <th>创建时间</th>
              <th class="right">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="task in store.sortedTasks" :key="task.id">
              <td>
                <strong class="evo-version-label">{{ task.versionLabel }}</strong>
              </td>
              <td>{{ task.userName }}</td>
              <td>
                <p class="evo-summary">{{ task.chatSummary }}</p>
              </td>
              <td>
                <span class="evo-status" :class="task.status">{{ statusLabel(task.status) }}</span>
              </td>
              <td>
                <time class="evo-time">{{ formatShortTime(task.createdAt) }}</time>
              </td>
              <td class="right">
                <button
                  v-if="canAdvance(task)"
                  type="button"
                  class="evo-ghost"
                  :disabled="actionTaskId === task.id"
                  @click="advance(task)"
                >
                  推进到 {{ statusLabel(nextTaskStatus(task.status)) }}
                </button>
                <span v-else class="evo-time">已结束</span>
              </td>
            </tr>
          </tbody>
        </table>
        <p v-if="!store.sortedTasks.length && !store.loading" class="evo-empty">
          还没有编码计划任务。
        </p>
      </div>
    </section>

    <Dialog
      v-model:visible="previewVisible"
      modal
      header="版本预览"
      :style="{ width: 'min(1100px, 94vw)' }"
    >
      <div class="evo-preview-dialog">
        <div class="evo-preview-bar">
          <span>
            {{ previewVersion?.userName }} · {{ previewVersion?.label }} ·
            {{ previewVersion ? statusLabel(previewVersion.status) : '' }}
          </span>
          <button type="button" class="evo-ghost" @click="previewNonce += 1">刷新</button>
        </div>
        <iframe
          v-if="previewUrl"
          :key="`${previewUrl}-${previewNonce}`"
          :src="previewUrl"
          title="集思进化版本预览"
        />
      </div>
    </Dialog>

    <Dialog
      v-model:visible="conversationVisible"
      modal
      :header="conversationTitle"
      :style="{ width: 'min(720px, 94vw)' }"
    >
      <div class="evo-conversation">
        <div class="evo-chat-body">
          <article
            v-for="message in store.selectedChat"
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
            v-for="proposal in store.selectedProposals"
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
          <p v-if="!store.selectedChat.length && !store.selectedProposals.length" class="evo-empty">
            该用户还没有对话与提议。
          </p>
        </div>
        <footer class="evo-reply">
          <textarea
            v-model="replyText"
            rows="3"
            placeholder="以运营身份回复，回复会追加到该用户聊天记录…"
          />
          <button
            type="button"
            class="evo-primary"
            :disabled="sending || !replyText.trim()"
            @click="sendReply"
          >
            追加回复
          </button>
        </footer>
      </div>
    </Dialog>
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

.evo-head {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 20px;
}

.evo-head-title h2 {
  color: var(--ink);
  font-size: 18px;
  font-weight: 600;
}

.evo-head-title p {
  margin-top: 4px;
  color: var(--muted);
  font-size: 11px;
  letter-spacing: 0.03em;
}

.evo-stats {
  display: flex;
  gap: 10px;
}

.evo-stat {
  display: flex;
  min-width: 96px;
  flex-direction: column;
  gap: 2px;
  padding: 10px 14px;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 10px;
}

.evo-stat strong {
  color: var(--ink);
  font-size: 20px;
  font-weight: 600;
  line-height: 1.1;
}

.evo-stat span {
  color: var(--muted);
  font-size: 10px;
}

.evo-notice,
.evo-error {
  margin: 0;
  padding: 9px 12px;
  border: 1px solid var(--line);
  border-radius: 9px;
  font-size: 12px;
}

.evo-notice {
  background: var(--blue-soft);
  color: var(--blue);
}

.evo-error {
  background: var(--surface);
  color: var(--ink);
}

.evo-panel {
  display: flex;
  min-width: 0;
  flex-direction: column;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 12px;
}

.evo-panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  padding: 13px 16px;
  border-bottom: 1px solid var(--line);
}

.evo-panel-title {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.evo-panel-title strong {
  color: var(--ink);
  font-size: 13px;
  font-weight: 600;
}

.evo-panel-title span {
  color: var(--muted);
  font-size: 10px;
}

.evo-filters {
  display: flex;
  align-items: center;
  gap: 8px;
}

.evo-filters input,
.evo-filters select {
  padding: 6px 9px;
  background: var(--canvas);
  border: 1px solid var(--line);
  border-radius: 7px;
  color: var(--ink);
  font-size: 11px;
  outline: none;
}

.evo-filters input {
  width: 220px;
}

.evo-filters input:focus,
.evo-filters select:focus {
  background: var(--surface);
  border-color: var(--blue);
}

.evo-table-wrap {
  overflow-x: auto;
}

.evo-table {
  width: 100%;
  border-collapse: collapse;
}

.evo-table th {
  padding: 9px 14px;
  background: var(--canvas);
  border-bottom: 1px solid var(--line);
  color: var(--muted);
  font-size: 10px;
  font-weight: 500;
  letter-spacing: 0.04em;
  text-align: left;
  white-space: nowrap;
}

.evo-table td {
  padding: 11px 14px;
  border-bottom: 1px solid var(--line);
  color: var(--text);
  font-size: 11px;
  vertical-align: middle;
}

.evo-table tbody tr:last-child td {
  border-bottom: 0;
}

.evo-table tbody tr:hover {
  background: var(--canvas);
}

.evo-table .right {
  text-align: right;
}

.evo-user {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 120px;
}

.evo-avatar {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 26px;
  background: var(--ink);
  border-radius: 999px;
  color: var(--surface);
  font-size: 10px;
  font-weight: 600;
}

.evo-user-text {
  display: flex;
  flex-direction: column;
  gap: 1px;
  min-width: 0;
}

.evo-user-text strong {
  color: var(--ink);
  font-size: 11px;
  font-weight: 600;
}

.evo-user-text span {
  color: var(--muted);
  font-size: 9px;
}

.evo-version-label {
  color: var(--ink);
  font-size: 11px;
  font-weight: 600;
  white-space: nowrap;
}

.evo-summary {
  max-width: 320px;
  overflow: hidden;
  color: var(--text);
  font-size: 11px;
  line-height: 1.5;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.evo-summary-meta {
  display: block;
  margin-top: 2px;
  color: var(--muted);
  font-size: 9px;
}

.evo-proposal-count {
  color: var(--ink);
  font-size: 11px;
  font-weight: 600;
}

.evo-time {
  color: var(--muted);
  font-size: 10px;
  white-space: nowrap;
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

.evo-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
}

.evo-ghost {
  padding: 5px 9px;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 7px;
  color: var(--muted);
  font-size: 10px;
  white-space: nowrap;
  cursor: pointer;
}

.evo-ghost:hover {
  background: var(--soft);
  color: var(--ink);
}

.evo-ghost:disabled {
  color: var(--line-strong);
  cursor: not-allowed;
}

.evo-primary {
  padding: 6px 10px;
  background: var(--ink);
  border: 1px solid var(--ink);
  border-radius: 7px;
  color: var(--surface);
  font-size: 10px;
  font-weight: 500;
  white-space: nowrap;
  cursor: pointer;
}

.evo-primary:hover {
  background: var(--text);
}

.evo-primary:disabled {
  background: var(--line);
  border-color: var(--line);
  color: var(--muted);
  cursor: not-allowed;
}

.evo-empty {
  margin: 0;
  padding: 26px 16px;
  color: var(--muted);
  font-size: 11px;
  text-align: center;
}

.evo-preview-dialog {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.evo-preview-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  color: var(--muted);
  font-size: 11px;
}

.evo-preview-dialog iframe {
  width: 100%;
  height: min(640px, 68vh);
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 9px;
}

.evo-conversation {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.evo-chat-body {
  display: flex;
  max-height: min(460px, 52vh);
  flex-direction: column;
  gap: 9px;
  padding-right: 4px;
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

.evo-reply {
  display: flex;
  align-items: flex-end;
  gap: 8px;
  padding-top: 10px;
  border-top: 1px solid var(--line);
}

.evo-reply textarea {
  flex: 1;
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

.evo-reply textarea:focus {
  background: var(--surface);
  border-color: var(--blue);
}

@media (max-width: 1180px) {
  .evo-head {
    align-items: flex-start;
    flex-direction: column;
  }

  .evo-summary {
    max-width: 220px;
  }
}
</style>
