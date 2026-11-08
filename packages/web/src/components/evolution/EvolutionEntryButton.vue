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
  background: var(--ink);
  border: 1px solid var(--ink);
  border-radius: 999px;
  box-shadow: 0 12px 30px rgb(17 17 17 / 18%);
  color: var(--surface);
  cursor: pointer;
  transition:
    transform 160ms ease,
    box-shadow 160ms ease;
}

.evolution-entry:hover {
  transform: translateY(-2px);
  box-shadow: 0 16px 36px rgb(17 17 17 / 24%);
}

.evolution-entry-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  background: var(--blue);
  border-radius: 999px;
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
  color: rgb(255 255 255 / 68%);
  font-size: 11px;
}
</style>
