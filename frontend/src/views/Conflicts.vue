<template>
  <div class="conflicts">
    <div class="card intro">
      <div class="row top">
        <div class="badge-icon"><NavIcon name="conflicts" :size="26" /></div>
        <div class="grow">
          <h3 class="row tool-title"><ToolIcon :tool="tool" :size="18" />{{ tool.name }} · {{ t('conflicts.title') }}<Refreshing :on="res.loading && !!list" /></h3>
          <div class="note flush">{{ t('conflicts.intro') }}</div>
        </div>
      </div>
    </div>

    <Notice :notice="notice" />

    <div v-if="loadErr" class="err-box">{{ loadErr }}</div>
    <div v-else-if="!list" class="card"><Skeleton :lines="3" /></div>

    <div v-else-if="!list.length" class="card empty">
      <NavIcon name="check" :size="44" />
      <div class="title">{{ t('conflicts.emptyTitle') }}</div>
      <div>{{ t('conflicts.emptyDesc') }}</div>
    </div>

    <div v-else class="card list">
      <div v-for="it in list" :key="it.id" class="item" :class="{ open: openId === it.id }">
        <div class="row line">
          <div class="kind-icon"><NavIcon name="conflict" :size="16" /></div>
          <div class="grow">
            <div class="what"><b>{{ toolLabel(it.tool) }} · {{ contentLabel(it.tool, it.category) }}</b> · <span class="mono">{{ it.rel }}</span></div>
            <div class="muted small ellipsis" :title="it.localPath">{{ it.localPath }} · {{ it.timeText }}</div>
          </div>
          <button class="sm" @click="toggle(it)">{{ openId === it.id ? t('backups.hide') : t('conflicts.handle') }}</button>
        </div>

        <div v-if="openId === it.id" class="detail">
          <div v-if="!content" class="muted">{{ t('common.loading') }}</div>
          <template v-else>
            <div class="info-box">{{ t('conflicts.safe') }}</div>
            <div v-if="content.binary" class="info-box">{{ t('conflicts.binary') }}</div>
            <template v-else>
              <div v-if="content.tooLarge" class="warn-box">{{ t('conflicts.tooLarge') }}</div>
              <div class="row tabs">
                <div class="segmented">
                  <button :class="{ on: tab === 'diff' }" @click="tab = 'diff'">{{ t('conflicts.tabDiff') }}</button>
                  <button :class="{ on: tab === 'local' }" @click="tab = 'local'">{{ t('conflicts.tabLocal') }}</button>
                  <button :class="{ on: tab === 'remote' }" @click="tab = 'remote'">{{ t('conflicts.tabRemote') }}</button>
                  <button v-if="content.hasBase" :class="{ on: tab === 'base' }" @click="tab = 'base'">{{ t('conflicts.tabBase') }}</button>
                  <button v-if="!content.tooLarge" :class="{ on: tab === 'edit' }" @click="startEdit">{{ t('conflicts.tabEdit') }}</button>
                </div>
                <span v-if="tab === 'diff' && diffRows" class="muted small legend">
                  <span class="add-txt">+ {{ t('conflicts.legendRemote') }}</span>
                  <span class="del-txt">− {{ t('conflicts.legendLocal') }}</span>
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
              <template v-else-if="tab === 'edit'">
                <div class="muted small">{{ t('conflicts.editHint') }}</div>
                <textarea v-model="merged" class="mono editor" spellcheck="false"></textarea>
              </template>
              <div v-else class="code-view">{{ content[tab] || t('backups.emptyFile') }}</div>
            </template>

            <div class="row wrap mt actions">
              <button :disabled="busy" @click="resolve(it, 'local')">{{ t('conflicts.keepLocal') }}</button>
              <button :disabled="busy" @click="resolve(it, 'remote')">{{ t('conflicts.useRemote') }}</button>
              <button v-if="content.canBoth" :disabled="busy" @click="resolve(it, 'both')">{{ t('conflicts.keepBoth') }}</button>
              <button v-if="tab === 'edit'" class="primary" :disabled="busy" @click="resolve(it, 'merged')">{{ t('conflicts.saveMerged') }}</button>
            </div>
            <div class="muted small mt">{{ t('conflicts.resolveNote') }}</div>
          </template>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onActivated } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api'
