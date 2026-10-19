// 演示模式的假 Wails 运行时（见 demo/App.js）：事件不会来，打开链接用浏览器新标签页。
export const EventsOn = () => () => {}
export const BrowserOpenURL = (url) => window.open(url, '_blank')
