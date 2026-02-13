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
          path: 'hosts',
          name: 'Hosts',
          component: () => import('@/pages/Hosts/index.vue'),
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
          path: 'tasks/:task_id',
          name: 'TaskDetail',
          component: () => import('@/pages/Tasks/detail.vue'),
        },
        {
          path: 'monitor',
          name: 'Monitor',
          component: () => import('@/pages/Monitor/index.vue'),
        },
        {
          path: 'support',
          name: 'Support',
          component: () => import('@/pages/Support/index.vue'),
        },
        {
          path: 'customers',
          name: 'Customers',
          component: () => import('@/pages/Customers/index.vue'),
        },
        {
          path: 'operation-logs',
          name: 'OperationLogs',
          component: () => import('@/pages/OperationLogs/index.vue'),
        },
      ],
    },
  ],
})

export default router
