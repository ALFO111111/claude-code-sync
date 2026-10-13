<template>
  <div>
    <div class="card" style="display:flex; align-items:center; gap:20px">
      <div style="flex:1">
        <h3>{{ t('home.status') }}</h3>
        <div v-if="status">
          <div class="muted" :title="status.lastSync">
            {{ t('home.lastSync', { when: fromNow(status.lastSyncAt, now) }) }}
            <span v-if="status.lastSync">({{ status.lastSync }})</span>
          </div>
          <div style="margin-top:6px">
            <span class="badge">{{ t('home.pushed', { n: status.stats.pushed }) }}</span>
            <span class="badge">{{ t('home.pulled', { n: status.stats.pulled }) }}</span>
            <span class="badge">{{ t('home.merged', { n: status.stats.merged }) }}</span>
            <span class="badge">{{ t('home.forked', { n: status.stats.forked }) }}</span>
            <span class="badge">{{ t('home.history', { n: status.stats.history }) }}</span>
            <span class="badge" :class="status.stats.errors ? 'warn' : 'ok'">
              {{ t('home.errors', { n: status.stats.errors }) }}
            </span>
          </div>
        </div>
      </div>
      <button class="primary" style="font-size:16px; padding:12px 28px" :disabled="syncState.running" @click="api.SyncNow()">
        <span v-if="syncState.running" class="spin">↻</span> {{ syncState.running ? t('common.syncing') : t('common.syncNow') }}
      </button>
    </div>

    <!-- 只在这次同步真有变化或出错时提示；自动同步每轮"什么都没变"不刷屏 -->
    <Notice :notice="result" />

    <div class="card" v-if="status">
      <h3>{{ t('home.myDevices') }}</h3>
      <table>
        <tr>
          <td style="width:40%"><b>{{ status.deviceName }}</b> <span class="badge ok">{{ t('home.thisDevice') }}</span></td>
          <td class="muted">{{ t('home.projects', status.projects) }}</td>
          <td class="muted" style="font-size:12px">{{ status.deviceId }}</td>
        </tr>
        <tr v-for="d in status.devices" :key="d.id">
          <td>{{ d.name }}</td>
          <td class="muted">{{ t('home.projects', d.projects) }}</td>
          <td class="muted" style="font-size:12px">{{ d.id }}</td>
        </tr>
      </table>
      <div v-if="!status.devices.length" class="muted" style="margin-top:8px; font-size:13px">
        {{ t('home.noOtherDevices') }}
      </div>
    </div>

    <!-- 自动同步平时不展示；只有没装好时才提示 -->
    <div class="card" v-if="status && !status.autoSync">
      <h3>{{ t('home.autoSyncNotReady') }}</h3>
      <div class="err">{{ status.autoSyncIssue }}</div>
      <div v-if="!status.sandboxed" style="margin-top:10px; display:flex; align-items:center; gap:10px">
        <button class="primary" :disabled="repairing" @click="repair">{{ repairing ? t('home.repairing') : t('home.repair') }}</button>
        <span class="muted" style="font-size:13px">{{ t('home.repairHint') }}</span>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api'
import { syncState } from '@/stores/sync'
import { fromNow } from '@/utils/format'
import Notice from '@/components/Notice.vue'

const { t, locale } = useI18n()
const status = ref(null)
const repairing = ref(false)
const now = ref(Date.now()) // 每 30 秒走一次，让"3 分钟前"自己更新
let timer

async function refresh() {
  status.value = await api.GetStatus()
}

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

// 每次同步结束刷新状态；切换语言后也刷新（"自动同步没就绪"的原因是后端按语言生成的）
watch(() => syncState.running, (running) => { if (!running) refresh() })
watch(locale, refresh)

onMounted(() => {
  refresh()
  timer = setInterval(() => { now.value = Date.now() }, 30000)
})
onUnmounted(() => clearInterval(timer))
</script>
