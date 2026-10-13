<template>
  <div>
    <div style="display:flex; gap:8px; margin-bottom:12px; align-items:center">
      <button @click="refresh">{{ t('logs.refresh') }}</button>
      <button class="danger" @click="clear">{{ t('logs.clear') }}</button>
      <span class="muted" style="font-size:12px">{{ logPath }}</span>
    </div>
    <div v-if="err" class="err" style="margin-bottom:8px">{{ err }}</div>
    <div ref="view" class="log-view">{{ log || t('logs.empty') }}</div>
  </div>
</template>

<script setup>
import { ref, nextTick, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api'

const { t } = useI18n()
const log = ref('')
const logPath = ref('')
const err = ref('')
const view = ref(null)

// 新日志在最后面：加载完滚到底，一眼看到最近发生了什么
async function refresh() {
  try {
    const v = await api.ReadLog()
    log.value = v.text
    logPath.value = v.path
    err.value = ''
    await nextTick()
    if (view.value) view.value.scrollTop = view.value.scrollHeight
  } catch (e) {
    err.value = String(e)
  }
}

async function clear() {
  try {
    await api.ClearLog()
  } catch (e) {
    err.value = String(e)
  }
  refresh()
}

onMounted(refresh)
</script>
