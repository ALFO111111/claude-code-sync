import { createApp } from 'vue'
import App from './App.vue'
import { i18n, initLocale } from './i18n'
import './styles/main.css'

// 先按后端的语言设置好再挂载，免得先闪一下中文再变英文
initLocale().finally(() => createApp(App).use(i18n).mount('#app'))
