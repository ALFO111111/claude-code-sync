<template>
  <div>
    <div class="card">
      <h3>{{ t('bindings.bound') }}</h3>
      <Skeleton v-if="!res.data" :lines="3" />
      <div v-else-if="!bound.length" class="muted">{{ t('bindings.noneBound') }}</div>
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
      <h3>{{ t('bindings.claimable') }}<Refreshing :on="res.loading && !!res.data" /></h3>
      <!-- 读云端失败时下面照样显示上次缓存的结果，这里提一句 -->
      <div v-if="remoteError" class="err-box">{{ t('bindings.remoteError', { err: remoteError }) }}</div>
      <Skeleton v-if="!res.data" :lines="2" />
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
      <div class="note">{{ t('bindings.claimHint') }}</div>
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
      <div class="note">{{ t('bindings.restoreHint') }}</div>
    </div>

    <Notice :notice="notice" />
  </div>
</template>

<script setup>
import { ref, computed, onActivated } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api'
import { kindLabel } from '@/utils/format'
import { useNotice } from '@/composables/useNotice'
import { resources } from '@/stores/data'
import Notice from '@/components/Notice.vue'
import Skeleton from '@/components/Skeleton.vue'
import Refreshing from '@/components/Refreshing.vue'

const { t } = useI18n()
// 数据来自全局缓存（stores/data.js）：启动时已在后台拉好；再进这页先显示上次的，后台悄悄刷新
const res = resources.projects
const bound = computed(() => res.data?.bound || [])
const claimable = computed(() => res.data?.claimable || [])
const ignored = computed(() => res.data?.ignored || [])
const remoteError = computed(() => res.data?.remoteError || res.error)
const busy = ref(false) // 操作要顺带跑一轮同步，期间按钮禁用，免得连点
const { notice, ok, fail } = useNotice()

const refresh = () => res.refresh()

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

// 每次进这页：30 秒内拉过就不重复拉（同步结束时缓存会自动刷新）
onActivated(() => res.load())
</script>

<style scoped>
/* 项目 ID 很长：按窗口宽度截断（全屏时显示得更多），鼠标悬停看全 */
.pid { font-size: 12px; max-width: 18vw; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
/* 操作列：表头和按钮都居中（按钮有内边距，靠左/靠右都会和表头错开） */
.ops { text-align: center; }
</style>
