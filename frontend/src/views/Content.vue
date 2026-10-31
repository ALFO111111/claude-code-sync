<template>
  <div>
    <div class="card">
      <div class="card-head">
        <h3 class="row tool-title">
          <ToolIcon :tool="tool" :size="20" />{{ tool.name }} · {{ t('content.title') }}
          <Refreshing :on="contentRes.loading && !!content" />
        </h3>
        <span class="muted small">{{ t('settings.instant') }}</span>
      </div>
      <div class="muted small">{{ t('content.intro') }}</div>
      <div v-if="tool.installed === false" class="warn-box mt">{{ t('tools.notInstalledHint', { dir: tool.dir, name: tool.name }) }}</div>
      <!-- CC Switch 也开着自动同步：规则、技能、MCP 可能被两边同时同步，让用户选一个负责（只有 Claude 有这个问题） -->
      <div v-if="tool.id === 'claude' && content?.pending.length" class="warn-box mt">{{ t('content.ccWarn') }}</div>

      <div v-if="!content" class="mt"><Skeleton :lines="4" /></div>
      <div v-else-if="!items.length" class="empty mt">{{ t('content.none') }}</div>
      <div v-else class="mt">
        <div v-for="it in items" :key="it.category" class="setting">
          <div class="grow">
            <div class="label">{{ contentLabel(tool.id, it.category) }}</div>
            <div class="desc">{{ contentDesc(tool.id, it.category) }}</div>
            <div v-if="it.ccOverlap && content.ccSwitch === 'enabled' && !it.pending" class="desc">
              {{ it.enabled ? t('content.ownerSync') : t('content.ownerCC') }}
            </div>
            <div v-if="it.pending" class="row wrap owner">
              <button class="sm primary" :disabled="busy" @click="setOwner(it.category, 'sync')">{{ t('content.pickSync') }}</button>
              <button class="sm" :disabled="busy" @click="setOwner(it.category, 'ccswitch')">{{ t('content.pickCC') }}</button>
            </div>
          </div>
          <label v-if="!it.pending" class="switch">
            <input type="checkbox" :checked="it.enabled" :disabled="busy" @change="toggle(it.category, $event.target.checked)" />
            <span></span>
          </label>
        </div>
      </div>
    </div>
    <Notice :notice="notice" />
  </div>
</template>

<script setup>
// 同步内容：当前工具除了会话以外还同步哪些东西（全局规则、技能、MCP……），改了马上生效。
import { ref, computed, onActivated } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api'
import { resources } from '@/stores/data'
import { currentTool, toolOf } from '@/stores/tools'
import { contentLabel, contentDesc } from '@/utils/format'
import { useNotice } from '@/composables/useNotice'
import Notice from '@/components/Notice.vue'
import Skeleton from '@/components/Skeleton.vue'
import Refreshing from '@/components/Refreshing.vue'
import ToolIcon from '@/components/ToolIcon.vue'

const { t } = useI18n()
const { notice, fail, clear } = useNotice()
const contentRes = resources.content
const content = computed(() => contentRes.data)
const tool = currentTool
const items = computed(() => (content.value?.items || []).filter((it) => toolOf(it) === tool.value.id))

const busy = ref(false)
async function change(fn) {
  busy.value = true
  clear()
  try {
    await fn()
  } catch (e) {
    fail(String(e))
  } finally {
    busy.value = false
    await contentRes.refresh()
    resources.status.refresh() // 首页的"待选择"提示
    resources.tools.refresh() // 首页工具卡片上的"同步内容 x/y"
  }
}
const toggle = (category, on) => change(() => api.SetToolContentEnabled(tool.value.id, category, on))
const setOwner = (category, owner) => change(() => api.SetContentOwner(category, owner))

onActivated(() => contentRes.refresh()) // CC Switch 的自动同步开关可能刚在它那边改过
</script>

<style scoped>
.tool-title { gap: 8px; }
.owner { gap: 8px; margin-top: 6px; }
</style>