import { resources } from '@/stores/data'
import { currentTool, toolOf } from '@/stores/tools'
import ToolIcon from '@/components/ToolIcon.vue'
import { lineDiff, collapse } from '@/utils/diff'
import { toolLabel, contentLabel } from '@/utils/format'
import { useNotice } from '@/composables/useNotice'
import Notice from '@/components/Notice.vue'
import NavIcon from '@/components/NavIcon.vue'
import Skeleton from '@/components/Skeleton.vue'
import Refreshing from '@/components/Refreshing.vue'

const { t } = useI18n()
const { notice, ok, fail, clear } = useNotice()
const res = resources.conflicts
// 只列当前工具的冲突（顶部工具切换）
const tool = currentTool
const list = computed(() => res.data && res.data.filter((it) => toolOf(it) === tool.value.id))
const loadErr = computed(() => (res.data ? '' : res.error))
const busy = ref(false)

const openId = ref('')
const content = ref(null)
const tab = ref('diff')
const merged = ref('')

async function toggle(it) {
  if (openId.value === it.id) {
    openId.value = ''
    return
  }
  openId.value = it.id
  content.value = null
  tab.value = 'diff'
  try {
    const c = await api.ReadConflict(it.id)
    if (openId.value !== it.id) return // 等待期间又点了别的
    content.value = c
  } catch (e) {
    openId.value = ''
    fail(String(e))
  }
}

// 对比方向：从"本机"到"另一台"——绿色是另一台多出来的，红色是只有本机有的
const diffRows = computed(() => {
  if (!content.value || content.value.binary) return null
  const d = lineDiff(content.value.local, content.value.remote)
  return d && collapse(d)
})

// 手工合并从本机版本开始改（本机这份是用户眼下正在用的）
function startEdit() {
  if (tab.value !== 'edit') merged.value = content.value.local
  tab.value = 'edit'
}

async function resolve(it, choice) {
  const yes = await api.Confirm(t('conflicts.confirmTitle'), t(`conflicts.confirm.${choice}`, { file: it.rel }))
  if (!yes) return
  busy.value = true
  clear()
  try {
    await api.ResolveConflict(it.id, choice, choice === 'merged' ? merged.value : '')
    ok(t('conflicts.resolved', { file: it.rel }))
    openId.value = ''
  } catch (e) {
    fail(t('conflicts.resolveFailed', { err: String(e) }))
  } finally {
    busy.value = false
    res.refresh()
    resources.status.refresh() // 侧边栏的数字
  }
}

onActivated(() => res.refresh())
</script>

<style scoped>
.intro .top { align-items: flex-start; gap: 14px; }
.badge-icon { width: 44px; height: 44px; border-radius: 12px; background: var(--err-soft); color: var(--err); display: flex; align-items: center; justify-content: center; flex-shrink: 0; }
.intro h3 { margin-bottom: 4px; }
.flush { margin: 0; }
.tabs { margin: 8px 0; }
.list { padding: 4px 16px; }
.tool-title { gap: 8px; }
.item + .item { border-top: 1px solid var(--border); }
.line { padding: 10px 0; }
.what { line-height: 1.5; }
.kind-icon { width: 30px; height: 30px; border-radius: 8px; display: flex; align-items: center; justify-content: center; flex-shrink: 0; background: var(--err-soft); color: var(--err); }
.detail { padding: 4px 0 14px 40px; }
.detail .code-view { max-height: 50vh; }
.editor { width: 100%; min-height: 40vh; margin-top: 6px; font-size: 12px; }
.actions { gap: 8px; }
.legend { display: inline-flex; gap: 12px; margin-left: 4px; }
.add-txt { color: var(--ok); }
.del-txt { color: var(--err); }
.diff > div { white-space: pre-wrap; padding-left: 22px; text-indent: -22px; }
.diff .sign { display: inline-block; width: 22px; text-indent: 0; opacity: 0.7; user-select: none; }
.diff .add { background: rgba(34, 197, 94, 0.16); color: #86efac; }
.diff .del { background: rgba(239, 68, 68, 0.16); color: #fca5a5; }
.diff .skip { color: #6b7280; font-style: italic; padding: 2px 0; }
</style>
