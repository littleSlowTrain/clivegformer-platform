import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import ShellView from '../views/ShellView.vue'

const router = createRouter({ history: createWebHistory(), routes: [
  { path: '/login', component: () => import('../views/LoginView.vue'), meta: { public: true } },
  { path: '/', component: ShellView, children: [
    { path: '', redirect: '/dashboard' }, { path: 'dashboard', component: () => import('../views/DashboardView.vue') },
    { path: 'team-files', component: () => import('../views/FilesView.vue'), props: { scope: 'team' } },
    { path: 'files', component: () => import('../views/FilesView.vue'), props: { scope: 'mine' } }, { path: 'uploads', component: () => import('../views/UploadsView.vue') },
    { path: 'analysis', component: () => import('../views/AnalysisView.vue') }, { path: 'profile', component: () => import('../views/ProfileView.vue') },
  ]},
] })
router.beforeEach((to) => { const auth = useAuthStore(); if (!to.meta.public && !auth.authenticated) return '/login'; if (to.path === '/login' && auth.authenticated) return '/dashboard' })
export default router
