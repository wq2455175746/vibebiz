import { createRouter, createWebHistory } from 'vue-router'

import OpsLayout from '@/layouts/OpsLayout.vue'

import { childRoutes } from './routes'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      component: OpsLayout,
      children: childRoutes,
    },
  ],
})

router.afterEach((to) => {
  const title = typeof to.meta.title === 'string' ? to.meta.title : '概览'
  document.title = `Vibebiz Ops · ${title}`
})

export default router
