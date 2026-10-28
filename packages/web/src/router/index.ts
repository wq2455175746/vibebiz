import { createRouter, createWebHistory } from 'vue-router'

import EditorView from '@/views/EditorView.vue'
import HomeView from '@/views/HomeView.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'home', component: HomeView },
    { path: '/editor/:id', name: 'editor', component: EditorView },
  ],
})

router.afterEach((to) => {
  document.title = to.name === 'editor' ? 'Vibebiz · 编辑器' : 'Vibebiz'
})

export default router
