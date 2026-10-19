import { it, expect } from 'vitest'
import { parseLog, levelOf } from '@/utils/log'

it('按内容判断轻重（中英文都认）', () => {
  expect(levelOf('同步完成：上传 1，下载 0，合并 0，分叉 0，失败 0，输入历史补进 0 条，用时 0.3s')).toBe('done')
  expect(levelOf('同步完成：上传 1，下载 0，合并 0，分叉 0，失败 2，输入历史补进 0 条，用时 0.3s')).toBe('error')
  expect(levelOf('Sync finished: 1 uploaded, 0 downloaded, 0 merged, 0 forked, 0 failed, 0 input-history entries added, took 0.3s')).toBe('done')
  expect(levelOf('Sync finished: 1 uploaded, 0 downloaded, 0 merged, 0 forked, 3 failed, 0 input-history entries added, took 0.3s')).toBe('error')
  expect(levelOf('失败 d--x/a.jsonl: timeout')).toBe('error')
  expect(levelOf('Failed d--x/a.jsonl: timeout')).toBe('error')
  expect(levelOf('d--x/a.jsonl 正在被修改，本轮先跳过，下次同步再处理')).toBe('warn')
  expect(levelOf('发现项目 D:/x（git:github.com/a/b）')).toBe('info')
})

it('拆出时间；认不出格式的行接到上一行', () => {
  const lines = parseLog('2026-10-19 10:00:00 [123] 发现项目 x\n第二行\n\n2026-10-19 10:00:01 [123] 失败 a: b\n')
  expect(lines).toEqual([
    { ts: '2026-10-19 10:00:00', msg: '发现项目 x\n第二行', level: 'info' },
    { ts: '2026-10-19 10:00:01', msg: '失败 a: b', level: 'error' },
  ])
})
