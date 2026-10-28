import { createApp } from 'vue'
import { createPinia } from 'pinia'
import PrimeVue from 'primevue/config'
import { definePreset } from '@primevue/themes'
import Aura from '@primevue/themes/aura'
import ToastService from 'primevue/toastservice'
import Tooltip from 'primevue/tooltip'
import 'primeicons/primeicons.css'

import App from './App.vue'
import { onUnauthorized } from './api/client'
import router from './router'
import { useAuthStore } from './stores/auth'
import './styles/global.css'

const VibebizPreset = definePreset(Aura, {
  semantic: {
    primary: {
      50: '{gray.50}',
      100: '{gray.100}',
      200: '{gray.200}',
      300: '{gray.300}',
      400: '{gray.400}',
      500: '#111111',
      600: '#1f2937',
      700: '#111111',
      800: '#111111',
      900: '#111111',
      950: '#111111',
      color: '{primary.500}',
      contrastColor: '#ffffff',
      hoverColor: '{primary.600}',
      activeColor: '{primary.700}',
    },
    focusRing: {
      width: '2px',
      style: 'solid',
      color: '{primary.500}',
      offset: '2px',
    },
  },
})

const app = createApp(App)
const pinia = createPinia()

app.use(pinia)
app.use(router)
app.use(PrimeVue, {
  theme: {
    preset: VibebizPreset,
    options: {
      darkModeSelector: '.dark',
      cssLayer: false,
    },
  },
  ripple: true,
})
app.use(ToastService)
app.directive('tooltip', Tooltip)

onUnauthorized(() => {
  const auth = useAuthStore(pinia)
  auth.clear()
  const current = router.currentRoute.value
  if (current.name && current.name !== 'login') {
    void router.replace({ name: 'login', query: { redirect: current.fullPath } })
  }
})

app.mount('#app')
