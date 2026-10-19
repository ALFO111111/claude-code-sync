// 界面主题：'' 跟随系统 / 'light' / 'dark'。选择存在后端（config.json），窗口标题栏也跟着变。
// 实际效果靠 <html data-theme="dark|light">，颜色变量见 styles/main.css。
import { ref } from 'vue'
import { api } from '@/api'

export const themeSetting = ref('')

const media = window.matchMedia ? window.matchMedia('(prefers-color-scheme: dark)') : null

function apply() {
  const dark = themeSetting.value === 'dark' || (themeSetting.value === '' && !!media?.matches)
  document.documentElement.dataset.theme = dark ? 'dark' : 'light'
}

// 跟随系统时，系统切换深浅色界面马上跟着变
media?.addEventListener?.('change', apply)

// initTheme 启动时调：按后端保存的设置显示。拿不到就跟随系统。
export async function initTheme() {
  try {
    themeSetting.value = (await api.GetTheme()).setting || ''
  } catch {
    themeSetting.value = ''
  }
  apply()
}

// changeTheme 切换主题并保存。
export async function changeTheme(setting) {
  themeSetting.value = setting
  apply()
  try {
    await api.SetTheme(setting)
  } catch {
    // 保存失败也不影响这次显示
  }
}
