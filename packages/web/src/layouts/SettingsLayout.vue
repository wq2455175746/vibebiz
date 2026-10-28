<script setup lang="ts">
import { RouterLink, RouterView, useRouter } from 'vue-router'

import { settingsMenu, settingsGroups } from '@/router/settingsRoutes'
import '@/styles/settings.css'

const router = useRouter()

function goHome() {
  void router.push({ name: 'home' })
}
</script>

<template>
  <div class="settings-shell">
    <aside class="settings-sidebar">
      <div class="settings-brand">
        <span class="settings-logo"><i class="pi pi-box" /></span>
        <span class="settings-brand-name">Vibebiz 设置</span>
      </div>

      <button class="settings-back" type="button" @click="goHome">
        <i class="pi pi-angle-left" />
        返回
      </button>

      <nav class="settings-menu" aria-label="设置导航">
        <div v-for="group in settingsGroups" :key="group" class="settings-menu-group">
          <p class="settings-menu-title">{{ group }}</p>
          <RouterLink
            v-for="item in settingsMenu.filter((entry) => entry.group === group)"
            :key="item.name"
            class="settings-menu-item"
            :to="item.to"
            active-class="active"
          >
            <i :class="item.icon" />
            <span>{{ item.label }}</span>
          </RouterLink>
        </div>
      </nav>
    </aside>

    <main class="settings-main">
      <div class="settings-content">
        <RouterView />
      </div>
    </main>
  </div>
</template>
