<template>
  <div class="sessions">
    <div class="card side">
      <div class="search-all">
        <NavIcon name="search" :size="15" class="icon" />
        <input v-model="globalQ" :placeholder="t('sessions.searchAll')" @keyup.enter="searchAll" />
      </div>
      <h3>{{ t('sessions.projects') }}</h3>
      <Skeleton v-if="!boundRes.data" :lines="4" />
      <div v-else-if="!projects.length" class="muted">{{ t('sessions.noBound') }}</div>
      <div
        v-for="p in projects" :key="p.pid"
        class="nav-item" :class="{ active: mode !== 'search' && curPid === p.pid }"
        @click="select(p)"
      >
        <div class="grow">
          <div class="ellipsis">{{ p.display || p.root }}</div>
          <div class="muted small">{{ kindLabel(p.kind) }}</div>
        </div>
      </div>
    </div>

    <div class="card main">
      <!-- 跨项目搜索结果 -->
      <template v-if="mode === 'search'">
        <div class="card-head">
          <h3>{{ t('sessions.searchTitle', { q: lastQ }) }}</h3>
          <button class="ghost sm" @click="mode = curPid ? 'list' : 'none'">{{ t('common.close') }}</button>
        </div>
        <div v-if="searching" class="muted">{{ t('sessions.searching') }}</div>
        <div v-else-if="!hits.length" class="empty"><div class="title">{{ t('sessions.noHits') }}</div>{{ t('sessions.noHitsHint') }}</div>
        <div v-else>
          <div class="muted small hit-count">{{ t('sessions.hitCount', { n: hits.length }) }}</div>
          <div v-for="h in hits" :key="h.pid + h.rel" class="hit clickable" @click="openHit(h)">
            <div class="row">
              <b class="grow ellipsis">{{ h.title || t('sessions.noPreview') }}</b>
              <span class="badge">{{ t('sessions.hitMsgs', { n: h.count }) }}</span>
            </div>
            <div class="muted small">{{ h.project }} · {{ h.mtime }}</div>
            <div class="snippet"><template v-for="(p, k) in highlight(h.snippet, lastQ)" :key="k"><mark v-if="p.hit">{{ p.t }}</mark><template v-else>{{ p.t }}</template></template></div>
          </div>
        </div>
      </template>

      <!-- 会话列表 -->
      <template v-else-if="!viewer">
        <h3>{{ t('sessions.title') }} <span class="muted root" v-if="curPid">{{ curRoot }}</span><Refreshing :on="listLoading && sessions.length > 0" /></h3>
        <div v-if="!curPid" class="empty">
          <NavIcon name="sessions" :size="40" />
          <div class="title">{{ t('sessions.pickProject') }}</div>
          {{ t('sessions.pickProjectHint') }}
        </div>
        <table v-else-if="sessions.length">
          <thead>
            <tr>
              <th>{{ t('sessions.colSummary') }}</th><th>{{ t('sessions.colTime') }}</th>
              <th>{{ t('sessions.colLines') }}</th><th>{{ t('sessions.colSize') }}</th><th class="ops"></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="s in sessions" :key="s.name" class="clickable" @click="open(s)">
              <td class="title-td">
                <div class="ellipsis" :title="s.preview">{{ s.title || s.preview || t('sessions.noPreview') }}</div>
                <div class="badges">
                  <span v-if="s.forkOf" class="badge warn" :title="t('sessions.forkHint')">{{ t('sessions.fork') }}</span>
                  <span v-if="!s.synced" class="badge" :title="t('sessions.pendingHint')">{{ t('sessions.pending') }}</span>
                </div>
              </td>
              <td class="muted nowrap">{{ s.mtime }}</td>
              <td>{{ s.lines }}</td>
              <td class="muted nowrap">{{ fmtSize(s.size) }}</td>
              <td class="ops" @click.stop>
                <button v-if="s.resume" class="sm" :title="t('sessions.resumeHint')" @click="resume(s.rel)"><NavIcon name="play" :size="12" /> {{ t('sessions.resume') }}</button>
                <button v-if="s.resume" class="sm ghost" :title="t('sessions.copyCmd')" @click="copyCmd(s.rel)"><NavIcon name="copy" :size="13" /></button>
              </td>
            </tr>
          </tbody>
        </table>
        <Skeleton v-else-if="listLoading" :lines="4" />
        <div v-else-if="!loadErr" class="empty"><div class="title">{{ t('sessions.empty') }}</div></div>
        <div v-if="loadErr" class="err">{{ t('sessions.loadFailed', { err: loadErr }) }}</div>
      </template>

      <!-- 会话查看器 -->
      <template v-else>
        <div class="row viewer-head">
          <button @click="closeViewer">{{ t('common.back') }}</button>
          <div class="grow ellipsis"><b>{{ viewerTitle }}</b></div>
          <template v-if="viewerResume">
            <button class="sm primary" :title="t('sessions.resumeHint')" @click="resume(viewerRel)"><NavIcon name="play" :size="12" /> {{ t('sessions.resume') }}</button>
            <button class="sm" @click="copyCmd(viewerRel)"><NavIcon name="copy" :size="13" /> {{ t('sessions.copyCmd') }}</button>
          </template>
        </div>
        <div class="row viewer-search">
          <input v-model="search" class="grow" :placeholder="t('sessions.search')" />
          <span v-if="search.trim()" class="muted small nowrap">{{ t('sessions.matches', filteredMsgs.length) }}</span>
        </div>
        <div class="chat">
          <div v-for="(m, i) in shownMsgs" :key="i" class="msg" :class="m.role">
            <div class="bubble">
              <div class="meta">{{ m.role === 'user' ? t('sessions.you') : 'Claude' }} · {{ m.time }}</div>
              <div v-if="m.text"><template v-for="(p, k) in highlight(m.text, search.trim())" :key="k"><mark v-if="p.hit">{{ p.t }}</mark><template v-else>{{ p.t }}</template></template></div>
              <div v-if="m.tools && m.tools.length">
                <details v-for="(tool, j) in m.tools" :key="j" class="tool">
                  <summary class="tool-chip">🔧 {{ tool.name }}</summary>
                  <div class="muted small tool-brief">{{ tool.brief }}</div>
                </details>
              </div>
            </div>
          </div>
          <div v-if="!filteredMsgs.length" class="muted">{{ search.trim() ? t('sessions.noMatch') : t('sessions.noMessages') }}</div>
          <div v-if="filteredMsgs.length > limit" class="more">
            <button @click="limit += PAGE">{{ t('sessions.showMore', { n: Math.min(PAGE, filteredMsgs.length - limit), left: filteredMsgs.length - limit }) }}</button>
          </div>
        </div>
      </template>
      <Notice :notice="notice" />
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onActivated } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api'
import { takeParams } from '@/stores/nav'
import { resources } from '@/stores/data'
import { fmtSize, kindLabel, highlight } from '@/utils/format'
import { useNotice } from '@/composables/useNotice'
import Notice from '@/components/Notice.vue'
import NavIcon from '@/components/NavIcon.vue'
import Skeleton from '@/components/Skeleton.vue'
import Refreshing from '@/components/Refreshing.vue'

