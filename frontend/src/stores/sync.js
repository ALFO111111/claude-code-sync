// 全局同步状态：整个前端只在这里监听一次后端的 "sync" 事件，各页面直接读 syncState。
import { reactive } from 'vue'
import { EventsOn } from '@/api'

export const syncState = reactive({
  running: false, // 是否正在同步
  last: null, // 上一次结束时的 { stats, error }
})

let listening = false

// listenSync 挂一次后端 "sync" 事件的监听（重复调用无效）。
// 要在读 GetStatus 快照之前调用：否则一轮同步恰好在间隙里结束，
// "结束"事件就丢了，界面会一直卡在"同步中"。
export function listenSync() {
  if (listening) return
  listening = true
  EventsOn('sync', (e) => {
    syncState.running = e.running
    if (!e.running) syncState.last = e
  })
}
