<script setup lang="ts">
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'

defineProps<{
  keyword: string
}>()

const emit = defineEmits<{
  'update:keyword': [value: string]
  search: []
}>()
</script>

<template>
  <div class="table-search">
    <InputText
      :model-value="keyword"
      placeholder="输入关键词搜索"
      @update:model-value="emit('update:keyword', String($event ?? ''))"
      @keyup.enter="emit('search')"
    />
    <Button label="搜索" icon="pi pi-search" @click="emit('search')" />
  </div>
</template>

<style scoped>
.table-search {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  margin-bottom: 12px;
}

.table-search :deep(input) {
  width: 240px;
}
</style>
