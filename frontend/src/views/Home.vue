<template>
  <div>
    <!-- 口令在别的设备上改了：同步已经停了，最要紧，放最上面 -->
    <div v-if="status?.keyMismatch" class="card alert">
      <h3>{{ t('home.keyTitle') }}</h3>
      <div class="note key-desc">{{ t('home.keyDesc') }}</div>
      <div class="row">
        <input v-model="newPass" type="password" class="grow" :placeholder="t('home.keyPlaceholder')" @keyup.enter="adopt" />
        <button class="primary" :disabled="!newPass || adopting" @click="adopt">{{ adopting ? t('common.working') : t('home.keyAdopt') }}</button>
      </div>
      <div v-if="adoptErr" class="err mt">{{ adoptErr }}</div>
    </div>

    <div class="card">
      <div class="row hero">
        <div class="grow">
          <h3>{{ t('home.status') }}</h3>
          <template v-if="status">
            <div class="last-sync" :title="status.lastSync">{{ t('home.lastSync', { when: fromNow(status.lastSyncAt, now) }) }}</div>
            <div class="muted small" v-if="status.lastSync">{{ status.lastSync }}</div>
            <div class="badges mt">
              <span class="badge">{{ t('home.pushed', { n: status.stats.pushed }) }}</span>
              <span class="badge">{{ t('home.pulled', { n: status.stats.pulled }) }}</span>
              <span class="badge">{{ t('home.merged', { n: status.stats.merged }) }}</span>
              <span class="badge">{{ t('home.forked', { n: status.stats.forked }) }}</span>
              <span class="badge">{{ t('home.history', { n: status.stats.history }) }}</span>
              <span class="badge" :class="status.stats.errors ? 'err' : 'ok'">{{ t('home.errors', { n: status.stats.errors }) }}</span>
              <button v-if="status.changes.length" class="link small" @click="showChanges = !showChanges">
                {{ showChanges ? t('home.hideChanges') : t('home.showChanges', { n: status.changes.length }) }}
              </button>
              <button v-if="status.stats.errors" class="link small" @click="go('logs', { problems: true })">{{ t('home.seeLog') }}</button>
            </div>
          </template>
        </div>
        <div class="sync-box">
          <button class="primary lg" :disabled="syncState.running" @click="api.SyncNow()">
            <span v-if="syncState.running" class="spin">↻</span> {{ syncState.running ? t('common.syncing') : t('common.syncNow') }}
          </button>
          <template v-if="syncState.running && syncState.total">
            <div class="progress mt"><div :style="{ width: pct + '%' }"></div></div>
            <div class="muted small mt">{{ t('home.progress', { done: syncState.done, total: syncState.total }) }}</div>
          </template>
        </div>
      </div>

      <!-- 本轮明细：哪些文件传了、拉了、合并了 -->
      <div v-if="showChanges && status?.changes.length" class="changes">
        <div v-for="(c, i) in status.changes" :key="i" class="change">
          <span class="kind" :class="c.kind"><NavIcon :name="CHANGE_ICON[c.kind]" :size="14" /> {{ t(`home.change.${c.kind}`) }}</span>
          <span class="grow ellipsis" :title="c.rel"><b>{{ changeName(c) }}</b> <span class="muted mono">{{ c.rel }}</span></span>
          <button v-if="c.kind === 'conflict' && !c.pid" class="link small" @click="go('conflicts')">{{ t('home.seeConflicts') }}</button>
          <button v-else-if="c.kind === 'fork' || c.kind === 'conflict'" class="link small" @click="go('backups')">{{ t('home.seeBackups') }}</button>
        </div>
        <div v-if="status.changes.length >= 200" class="muted small mt">{{ t('home.changesCapped') }}</div>
      </div>
    </div>

    <!-- 全局内容的冲突、和 CC Switch 谁负责还没选：都不挡其他同步，只提醒一句 -->
    <div v-if="status?.conflicts" class="card alert row">
      <span class="grow">{{ t('home.conflicts', { n: status.conflicts }) }}</span>
      <button class="sm primary" @click="go('conflicts')">{{ t('home.seeConflicts') }}</button>
    </div>
    <div v-if="status?.pendingContent?.length" class="card alert row">
      <span class="grow">{{ t('home.pendingContent') }}</span>
      <button class="sm" @click="go('settings')">{{ t('home.chooseOwner') }}</button>
    </div>

    <!-- 只在这次同步真有变化或出错时提示；自动同步每轮"什么都没变"不刷屏 -->
    <Notice :notice="result" />

    <!-- 自动同步平时不展示；只有没装好时才提示 -->
    <div class="card alert" v-if="status && !status.autoSync">
      <h3>{{ t('home.autoSyncNotReady') }}</h3>
      <div class="err">{{ status.autoSyncIssue }}</div>
      <div v-if="!status.sandboxed" class="row mt">
        <button class="primary" :disabled="repairing" @click="repair">{{ repairing ? t('home.repairing') : t('home.repair') }}</button>
        <span class="muted small">{{ t('home.repairHint') }}</span>
      </div>
    </div>

    <!-- 每个工具一张卡片：同步了哪些、有多少会话；点卡片切到那个工具 -->
    <div v-if="toolList.length" class="tool-cards">
      <div v-for="x in toolList" :key="x.id" class="card tool-card clickable" :class="{ off: !x.installed }" @click="openTool(x)">
        <div class="row">
          <ToolIcon :tool="x" :size="30" :off="!x.installed" />
          <div class="grow">
            <div class="tool-name">{{ x.name }}</div>
            <div class="muted small">{{ x.installed ? t('tools.sessions', { n: x.sessionCount }) : t('tools.notInstalled') }}</div>
          </div>
        </div>
        <div class="muted small mt">{{ t('tools.contentOn', { on: x.contentOn, total: x.contentTotal }) }}</div>
        <div v-if="toolConflicts(x.id)" class="err small">{{ t('tools.conflicts', { n: toolConflicts(x.id) }) }}</div>
      </div>
    </div>

    <div class="grid2">
      <div class="card" v-if="status">
        <h3>{{ t('home.myDevices') }}<Refreshing :on="statusRes.loading" /></h3>
        <div class="device">
          <NavIcon name="device" class="dev-icon" />
          <div class="grow">
            <div><b>{{ status.deviceName }}</b> <span class="badge ok">{{ t('home.thisDevice') }}</span></div>
            <div class="muted small">{{ t('home.projects', status.projects) }}</div>
          </div>
        </div>
        <div v-for="d in status.devices" :key="d.id" class="device" :title="t('home.deviceId', { id: d.id })">
          <NavIcon name="device" class="dev-icon" />
          <div class="grow">
            <div>{{ d.name }}</div>
            <div class="muted small">{{ t('home.projects', d.projects) }} · {{ lastSeen(d.lastSeen) }}</div>
          </div>
        </div>
        <div v-if="!status.devices.length" class="note">{{ t('home.noOtherDevices') }}</div>
      </div>

      <div class="card">
        <div class="card-head">
          <h3>{{ t('home.usage') }}<Refreshing :on="usageRes.loading && !!usage" /></h3>
          <button class="ghost sm" :disabled="usageRes.loading" @click="usageRes.refresh()">{{ t('common.refresh') }}</button>
        </div>
        <!-- 有旧数据时照样显示旧数据，错误只在上面提一句 -->
        <div v-if="usageRes.error" class="err small">{{ t('home.usageFailed', { err: usageRes.error }) }}</div>
        <Skeleton v-if="!usage && !usageRes.error" :lines="5" />
        <template v-else-if="usage">
          <div class="usage-total">{{ fmtSize(usage.total) }} <span class="muted small">{{ t('home.usageFiles', { n: usage.objects }) }}</span></div>
          <div class="stack mt" v-if="usage.total">
            <div class="s1" :style="{ width: share(usage.sessions) }" :title="t('home.usageSessions')"></div>
            <div class="s2" :style="{ width: share(usage.history) }" :title="t('home.usageHistory')"></div>
            <div class="s3" :style="{ width: share(usage.other) }" :title="t('home.usageOther')"></div>
          </div>
          <div class="legend small muted">
            <span><i class="s1"></i>{{ t('home.usageSessions') }} {{ fmtSize(usage.sessions) }}</span>
            <span><i class="s2"></i>{{ t('home.usageHistory') }} {{ fmtSize(usage.history) }}</span>
            <span><i class="s3"></i>{{ t('home.usageOther') }} {{ fmtSize(usage.other) }}</span>
          </div>
          <div v-for="p in topProjects" :key="p.pid || p.display" class="proj-usage">
            <div class="row">
              <span class="grow ellipsis" :title="p.pid">{{ p.display || t('home.usageUnknown') }}</span>
              <span class="muted small">{{ fmtSize(p.bytes) }}</span>
            </div>
            <div class="bar"><div :style="{ width: barWidth(p.bytes) }"></div></div>
          </div>
          <div v-if="usage.projects.length > TOP" class="muted small mt">
            {{ t('home.usageMore', { n: usage.projects.length - TOP, size: fmtSize(restBytes) }) }}
          </div>
          <div class="note">{{ t('home.usageNote') }}</div>
        </template>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted, onUnmounted, onActivated } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api'
