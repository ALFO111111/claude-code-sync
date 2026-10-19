import { createApp } from 'vue'
import App from './App.vue'
import { i18n, initLocale } from './i18n'
import { initTheme } from './utils/theme'
import './styles/main.css'

// 先按后端的语言、主题设置好再挂载，免得先闪一下中文再变英文、先白一下再变深色
Promise.allSettled([initLocale(), initTheme()]).finally(() => createApp(App).use(i18n).mount('#app'))
