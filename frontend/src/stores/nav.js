// 页面导航：侧边栏点哪页就显示哪页；页面之间也能互相跳（比如备份页"查看会话"跳到会话页并打开那个会话）。
import { reactive } from 'vue'

// 侧边栏分三段：首页（所有工具一起看）；当前工具的页面（跟着顶部的工具切换走）；通用设置。
export const TOOL_PAGES = ['sessions', 'content', 'conflicts', 'backups']
export const GENERAL_PAGES = ['bindings', 'settings', 'logs']
export const PAGES = ['home', ...TOOL_PAGES, ...GENERAL_PAGES]

export const nav = reactive({
  page: 'home',
  params: null, // 跳转时带给目标页的参数，目标页读完调 takeParams() 清掉
})

export function go(page, params = null) {
  nav.params = params
  nav.page = page
}

// takeParams 取出并清掉跳转参数（只用一次，免得来回切页又被套用）。
export function takeParams() {
  const p = nav.params
  nav.params = null
  return p
}
