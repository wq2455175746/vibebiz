import { onBeforeUnmount, ref } from 'vue'

import { buildNotificationSocketUrl } from '@/utils/notificationSocket'

export function useNotificationSocket(onChange: () => void) {
  const connected = ref(false)
  let socket: WebSocket | null = null
  let retry = 1000
  let timer: ReturnType<typeof setTimeout> | null = null
  let stopped = true

  function open(): void {
    const base = import.meta.env.VITE_CP_BASE || '/cp/api/v1'
    const ws = new WebSocket(buildNotificationSocketUrl(base, window.location))
    socket = ws
    ws.onopen = () => {
      if (ws !== socket) {
        return
      }
      connected.value = true
      retry = 1000
    }
    ws.onmessage = (event) => {
      if (ws !== socket) {
        return
      }
      try {
        const payload = JSON.parse(event.data) as { type?: string }
        if (payload.type === 'notifications.changed') {
          onChange()
        }
      } catch {
        // ignore malformed events
      }
    }
    ws.onclose = () => {
      if (ws !== socket) {
        return
      }
      connected.value = false
      socket = null
      if (!stopped) {
        schedule()
      }
    }
    ws.onerror = () => ws.close()
  }

  function schedule(): void {
    if (timer) {
      return
    }
    timer = setTimeout(() => {
      timer = null
      open()
    }, retry)
    retry = Math.min(retry * 2, 30_000)
  }

  function connect(): void {
    stopped = false
    if (socket || timer) {
      return
    }
    open()
  }

  function disconnect(): void {
    stopped = true
    if (timer) {
      clearTimeout(timer)
    }
    timer = null
    const ws = socket
    socket = null
    connected.value = false
    ws?.close()
  }

  onBeforeUnmount(disconnect)

  return { connected, connect, disconnect }
}
