// 检查更新的状态：侧边栏红点和设置页"关于"共用。后端启动后自动检查，结果通过 "update" 事件推过来。
import { reactive, computed } from 'vue'
import { api, EventsOn } from '@/api'

export const upd = reactive({
  current: '', // 当前版本
  auto: true, // 自动检查更新
  skipped: '', // 点过"以后再说"的版本
  info: null, // 最近一次检查结果 { latest, available, dev, notes, url, published, asset }
  checking: false,
  installing: false,
  progress: { done: 0, total: 0 },
})

// 有新版本（设置页按钮变成"更新到 vX"）
export const hasUpdate = computed(() => !!upd.info?.available)
// 侧边栏红点：有新版本、而且不是点过"以后再说"的那个
export const showBadge = computed(() => hasUpdate.value && upd.info.latest !== upd.skipped)

let listening = false

export async function loadUpdate() {
  if (!listening) {
    listening = true
    EventsOn('update', (info) => { upd.info = info })
    EventsOn('update-progress', (p) => { upd.progress = p })
  }
  const s = await api.GetUpdateState()
  upd.current = s.current
  upd.auto = s.auto
  upd.skipped = s.skipped
  if (s.info) upd.info = s.info
}

export async function checkUpdate() {
  upd.checking = true
  try {
    upd.info = await api.CheckUpdate()
    return upd.info
  } finally {
    upd.checking = false
  }
}

// 成功时程序会自己退出并重新打开新版本；失败抛错，原来的程序原样留着
export async function installUpdate() {
  upd.installing = true
  upd.progress = { done: 0, total: 0 }
  try {
    await api.InstallUpdate()
  } finally {
    upd.installing = false
  }
}

export async function skipUpdate() {
  await api.SkipUpdate(upd.info.latest)
  upd.skipped = upd.info.latest
}

export async function setAutoUpdate(on) {
  await api.SetAutoUpdateCheck(on)
  upd.auto = on
}
