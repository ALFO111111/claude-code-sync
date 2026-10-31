<template>
  <div class="backups">
    <!-- 先让人安心：这一页是"安全网"，平时空着才正常 -->
    <div class="card intro">
      <div class="row top">
        <div class="shield"><NavIcon name="backups" :size="26" /></div>
        <div class="grow">
          <h3>{{ t('backups.title') }}<Refreshing :on="res.loading && !!data" /></h3>
          <div class="note flush">{{ t('backups.intro', { days: data?.keepDays || 14 }) }}</div>
        </div>
      </div>
      <div class="row wrap mt" v-if="data">
        <span class="muted small">{{ t('backups.summary', { n: data.items.length, size: fmtSize(data.total) }) }}</span>
        <span class="grow"></span>
        <div class="segmented" v-if="data.items.length">
          <button v-for="f in filters" :key="f.key" :class="{ on: filter === f.key }" @click="filter = f.key">
            {{ t(`backups.filter.${f.key}`) }} <span class="muted">{{ f.count }}</span>
          </button>
        </div>
        <button class="sm" @click="openFolder"><NavIcon name="folder" :size="13" /> {{ t('backups.openFolder') }}</button>
      </div>
    </div>

    <Notice :notice="notice" />

    <div v-if="loadErr" class="err-box">{{ loadErr }}</div>
    <div v-else-if="!data" class="card"><Skeleton :lines="4" /></div>

    <div v-else-if="data && !data.items.length" class="card empty">
      <NavIcon name="backups" :size="44" />
      <div class="title">{{ t('backups.emptyTitle') }}</div>
      <div>{{ t('backups.emptyDesc') }}</div>
    </div>

    <template v-for="g in groups" :key="g.day">
      <div class="day">{{ g.label }}</div>
      <div class="card list">
        <div v-for="it in g.items" :key="it.id" class="item" :class="{ open: openId === it.id }">
          <div class="row line">
            <div class="kind-icon" :class="it.kind"><NavIcon :name="KIND_ICON[it.kind]" :size="16" /></div>
            <div class="grow">
              <div class="what">
                <i18n-t :keypath="`backups.what.${it.kind}`" tag="span">
                  <template #file><b class="mono">{{ fileName(it.rel) }}</b></template>
                </i18n-t>
              </div>
              <div class="muted small ellipsis" :title="it.rel">
                {{ projectName(it) }} · <span class="mono">{{ it.rel }}</span> · {{ clock(it.time) }}<template v-if="it.size"> · {{ fmtSize(it.size) }}</template>
              </div>
            </div>
            <template v-if="it.kind === 'fork'">
              <button v-if="it.bound" class="sm" @click="go('sessions', { pid: it.pid, rel: it.fork })">{{ t('backups.viewSession') }}</button>
            </template>
            <template v-else>
              <button class="sm ghost" @click="toggle(it)">{{ openId === it.id ? t('backups.hide') : t('backups.view') }}</button>
              <button class="sm" :disabled="busy" @click="restore(it)"><NavIcon name="restore" :size="13" /> {{ t('backups.restore') }}</button>
            </template>
          </div>

          <!-- 展开：看看这份备份和本机现在的差别，再决定要不要恢复 -->
          <div v-if="openId === it.id" class="detail">
            <div v-if="!content" class="muted">{{ t('common.loading') }}</div>
            <template v-else>
              <div v-if="content.binary" class="info-box">{{ t('backups.binary') }}</div>
              <template v-else>
                <div v-if="content.same" class="info-box">{{ t('backups.same') }}</div>
                <div v-else-if="!content.currentExists" class="info-box">{{ t('backups.noCurrent') }}</div>
                <div v-if="content.tooLarge" class="warn-box">{{ t('backups.tooLarge') }}</div>
                <div class="row tabs">
                  <div class="segmented">
                    <button v-if="canDiff" :class="{ on: tab === 'diff' }" @click="tab = 'diff'">{{ t('backups.tabDiff') }}</button>
                    <button :class="{ on: tab === 'backup' }" @click="tab = 'backup'">{{ t('backups.tabBackup') }}</button>
                    <button v-if="content.currentExists" :class="{ on: tab === 'current' }" @click="tab = 'current'">{{ t('backups.tabCurrent') }}</button>
                  </div>
                  <span v-if="tab === 'diff' && diffRows" class="muted small legend">
                    <span class="add-txt">+ {{ t('backups.legendAdd') }}</span>
                    <span class="del-txt">− {{ t('backups.legendDel') }}</span>
                  </span>
                </div>
                <div v-if="tab === 'diff'" class="code-view diff">
                  <template v-if="diffRows">
                    <div v-for="(d, i) in diffRows" :key="i" :class="d.type">
                      <template v-if="d.type === 'skip'">{{ t('backups.skipped', { n: d.count }) }}</template>
                      <template v-else><span class="sign">{{ d.type === 'add' ? '+' : d.type === 'del' ? '−' : '' }}</span>{{ d.text }}</template>
                    </div>
                  </template>
                  <div v-else class="muted">{{ t('backups.diffTooBig') }}</div>
                </div>
                <div v-else class="code-view">{{ (tab === 'current' ? content.current : content.backup) || t('backups.emptyFile') }}</div>
              </template>
              <div class="row mt">
                <span class="grow muted small">{{ t('backups.restoreNote') }}</span>
                <button class="primary sm" :disabled="busy || content.same" @click="restore(it)">{{ t('backups.restoreThis') }}</button>
              </div>
            </template>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup>
