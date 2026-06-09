import { createRouter, createWebHistory } from 'vue-router'
import AppLayout from '@/layouts/AppLayout.vue'
import { useAuthStore } from '@/stores/auth'
import { useSetupStore } from '@/stores/setup'
import HomeView from '@/views/HomeView.vue'
import LoginView from '@/views/LoginView.vue'
import AccountsView from '@/views/AccountsView.vue'
import SettingsView from '@/views/SettingsView.vue'
import SearchView from '@/views/SearchView.vue'
import SetupAdminView from '@/views/SetupAdminView.vue'
import SetupAPIKeyView from '@/views/SetupAPIKeyView.vue'
import SetupTelegramApiView from '@/views/SetupTelegramApiView.vue'
import SetupTelegramLoginView from '@/views/SetupTelegramLoginView.vue'
import SetupListenRulesView from '@/views/SetupListenRulesView.vue'
import SetupChannelSelectionView from '@/views/SetupChannelSelectionView.vue'
import ChannelsView from '@/views/ChannelsView.vue'
import ResourcesView from '@/views/ResourcesView.vue'
import TasksView from '@/views/TasksView.vue'
import LogsView from '@/views/LogsView.vue'
import ApiHelpView from '@/views/ApiHelpView.vue'

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
      path: '/setup/api-key',
      name: 'setup-api-key',
      component: SetupAPIKeyView
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
    {
      path: '/setup/listen-rules',
      name: 'setup-listen-rules',
      component: SetupListenRulesView
    },
    {
      path: '/setup/channels',
      name: 'setup-channels',
      component: SetupChannelSelectionView
    },
    { path: '/login', name: 'login', component: LoginView, meta: { public: true } },
    {
      path: '/',
      component: AppLayout,
      children: [
        { path: '', name: 'home', component: HomeView },
        { path: 'search', name: 'search', component: SearchView },
        { path: 'channels', name: 'channels', component: ChannelsView },
        { path: 'resources', name: 'resources', component: ResourcesView },
        { path: 'accounts', name: 'accounts', component: AccountsView },
        { path: 'tasks', name: 'tasks', component: TasksView },
        { path: 'logs', name: 'logs', component: LogsView },
        { path: 'settings', name: 'settings', component: SettingsView },
        { path: 'api-help', name: 'api-help', component: ApiHelpView }
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
  if (!setup.status?.complete) {
    const target = setupRouteName(setup.status?.current_step)
    if (target && to.name !== target) {
      return { name: target }
    }
  }
  return true
})

function setupRouteName(step?: string) {
  switch (step) {
    case 'api_key':
      return 'setup-api-key'
    case 'telegram_api':
      return 'setup-telegram-api'
    case 'telegram_login':
      return 'setup-telegram-login'
    case 'listen_rules':
      return 'setup-listen-rules'
    case 'channel_selection':
      return 'setup-channels'
    default:
      return ''
  }
}
