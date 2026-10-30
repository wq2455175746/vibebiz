import { createRouter, createWebHistory } from 'vue-router'

import { useAuthStore } from '@/stores/auth'
import EditorView from '@/views/EditorView.vue'
import HomeView from '@/views/HomeView.vue'
import LoginView from '@/views/LoginView.vue'

import { settingsRoute } from './settingsRoutes'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', name: 'login', component: LoginView, meta: { public: true } },
    { path: '/', name: 'home', component: HomeView },
    { path: '/projects', name: 'projects', component: HomeView },
    { path: '/resources', name: 'resources', component: HomeView },
    settingsRoute,
    { path: '/editor/:id', name: 'editor', component: EditorView },
  ],
})

router.beforeEach(async (to) => {
  if (to.meta.public) {
    return true
  }
  const auth = useAuthStore()
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
  const name = typeof to.name === 'string' ? to.name : ''
  document.title =
    name === 'editor'
      ? 'Vibebiz · 编辑器'
      : name === 'login'
        ? 'Vibebiz · 登录'
        : name.startsWith('settings')
          ? 'Vibebiz · 设置'
          : 'Vibebiz'
})

export default router
