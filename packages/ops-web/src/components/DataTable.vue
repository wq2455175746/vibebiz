<script setup lang="ts">
import type { TableColumn, TableRow } from '@/types'

const props = withDefaults(
  defineProps<{
    columns: TableColumn[]
    rows: TableRow[]
    loading?: boolean
    error?: string
    emptyText?: string
  }>(),
  {
    loading: false,
    error: '',
    emptyText: '暂无数据',
  },
)

const emit = defineEmits<{ retry: [] }>()

function raw(row: TableRow, key: string): unknown {
  return (row as Record<string, unknown>)[key]
}

function display(row: TableRow, column: TableColumn): string {
  const value = raw(row, column.key)
  if (column.format) {
    return column.format(value, row)
  }
  if (value === null || value === undefined || value === '') {
    return '—'
  }
  return String(value)
}

function highlighted(row: TableRow, column: TableColumn): boolean {
  if (!column.highlight) {
    return false
  }
  return column.highlight.includes(String(raw(row, column.key)))
}
</script>

<template>
  <div class="data-table">
    <div v-if="props.loading" class="table-state">加载中...</div>
    <div v-else-if="props.error" class="table-state table-error">
      <span class="error-text">{{ props.error }}</span>
      <button class="retry-btn" type="button" @click="emit('retry')">重试</button>
    </div>
    <div v-else-if="props.rows.length === 0" class="table-state">{{ props.emptyText }}</div>
    <div v-else class="table-scroll">
      <table>
        <thead>
          <tr>
            <th
              v-for="column in props.columns"
              :key="column.key"
              :style="{ width: column.width }"
              scope="col"
            >
              {{ column.label }}
            </th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(row, index) in props.rows" :key="index">
            <td v-for="column in props.columns" :key="column.key">
              <span
                v-if="column.tag"
                class="tag"
                :class="{ 'tag-accent': highlighted(row, column) }"
              >
                {{ display(row, column) }}
              </span>
              <template v-else>{{ display(row, column) }}</template>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<style scoped>
.data-table {
  overflow: hidden;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 14px;
}

.table-state {
  padding: 44px 20px;
  color: var(--muted);
  text-align: center;
}

.table-error {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}

.retry-btn {
  padding: 6px 16px;
  background: var(--surface);
  border: 1px solid var(--line-strong);
  border-radius: 999px;
  color: var(--ink);
  cursor: pointer;
}

.retry-btn:hover {
  background: var(--soft);
}

.table-scroll {
  overflow-x: auto;
}

table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

th {
  padding: 12px 16px;
  background: var(--soft);
  color: var(--muted);
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.02em;
  text-align: left;
  white-space: nowrap;
}

td {
  padding: 14px 16px;
  border-top: 1px solid var(--line);
  color: var(--text);
}

tbody tr:hover {
  background: var(--blue-soft);
}

.tag-accent {
  background: var(--blue-soft);
  border-color: rgba(37, 99, 235, 0.35);
  color: var(--blue);
}
</style>
