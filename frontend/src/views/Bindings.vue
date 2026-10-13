<template>
  <div>
    <div class="card">
      <h3>{{ t('bindings.bound') }}</h3>
      <div v-if="!bound.length" class="muted">{{ t('bindings.noneBound') }}</div>
      <table v-else>
        <thead>
          <tr>
            <th>{{ t('bindings.colType') }}</th><th>{{ t('bindings.colProject') }}</th><th>{{ t('bindings.colLocalPath') }}</th>
            <th>{{ t('bindings.colPid') }}</th><th class="ops">{{ t('bindings.colOps') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="b in bound" :key="b.pid">
            <td><span class="badge">{{ kindLabel(b.kind) }}</span></td>
            <td>{{ b.display }}</td>
            <td>{{ b.root }}</td>
            <td class="muted pid" :title="b.pid">{{ b.pid }}</td>
            <td class="ops"><button class="sm ghost danger" :disabled="busy" @click="unbind(b)">{{ t('bindings.stopSync') }}</button></td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="card">
      <h3>{{ t('bindings.claimable') }}</h3>
      <div v-if="remoteError" class="err">{{ t('bindings.remoteError', { err: remoteError }) }}</div>
      <div v-else-if="!claimable.length" class="muted">{{ t('bindings.noClaimable') }}</div>
      <table v-else>
        <thead>
          <tr>
            <th>{{ t('bindings.colProject') }}</th><th>{{ t('bindings.colDevice') }}</th>
            <th>{{ t('bindings.colTheirPath') }}</th><th class="ops">{{ t('bindings.colOps') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="c in claimable" :key="c.pid + c.deviceId">
            <td>{{ c.display || c.pid }}</td>
            <td>{{ c.device }}</td>
            <td class="muted">{{ c.root }}</td>
            <td class="ops">
              <button class="sm primary" :disabled="busy" @click="claim(c)">{{ t('bindings.claim') }}</button>
              <button class="sm ghost danger" :disabled="busy" @click="ignore(c)">{{ t('bindings.ignore') }}</button>
            </td>
          </tr>
        </tbody>
      </table>
      <div class="muted" style="margin-top:8px; font-size:13px">{{ t('bindings.claimHint') }}</div>
    </div>

    <div class="card" v-if="ignored.length">
      <h3>{{ t('bindings.ignored') }}</h3>
      <table>
        <thead>
          <tr>
            <th>{{ t('bindings.colType') }}</th><th>{{ t('bindings.colProject') }}</th>
            <th>{{ t('bindings.colPid') }}</th><th class="ops">{{ t('bindings.colOps') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="g in ignored" :key="g.pid">
            <td><span class="badge">{{ kindLabel(g.kind) }}</span></td>
            <td>{{ g.display || '—' }}</td>
            <td class="muted pid" :title="g.pid">{{ g.pid }}</td>
            <td class="ops"><button class="sm" :disabled="busy" @click="unignore(g)">{{ t('bindings.restore') }}</button></td>
          </tr>
        </tbody>
      </table>
      <div class="muted" style="margin-top:8px; font-size:13px">{{ t('bindings.restoreHint') }}</div>
    </div>

    <Notice :notice="notice" />
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api'
import { kindLabel } from '@/utils/format'
import { useNotice } from '@/composables/useNotice'
import Notice from '@/components/Notice.vue'

const { t } = useI18n()
const bound = ref([])
const claimable = ref([])
const ignored = ref([])
const remoteError = ref('')
const busy = ref(false) // 操作要顺带跑一轮同步，期间按钮禁用，免得连点
const { notice, ok, fail } = useNotice()

async function refresh() {
  try {
    const ov = await api.ListProjects()
    bound.value = ov.bound
    claimable.value = ov.claimable
    ignored.value = ov.ignored
    remoteError.value = ov.remoteError || ''
  } catch (e) {
    fail(e)
  }
}

// act 执行一个会顺带同步的操作。failKey 是失败提示的文字 key（带 {err} 参数）。
async function act(fn, okText, failKey) {
  busy.value = true
  try {
    await fn()
    ok(okText)
  } catch (e) {
    fail(t(failKey, { err: String(e) }))
  } finally {
    busy.value = false
  }
  refresh()
}

const nameOf = (p) => p.display || p.pid

async function claim(c) {
  const dir = await api.PickDirectory()
  if (!dir) return
  act(() => api.ClaimProject(c.pid, dir), t('bindings.claimed', { name: nameOf(c), dir }), 'bindings.claimFailed')
}

function ignore(c) {
  act(() => api.IgnoreProject(c.pid), t('bindings.ignoredOk', { name: nameOf(c) }), 'bindings.opFailed')
}

async function unbind(b) {
  const yes = await api.Confirm(t('bindings.stopTitle'), t('bindings.stopConfirm', { name: nameOf(b) }))
  if (!yes) return
  act(() => api.IgnoreProject(b.pid), t('bindings.stopped', { name: nameOf(b) }), 'bindings.opFailed')
}

function unignore(g) {
  act(() => api.UnignoreProject(g.pid), t('bindings.restored', { name: nameOf(g) }), 'bindings.opFailed')
}

onMounted(refresh)
</script>

<style scoped>
/* 项目 ID 很长：截断显示，鼠标悬停看全 */
.pid { font-size: 12px; max-width: 220px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
/* 操作列：按内容收紧宽度、表头和按钮都居中（按钮有内边距，靠左/靠右都会和表头错开）、不折行 */
.ops { width: 1%; white-space: nowrap; text-align: center; }
.ops button + button { margin-left: 6px; }
</style>