import { syncState } from '@/stores/sync'
import { go } from '@/stores/nav'
import { resources } from '@/stores/data'
import { fromNow, fmtSize, toolLabel, contentLabel } from '@/utils/format'
import { tools, setTool } from '@/stores/tools'
import ToolIcon from '@/components/ToolIcon.vue'
import Notice from '@/components/Notice.vue'
import NavIcon from '@/components/NavIcon.vue'
import Skeleton from '@/components/Skeleton.vue'
import Refreshing from '@/components/Refreshing.vue'

const { t, locale } = useI18n()
// 数据都来自全局缓存（stores/data.js）：再进首页时先显示上次的，后台悄悄刷新
const statusRes = resources.status
const usageRes = resources.usage
const status = computed(() => statusRes.data)
const usage = computed(() => usageRes.data)
const repairing = ref(false)
const showChanges = ref(false)
const now = ref(Date.now()) // 每 30 秒走一次，让"3 分钟前"自己更新
let timer

const CHANGE_ICON = { push: 'upload', pull: 'download', merge: 'merge', fork: 'fork', conflict: 'conflict' }
const changeName = (c) => (c.pid ? c.project : `${toolLabel(c.tool)} · ${contentLabel(c.tool, c.project)}`)

const refresh = () => statusRes.refresh()

