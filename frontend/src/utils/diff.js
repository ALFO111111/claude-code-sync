// 按行对比两段文字（备份页"恢复后会变成什么样"用）。经典的最长公共子序列（LCS）算法。
//
// lineDiff(from, to) → [{ type: 'same' | 'add' | 'del', text }]
//   add：to 里有、from 里没有（恢复后会出现的行）
//   del：from 里有、to 里没有（恢复后会消失的行）
// 两边都很大时（行数相乘超过 MAX_CELLS）算不动，返回 null，界面就只显示内容不做对比。

const MAX_CELLS = 4_000_000

export function splitLines(s) {
  if (!s) return []
  const lines = s.replace(/\r\n/g, '\n').split('\n')
  if (lines[lines.length - 1] === '') lines.pop() // 结尾的换行不算一行
  return lines
}

export function lineDiff(from, to) {
  const a = splitLines(from)
  const b = splitLines(to)
  // 先剥掉首尾相同的部分：大多数修改只动了中间几行，能把计算量降到很小
  let start = 0
  while (start < a.length && start < b.length && a[start] === b[start]) start++
  let endA = a.length
  let endB = b.length
  while (endA > start && endB > start && a[endA - 1] === b[endB - 1]) {
    endA--
    endB--
  }
  const n = endA - start
  const m = endB - start
  if (n * m > MAX_CELLS) return null

  // lcs[i][j] = a[start+i..endA) 和 b[start+j..endB) 的最长公共子序列长度（一维数组存二维表）
  const w = m + 1
  const lcs = new Uint32Array((n + 1) * w)
  for (let i = n - 1; i >= 0; i--) {
    for (let j = m - 1; j >= 0; j--) {
      lcs[i * w + j] = a[start + i] === b[start + j]
        ? lcs[(i + 1) * w + j + 1] + 1
        : Math.max(lcs[(i + 1) * w + j], lcs[i * w + j + 1])
    }
  }

  const out = []
  for (let k = 0; k < start; k++) out.push({ type: 'same', text: a[k] })
  let i = 0
  let j = 0
  while (i < n && j < m) {
    if (a[start + i] === b[start + j]) {
      out.push({ type: 'same', text: a[start + i] })
      i++
      j++
    } else if (lcs[(i + 1) * w + j] >= lcs[i * w + j + 1]) {
      out.push({ type: 'del', text: a[start + i++] })
    } else {
      out.push({ type: 'add', text: b[start + j++] })
    }
  }
  while (i < n) out.push({ type: 'del', text: a[start + i++] })
  while (j < m) out.push({ type: 'add', text: b[start + j++] })
  for (let k = endA; k < a.length; k++) out.push({ type: 'same', text: a[k] })
  return out
}

// collapse 把大段没变的行折叠起来，只在改动前后各留 context 行。
// 返回的数组里多一种 { type: 'skip', count }：表示这里省略了 count 行没变的。
export function collapse(diff, context = 3) {
  const out = []
  let run = []
  const flush = (atStart, atEnd) => {
    const keepHead = atStart ? 0 : context
    const keepTail = atEnd ? 0 : context
    if (run.length > keepHead + keepTail + 1) {
      out.push(...run.slice(0, keepHead))
      out.push({ type: 'skip', count: run.length - keepHead - keepTail })
      out.push(...run.slice(run.length - keepTail))
    } else {
      out.push(...run)
    }
    run = []
  }
  let seenChange = false
  for (const d of diff) {
    if (d.type === 'same') {
      run.push(d)
    } else {
      flush(!seenChange, false)
      seenChange = true
      out.push(d)
    }
  }
  flush(!seenChange, true)
  return out
}
