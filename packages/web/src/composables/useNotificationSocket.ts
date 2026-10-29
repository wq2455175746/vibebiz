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
    socket = new WebSocket(buildNotificationSocketUrl(base, window.location))
    socket.onopen = () => {
      connected.value = true
      retry = 1000
    }
    socket.onmessage = (event) => {
      try {
        const payload = JSON.parse(event.data) as { type?: string }
        if (payload.type === 'notifications.changed') {
          onChange()
        }
      } catch {
        // ignore malformed events
      }
    }
    socket.onclose = () => {
      connected.value = false
      socket = null
      if (!stopped) {
        schedule()
      }
    }
    socket.onerror = () => socket?.close()
  }

  function schedule(): void {
    timer = setTimeout(open, retry)
    retry = Math.min(retry * 2, 30_000)
  }

  function connect(): void {
    stopped = false
    if (socket) {
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
    socket?.close()
    socket = null
    connected.value = false
  }

  onBeforeUnmount(disconnect)

  return { connected, connect, disconnect }
}
