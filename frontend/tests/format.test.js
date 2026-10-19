import { it, expect, vi } from 'vitest'

// format.js 通过 i18n 间接引用后端接口（wailsjs）；测试里用不到，换成空的
vi.mock('@/api', () => ({ api: {}, EventsOn: () => () => {}, BrowserOpenURL: () => {} }))

const { fmtSize, fromNow, highlight } = await import('@/utils/format')

it('fmtSize', () => {
  expect(fmtSize(0)).toBe('0 B')
  expect(fmtSize(undefined)).toBe('0 B')
  expect(fmtSize(1536)).toBe('1.5 KB')
  expect(fmtSize(5 * 1024 ** 2)).toBe('5.0 MB')
  expect(fmtSize(3 * 1024 ** 3)).toBe('3.00 GB')
})

it('fromNow', () => {
  const now = Date.UTC(2026, 9, 19, 12)
  expect(fromNow(0, now)).toBe('从未')
  expect(fromNow(now - 10_000, now)).toBe('刚刚')
  expect(fromNow(now - 5 * 60_000, now)).toBe('5 分钟前')
  expect(fromNow(now - 3 * 3600_000, now)).toBe('3 小时前')
  expect(fromNow(now - 2 * 86400_000, now)).toBe('2 天前')
})

it('highlight 不分大小写切段', () => {
  expect(highlight('Login 和 login', 'LOGIN')).toEqual([
    { t: 'Login', hit: true }, { t: ' 和 ', hit: false }, { t: 'login', hit: true },
  ])
  expect(highlight('abc', '')).toEqual([{ t: 'abc', hit: false }])
})
