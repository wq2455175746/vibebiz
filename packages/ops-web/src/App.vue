<template>
  <RouterView />
  <Toast position="top-right" />
</template>

<script setup lang="ts">
import Toast from 'primevue/toast'
import { RouterView } from 'vue-router'
</script>
