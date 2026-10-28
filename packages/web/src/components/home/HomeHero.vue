<script setup lang="ts">
const prompt = defineModel<string>({ required: true })

defineProps<{
  creating: boolean
  error: string
}>()

const emit = defineEmits<{ submit: [] }>()
</script>

<template>
  <section class="home-hero">
    <h1 class="hero-title">你想创造什么，wangh11？</h1>

    <div class="hero-composer">
      <textarea
        v-model="prompt"
        class="hero-input"
        rows="3"
        placeholder="请 Vibebiz 生成你的 LinkHub。"
        @keydown.ctrl.enter.prevent="emit('submit')"
        @keydown.meta.enter.prevent="emit('submit')"
      />
      <div class="composer-bar">
        <button class="composer-plus" type="button" aria-label="添加附件">
          <i class="pi pi-plus" />
        </button>
        <div class="composer-actions">
          <button class="composer-icon" type="button" aria-label="语音输入">
            <i class="pi pi-microphone" />
          </button>
          <button
            class="composer-send"
            type="button"
            :disabled="!prompt.trim() || creating"
            aria-label="生成"
            @click="emit('submit')"
          >
            <i :class="creating ? 'pi pi-spin pi-spinner' : 'pi pi-arrow-up'" />
          </button>
        </div>
      </div>
    </div>

    <p v-if="error" class="error-text hero-error">{{ error }}</p>
  </section>
</template>
