import { i18n } from '@/i18n'

const t = (...args) => i18n.global.t(...args)

export function fmtSize(n) {
  n = n || 0
  if (n < 1024) return n + ' B'
  if (n < 1024 ** 2) return (n / 1024).toFixed(1) + ' KB'
  if (n < 1024 ** 3) return (n / 1024 ** 2).toFixed(1) + ' MB'
  return (n / 1024 ** 3).toFixed(2) + ' GB'
}

// kindLabel 项目种类（后端 project.Kind：home / git / local）的显示名。
export const kindLabel = (k) => (['home', 'git', 'local'].includes(k) ? t(`kind.${k}`) : k)

// 全局内容属于哪个工具；旧数据没有 tool，按 Claude Code 显示。
export const toolLabel = (tool) => t(`content.tool.${tool || 'claude'}`)
const contentPrefix = (tool) => (tool === 'codex' ? 'content.codex' : 'content')
export const contentLabel = (tool, category) => t(`${contentPrefix(tool)}.cat.${category}`)
export const contentDesc = (tool, category) => t(`${contentPrefix(tool)}.desc.${category}`)

// fromNow 毫秒时间戳 → "刚刚 / 3 分钟前 / 2 小时前 / 5 天前"。0 表示从没发生过。
export function fromNow(ms, now = Date.now()) {
  if (!ms) return t('time.never')
  const s = Math.max(0, Math.round((now - ms) / 1000))
  if (s < 60) return t('time.justNow')
  if (s < 3600) return t('time.minutesAgo', Math.floor(s / 60))
  if (s < 86400) return t('time.hoursAgo', Math.floor(s / 3600))
  return t('time.daysAgo', Math.floor(s / 86400))
}

// highlight 把文本按关键词（不区分大小写）切成段，命中的段 hit=true，模板里用 <mark> 包起来。
// 不用 v-html：会话内容里什么字符都有，拼 HTML 容易出事。
export function highlight(text, q) {
  if (!q) return [{ t: text, hit: false }]
  const lower = text.toLowerCase()
  const lq = q.toLowerCase()
  const out = []
  let i = 0
  for (let j = lower.indexOf(lq); j !== -1; j = lower.indexOf(lq, i)) {
    if (j > i) out.push({ t: text.slice(i, j), hit: false })
    out.push({ t: text.slice(j, j + q.length), hit: true })
    i = j + q.length
  }
  if (i < text.length) out.push({ t: text.slice(i), hit: false })
  return out
}

// releaseNotes 更新说明（CHANGELOG 那一节）只留当前界面语言：每条是 "- 中文" 下面跟一行缩进的英文。
// 小标题"### 修复 / Fixed"按语言取一半；认不出这种格式的原样返回（去掉 Markdown 记号）。
export function releaseNotes(body, lang) {
  const en = lang === 'en'
  const out = []
  let bilingual = false
  for (const line of (body || '').replace(/\r\n/g, '\n').split('\n')) {
    if (!line.trim()) continue
    const head = line.match(/^#+\s*(.*)$/)
    if (head) {
      const [zh, eng] = head[1].split(' / ')
      out.push(((en ? eng : zh) || head[1]).trim())
    } else if (/^\s+\S/.test(line) && out.length) {
      bilingual = true
      if (en) out.push('• ' + line.trim())
    } else {
      const item = line.replace(/^[-*]\s+/, '')
      if (!en || !line.match(/^[-*]\s/)) out.push((item !== line ? '• ' : '') + item)
    }
  }
  // 不是双语格式（没有缩进的英文行）：英文界面也把每一条都显示出来
  if (en && !bilingual) return releaseNotes(body, 'zh-CN')
  return out.join('\n').replace(/`/g, '')
}
