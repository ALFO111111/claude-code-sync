// 页面数据缓存："先显示上次的，后台悄悄刷新"（stale-while-revalidate）。
//
// 为什么：以前每进一次页面都从零请求一遍，等数据回来之前整块显示"加载中"或先闪一下空状态。
// 现在每份数据全局只有一份：
//   - 第一次用到才拉（或者启动时 prefetch 预先拉好），没数据时页面显示骨架
//   - 再进页面：立刻显示上次的数据；超过 maxAge 才在后台刷新，刷新时只在卡片角上转个小圈
//   - 同步结束后：已经拉过的数据自动后台刷新（同步可能改了它们）
//   - initial：第一次拉取前先用一个不联网的快速版本垫底（项目页用本机缓存的设备信息秒开）
//
// 用法：const r = resources.projects; r.load()（按需）/ r.refresh()（强制）；模板里读 r.data / r.loading / r.error
import { reactive, watch } from 'vue'
import { api } from '@/api'
import { syncState } from '@/stores/sync'

export function resource(fetcher, { maxAge = 0, initial = null, lazy = false } = {}) {
  const r = reactive({
    data: null, // 还没拉到过为 null
    error: '', // 最近一次刷新的错误（有旧数据时照样显示旧数据）
    loading: false, // 正在刷新（有旧数据时页面只显示一个小圈）
    at: 0, // 上次成功的时间
  })
  let pending = null

  // refresh 强制刷新；同时只发一个请求，重复调用拿到同一个 Promise
  r.refresh = () => {
    if (pending) return pending
    r.loading = true
    pending = (async () => {
      if (r.data === null && initial) {
        try {
          r.data = await initial()
        } catch {
          // 快速版拿不到没关系，等正式的
        }
      }
      try {
        r.data = await fetcher()
        r.error = ''
        r.at = Date.now()
      } catch (e) {
        r.error = String(e)
      } finally {
        r.loading = false
        pending = null
      }
    })()
    return pending
  }

  // load 按需刷新：没拉过、或者超过 maxAge 才真的发请求
  r.load = () => (!r.at || Date.now() - r.at > maxAge ? r.refresh() : Promise.resolve())

  // invalidate 标记过期：下次 load 会刷新；已经拉过的现在就后台刷新（lazy 的只标记，等下次进页面）
  r.invalidate = () => {
    r.at = 0
    if (r.data !== null && !lazy) r.refresh()
  }
  return r
}

export const resources = {
  status: resource(() => api.GetStatus()), // 本地读，很快；每次进首页都刷新
  // 要列一遍云端：5 分钟内不重复拉；同步结束只标记过期（文件监听触发的同步很频繁，不能每次都列）
  usage: resource(() => api.GetUsage(), { maxAge: 5 * 60_000, lazy: true }),
  projects: resource(() => api.ListProjects(), { maxAge: 30_000, initial: () => api.ListProjectsCached() }),
  bound: resource(() => api.ListBound()),
  backups: resource(() => api.ListBackups()),
}

let started = false

// startData 启动时调一次：后台预取首页、项目页要用的数据；之后每次同步结束都让数据过期。
// status 是启动时已经拿到的首页状态，直接放进缓存，不再请求一遍。
export function startData(status) {
  if (started) return
  started = true
  if (status) {
    resources.status.data = status
    resources.status.at = Date.now()
  } else {
    resources.status.load()
  }
  resources.projects.load()
  resources.usage.load()
  watch(() => syncState.running, (running) => {
    if (running) return
    for (const r of Object.values(resources)) r.invalidate()
  })
}
