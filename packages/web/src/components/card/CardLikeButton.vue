<script setup lang="ts">
defineProps<{ liked: boolean; count: number; busy?: boolean }>()
const emit = defineEmits<{ toggle: [] }>()
</script>

<template>
  <button
    class="card-like-button"
    :class="{ 'is-liked': liked }"
    type="button"
    :disabled="busy"
    @click.stop="emit('toggle')"
  >
    <i :class="liked ? 'pi pi-heart-fill' : 'pi pi-heart'" />
    {{ count }}
  </button>
</template>

<style scoped>
.card-like-button {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  height: 26px;
  padding: 0 10px;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 999px;
  color: var(--muted);
  font-size: 12px;
  cursor: pointer;
}

.card-like-button.is-liked {
  border-color: var(--blue);
  color: var(--blue);
}

.card-like-button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
