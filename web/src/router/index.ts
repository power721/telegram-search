import { createRouter, createWebHistory } from 'vue-router'
import AppLayout from '@/layouts/AppLayout.vue'
import { useAuthStore } from '@/stores/auth'
import { useSetupStore } from '@/stores/setup'
import HomeView from '@/views/HomeView.vue'
import LoginView from '@/views/LoginView.vue'
import { placeholderView } from '@/views/placeholders'
import AccountsView from '@/views/AccountsView.vue'
import SettingsView from '@/views/SettingsView.vue'
import SetupAdminView from '@/views/SetupAdminView.vue'
import SetupTelegramApiView from '@/views/SetupTelegramApiView.vue'
import SetupTelegramLoginView from '@/views/SetupTelegramLoginView.vue'
import ChannelsView from '@/views/ChannelsView.vue'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/setup/admin',
      name: 'setup-admin',
      component: SetupAdminView,
      meta: { public: true }
    },
    {
      path: '/setup/telegram-api',
      name: 'setup-telegram-api',
      component: SetupTelegramApiView
    },
    {
      path: '/setup/telegram-login',
      name: 'setup-telegram-login',
      component: SetupTelegramLoginView
    },
    { path: '/login', name: 'login', component: LoginView, meta: { public: true } },
    {
      path: '/',
      component: AppLayout,
      children: [
        { path: '', name: 'home', component: HomeView },
        { path: 'search', name: 'search', component: placeholderView('Search') },
        { path: 'channels', name: 'channels', component: ChannelsView },
        { path: 'resources', name: 'resources', component: placeholderView('Resources') },
        { path: 'accounts', name: 'accounts', component: AccountsView },
        { path: 'tasks', name: 'tasks', component: placeholderView('Tasks') },
        { path: 'settings', name: 'settings', component: SettingsView }
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
  if (!setup.status?.telegram_configured && to.name !== 'setup-telegram-api') {
    return { name: 'setup-telegram-api' }
  }
  if (
    setup.status?.telegram_configured &&
    !setup.status.complete &&
    to.name !== 'setup-telegram-login'
  ) {
    return { name: 'setup-telegram-login' }
  }
  return true
})