const { t } = useI18n()
const { notice, ok, fail, clear } = useNotice()
const boundRes = resources.bound
const projects = computed(() => boundRes.data || [])
const curPid = ref('')
const curRoot = ref('')
const sessions = ref([])
const loadErr = ref('')
const mode = ref('none') // none | list | search（查看器开着时看 viewer）

const viewer = ref(null)
const viewerTitle = ref('')
const viewerRel = ref('')
const viewerResume = ref(false)
const search = ref('')

// 每个项目的会话列表记一份：切回看过的项目时先显示上次的，后台再刷新（不会先闪一下"没有会话"）
const listCache = new Map()
const listLoading = ref(false)

// 加载失败时不能还显示着上一个项目的会话：没缓存就先清空
async function select(p) {
  curPid.value = p.pid
  curRoot.value = p.root
  viewer.value = null
  sessions.value = listCache.get(p.pid) || []
  loadErr.value = ''
  mode.value = 'list'
  clear()
  await reloadList()
}

// reloadList 刷新当前项目的会话列表（不清空，刷新期间照样显示旧的）
async function reloadList() {
  const pid = curPid.value
  if (!pid) return
  listLoading.value = true
  try {
    const list = await api.ListSessions(pid)
    listCache.set(pid, list)
    if (curPid.value === pid) sessions.value = list
  } catch (e) {
    if (curPid.value === pid) loadErr.value = String(e)
  } finally {
    listLoading.value = false
  }
}

async function openRel(rel, title, resumable, q = '') {
  loadErr.value = ''
  try {
    viewer.value = await api.ReadSession(curPid.value, rel)
    viewerRel.value = rel
    viewerTitle.value = title || rel
    viewerResume.value = resumable
    search.value = q
    limit.value = PAGE
  } catch (e) {
    loadErr.value = String(e)
  }
}
const open = (s) => openRel(s.rel, s.title || s.preview, s.resume)

