<script setup lang="ts">
const tabs = ['发现', '我的项目', '模板']
</script>

<template>
  <section class="home-discovery">
    <div class="discovery-head">
      <div class="discovery-tabs">
        <button
          v-for="(tab, index) in tabs"
          :key="tab"
          class="discovery-tab"
          :class="{ active: index === 0 }"
          type="button"
        >
          {{ tab }}
        </button>
      </div>
      <button class="discovery-more" type="button">
        查看全部
        <i class="pi pi-angle-right" />
      </button>
    </div>

    <div class="discovery-grid">
      <article v-for="index in 4" :key="index" class="discovery-card">
        <div class="card-skeleton">
          <span class="skeleton-chip" />
          <span class="skeleton-bar wide" />
          <span class="skeleton-bar" />
          <span class="skeleton-block" />
        </div>
        <div class="card-caption">
          <span class="skeleton-title" />
          <span class="skeleton-meta">模板占位 {{ index }}</span>
        </div>
      </article>
    </div>
  </section>
</template>
