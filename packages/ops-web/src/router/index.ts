import { createRouter, createWebHistory } from 'vue-router'

import OpsLayout from '@/layouts/OpsLayout.vue'
import LoginView from '@/views/LoginView.vue'
import { useOpsAuthStore } from '@/stores/auth'

import { childRoutes } from './routes'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/login',
      name: 'login',
      component: LoginView,
      meta: { title: '登录', public: true },
    },
    {
      path: '/',
      component: OpsLayout,
      children: childRoutes,
    },
  ],
})

router.beforeEach(async (to) => {
  if (to.meta.public) return true

  const auth = useOpsAuthStore()
  try {
    await auth.ensure()
  } catch {
    return { name: 'login', query: { redirect: to.fullPath } }
  }

  if (!auth.isAuthenticated) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  return true
})

router.afterEach((to) => {
  const title = typeof to.meta.title === 'string' ? to.meta.title : '概览'
  document.title = `Vibebiz Ops · ${title}`
})

export default router
