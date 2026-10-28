<script setup lang="ts">
import { useOpsStore } from '@/stores/ops'

const ops = useOpsStore()

const services = [
  { label: 'CP 控制面', value: 'vibebiz-dev.com:8080', note: '业务前缀 /cp/api/v1' },
  {
    label: 'DP 数据面',
    value: 'vibebiz-dev.com:8081',
    note: '文件 / SSE / 预览，业务前缀由 DP 提供',
  },
  { label: 'sandbox', value: '8000', note: '文件 API / Agent 循环 / 静态预览' },
  { label: 'web', value: 'vibebiz-dev.com:5173', note: '用户控制台' },
  { label: 'ops-web', value: 'vibebiz-dev-ops.com:5174', note: '本运营后台' },
]

const runtime = [
  { label: 'API 基础路径', value: ops.apiBase, note: '仅访问 CP admin 路由' },
  {
    label: 'OTEL_EXPORTER_OTLP_ENDPOINT',
    value: '未配置',
    note: '为空时关闭 Trace 导出，不产生 exporter 错误',
  },
  { label: 'MySQL 目标', value: '未接入', note: '本轮 admin 使用 CP 内存 seed 数据' },
  { label: '日志', value: 'stdout JSON', note: 'CP / DP / sandbox 统一结构化日志' },
]

const security = [
  { label: '环境', value: ops.environment, note: '仅允许本地访问' },
  {
    label: '鉴权',
    value: 'HttpOnly Cookie',
    note: 'Redis 服务端 Session，生产环境必须接入 SSO + RBAC',
  },
  { label: 'admin 路由', value: '不得裸露在公网', note: '由网关或内网访问策略限制' },
]
</script>

<template>
  <div class="page">
    <header class="page-head">
      <div>
        <h1>系统设置</h1>
        <p class="page-desc">静态只读运行信息，不在本页修改配置。</p>
      </div>
      <span class="tag">只读</span>
    </header>

    <div class="settings-grid">
      <section class="setting-card">
        <h2>服务与端口</h2>
        <div class="setting-list">
          <div v-for="item in services" :key="item.label" class="setting-row">
            <div>
              <p class="setting-label">{{ item.label }}</p>
              <p class="setting-note">{{ item.note }}</p>
            </div>
            <span class="setting-value">{{ item.value }}</span>
          </div>
        </div>
      </section>

      <section class="setting-card">
        <h2>数据与观测</h2>
        <div class="setting-list">
          <div v-for="item in runtime" :key="item.label" class="setting-row">
            <div>
              <p class="setting-label">{{ item.label }}</p>
              <p class="setting-note">{{ item.note }}</p>
            </div>
            <span class="setting-value">{{ item.value }}</span>
          </div>
        </div>
      </section>

      <section class="setting-card">
        <h2>安全边界</h2>
        <div class="setting-list">
          <div v-for="item in security" :key="item.label" class="setting-row">
            <div>
              <p class="setting-label">{{ item.label }}</p>
              <p class="setting-note">{{ item.note }}</p>
            </div>
            <span class="setting-value">{{ item.value }}</span>
          </div>
        </div>
      </section>
    </div>
  </div>
</template>

<style scoped>
.settings-grid {
  display: grid;
  gap: 16px;
}

.setting-card {
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 14px;
}

.setting-card h2 {
  padding: 16px 20px;
  border-bottom: 1px solid var(--line);
  color: var(--ink);
  font-size: 14px;
  font-weight: 600;
}

.setting-list {
  display: flex;
  flex-direction: column;
}

.setting-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 20px;
  padding: 14px 20px;
  border-top: 1px solid var(--line);
}

.setting-row:first-child {
  border-top: 0;
}

.setting-label {
  color: var(--text);
  font-size: 13px;
}

.setting-note {
  margin-top: 4px;
  color: var(--muted);
  font-size: 12px;
}

.setting-value {
  color: var(--ink);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12px;
  text-align: right;
}
</style>
