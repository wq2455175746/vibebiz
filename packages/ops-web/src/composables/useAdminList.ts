import { onMounted, ref, type Ref } from 'vue'

export interface AdminList<T> {
  rows: Ref<T[]>
  loading: Ref<boolean>
  error: Ref<string>
  load: () => Promise<void>
}

export function useAdminList<T>(loader: () => Promise<T[]>): AdminList<T> {
  const rows = ref<T[]>([]) as Ref<T[]>
  const loading = ref(false)
  const error = ref('')

  async function load(): Promise<void> {
    loading.value = true
    error.value = ''
    try {
      rows.value = await loader()
    } catch (err) {
      error.value = err instanceof Error ? err.message : '加载失败'
    } finally {
      loading.value = false
    }
  }

  onMounted(load)

  return { rows, loading, error, load }
}
