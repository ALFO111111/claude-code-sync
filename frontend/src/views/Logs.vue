<template>
  <div class="logs">
    <div class="row wrap toolbar">
      <div class="segmented">
        <button :class="{ on: !problemsOnly }" @click="problemsOnly = false">{{ t('logs.all') }}</button>
        <button :class="{ on: problemsOnly }" @click="problemsOnly = true">{{ t('logs.problems') }} <span class="muted">{{ problemCount }}</span></button>
      </div>
      <input v-model="q" class="search" :placeholder="t('logs.search')" />
      <label class="row small auto"><input v-model="live" type="checkbox" /> {{ t('logs.live') }}</label>
      <span class="grow"></span>
      <button class="sm" @click="refresh">{{ t('common.refresh') }}</button>
      <button class="sm" @click="openFolder"><NavIcon name="folder" :size="13" /> {{ t('logs.openFolder') }}</button>
      <button class="sm danger" @click="clearLog">{{ t('logs.clear') }}</button>
    </div>
    <div class="muted small path mono">{{ logPath }}</div>
    <div v-if="err" class="err path">{{ err }}</div>
    <div ref="view" class="code-view log-view" @scroll="onScroll">
      <template v-if="shown.length">
        <div v-for="(l, i) in shown" :key="i" :class="l.level"><span class="ts">{{ l.ts }}</span><template v-for="(p, k) in highlight(l.msg, q.trim())" :key="k"><mark v-if="p.hit">{{ p.t }}</mark><template v-else>{{ p.t }}</template></template></div>
      </template>
      <div v-else class="muted">{{ lines.length ? t('logs.noMatch') : t('logs.empty') }}</div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, nextTick, onActivated, onDeactivated, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api'
import { syncState } from '@/stores/sync'
import { takeParams } from '@/stores/nav'
import { highlight } from '@/utils/format'
import { parseLog } from '@/utils/log'
import NavIcon from '@/components/NavIcon.vue'

const { t } = useI18n()
const lines = ref([])
const logPath = ref('')
const err = ref('')
const view = ref(null)
const q = ref('')
const problemsOnly = ref(false)
const live = ref(true)
let atBottom = true // 用户往上翻着看的时候，刷新不要把他拽回底部
let timer

const problemCount = computed(() => lines.value.filter((l) => l.level === 'error' || l.level === 'warn').length)
const shown = computed(() => {
  const lq = q.value.trim().toLowerCase()
  return lines.value.filter((l) =>
    (!problemsOnly.value || l.level === 'error' || l.level === 'warn') && (!lq || l.msg.toLowerCase().includes(lq)))
})

function onScroll() {
  const el = view.value
  if (el) atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 30
}

// 新日志在最后面：加载完滚到底，一眼看到最近发生了什么
async function refresh() {
  try {
    const v = await api.ReadLog()
    lines.value = parseLog(v.text)
    logPath.value = v.path
    err.value = ''
    await nextTick()
    if (atBottom && view.value) view.value.scrollTop = view.value.scrollHeight
  } catch (e) {
    err.value = String(e)
  }
}

async function clearLog() {
  try {
    await api.ClearLog()
  } catch (e) {
    err.value = String(e)
  }
  refresh()
}

async function openFolder() {
  try {
    await api.OpenLogFolder()
  } catch (e) {
    err.value = String(e)
  }
}

// 实时：页面开着时每 3 秒看一眼；切到别的页面就停（页面切走不销毁，见 App.vue 的 KeepAlive）
const active = ref(false)
function schedule() {
  clearInterval(timer)
  if (live.value && active.value) timer = setInterval(refresh, 3000)
}
watch(live, schedule)
// 同步一结束也马上刷新（只在看着这页时）
watch(() => syncState.running, (running) => { if (!running && active.value) refresh() })
watch([problemsOnly, q], async () => {
  atBottom = true
  await nextTick()
  if (view.value) view.value.scrollTop = view.value.scrollHeight
})

onActivated(() => {
  if (takeParams()?.problems) problemsOnly.value = true // 首页"看日志"跳过来：直接只看问题
  active.value = true
  refresh()
  schedule()
})
onDeactivated(() => {
  active.value = false
  schedule()
})
onUnmounted(() => clearInterval(timer))
</script>

<style scoped>
.toolbar { margin-bottom: 8px; }
.search { width: 220px; }
.auto { gap: 6px; cursor: pointer; color: var(--muted); }
.path { margin-bottom: 8px; }
.log-view { max-height: calc(100vh - 150px); }
.log-view .ts { color: #6b7280; margin-right: 8px; }
.log-view .error { color: #fca5a5; }
.log-view .warn { color: #fcd34d; }
.log-view .done { color: #86efac; }
</style>
