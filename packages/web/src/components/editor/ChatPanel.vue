<script setup lang="ts">
import Button from 'primevue/button'
</script>

<template>
  <aside class="chat-pane">
    <div class="panel-head">
      <span>对话</span>
      <span class="tag">M1</span>
    </div>

    <div class="stub-note">
      <p>这里将承载 SSE 消息流与工具调用卡片。</p>
      <p class="muted">当前 harness 已在 Gateway / Sandbox 打通流式契约，前端交互留到 M1。</p>
    </div>

    <div class="stub-composer">
      <span class="muted">让智能体团队实现你的想法</span>
      <Button icon="pi pi-arrow-up" rounded disabled aria-label="发送" />
    </div>
  </aside>
</template>
