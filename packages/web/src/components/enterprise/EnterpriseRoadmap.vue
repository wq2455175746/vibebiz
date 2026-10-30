<script setup lang="ts">
import { ENTERPRISE_STAGES } from '@/utils/enterprise'
</script>

<template>
  <section class="enterprise-roadmap" aria-label="企业工程开发阶段">
    <header class="enterprise-roadmap-head">
      <p>交付流程</p>
      <span>7 个阶段 · 逐步确认</span>
    </header>
    <ol class="enterprise-roadmap-list">
      <li
        v-for="stage in ENTERPRISE_STAGES"
        :key="stage.key"
        class="enterprise-roadmap-item"
        :class="{ placeholder: stage.workspace === 'placeholder' }"
      >
        <span class="enterprise-roadmap-seq">{{ stage.seq }}</span>
        <span class="enterprise-roadmap-copy">
          <strong>{{ stage.title }}</strong>
          <small>{{ stage.roleLabel }}</small>
        </span>
        <i v-if="stage.workspace === 'placeholder'" class="pi pi-lock" />
      </li>
    </ol>
  </section>
</template>
