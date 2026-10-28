import { createApp } from 'vue'
import { createPinia } from 'pinia'
import PrimeVue from 'primevue/config'
import Aura from '@primevue/themes/aura'
import ToastService from 'primevue/toastservice'
import Tooltip from 'primevue/tooltip'
import 'primeicons/primeicons.css'

import App from './App.vue'
import { onUnauthorized } from './api/client'
import router from './router'
import { useAuthStore } from './stores/auth'
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
