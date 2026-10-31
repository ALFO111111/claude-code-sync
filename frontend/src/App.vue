<template>
  <Wizard v-if="!initialized" @done="boot" />
  <div v-else class="layout">
    <aside class="sidebar">
      <div class="logo"><img :src="logo" alt="" /><span>claude<b>-sync</b></span></div>
      <div
        v-for="key in PAGES" :key="key"
        class="nav-item" :class="{ active: nav.page === key }"
        @click="go(key)"
      >
        <NavIcon :name="key" /> {{ t(`nav.${key}`) }}
        <span v-if="key === 'home' && syncState.last?.error" class="dot" :title="t('common.lastSyncFailed')"></span>
        <span v-if="key === 'conflicts' && conflictCount" class="count">{{ conflictCount }}</span>
        <span v-if="key === 'settings' && showBadge" class="dot" :title="t('about.newBadge', { v: upd.info.latest })"></span>
      </div>
      <div class="foot">
        <template v-if="syncState.running">
          <span class="spin">↻</span> {{ t('common.syncing') }}
          <span v-if="syncState.total">{{ syncState.done }}/{{ syncState.total }}</span>
        </template>
      </div>
    </aside>
    <main ref="main" class="content">
      <!-- 切走的页面不销毁：切回来时滚动位置、展开的内容、没保存的输入都还在，也不用重新加载 -->
      <KeepAlive>
        <component :is="VIEWS[nav.page]" />
      </KeepAlive>
    </main>
  </div>
</template>

<script setup>
import { ref, computed, watch, nextTick, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api'
import { syncState, listenSync } from '@/stores/sync'
import { nav, go, PAGES } from '@/stores/nav'
import { startData, resources } from '@/stores/data'
import { upd, showBadge, loadUpdate } from '@/stores/update'
import Wizard from '@/views/Wizard.vue'
import Home from '@/views/Home.vue'
import Sessions from '@/views/Sessions.vue'
import Bindings from '@/views/Bindings.vue'
import Backups from '@/views/Backups.vue'
import Conflicts from '@/views/Conflicts.vue'
import Settings from '@/views/Settings.vue'
import Logs from '@/views/Logs.vue'
import NavIcon from '@/components/NavIcon.vue'
import logo from '@/assets/logo.png'

const { t } = useI18n()
const initialized = ref(true)
const VIEWS = { home: Home, sessions: Sessions, bindings: Bindings, backups: Backups, conflicts: Conflicts, settings: Settings, logs: Logs }
// 侧边栏"冲突"的数字：首页状态里带着，不用另外请求
const conflictCount = computed(() => resources.status.data?.conflicts || 0)

async function boot() {
  listenSync() // 先挂监听再取快照，避免漏掉间隙里的"同步结束"事件
  const s = await api.GetStatus()
  initialized.value = s.initialized
  syncState.running = s.syncing
  if (s.initialized) startData(s) // 后台预取首页、项目页的数据，第一次点进去就不用等
  loadUpdate() // 检查更新的状态（后端启动后自动检查，有新版本推 "update" 事件）
}

// 每页记住自己的滚动位置（页面都在同一个可滚动区域里，切页时不记的话会带着上一页的位置）
const main = ref(null)
const scrollOf = {}
watch(() => nav.page, async (to, from) => {
  if (main.value) scrollOf[from] = main.value.scrollTop
  await nextTick()
  if (main.value) main.value.scrollTop = scrollOf[to] || 0
}, { flush: 'pre' })

onMounted(boot)
</script>