import { ref, computed, watch, onActivated } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api'
import { go } from '@/stores/nav'
import { resources } from '@/stores/data'
import { fmtSize, toolLabel, contentLabel } from '@/utils/format'
import { lineDiff, collapse } from '@/utils/diff'
import { useNotice } from '@/composables/useNotice'
import Notice from '@/components/Notice.vue'
import NavIcon from '@/components/NavIcon.vue'
import Skeleton from '@/components/Skeleton.vue'
import Refreshing from '@/components/Refreshing.vue'

const { t, locale } = useI18n()
const { notice, ok, fail, clear } = useNotice()
// 数据来自全局缓存（stores/data.js）：再进这页先显示上次的，后台悄悄刷新
const res = resources.backups
const data = computed(() => res.data)
const loadErr = computed(() => (res.data ? '' : res.error))
const filter = ref('all')
const busy = ref(false)

const KIND_ICON = { remote: 'conflict', local: 'download', restore: 'restore', fork: 'fork' }
const KINDS = ['remote', 'local', 'fork', 'restore']

const refresh = () => res.refresh()

// 只显示有内容的筛选项，免得一排"0"
const filters = computed(() => {
  const items = data.value?.items || []
  const out = [{ key: 'all', count: items.length }]
  for (const k of KINDS) {
    const n = items.filter((i) => i.kind === k).length
    if (n) out.push({ key: k, count: n })
  }
  return out
})

// 按天分组：今天 / 昨天 / 具体日期
const groups = computed(() => {
  const items = (data.value?.items || []).filter((i) => filter.value === 'all' || i.kind === filter.value)
  const out = []
  for (const it of items) {
    const d = new Date(it.time)
    const day = d.toDateString()
    if (!out.length || out[out.length - 1].day !== day) out.push({ day, label: dayLabel(d), items: [] })
    out[out.length - 1].items.push(it)
  }
  return out
})

function dayLabel(d) {
  const today = new Date()
  const yesterday = new Date(Date.now() - 86400_000)
  if (d.toDateString() === today.toDateString()) return t('backups.today')
  if (d.toDateString() === yesterday.toDateString()) return t('backups.yesterday')
  return d.toLocaleDateString(locale.value, { month: 'long', day: 'numeric', weekday: 'short' })
}
const clock = (ms) => new Date(ms).toLocaleTimeString(locale.value, { hour: '2-digit', minute: '2-digit' })
const fileName = (rel) => rel.split('/').pop()
// 全局内容（规则、技能、settings…）的备份没有项目，显示工具和类别
const projectName = (it) => (it.scope === 'global' ? `${toolLabel(it.tool)} · ${contentLabel(it.tool, it.category)}` : it.project)

