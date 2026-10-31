// 演示模式的假后端（npm run demo）：不连 Go 后端、不碰任何真实数据，界面用编出来的数据跑起来。
// 用途：README 截图（不会露出个人用户名、路径、bucket）、不启动 Wails 也能改界面。
// 和 wailsjs/go/gui/App.js 导出同样的函数；vite.config.js 在 demo 模式下把那个模块换成这个。
//
// 地址参数：?page=backups 打开哪一页；&theme=dark 深色；&lang=en 英文；&key=1 模拟口令被别的设备改了；
// &cc=1 模拟 CC Switch 开着自动同步（规则、技能、MCP 待选择负责人）；&click=… 见文件末尾
import { go } from '@/stores/nav'

const q = new URLSearchParams(location.search)
const NOW = Date.now()
const min = 60_000
const hour = 60 * min
const day = 24 * hour
const fmt = (ms) => {
  const d = new Date(ms)
  const p = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`
}
const en = q.get('lang') === 'en'
const L = (zh, e) => (en ? e : zh)

if (q.get('page')) setTimeout(() => go(q.get('page')), 0)

const PROJECTS = [
  { pid: 'git:github.com/acme/web-shop', kind: 'git', root: 'D:\\code\\web-shop', display: 'web-shop' },
  { pid: 'git:github.com/acme/api-server', kind: 'git', root: 'D:\\code\\api-server', display: 'api-server' },
  { pid: 'local:3f9a:notes', kind: 'local', root: 'D:\\notes', display: 'notes' },
  { pid: 'home', kind: 'home', root: 'C:\\Users\\alex', display: L('用户目录（未打开项目）', 'Home folder (no project open)') },
]

const SESSIONS = [
  { name: '7c1e2a40-1b2c-4d3e-8f90-1a2b3c4d5e6f.jsonl', title: L('购物车结算页加优惠券', 'Add coupons to the checkout page'), lines: 412, size: 1_830_000, ago: 25 * min },
  { name: '2b8d9f10-3c4d-4e5f-9a0b-2c3d4e5f6a7b.jsonl', title: L('修复登录后跳转丢参数', 'Fix lost query params after login'), lines: 188, size: 640_000, ago: 3 * hour, forkOf: 'x' },
  { name: '9e0f1a2b-4d5e-4f60-8b1c-3d4e5f6a7b8c.jsonl', title: L('商品列表虚拟滚动', 'Virtual scrolling for product list'), lines: 932, size: 4_120_000, ago: day + 2 * hour },
  { name: '5a6b7c8d-5e6f-4a71-9c2d-4e5f6a7b8c9d.jsonl', title: L('升级 Vite 和依赖', 'Upgrade Vite and dependencies'), lines: 96, size: 210_000, ago: 3 * day, pending: true },
]

const MESSAGES = [
  { role: 'user', time: '10-19 14:02:11', text: L('结算页要支持优惠券：输入券码、校验、显示减了多少钱。', 'The checkout page needs coupon support: enter a code, validate it, show the discount.') },
  { role: 'assistant', time: '10-19 14:02:19', text: L('好的。我先看一下现在的结算页组件和价格计算逻辑。', 'Sure. Let me look at the current checkout component and price calculation first.'), tools: [{ name: 'Read', brief: 'src/pages/Checkout.vue' }, { name: 'Grep', brief: 'calcTotal' }] },
  { role: 'assistant', time: '10-19 14:03:40', text: L('价格在 calcTotal 里算。我会加一个 applyCoupon，并在后端加 /api/coupons/verify 校验接口。', 'Prices are computed in calcTotal. I will add applyCoupon and a /api/coupons/verify endpoint on the server.'), tools: [{ name: 'Edit', brief: 'src/utils/price.js' }] },
  { role: 'user', time: '10-19 14:10:02', text: L('券过期的时候提示要友好一点。', 'Make the message friendlier when a coupon has expired.') },
  { role: 'assistant', time: '10-19 14:10:15', text: L('已改成“这张券已过期（有效期到 10 月 15 日）”，并加了测试。', 'Changed to “This coupon expired on Oct 15” and added tests.'), tools: [{ name: 'Bash', brief: 'npm test -- price' }] },
]

const BACKUPS = [
  { id: 'b1', kind: 'remote', time: NOW - 40 * min, project: 'web-shop', rel: 'memory/coding-style.md', size: 1240, pid: PROJECTS[0].pid, bound: true },
  { id: 'b2', kind: 'fork', time: NOW - 3 * hour, project: 'web-shop', rel: '2b8d9f10-3c4d-4e5f-9a0b-2c3d4e5f6a7b.jsonl', size: 0, pid: PROJECTS[0].pid, bound: true, fork: SESSIONS[1].name },
  { id: 'b3', kind: 'local', time: NOW - day - hour, project: 'api-server', rel: 'memory/MEMORY.md', size: 860, pid: PROJECTS[1].pid, bound: true },
  { id: 'b4', kind: 'restore', time: NOW - 2 * day, project: 'notes', rel: 'memory/todo.md', size: 410, pid: PROJECTS[2].pid, bound: true },
].map((b) => ({ ...b, timeText: fmt(b.time) }))

const BACKUP_TEXT = L(
  '# 代码风格\n\n- 组件用 <script setup>\n- 命名用驼峰\n- 提交前跑 npm test\n- 价格计算统一放在 utils/price.js\n- 金额用分为单位存整数\n',
  '# Coding style\n\n- Use <script setup> in components\n- camelCase names\n- Run npm test before committing\n- Keep price math in utils/price.js\n- Store money as integer cents\n')
const CURRENT_TEXT = L(
  '# 代码风格\n\n- 组件用 <script setup>\n- 命名用驼峰\n- 提交前跑 npm test 和 npm run lint\n- 金额用分为单位存整数\n',
  '# Coding style\n\n- Use <script setup> in components\n- camelCase names\n- Run npm test and npm run lint before committing\n- Store money as integer cents\n')

const LOG = [
  [3 * hour, '发现项目 D:\\code\\api-server（git:github.com/acme/api-server）', 'Found project D:\\code\\api-server (git:github.com/acme/api-server)'],
  [3 * hour, '会话 2b8d9f10 在两台设备上各自继续了，另一份已另存为新会话 8f2e…', 'Session 2b8d9f10 was continued on both devices; the other copy was saved as new session 8f2e…'],
  [3 * hour, '同步完成：上传 3，下载 2，合并 0，分叉 1，失败 0，输入历史补进 4 条，用时 2.1s', 'Sync finished: 3 uploaded, 2 downloaded, 0 merged, 1 forked, 0 failed, 4 input-history entries added, took 2.1s'],
  [2 * hour, 'web-shop/7c1e2a40.jsonl 正在被修改，本轮先跳过，下次同步再处理', 'web-shop/7c1e2a40.jsonl is being modified; skipped this round, will retry next sync'],
  [2 * hour, '失败 api-server/tool-results/a1.txt: dial tcp: i/o timeout', 'Failed api-server/tool-results/a1.txt: dial tcp: i/o timeout'],
  [2 * hour, '同步完成：上传 1，下载 0，合并 0，分叉 0，失败 1，输入历史补进 0 条，用时 10.4s', 'Sync finished: 1 uploaded, 0 downloaded, 0 merged, 0 forked, 1 failed, 0 input-history entries added, took 10.4s'],
  [40 * min, '同步完成：上传 2，下载 3，合并 1，分叉 0，失败 0，输入历史补进 6 条，用时 1.4s', 'Sync finished: 2 uploaded, 3 downloaded, 1 merged, 0 forked, 0 failed, 6 input-history entries added, took 1.4s'],
].map(([ago, zh, e]) => `${fmt(NOW - ago)}:00 [4120] ${L(zh, e)}`).join('\n') + '\n'

const CC = q.get('cc') === '1'
const CONFLICTS = [
  { id: 'c1', kind: 'file', category: 'rules', rel: 'CLAUDE.md', localPath: 'C:\\Users\\alex\\.claude\\CLAUDE.md', time: NOW - 50 * min, timeText: fmt(NOW - 50 * min) },
]
const CONFLICT_BASE = L('# 全局规则\n\n- 用中文回答\n- 改完代码跑测试\n', '# Global rules\n\n- Answer concisely\n- Run tests after changing code\n')
const CONFLICT_LOCAL = L('# 全局规则\n\n- 用中文回答，通俗易懂\n- 改完代码跑测试\n', '# Global rules\n\n- Answer concisely, in plain words\n- Run tests after changing code\n')
const CONFLICT_REMOTE = L('# 全局规则\n\n- 用中文回答，代码保持英文\n- 改完代码跑测试\n', '# Global rules\n\n- Answer concisely, keep code in English\n- Run tests after changing code\n')
const CONTENT_CATS = ['rules', 'plans', 'skills', 'agents', 'commands', 'tasks', 'settings', 'mcp']
const CONTENT_ITEMS = [
  ...CONTENT_CATS.map((category) => ({ tool: 'claude', category })),
  { tool: 'codex', category: 'rules' },
  { tool: 'codex', category: 'skills' },
  { tool: 'codex', category: 'legacy-skills' },
  { tool: 'codex', category: 'sessions' },
]
const CC_CATS = ['rules', 'skills', 'mcp']

const wait = (v, ms = 120) => new Promise((r) => setTimeout(() => r(v), ms))

export const GetStatus = () => wait({
  initialized: true, deviceName: 'office-pc', deviceId: '3f9a1c2b7d4e5f60', lastSync: fmt(NOW - 3 * min), lastSyncAt: NOW - 3 * min,
  projects: 4, syncing: false, autoSync: true, sandboxed: false, keyMismatch: q.get('key') === '1',
  conflicts: CONFLICTS.length, pendingContent: CC ? ['rules', 'skills', 'mcp'] : [],
  stats: { pushed: 2, pulled: 3, merged: 1, forked: 0, errors: 0, history: 6 },
  devices: [
    { id: 'a81c4e2f90b3d7e1', name: 'home-laptop', projects: 5, lastSeen: NOW - 20 * min },
    { id: 'c02d93e7a1f45b68', name: 'old-desktop', projects: 2, lastSeen: NOW - 6 * day },
  ],
  changes: [
    { kind: 'merge', pid: PROJECTS[1].pid, project: 'api-server', rel: 'memory/MEMORY.md' },
    { kind: 'pull', pid: PROJECTS[0].pid, project: 'web-shop', rel: SESSIONS[0].name },
    { kind: 'pull', pid: PROJECTS[0].pid, project: 'web-shop', rel: 'memory/coding-style.md' },
    { kind: 'pull', pid: PROJECTS[2].pid, project: 'notes', rel: 'memory/todo.md' },
    { kind: 'push', pid: PROJECTS[1].pid, project: 'api-server', rel: '1d2e3f4a-6b7c-4d8e-9f0a-5b6c7d8e9f0a.jsonl' },
    { kind: 'push', pid: PROJECTS[3].pid, project: L('用户目录（未打开项目）', 'Home folder (no project open)'), rel: '0a1b2c3d-7e8f-4a9b-8c0d-6e7f8a9b0c1d.jsonl' },
  ],
})
export const GetUsage = () => wait({
  total: 48_200_000, objects: 1342, sessions: 46_900_000, history: 1_100_000, other: 200_000,
  projects: [
    { pid: PROJECTS[0].pid, display: 'web-shop', bytes: 21_400_000, files: 410, bound: true },
    { pid: PROJECTS[1].pid, display: 'api-server', bytes: 14_800_000, files: 520, bound: true },
    { pid: PROJECTS[3].pid, display: L('用户目录', 'Home folder'), bytes: 6_100_000, files: 230, bound: true },
    { pid: PROJECTS[2].pid, display: 'notes', bytes: 3_300_000, files: 120, bound: true },
    { pid: 'git:github.com/acme/infra', display: 'infra', bytes: 1_300_000, files: 62, bound: false },
  ],
}, 300)
export const SyncNow = () => {}
export const RepairAutoSync = () => wait(null)

export const ListBound = () => wait(PROJECTS)
export const ListProjects = () => wait({
  bound: PROJECTS,
  claimable: [{ pid: 'local:a81c:design-docs', kind: 'local', device: 'home-laptop', deviceId: 'a81c4e2f90b3d7e1', root: 'E:\\work\\design-docs', display: 'design-docs' }],
  ignored: [],
  devices: [],
})
export const ListProjectsCached = () => ListProjects()
export const ClaimProject = () => wait(null)
export const IgnoreProject = () => wait(null)
export const UnignoreProject = () => wait(null)
export const PickDirectory = () => wait('')

export const ListSessions = () => wait(SESSIONS.map((s) => ({
  name: s.name, rel: s.name, size: s.size, lines: s.lines, mtime: fmt(NOW - s.ago), preview: s.title, title: s.title,
  synced: !s.pending, forkOf: s.forkOf || '', resume: true,
})))
export const ReadSession = () => wait(MESSAGES)
export const SearchSessions = (kw) => wait([
  { pid: PROJECTS[0].pid, project: 'web-shop', rel: SESSIONS[0].name, title: SESSIONS[0].title, mtime: fmt(NOW - 25 * min), count: 3, snippet: L(`结算页要支持优惠券：输入券码、校验、显示减了多少钱（${kw}）`, `The checkout page needs coupon support (${kw})`) },
  { pid: PROJECTS[1].pid, project: 'api-server', rel: '1d2e3f4a-6b7c-4d8e-9f0a-5b6c7d8e9f0a.jsonl', title: L('优惠券校验接口', 'Coupon verification endpoint'), mtime: fmt(NOW - 2 * day), count: 1, snippet: `/api/coupons/verify … ${kw}` },
], 300)
export const ResumeCommand = (pid, rel) => wait({ dir: 'D:\\code\\web-shop', command: `cd "D:\\code\\web-shop"; claude --resume ${rel.replace('.jsonl', '')}` })
export const OpenInClaude = () => wait(null)
export const CopyText = () => wait(null)

export const ListBackups = () => wait({ items: BACKUPS, dir: 'C:\\Users\\alex\\.claude-code-sync\\backup', total: 2510, keepDays: 14 })
export const ReadBackup = () => wait({ backup: BACKUP_TEXT, current: CURRENT_TEXT, currentExists: true, same: false, binary: false, tooLarge: false })
export const RestoreBackup = () => wait(null, 600)
export const OpenBackupFolder = () => wait(null)

export const ListConflicts = () => wait(CONFLICTS)
export const ReadConflict = (id) => wait({
  conflict: CONFLICTS.find((c) => c.id === id), base: CONFLICT_BASE, local: CONFLICT_LOCAL, remote: CONFLICT_REMOTE,
  hasBase: true, binary: false, tooLarge: false, canBoth: false,
})
export const ResolveConflict = () => wait(null, 500)
export const RestoreContentItem = () => wait(null)
export const GetContent = () => wait({
  ccSwitch: CC ? 'enabled' : 'notInstalled', ccSwitchKnown: true, pending: CC ? CC_CATS : [],
  items: CONTENT_ITEMS.map(({ tool, category }) => ({
    tool, category,
    enabled: !(tool === 'claude' && CC && CC_CATS.includes(category)),
    pending: tool === 'claude' && CC && CC_CATS.includes(category),
    ccOverlap: tool === 'claude' && CC_CATS.includes(category),
  })),
})
export const SetContentEnabled = () => wait(null)
export const SetToolContentEnabled = () => wait(null)
export const SetContentOwner = () => wait(null)

export const Confirm = () => wait(true)
export const GetSettings = () => wait({
  deviceName: 'office-pc', hasSecret: true, scanRoots: ['D:\\code'],
  storage: { type: 's3', provider: 'r2', endpoint: 'https://<ACCOUNT_ID>.r2.cloudflarestorage.com', region: 'auto', bucket: 'my-claude-sync', accessKeyId: 'AKIAEXAMPLE', accessKeySecret: '', pathStyle: false, prefix: 'v1/', path: '' },
})
export const SaveSettings = () => wait(null)
export const TestStorage = () => wait(null, 400)
export const ChangePassphrase = () => wait(null, 800)
export const AdoptPassphrase = () => wait(null)

export const GetLanguage = () => wait({ setting: en ? 'en' : 'zh-CN', effective: en ? 'en' : 'zh-CN', supported: ['zh-CN', 'en'] }, 0)
export const SetLanguage = (l) => wait({ setting: l, effective: l || 'zh-CN', supported: ['zh-CN', 'en'] }, 0)
export const GetTheme = () => wait({ setting: q.get('theme') || '', dark: q.get('theme') === 'dark' }, 0)
export const SetTheme = (s) => wait({ setting: s, dark: s === 'dark' }, 0)
export const GetAutostart = () => wait({ enabled: true, available: true })
export const SetAutostart = (on) => wait({ enabled: on, available: true })

export const ReadLog = () => wait({ path: 'C:\\Users\\alex\\.claude-code-sync\\sync.log', text: LOG })
export const ClearLog = () => wait(null)
export const OpenLogFolder = () => wait(null)

export const DefaultDeviceName = () => wait('alex')
export const Initialize = () => wait('created', 600)

// &click=选择器1,选择器2：页面渲染好后依次点这些元素（截图时用来展开详情、打开会话；只在演示模式里，不碰正式页面的代码）
if (q.get('click')) {
  q.get('click').split(',').forEach((sel, i) => {
    setTimeout(() => document.querySelector(sel)?.click(), 800 + i * 600)
  })
}
