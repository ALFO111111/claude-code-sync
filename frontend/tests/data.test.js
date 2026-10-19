import { it, expect, vi } from 'vitest'

vi.mock('@/api', () => ({ api: {}, EventsOn: () => () => {}, BrowserOpenURL: () => {} }))
const { resource } = await import('@/stores/data')

it('同一时间只发一个请求；maxAge 内 load 不重复拉', async () => {
  let n = 0
  const r = resource(async () => ++n, { maxAge: 60_000 })
  await Promise.all([r.refresh(), r.refresh(), r.load()])
  expect(n).toBe(1)
  expect(r.data).toBe(1)
  await r.load()
  expect(n).toBe(1) // 没过期，不拉
  await r.refresh()
  expect(n).toBe(2) // 强制刷新
})

it('刷新失败时保留旧数据，只记错误', async () => {
  let fail = false
  const r = resource(async () => {
    if (fail) throw new Error('断网')
    return 'ok'
  })
  await r.refresh()
  fail = true
  await r.refresh()
  expect(r.data).toBe('ok')
  expect(r.error).toContain('断网')
  expect(r.loading).toBe(false)
})

it('第一次拉取前先用 initial 垫底，之后用正式数据', async () => {
  let release
  const slow = new Promise((resolve) => { release = resolve })
  const r = resource(() => slow, { initial: async () => 'cached' })
  const p = r.refresh()
  await vi.waitFor(() => expect(r.data).toBe('cached'))
  expect(r.loading).toBe(true)
  release('fresh')
  await p
  expect(r.data).toBe('fresh')
})

it('invalidate：普通的马上后台刷新，lazy 的只标记过期', async () => {
  let a = 0
  let b = 0
  const eager = resource(async () => ++a, { maxAge: 60_000 })
  const lazy = resource(async () => ++b, { maxAge: 60_000, lazy: true })
  await eager.load()
  await lazy.load()
  eager.invalidate()
  lazy.invalidate()
  await vi.waitFor(() => expect(a).toBe(2))
  expect(b).toBe(1)
  await lazy.load() // 下次进页面才刷新
  expect(b).toBe(2)
})
