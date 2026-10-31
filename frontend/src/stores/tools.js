// 当前选中的工具（Claude Code / Codex / 以后的 pi、hermes……）：侧边栏顶部切换，
// 会话、同步内容、冲突、备份这几页只显示当前工具的东西。工具列表来自后端 ListTools，不在前端写死。
import { reactive, computed } from 'vue'
import { resources } from '@/stores/data'

const KEY = 'agent-sync.tool'

function saved() {
  try {
    return localStorage.getItem(KEY) || 'claude'
  } catch {
    return 'claude'
  }
}

export const toolState = reactive({ current: saved() })

export const tools = computed(() => resources.tools.data || [])

// currentTool 当前工具的信息；列表还没拉到（或记住的工具已经不支持了）时给一个够用的占位
export const currentTool = computed(() =>
  tools.value.find((x) => x.id === toolState.current) || tools.value[0] || { id: toolState.current, name: toolState.current, sessions: true })

export function setTool(id) {
  toolState.current = id
  try {
    localStorage.setItem(KEY, id)
  } catch {
    // 存不了就只在这次打开期间记住
  }
}

// toolOf 后端记录里的 tool 字段：旧记录没有这个字段，都是 Claude
export const toolOf = (it) => it?.tool || 'claude'

// 每个工具的图标：颜色 + 一两个字符（不用官方标志）。没登记的工具按名字自动配色、取首字母。
const LOOKS = {
  claude: { color: '#D97757', mark: 'C', short: 'Claude' },
  codex: { color: '#3B3B3F', mark: '>_', short: 'Codex' },
}
const PALETTE = ['#5B8DEF', '#3FA66A', '#B068D8', '#D9A33B', '#4AA3B5', '#D9607A']

export function toolLook(tool) {
  const id = typeof tool === 'string' ? tool : tool?.id || ''
  if (LOOKS[id]) return LOOKS[id]
  const name = (typeof tool === 'object' && tool?.name) || id
  let h = 0
  for (const c of id) h = (h * 31 + c.charCodeAt(0)) >>> 0
  return { color: PALETTE[h % PALETTE.length], mark: (name[0] || '?').toUpperCase(), short: name }
}
