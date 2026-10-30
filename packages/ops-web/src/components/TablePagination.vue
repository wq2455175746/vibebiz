<script setup lang="ts">
import Paginator from 'primevue/paginator'

const props = defineProps<{
  page: number
  pageSize: number
  total: number
}>()

const emit = defineEmits<{
  page: [value: number]
  'page-size': [value: number]
}>()

interface PaginatorEvent {
  page: number
  rows: number
}

function onPage(event: PaginatorEvent) {
  if (event.rows !== props.pageSize) {
    emit('page-size', event.rows)
    return
  }
  emit('page', event.page + 1)
}
</script>

<template>
  <div class="table-pagination">
    <Paginator
      :first="(page - 1) * pageSize"
      :rows="pageSize"
      :total-records="total"
      :rows-per-page-options="[10, 20, 50]"
      @page="onPage"
    />
  </div>
</template>

<style scoped>
.table-pagination {
  display: flex;
  justify-content: flex-end;
  margin-top: 12px;
}
</style>
