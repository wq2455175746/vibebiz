import { createApp } from 'vue'
import { createPinia } from 'pinia'
import PrimeVue from 'primevue/config'
import Aura from '@primevue/themes/aura'
import 'primeicons/primeicons.css'

import App from './App.vue'
import { onUnauthorized } from './api/client'
import router from './router'
import { useOpsAuthStore } from './stores/auth'
import './styles/global.css'

const app = createApp(App)
const pinia = createPinia()

app.use(pinia)
app.use(router)
app.use(PrimeVue, {
  theme: {
    preset: Aura,
    options: {
      darkModeSelector: '.dark',
      cssLayer: false,
    },
  },
  ripple: true,
})

onUnauthorized(() => {
  const auth = useOpsAuthStore(pinia)
  auth.clear()

  const current = router.currentRoute.value
  if (current.name !== 'login') {
    router.replace({ name: 'login', query: { redirect: current.fullPath } }).catch(() => undefined)
  }
})

app.mount('#app')
