<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink, RouterView, useRoute } from 'vue-router'

import { navItems } from '@/router/navigation'
import { useOpsStore } from '@/stores/ops'

const route = useRoute()
const ops = useOpsStore()

const title = computed(() => (typeof route.meta.title === 'string' ? route.meta.title : '概览'))
</script>

<template>
  <div class="ops-shell">
    <aside class="ops-sidebar">
      <div class="ops-brand">
        <span class="ops-logo"><i class="pi pi-box" /></span>
        <div class="ops-brand-text">
          <strong>Vibebiz Ops</strong>
          <span>运营后台</span>
        </div>
      </div>

      <nav class="ops-nav" aria-label="运营后台导航">
        <RouterLink
          v-for="item in navItems"
          :key="item.name"
          :to="item.path"
          class="ops-nav-item"
          :class="{ active: route.name === item.name }"
        >
          <i :class="item.icon" />
          <span>{{ item.label }}</span>
        </RouterLink>
      </nav>

      <div class="ops-sidebar-foot">
        <span class="tag">本机模式</span>
      </div>
    </aside>

    <div class="ops-main">
      <header class="ops-topbar">
        <span class="ops-topbar-title">{{ title }}</span>
        <div class="ops-topbar-meta">
          <span class="ops-env">{{ ops.environment }}</span>
          <span class="muted small">{{ ops.apiBase }}</span>
        </div>
      </header>

      <main class="ops-content">
        <RouterView />
      </main>
    </div>
  </div>
</template>

<style scoped>
.ops-shell {
  display: grid;
  grid-template-columns: 232px minmax(0, 1fr);
  height: 100%;
  background: var(--canvas);
}

.ops-sidebar {
  display: flex;
  flex-direction: column;
  min-height: 0;
  background: var(--surface);
  border-right: 1px solid var(--line);
}

.ops-brand {
  display: flex;
  align-items: center;
  gap: 12px;
  height: 64px;
  padding: 0 20px;
  border-bottom: 1px solid var(--line);
}

.ops-logo {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 34px;
  height: 34px;
  background: var(--ink);
  border-radius: 9px;
  color: var(--surface);
  font-size: 15px;
}

.ops-brand-text {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.ops-brand-text strong {
  color: var(--ink);
  font-size: 14px;
  font-weight: 600;
}

.ops-brand-text span {
  color: var(--muted);
  font-size: 11px;
  letter-spacing: 0.08em;
}

.ops-nav {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 2px;
  padding: 14px 10px;
  overflow-y: auto;
}

.ops-nav-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border-radius: 9px;
  color: var(--text);
  font-size: 13px;
}

.ops-nav-item i {
  width: 16px;
  color: var(--muted);
  font-size: 14px;
}

.ops-nav-item:hover {
  background: var(--soft);
}

.ops-nav-item.active {
  background: var(--blue-soft);
  color: var(--blue);
  font-weight: 600;
}

.ops-nav-item.active i {
  color: var(--blue);
}

.ops-sidebar-foot {
  padding: 16px 20px;
  border-top: 1px solid var(--line);
}

.ops-main {
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
}

.ops-topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 64px;
  padding: 0 28px;
  background: var(--surface);
  border-bottom: 1px solid var(--line);
}

.ops-topbar-title {
  color: var(--ink);
  font-size: 15px;
  font-weight: 600;
}

.ops-topbar-meta {
  display: flex;
  align-items: center;
  gap: 12px;
}

.ops-env {
  display: inline-flex;
  align-items: center;
  padding: 3px 9px;
  background: var(--ink);
  border-radius: 999px;
  color: var(--surface);
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.12em;
}

.ops-content {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
}

@media (max-width: 860px) {
  .ops-shell {
    grid-template-columns: 72px minmax(0, 1fr);
  }

  .ops-brand {
    justify-content: center;
    padding: 0;
  }

  .ops-brand-text,
  .ops-nav-item span,
  .ops-sidebar-foot {
    display: none;
  }

  .ops-nav-item {
    justify-content: center;
  }
}
</style>
