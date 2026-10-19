// 日志页：把 sync.log 的每一行拆成时间和内容，并大致判断轻重（只用来上色和"只看问题"筛选）。
// 日志行格式：2026-10-19 10:00:00 [进程号] 内容。内容是写日志时的界面语言，所以中英文关键词都认。

const LINE = /^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}) \[\d+\] (.*)$/
const DONE = /^(同步完成|Sync finished)/
const DONE_WITH_ERRORS = /失败 [1-9]|\b[1-9]\d* failed/
const ERROR = /失败|错误|解不开|error|failed|cannot|can't/i
const WARN = /跳过|暂停|不见了|并发覆盖|正在被修改|skipped|paused|missing|overwritten|being modified/i

export function levelOf(msg) {
  if (DONE.test(msg)) return DONE_WITH_ERRORS.test(msg) ? 'error' : 'done'
  if (ERROR.test(msg)) return 'error'
  if (WARN.test(msg)) return 'warn'
  return 'info'
}

// parseLog 日志全文 → [{ ts, msg, level }]。认不出格式的行（比如多行错误信息的后几行）接在上一行后面。
export function parseLog(text) {
  const out = []
  for (const raw of (text || '').split(/\r?\n/)) {
    if (!raw) continue
    const m = LINE.exec(raw)
    if (m) {
      out.push({ ts: m[1], msg: m[2], level: levelOf(m[2]) })
    } else if (out.length) {
      out[out.length - 1].msg += '\n' + raw
    } else {
      out.push({ ts: '', msg: raw, level: levelOf(raw) })
    }
  }
  return out
}
