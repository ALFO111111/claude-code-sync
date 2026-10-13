<template>
  <Wizard v-if="!initialized" @done="boot" />
  <div v-else class="layout">
    <aside class="sidebar">
      <div class="logo"><img :src="logo" alt="" />claude<span>-sync</span></div>
      <div
        v-for="key in NAV" :key="key"
        class="nav-item" :class="{ active: page === key }"
        @click="page = key"
      ><NavIcon :name="key" /> {{ t(`nav.${key}`) }}</div>
      <div class="foot">
        <span v-if="syncState.running" class="spin">↻</span>
        {{ syncState.running ? ' ' + t('common.syncing') : ' ' }}
      </div>
    </aside>
    <main class="content">
      <Home v-if="page === 'home'" />
      <Sessions v-else-if="page === 'sessions'" />
      <Bindings v-else-if="page === 'bindings'" />
      <Settings v-else-if="page === 'settings'" />
      <Logs v-else-if="page === 'logs'" />
    </main>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api'
import { syncState, listenSync } from '@/stores/sync'
import Wizard from '@/views/Wizard.vue'
import Home from '@/views/Home.vue'
import Sessions from '@/views/Sessions.vue'
import Bindings from '@/views/Bindings.vue'
import Settings from '@/views/Settings.vue'
import Logs from '@/views/Logs.vue'
import NavIcon from '@/components/NavIcon.vue'
import logo from '@/assets/logo.png'

const { t } = useI18n()
const initialized = ref(true)
const page = ref('home')

const NAV = ['home', 'sessions', 'bindings', 'settings', 'logs']

async function boot() {
  listenSync() // 先挂监听再取快照，避免漏掉间隙里的"同步结束"事件
  const s = await api.GetStatus()
  initialized.value = s.initialized
  syncState.running = s.syncing
}

onMounted(boot)
</script>
