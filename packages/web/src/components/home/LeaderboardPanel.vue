<script setup lang="ts">
import { onMounted, ref } from 'vue'

import { getLeaderboard } from '@/api/leaderboard'
import type { LeaderboardEntry } from '@/types'

const entries = ref<LeaderboardEntry[]>([])
const loading = ref(true)
const error = ref('')
const failedCovers = ref<Record<string, boolean>>({})

async function load() {
  loading.value = true
  error.value = ''
  try {
    entries.value = await getLeaderboard()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '排行榜加载失败'
    entries.value = []
  } finally {
    loading.value = false
  }
}

function hasCover(entry: LeaderboardEntry) {
  return Boolean(entry.coverUrl) && !failedCovers.value[entry.projectId]
}

function markCoverFailed(projectId: string) {
  failedCovers.value = { ...failedCovers.value, [projectId]: true }
}

function open(entry: LeaderboardEntry) {
  window.open(entry.url, '_blank', 'noopener,noreferrer')
}

onMounted(load)
</script>

<template>
  <div class="projects-page leaderboard-page">
    <header class="projects-head">
      <h1>发布作品排行榜</h1>
      <span v-if="!loading && !error">{{ entries.length }} 个作品</span>
    </header>

    <p v-if="loading" class="discovery-empty">加载中...</p>
    <p v-else-if="error" class="error-text" role="alert">{{ error }}</p>
    <p v-else-if="entries.length === 0" class="discovery-empty">暂无排行榜数据</p>
    <div v-else class="leaderboard-grid">
      <article v-for="entry in entries" :key="entry.projectId" class="project-card leaderboard-card">
        <button class="project-card-open" type="button" @click="open(entry)">
          <span class="project-cover leaderboard-cover" :class="{ empty: !hasCover(entry) }">
            <img
              v-if="hasCover(entry)"
              :src="entry.coverUrl"
              alt=""
              @error="markCoverFailed(entry.projectId)"
            />
            <span v-else class="project-cover-empty">
              <i class="pi pi-image" />
              <span>暂无预览图</span>
            </span>
            <span class="leaderboard-rank" :class="{ top: entry.rank <= 3 }">
              #{{ entry.rank }}
            </span>
          </span>
          <span class="project-info leaderboard-info">
            <strong>{{ entry.name }}</strong>
            <span class="leaderboard-author">作者：{{ entry.author || '未知作者' }}</span>
            <span class="leaderboard-visits">{{ entry.visits }} 次访问</span>
          </span>
        </button>
      </article>
    </div>
  </div>
</template>

<style scoped>
.leaderboard-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}

.leaderboard-card {
  min-width: 0;
}

.leaderboard-cover {
  position: relative;
}

.leaderboard-rank {
  position: absolute;
  top: 12px;
  left: 12px;
  display: grid;
  min-width: 38px;
  height: 30px;
  padding: 0 8px;
  place-items: center;
  background: var(--ink);
  border-radius: 9px;
  color: var(--surface);
  font-size: 12px;
  font-weight: 700;
}

.leaderboard-rank.top {
  background: var(--blue);
}

.leaderboard-info {
  gap: 6px;
  padding-right: 16px;
}

.leaderboard-author,
.leaderboard-visits {
  color: var(--muted);
  font-size: 12px;
}

@media (max-width: 720px) {
  .leaderboard-grid {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