function closeViewer() {
  viewer.value = null
  if (mode.value === 'none' && curPid.value) mode.value = 'list'
}

// ---------- 在 Claude Code 里继续 ----------
async function resume(rel) {
  clear()
  try {
    await api.OpenInClaude(curPid.value, rel)
    ok(t('sessions.resumeOpened'))
  } catch (e) {
    fail(t('sessions.resumeFailed', { err: String(e) }))
  }
}
async function copyCmd(rel) {
  clear()
  try {
    const r = await api.ResumeCommand(curPid.value, rel)
    await api.CopyText(r.command)
    ok(t('sessions.copied', { cmd: r.command }))
  } catch (e) {
    fail(String(e))
  }
}

// ---------- 跨项目搜索 ----------
const globalQ = ref('')
const lastQ = ref('')
const hits = ref([])
const searching = ref(false)
async function searchAll() {
  const q = globalQ.value.trim()
  if ([...q].length < 2) {
    fail(t('sessions.searchTooShort'))
    return
  }
  clear()
  lastQ.value = q
  mode.value = 'search'
  viewer.value = null
  searching.value = true
  try {
    hits.value = await api.SearchSessions(q)
  } catch (e) {
    hits.value = []
    fail(String(e))
  } finally {
    searching.value = false
  }
}
async function openHit(h) {
  const p = projects.value.find((x) => x.pid === h.pid)
  if (p) {
    curPid.value = p.pid
    curRoot.value = p.root
  }
  mode.value = 'none' // 返回时回到这个项目的列表
  await openRel(h.rel, h.title, /^[0-9a-f-]{36}\.jsonl$/i.test(h.rel), lastQ.value)
  sessions.value = await api.ListSessions(h.pid).catch(() => [])
}

// 长会话一次渲染几千条会卡：先显示 PAGE 条，往下点"再显示"
const PAGE = 200
const limit = ref(PAGE)
watch(search, () => { limit.value = PAGE })

const filteredMsgs = computed(() => {
  if (!viewer.value) return []
  const q = search.value.trim().toLowerCase()
  if (!q) return viewer.value
  return viewer.value.filter((m) => (m.text || '').toLowerCase().includes(q))
})
const shownMsgs = computed(() => filteredMsgs.value.slice(0, limit.value))

// 每次进这页：项目列表（只读本机，很快）和当前项目的会话列表都在后台刷新，期间照样显示上次的
onActivated(async () => {
  await boundRes.refresh()
  if (mode.value !== 'search' && !viewer.value) reloadList()
  // 从别的页面跳过来（备份页"查看会话"）：直接打开那个会话
  const p = takeParams()
  if (p?.pid) {
    const proj = projects.value.find((x) => x.pid === p.pid)
    if (proj) {
      await select(proj)
      const s = sessions.value.find((x) => x.rel === p.rel)
      if (s) open(s)
    }
  }
})
</script>

<style scoped>
.sessions { display: flex; gap: 14px; height: 100%; }
.side { width: 240px; flex-shrink: 0; overflow-y: auto; margin-bottom: 0; }
.side .nav-item { padding: 8px 10px; }
.main { flex: 1; min-width: 0; overflow-y: auto; margin-bottom: 0; }
.search-all { position: relative; margin-bottom: 14px; }
.search-all input { padding-left: 30px; }
.search-all .icon { position: absolute; left: 10px; top: 50%; transform: translateY(-50%); color: var(--muted); }
/* 标题列吃掉剩下的全部宽度（max-width: 0 让长标题在这一格里截断，而不是把表格撑宽），窗口多宽就显示多长 */
.title-td { width: 100%; max-width: 0; }
.nowrap { white-space: nowrap; }
.viewer-head { margin-bottom: 10px; }
.viewer-search { margin-bottom: 12px; }
.hit { padding: 10px 12px; border-radius: 8px; border: 1px solid var(--border); margin-bottom: 8px; }
.hit:hover { border-color: var(--accent); }
.hit-count { margin-bottom: 8px; }
.root { font-weight: 400; }
.tool { display: inline; }
.tool-brief { padding: 4px 2px; }
.more { text-align: center; margin: 12px 0; }
.snippet { font-size: 13px; margin-top: 6px; line-height: 1.6; color: var(--text); }
</style>
