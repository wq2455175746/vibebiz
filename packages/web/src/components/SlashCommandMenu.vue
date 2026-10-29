<script setup lang="ts">
import type { SlashCommand } from '@/utils/slashCommands'

defineProps<{
  commands: SlashCommand[]
  activeIndex: number
}>()

const emit = defineEmits<{ select: [command: SlashCommand] }>()
</script>

<template>
  <ul v-if="commands.length" class="slash-menu" role="listbox">
    <li v-for="(command, index) in commands" :key="command.name">
      <button
        type="button"
        role="option"
        :aria-selected="index === activeIndex"
        :class="{ active: index === activeIndex, disabled: command.disabled }"
        @mousedown.prevent="emit('select', command)"
      >
        <span class="slash-title">{{ command.title }}</span>
        <span class="slash-description">{{ command.description }}</span>
      </button>
    </li>
  </ul>
</template>
