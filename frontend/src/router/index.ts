import { createRouter, createWebHistory } from 'vue-router'
import AppLayout from '@/layouts/AppLayout.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      component: AppLayout,
      children: [
        {
          path: '',
          name: 'Dashboard',
          component: () => import('@/pages/Dashboard/index.vue'),
        },
        {
          path: 'agents',
          name: 'Agents',
          component: () => import('@/pages/Agents/index.vue'),
        },
        {
          path: 'dispatch',
          name: 'Dispatch',
          component: () => import('@/pages/Dispatch/index.vue'),
        },
        {
          path: 'tasks',
          name: 'Tasks',
          component: () => import('@/pages/Tasks/index.vue'),
        },
        {
          path: 'monitor',
          name: 'Monitor',
          component: () => import('@/pages/Monitor/index.vue'),
        },
      ],
    },
  ],
})

export default router
