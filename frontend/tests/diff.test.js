import { describe, it, expect } from 'vitest'
import { lineDiff, collapse, splitLines } from '@/utils/diff'

const types = (d) => d.map((x) => (x.type === 'skip' ? `skip${x.count}` : x.type[0] + x.text)).join(' ')

describe('lineDiff', () => {
  it('相同的内容全是 same', () => {
    expect(types(lineDiff('a\nb\n', 'a\nb'))).toBe('sa sb')
  })

  it('认出中间改了一行', () => {
    expect(types(lineDiff('a\nb\nc', 'a\nB\nc'))).toBe('sa db aB sc')
  })

  it('开头新增、结尾删除', () => {
    expect(types(lineDiff('a\nb\nc', 'x\na\nb'))).toBe('ax sa sb dc')
  })

  it('一边是空的', () => {
    expect(types(lineDiff('', 'a\nb'))).toBe('aa ab')
    expect(types(lineDiff('a', ''))).toBe('da')
  })

  it('CRLF 和 LF 当成一样', () => {
    expect(types(lineDiff('a\r\nb\r\n', 'a\nb\n'))).toBe('sa sb')
  })

  it('两边都太大时放弃（返回 null）', () => {
    const big = (p) => Array.from({ length: 3000 }, (_, i) => p + i).join('\n')
    expect(lineDiff(big('x'), big('y'))).toBeNull()
  })

  it('首尾相同的大文件只算中间，不会放弃', () => {
    const lines = Array.from({ length: 5000 }, (_, i) => 'line' + i)
    const changed = [...lines]
    changed[2500] = 'CHANGED'
    const d = lineDiff(lines.join('\n'), changed.join('\n'))
    expect(d.filter((x) => x.type !== 'same').map((x) => x.type + ':' + x.text)).toEqual(['del:line2500', 'add:CHANGED'])
  })
})

describe('collapse', () => {
  it('大段没变的折叠起来，改动前后各留 3 行', () => {
    const from = Array.from({ length: 20 }, (_, i) => 'l' + i).join('\n')
    const to = from.replace('l10', 'X')
    const c = collapse(lineDiff(from, to))
    expect(types(c)).toBe('skip7 sl7 sl8 sl9 dl10 aX sl11 sl12 sl13 skip6')
  })

  it('没有改动时整段折叠', () => {
    expect(types(collapse(lineDiff('a\nb\nc', 'a\nb\nc')))).toBe('skip3')
  })
})

it('splitLines 结尾的换行不算一行', () => {
  expect(splitLines('a\nb\n')).toEqual(['a', 'b'])
  expect(splitLines('')).toEqual([])
})
