import { onMounted, ref, type Ref } from 'vue'

import type { PageQuery, PageResult } from '@/types'

export interface AdminPage<T> {
  rows: Ref<T[]>
  total: Ref<number>
  page: Ref<number>
  pageSize: Ref<number>
  keyword: Ref<string>
  loading: Ref<boolean>
  error: Ref<string>
  load: () => Promise<void>
  search: () => Promise<void>
  setPage: (next: number) => Promise<void>
  setPageSize: (next: number) => Promise<void>
}

export function useAdminPage<T>(
  loader: (query: PageQuery) => Promise<PageResult<T>>,
  initialPageSize = 10,
): AdminPage<T> {
  const rows = ref<T[]>([]) as Ref<T[]>
  const total = ref(0)
  const page = ref(1)
  const pageSize = ref(initialPageSize)
  const keyword = ref('')
  const loading = ref(false)
  const error = ref('')

  async function load(): Promise<void> {
    loading.value = true
    error.value = ''
    try {
      const result = await loader({
        page: page.value,
        pageSize: pageSize.value,
        keyword: keyword.value.trim(),
      })
      rows.value = result.items
      total.value = result.total
      page.value = result.page
      pageSize.value = result.pageSize
    } catch (err) {
      error.value = err instanceof Error ? err.message : '加载失败'
    } finally {
      loading.value = false
    }
  }

  async function search(): Promise<void> {
    page.value = 1
    await load()
  }

  async function setPage(next: number): Promise<void> {
    page.value = next
    await load()
  }

  async function setPageSize(next: number): Promise<void> {
    pageSize.value = next
    page.value = 1
    await load()
  }

  onMounted(load)

  return {
    rows,
    total,
    page,
    pageSize,
    keyword,
    loading,
    error,
    load,
    search,
    setPage,
    setPageSize,
  }
}
