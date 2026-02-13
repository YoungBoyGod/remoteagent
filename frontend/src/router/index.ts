import { createRouter, createWebHistory } from 'vue-router'
import AppLayout from '@/layouts/AppLayout.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    // 主站 — AppLayout
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
          path: 'distribution',
          name: 'Distribution',
          component: () => import('@/pages/Distribution/index.vue'),
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
        // 文档中心
        {
          path: 'documents',
          name: 'Documents',
          component: () => import('@/pages/Documents/index.vue'),
        },
        {
          path: 'documents/:docId',
          name: 'DocumentDetail',
          component: () => import('@/pages/Documents/index.vue'),
        },
        {
          path: 'documents/admin',
          name: 'DocumentAdmin',
          component: () => import('@/pages/Documents/admin.vue'),
        },
        {
          path: 'documents/editor/:slug?',
          name: 'DocumentEditor',
          component: () => import('@/pages/Documents/editor.vue'),
        },
        {
          path: 'documents/categories',
          name: 'DocumentCategories',
          component: () => import('@/pages/Documents/categories.vue'),
        },
      ],
    },
  ],
})

export default router
