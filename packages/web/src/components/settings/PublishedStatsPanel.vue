<script setup lang="ts">
import type { PublishedStats } from '@/types'

const props = withDefaults(
  defineProps<{
    stats: PublishedStats | null
    loading?: boolean
    error?: string
  }>(),
  {
    loading: false,
    error: '',
  },
)
</script>

<template>
  <section class="settings-published-stats">
    <header class="stats-head">
      <div>
        <h2>发布数据</h2>
        <p>统计已发布作品的曝光与点击。</p>
      </div>
    </header>

    <p v-if="props.loading" class="settings-state">加载中…</p>
    <p v-else-if="props.error" class="settings-error" role="alert">{{ props.error }}</p>
    <template v-else>
      <div class="stats-grid">
        <article class="stat-card">
          <span>曝光量</span>
          <strong>{{ props.stats?.exposure ?? 0 }}</strong>
        </article>
        <article class="stat-card">
          <span>点击数</span>
          <strong>{{ props.stats?.clicks ?? 0 }}</strong>
        </article>
        <article class="stat-card">
          <span>停留时长</span>
          <strong>{{ props.stats?.dwellSeconds ?? 0 }} 秒</strong>
        </article>
        <article class="stat-card">
          <span>消费额度</span>
          <strong>{{ props.stats?.spend ?? 0 }}</strong>
        </article>
      </div>

      <p v-if="!props.stats || props.stats.projects.length === 0" class="settings-state">
        暂无发布数据
      </p>
      <table v-else class="stats-table">
        <thead>
          <tr>
            <th>项目</th>
            <th>曝光量</th>
            <th>点击数</th>
            <th>停留时长</th>
            <th>消费额度</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="item in props.stats.projects" :key="item.projectId">
            <td>{{ item.name }}</td>
            <td>{{ item.exposure }}</td>
            <td>{{ item.clicks }}</td>
            <td>{{ item.dwellSeconds }} 秒</td>
            <td>{{ item.spend }}</td>
          </tr>
        </tbody>
      </table>
    </template>
  </section>
</template>

<style scoped>
.settings-published-stats {
  margin-top: 32px;
}

.stats-head h2 {
  margin: 0;
  color: var(--ink);
  font-size: 16px;
}

.stats-head p {
  margin: 6px 0 16px;
  color: var(--muted);
  font-size: 12px;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 16px;
}

.stat-card {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 16px;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 12px;
}

.stat-card span {
  color: var(--muted);
  font-size: 12px;
}

.stat-card strong {
  color: var(--ink);
  font-size: 22px;
}

.stats-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

.stats-table th,
.stats-table td {
  padding: 10px 12px;
  border-top: 1px solid var(--line);
  text-align: left;
}

.stats-table th {
  background: var(--soft);
  color: var(--muted);
  font-size: 12px;
  font-weight: 600;
}

.stats-table td {
  color: var(--text);
}

@media (max-width: 720px) {
  .stats-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