async function repair() {
  repairing.value = true
  try {
    await api.RepairAutoSync()
  } catch {
    // 失败原因已写进日志；刷新后卡片上仍会显示哪一项没装好
  } finally {
    repairing.value = false
    refresh()
  }
}

// ---------- 口令在别的设备上改了 ----------
const newPass = ref('')
const adopting = ref(false)
const adoptErr = ref('')
async function adopt() {
  if (!newPass.value) return
  adopting.value = true
  adoptErr.value = ''
  try {
    await api.AdoptPassphrase(newPass.value)
    newPass.value = ''
    refresh()
  } catch (e) {
    adoptErr.value = String(e)
  } finally {
    adopting.value = false
  }
}

// ---------- 同步进度、结果 ----------
const pct = computed(() => (syncState.total ? Math.round((syncState.done / syncState.total) * 100) : 0))

// 上一次同步的结果提示：同步中、或者什么都没变时不显示
const result = computed(() => {
  const e = syncState.last
  if (!e || syncState.running) return null
  if (e.error) return { error: true, text: e.error }
  const s = e.stats
  const parts = [
    s.pushed && t('home.pushed', { n: s.pushed }),
    s.pulled && t('home.pulled', { n: s.pulled }),
    s.merged && t('home.merged', { n: s.merged }),
    s.forked && t('home.forked', { n: s.forked }),
    s.history && t('home.historyAdded', s.history),
  ].filter(Boolean)
  return parts.length ? { error: false, text: t('home.done', { parts: parts.join(t('common.listSep')) }) } : null
})

