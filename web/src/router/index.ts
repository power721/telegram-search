import { createRouter, createWebHistory } from 'vue-router'
import AppLayout from '@/layouts/AppLayout.vue'
import { useAuthStore } from '@/stores/auth'
import { useSetupStore } from '@/stores/setup'
import { placeholderView } from '@/views/placeholders'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/setup/admin',
      name: 'setup-admin',
      component: placeholderView('Create Admin'),
      meta: { public: true }
    },
    { path: '/login', name: 'login', component: placeholderView('Login'), meta: { public: true } },
    {
      path: '/',
      component: AppLayout,
      children: [
        { path: '', name: 'home', component: placeholderView('Home') },
        { path: 'search', name: 'search', component: placeholderView('Search') },
        { path: 'channels', name: 'channels', component: placeholderView('Channels') },
        { path: 'resources', name: 'resources', component: placeholderView('Resources') },
        { path: 'accounts', name: 'accounts', component: placeholderView('Accounts') },
        { path: 'tasks', name: 'tasks', component: placeholderView('Tasks') },
        { path: 'settings', name: 'settings', component: placeholderView('Settings') }
      ]
    }
  ]
})

router.beforeEach(async (to) => {
  const setup = useSetupStore()
  if (!setup.loaded) {
    await setup.load()
  }
  if (!setup.status?.admin_configured && to.name !== 'setup-admin') {
    return { name: 'setup-admin' }
  }
  if (setup.status?.admin_configured && to.name === 'setup-admin') {
    return { name: 'login' }
  }
  if (to.meta.public) {
    return true
  }
  const auth = useAuthStore()
  if (!auth.loaded) {
    await auth.loadMe()
  }
  if (!auth.authenticated) {
    return { name: 'login' }
  }
  return true
})
