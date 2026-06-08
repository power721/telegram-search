import '@unocss/reset/tailwind.css'
import 'uno.css'
import './styles/base.css'

import naive from 'naive-ui'
import { createPinia } from 'pinia'
import { createApp } from 'vue'
import App from './App.vue'

createApp(App).use(createPinia()).use(naive).mount('#app')
