import { onBeforeUnmount, ref, watch, type Ref } from 'vue'

import { dataUrlToBlob } from '@/utils/media'

export function useBlobUrl(source: Ref<string>) {
  const url = ref('')
  let objectUrl = ''

  function revoke() {
    if (objectUrl && typeof URL.revokeObjectURL === 'function') {
      URL.revokeObjectURL(objectUrl)
    }
    objectUrl = ''
  }

  watch(
    source,
    (value) => {
      revoke()
      if (!value) {
        url.value = ''
        return
      }
      if (!value.startsWith('data:') || typeof URL.createObjectURL !== 'function') {
        url.value = value
        return
      }
      try {
        objectUrl = URL.createObjectURL(dataUrlToBlob(value))
        url.value = objectUrl
      } catch {
        url.value = value
      }
    },
    { immediate: true },
  )

  onBeforeUnmount(revoke)

  return url
}
