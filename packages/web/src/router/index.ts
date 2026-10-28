import { createRouter, createWebHistory } from 'vue-router'

import { useAuthStore } from '@/stores/auth'
import EditorView from '@/views/EditorView.vue'
import HomeView from '@/views/HomeView.vue'
import LoginView from '@/views/LoginView.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', name: 'login', component: LoginView, meta: { public: true } },
    { path: '/', name: 'home', component: HomeView },
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
  document.title =
    to.name === 'editor' ? 'Vibebiz · 编辑器' : to.name === 'login' ? 'Vibebiz · 登录' : 'Vibebiz'
})

export default router
