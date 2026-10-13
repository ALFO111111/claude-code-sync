// 调后端的唯一入口：组件里统一 `import { api } from '@/api'`，不直接碰 wailsjs。
// wailsjs 目录由 `wails build` / `wails generate module` 根据 internal/gui/app.go 自动生成，
// 全部接口说明见那个文件顶部的注释。
import * as App from '../../wailsjs/go/gui/App'
import { EventsOn, BrowserOpenURL } from '../../wailsjs/runtime/runtime'

export const api = App
export { EventsOn, BrowserOpenURL } // BrowserOpenURL 用系统浏览器打开链接（WebView 里直接点会在窗口内跳走）
