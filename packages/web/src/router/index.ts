import { createRouter, createWebHistory } from 'vue-router'

import { useAuthStore } from '@/stores/auth'
import CloneEditorView from '@/views/CloneEditorView.vue'
import EditorView from '@/views/EditorView.vue'
import EnterpriseEditorView from '@/views/EnterpriseEditorView.vue'
import HomeView from '@/views/HomeView.vue'
import LoginView from '@/views/LoginView.vue'

import { settingsRoute } from './settingsRoutes'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', name: 'login', component: LoginView, meta: { public: true } },
    { path: '/', name: 'home', component: HomeView },
    { path: '/enterprise', name: 'enterprise', component: HomeView },
    { path: '/projects', name: 'projects', component: HomeView },
    { path: '/resources', name: 'resources', component: HomeView },
    { path: '/clones', name: 'clones', component: HomeView },
    { path: '/scheduled-tasks', name: 'scheduled-tasks', component: HomeView },
    { path: '/clone/editor/:id', name: 'clone-editor', component: CloneEditorView },
    { path: '/enterprise/editor/:id', name: 'enterprise-editor', component: EnterpriseEditorView },
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
      : name === 'enterprise-editor'
        ? 'Vibebiz · 企业工程开发'
        : name === 'clone-editor'
          ? 'Vibebiz · 克隆'
          : name === 'login'
          ? 'Vibebiz · 登录'
          : name.startsWith('settings')
            ? 'Vibebiz · 设置'
            : name === 'clones'
              ? 'Vibebiz · 克隆'
              : 'Vibebiz'
})

export default router
