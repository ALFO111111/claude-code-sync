// 界面多语言（vue-i18n）。文字都在同目录的 zh-CN.js / en.js 里，两边的 key 必须一致
// （npm run check-i18n 会检查，CI 也跑）。
//
// 语言以后端为准：后端存在 config.json，托盘菜单、错误信息、日志也跟着它走。
// 启动时 initLocale() 读一次；用户切换时调 changeLanguage()。
import { ref } from 'vue'
import { createI18n } from 'vue-i18n'
import { api } from '@/api'
import zhCN from './zh-CN'
import en from './en'

export const i18n = createI18n({
  legacy: false,
  locale: 'zh-CN',
  fallbackLocale: 'zh-CN',
  messages: { 'zh-CN': zhCN, en },
})

// 用户的选择：'' = 跟随系统，'zh-CN'，'en'（向导里初始化时要带上它）
export const languageSetting = ref('')

function apply(info) {
  languageSetting.value = info.setting || ''
  i18n.global.locale.value = info.effective
  document.documentElement.lang = info.effective
}

// initLocale 启动时调：按后端的语言设置显示，避免先闪一下中文再变英文。
export async function initLocale() {
  try {
    apply(await api.GetLanguage())
  } catch {
    // 拿不到就用默认中文，不影响使用
  }
}

// changeLanguage 切换语言（'' = 跟随系统），后端同时刷新托盘菜单并存进配置。
export async function changeLanguage(lang) {
  apply(await api.SetLanguage(lang))
}
