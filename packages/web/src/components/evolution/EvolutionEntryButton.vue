<script setup lang="ts">
import { useRouter } from 'vue-router'

const router = useRouter()

function open(): void {
  void router.push({ name: 'evolution' })
}
</script>

<template>
  <Teleport to="body">
    <button class="evolution-entry" type="button" @click="open">
      <span class="evolution-entry-icon"><i class="pi pi-sparkles" /></span>
      <span class="evolution-entry-text">
        <strong>集思进化计划</strong>
        <small>把痛点变成下一个版本</small>
      </span>
    </button>
  </Teleport>
</template>

<style scoped>
.evolution-entry {
  position: fixed;
  right: 24px;
  bottom: 24px;
  z-index: 9999;
  display: inline-flex;
  align-items: center;
  gap: 10px;
  padding: 10px 16px 10px 12px;
  background: rgb(31 41 55 / 30%);
  border: 1px solid rgb(31 41 55 / 18%);
  border-radius: 999px;
  backdrop-filter: blur(8px);
  box-shadow: 0 10px 24px rgb(17 17 17 / 8%);
  color: rgb(255 255 255 / 94%);
  cursor: pointer;
  transition:
    background 160ms ease,
    transform 160ms ease,
    box-shadow 160ms ease;
  -webkit-backdrop-filter: blur(8px);
}

.evolution-entry:hover {
  background: rgb(31 41 55 / 45%);
  box-shadow: 0 14px 30px rgb(17 17 17 / 14%);
  transform: translateY(-2px);
}

.evolution-entry-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  background: rgb(255 255 255 / 18%);
  border-radius: 999px;
  color: rgb(255 255 255 / 96%);
  font-size: 13px;
}

.evolution-entry-text {
  display: flex;
  flex-direction: column;
  gap: 2px;
  text-align: left;
}

.evolution-entry-text strong {
  font-size: 13px;
  font-weight: 600;
  letter-spacing: 0.02em;
}

.evolution-entry-text small {
  color: rgb(255 255 255 / 62%);
  font-size: 11px;
}
</style>