// ---------- 展开查看 ----------
const openId = ref('')
const content = ref(null)
const tab = ref('diff')
const canDiff = computed(() => content.value && content.value.currentExists && !content.value.same)

async function toggle(it) {
  if (openId.value === it.id) {
    openId.value = ''
    return
  }
  openId.value = it.id
  content.value = null
  try {
    const c = await api.ReadBackup(it.id)
    if (openId.value !== it.id) return // 等待期间又点了别的
    content.value = c
    tab.value = c.currentExists && !c.same ? 'diff' : 'backup'
  } catch (e) {
    openId.value = ''
    fail(String(e))
  }
}

// 对比方向：从"本机现在"到"这份备份"——也就是"恢复后会变成什么样"
const diffRows = computed(() => {
  if (!canDiff.value) return null
  const d = lineDiff(content.value.current, content.value.backup)
  return d && collapse(d)
})
watch(filter, () => { openId.value = '' })

// ---------- 恢复 ----------
async function restore(it) {
  const name = fileName(it.rel)
  const yes = await api.Confirm(t('backups.confirmTitle'), t('backups.confirmMsg', { file: it.rel, project: projectName(it) }))
  if (!yes) return
  busy.value = true
  clear()
  try {
    await api.RestoreBackup(it.id)
    ok(t('backups.restored', { file: name }))
    openId.value = ''
  } catch (e) {
    fail(t('backups.restoreFailed', { err: String(e) }))
  } finally {
    busy.value = false
    refresh()
  }
}

async function openFolder() {
  try {
    await api.OpenBackupFolder()
  } catch (e) {
    fail(String(e))
  }
}

// 每次进这页都在后台刷新一下（本地读，很快），刷新期间照样显示上次的列表
onActivated(() => res.refresh())
</script>

<style scoped>
.intro .top { align-items: flex-start; gap: 14px; }
.shield { width: 44px; height: 44px; border-radius: 12px; background: var(--accent-soft); color: var(--accent); display: flex; align-items: center; justify-content: center; flex-shrink: 0; }
.intro h3 { margin-bottom: 4px; }
.flush { margin: 0; }
.tabs { margin-bottom: 8px; }

.day { font-size: 13px; color: var(--muted); font-weight: 500; margin: 18px 4px 8px; }
.list { padding: 4px 16px; }
.item + .item { border-top: 1px solid var(--border); }
.line { padding: 10px 0; }
.what { line-height: 1.5; }
.kind-icon { width: 30px; height: 30px; border-radius: 8px; display: flex; align-items: center; justify-content: center; flex-shrink: 0; }
.kind-icon.remote { background: var(--err-soft); color: var(--err); }
.kind-icon.local { background: var(--info-soft); color: var(--info); }
.kind-icon.fork { background: var(--accent-soft); color: var(--accent); }
.kind-icon.restore { background: var(--panel-2); color: var(--muted); }

.detail { padding: 4px 0 14px 40px; }
.detail .code-view { max-height: 50vh; } /* 跟着窗口高度走：全屏时能看到更多 */
.legend { display: inline-flex; gap: 12px; margin-left: 4px; }
.add-txt { color: var(--ok); }
.del-txt { color: var(--err); }
.diff > div { white-space: pre-wrap; padding-left: 22px; text-indent: -22px; }
.diff .sign { display: inline-block; width: 22px; text-indent: 0; opacity: 0.7; user-select: none; }
.diff .add { background: rgba(34, 197, 94, 0.16); color: #86efac; }
.diff .del { background: rgba(239, 68, 68, 0.16); color: #fca5a5; text-decoration: line-through; text-decoration-color: rgba(252, 165, 165, 0.5); }
.diff .skip { color: #6b7280; font-style: italic; padding: 2px 0; }
</style>