// 其他设备最近一次上报的时间（同步时至少每小时报一次，所以一小时内的都算"在线"）
function lastSeen(ms) {
  if (!ms) return t('home.lastSeenUnknown')
  if (now.value - ms < 3600_000) return t('home.activeRecently')
  return t('home.lastSeen', { when: fromNow(ms, now.value) })
}

// ---------- 云端用量 ----------
const TOP = 5
const topProjects = computed(() => usage.value?.projects.slice(0, TOP) || [])
const restBytes = computed(() => (usage.value?.projects.slice(TOP) || []).reduce((n, p) => n + p.bytes, 0))
const share = (n) => (usage.value?.total ? (n / usage.value.total) * 100 + '%' : '0')
const barWidth = (n) => (topProjects.value[0]?.bytes ? Math.max(2, (n / topProjects.value[0].bytes) * 100) + '%' : '0')

// 同步结束后的刷新由 stores/data.js 统一做；切换语言后要刷新（"自动同步没就绪"的原因是后端按语言生成的）
watch(locale, refresh)

// 每次进首页（包括第一次）：状态是本地读的，总是刷新；云端用量 5 分钟内不重复拉
// 工具卡片
const toolList = tools
const toolConflicts = (id) => status.value?.toolConflicts?.[id] || 0
function openTool(x) {
  setTool(x.id)
  go('sessions')
}

onActivated(() => {
  statusRes.load()
  usageRes.load()
  resources.tools.refresh() // 会话数随时在变，本地读很快
  now.value = Date.now()
})

onMounted(() => {
  timer = setInterval(() => { now.value = Date.now() }, 30000)
})
onUnmounted(() => clearInterval(timer))
</script>

<style scoped>
.tool-cards { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 14px; margin-bottom: 14px; }
.tool-cards .card { margin-bottom: 0; }
.tool-card .row { gap: 12px; }
.tool-card:hover { border-color: var(--accent); }
.tool-card.off { opacity: 0.7; }
.tool-name { font-weight: 600; }
.key-desc { margin: 0 0 10px; }
.hero { align-items: flex-start; gap: 20px; }
.last-sync { font-size: 18px; font-weight: 600; }
.sync-box { width: 200px; text-align: center; }
.sync-box button { width: 100%; }

.changes { margin-top: 14px; border-top: 1px solid var(--border); padding-top: 10px; max-height: 280px; overflow-y: auto; }
.change { display: flex; align-items: center; gap: 10px; padding: 5px 0; font-size: 13px; }
.change .kind { display: inline-flex; align-items: center; gap: 4px; width: 64px; flex-shrink: 0; color: var(--muted); }
.change .kind.pull { color: var(--info); }
.change .kind.push { color: var(--ok); }
.change .kind.merge, .change .kind.fork { color: var(--accent); }
.change .kind.conflict { color: var(--err); }

.grid2 { display: grid; grid-template-columns: 1fr 1fr; gap: 14px; }
.grid2 .card { margin-bottom: 0; }
.device { display: flex; align-items: center; gap: 12px; padding: 8px 0; }
.device + .device { border-top: 1px solid var(--border); }
.dev-icon { color: var(--muted); flex-shrink: 0; }

.usage-total { font-size: 22px; font-weight: 600; }
.stack { display: flex; height: 8px; border-radius: 4px; overflow: hidden; background: var(--border); }
.s1 { background: var(--accent); }
.s2 { background: var(--info); }
.s3 { background: var(--muted); }
.legend { display: flex; flex-wrap: wrap; gap: 4px 14px; margin: 8px 0 12px; }
.legend i { display: inline-block; width: 8px; height: 8px; border-radius: 2px; margin-right: 5px; }
.proj-usage { margin-bottom: 8px; font-size: 13px; }
.proj-usage .bar { height: 4px; border-radius: 2px; background: var(--panel-2); margin-top: 3px; }
.proj-usage .bar div { height: 100%; border-radius: 2px; background: var(--accent); opacity: 0.6; }
</style>
