<template>
  <div style="display:flex; gap:14px; height:100%">
    <div class="card" style="width:240px; flex-shrink:0; overflow-y:auto; margin-bottom:0">
      <h3>{{ t('sessions.projects') }}</h3>
      <div v-if="!projects.length" class="muted">{{ t('sessions.noBound') }}</div>
      <div
        v-for="p in projects" :key="p.pid"
        class="nav-item clickable" :class="{ active: curPid === p.pid }"
        style="padding:8px 10px; border-radius:8px"
        @click="select(p)"
      >
        <div>{{ p.display || p.root }}</div>
        <div class="muted" style="font-size:12px">{{ kindLabel(p.kind) }}</div>
      </div>
    </div>

    <div class="card" style="flex:1; overflow-y:auto; margin-bottom:0">
      <template v-if="!viewer">
        <h3>{{ t('sessions.title') }} <span class="muted" v-if="curPid" style="font-weight:400">{{ curRoot }}</span></h3>
        <div v-if="!curPid" class="muted">{{ t('sessions.pickProject') }}</div>
        <table v-else>
          <thead>
            <tr>
              <th>{{ t('sessions.colTime') }}</th><th>{{ t('sessions.colSummary') }}</th><th>{{ t('sessions.colLines') }}</th>
              <th>{{ t('sessions.colSize') }}</th><th>{{ t('sessions.colCloud') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="s in sessions" :key="s.name" class="clickable" @click="open(s)">
              <td class="muted">{{ s.mtime }}</td>
              <td>{{ s.preview || t('sessions.noPreview') }}</td>
              <td>{{ s.lines }}</td>
              <td class="muted">{{ fmtSize(s.size) }}</td>
              <td><span class="badge" :class="s.synced ? 'ok' : 'warn'">{{ s.synced ? t('sessions.synced') : t('sessions.pending') }}</span></td>
            </tr>
          </tbody>
        </table>
        <div v-if="loadErr" class="err">{{ t('sessions.loadFailed', { err: loadErr }) }}</div>
        <div v-else-if="curPid && !sessions.length" class="muted">{{ t('sessions.empty') }}</div>
      </template>

      <template v-else>
        <div style="display:flex; align-items:center; gap:10px; margin-bottom:12px">
          <button @click="viewer = null">{{ t('common.back') }}</button>
          <div style="flex:1" class="muted">{{ viewerName }}</div>
          <span v-if="search.trim()" class="muted" style="font-size:12px">{{ t('sessions.matches', filteredMsgs.length) }}</span>
          <input v-model="search" :placeholder="t('sessions.search')" style="width:240px" />
        </div>
        <div class="chat">
          <div v-for="(m, i) in shownMsgs" :key="i" class="msg" :class="m.role">
            <div class="bubble">
              <div class="meta">{{ m.role === 'user' ? t('sessions.you') : 'Claude' }} · {{ m.time }}</div>
              <div v-if="m.text"><template v-for="(p, k) in highlight(m.text, search.trim())" :key="k"><mark v-if="p.hit">{{ p.t }}</mark><template v-else>{{ p.t }}</template></template></div>
              <div v-if="m.tools && m.tools.length">
                <details v-for="(tool, j) in m.tools" :key="j" style="display:inline">
                  <summary class="tool-chip">🔧 {{ tool.name }}</summary>
                  <div class="muted" style="font-size:12px; padding:4px 2px">{{ tool.brief }}</div>
                </details>
              </div>
            </div>
          </div>
          <div v-if="!filteredMsgs.length" class="muted">{{ search.trim() ? t('sessions.noMatch') : t('sessions.noMessages') }}</div>
          <div v-if="filteredMsgs.length > limit" style="text-align:center; margin:12px 0">
            <button @click="limit += PAGE">{{ t('sessions.showMore', { n: Math.min(PAGE, filteredMsgs.length - limit), left: filteredMsgs.length - limit }) }}</button>
          </div>
        </div>
      </template>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api'
import { fmtSize, kindLabel, highlight } from '@/utils/format'

const { t } = useI18n()
const projects = ref([])
const curPid = ref('')
const curRoot = ref('')
const sessions = ref([])
const viewer = ref(null)
const viewerName = ref('')
const search = ref('')
const loadErr = ref('')

// 先清空再加载：加载失败时不能还显示着上一个项目的会话
async function select(p) {
  curPid.value = p.pid
  curRoot.value = p.root
  viewer.value = null
  sessions.value = []
  loadErr.value = ''
  try {
    sessions.value = await api.ListSessions(p.pid)
  } catch (e) {
    loadErr.value = String(e)
  }
}

async function open(s) {
  loadErr.value = ''
  try {
    viewer.value = await api.ReadSession(curPid.value, s.rel)
    viewerName.value = s.name
    search.value = ''
    limit.value = PAGE
  } catch (e) {
    loadErr.value = String(e)
  }
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

// 只读本机绑定，不连云端：打开会话页不用等网络
onMounted(async () => {
  try {
    projects.value = await api.ListBound()
  } catch (e) {
    loadErr.value = String(e)
  }
})
</script>
