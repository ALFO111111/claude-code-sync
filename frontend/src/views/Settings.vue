<template>
  <div>
    <!-- 窗口窄时一列（按 order 排）；窗口宽时左右两列，各列卡片自然往下排，不留空洞 -->
    <div class="settings">
      <div class="col">
        <!-- 改了马上生效的开关：不用点下面的"保存" -->
        <div class="card general">
          <div class="card-head"><h3>{{ t('settings.general') }}</h3><span class="muted small">{{ t('settings.instant') }}</span></div>
          <div class="setting">
            <div><div class="label">{{ t('lang.label') }}</div></div>
            <LangSwitch />
          </div>
          <div class="setting">
            <div><div class="label">{{ t('settings.theme') }}</div></div>
            <div class="segmented">
              <button v-for="o in THEMES" :key="o" :class="{ on: themeSetting === o }" @click="changeTheme(o)">{{ t(`settings.themes.${o || 'system'}`) }}</button>
            </div>
          </div>
          <div class="setting">
            <div>
              <div class="label">{{ t('settings.autostart') }}</div>
              <div class="desc">{{ autostart.available ? t('settings.autostartDesc') : t('settings.autostartSandbox') }}</div>
            </div>
            <label class="switch">
              <input type="checkbox" :checked="autostart.enabled" :disabled="!autostart.available || autoBusy" @change="toggleAutostart($event.target.checked)" />
              <span></span>
            </label>
          </div>
          <div class="setting">
            <div>
              <div class="label">{{ t('settings.autoUpdate') }}</div>
              <div class="desc">{{ t('settings.autoUpdateDesc') }}</div>
            </div>
            <label class="switch">
              <input type="checkbox" :checked="upd.auto" @change="toggleAutoUpdate($event.target.checked)" />
              <span></span>
            </label>
          </div>
        </div>

        <!-- 除了会话之外还同步哪些东西：改了马上生效 -->
        <div class="card content">
          <div class="card-head"><h3>{{ t('content.title') }}<Refreshing :on="contentRes.loading && !!content" /></h3><span class="muted small">{{ t('settings.instant') }}</span></div>
          <div class="muted small">{{ t('content.intro') }}</div>
          <!-- CC Switch 也开着自动同步：规则、技能、MCP 可能被两边同时同步，让用户选一个负责 -->
          <div v-if="content?.pending.length" class="warn-box mt">{{ t('content.ccWarn') }}</div>
          <div v-if="!content" class="mt"><Skeleton :lines="4" /></div>
          <template v-else>
            <div v-for="group in contentGroups" :key="group.tool" class="content-group">
              <h4 class="content-tool">{{ t(`content.tool.${group.tool}`) }}</h4>
              <div v-for="it in group.items" :key="`${group.tool}:${it.category}`" class="setting">
                <div class="grow">
                  <div class="label">{{ contentLabel(group.tool, it.category) }}</div>
                  <div class="desc">{{ contentDesc(group.tool, it.category) }}</div>
                  <div v-if="group.tool === 'claude' && it.ccOverlap && content.ccSwitch === 'enabled' && !it.pending" class="desc">
                    {{ it.enabled ? t('content.ownerSync') : t('content.ownerCC') }}
                  </div>
                  <div v-if="group.tool === 'claude' && it.pending" class="row wrap owner">
                    <button class="sm primary" :disabled="contentBusy" @click="setOwner(it.category, 'sync')">{{ t('content.pickSync') }}</button>
                    <button class="sm" :disabled="contentBusy" @click="setOwner(it.category, 'ccswitch')">{{ t('content.pickCC') }}</button>
                  </div>
                </div>
                <label v-if="!it.pending" class="switch">
                  <input type="checkbox" :checked="it.enabled" :disabled="contentBusy" @change="toggleContent(group.tool, it.category, $event.target.checked)" />
                  <span></span>
                </label>
              </div>
            </div>
          </template>
        </div>

        <div class="card device">
          <h3>{{ t('settings.device') }}</h3>
          <div class="form-grid">
            <div class="form-row">
              <label>{{ t('settings.deviceName') }}</label>
              <input v-model="form.deviceName" :placeholder="t('settings.deviceExample')" />
            </div>
          </div>
        </div>

        <!-- 修改口令：影响所有设备，单独放、单独确认 -->
        <div class="card pass">
          <div class="card-head">
            <h3>{{ t('settings.passphrase') }}</h3>
            <button v-if="!passOpen" class="sm" @click="passOpen = true"><NavIcon name="key" :size="13" /> {{ t('settings.changePass') }}</button>
          </div>
          <div v-if="!passOpen" class="muted small">{{ t('settings.passDesc') }}</div>
          <template v-else>
            <div class="warn-box">{{ t('settings.passWarn') }}</div>
            <div class="form-grid">
              <div class="form-row"><label>{{ t('settings.passOld') }}</label><input v-model="pass.old" type="password" :disabled="rekeying" /></div>
              <div class="form-row"><label>{{ t('settings.passNew') }}</label><input v-model="pass.new1" type="password" :disabled="rekeying" /></div>
              <div class="form-row"><label>{{ t('settings.passNew2') }}</label><input v-model="pass.new2" type="password" :disabled="rekeying" /></div>
            </div>
            <div v-if="pass.new1 && pass.new2 && pass.new1 !== pass.new2" class="err small">{{ t('wizard.mismatch') }}</div>
            <template v-if="rekeying">
              <div class="progress mt"><div :style="{ width: rekeyPct + '%' }"></div></div>
              <div class="muted small mt">{{ t('settings.rekeying', { done: rekey.done, total: rekey.total }) }}</div>
            </template>
            <div v-if="passErr" class="err-box mt">{{ passErr }}</div>
            <div class="row mt">
              <button class="primary" :disabled="!canChangePass || rekeying" @click="changePass">{{ rekeying ? t('common.working') : t('settings.changePassGo') }}</button>
              <button class="ghost" :disabled="rekeying" @click="closePass">{{ t('common.cancel') }}</button>
            </div>
          </template>
          <div v-if="passOk" class="ok mt">{{ t('settings.passChanged') }}</div>
        </div>
      </div>

      <div class="col">
        <div class="card storage">
          <h3>{{ t('settings.storage') }}</h3>
          <StorageForm v-if="st.type === 's3'" :st="st" :has-secret="form.hasSecret" />
          <!-- 本地文件夹只给开发测试用，界面不提供，但已保存的要能看到、能改 -->
          <div v-else class="form-row">
            <label>{{ t('storage.testOnly', { type: st.type }) }} · {{ t('storage.path') }}</label>
            <input v-model.trim="st.path" />
          </div>
          <div class="form-grid">
            <div class="form-row"><label>{{ t('storage.prefix') }}</label><input v-model.trim="st.prefix" placeholder="v1/" /></div>
          </div>
          <div v-if="locationChanged" class="warn-box">
            <i18n-t keypath="settings.locationWarn" tag="span">
              <template #fields>{{ changedFields.join(t('common.listSep')) }}</template>
              <template #repo><b>{{ t('settings.newRepo') }}</b></template>
            </i18n-t>
          </div>
          <button @click="testConn" :disabled="busy">{{ t('common.testConn') }}</button>
        </div>

        <div class="card scan">
          <h3>{{ t('settings.scanRoots') }}</h3>
          <div class="form-row">
            <label>{{ t('settings.scanRootsHint') }}</label>
            <textarea v-model="scanRoots" rows="3" placeholder="D:\code"></textarea>
          </div>
        </div>

        <!-- 关于 + 检查更新（参照 CC Switch：一个按钮四种状态，有新版本时下面展开说明） -->
        <div class="card about">
          <div class="about-head">
            <img :src="logo" alt="" class="about-logo" />
            <div class="grow">
              <div class="about-name">claude-code-sync</div>
              <div class="muted small">{{ upd.current && upd.current !== 'dev' ? t('about.version', { v: upd.current }) : t('about.devBuild') }}</div>
            </div>
            <button :class="{ primary: hasUpdate }" :disabled="upd.checking || upd.installing" @click="onUpdateClick">
              <span v-if="upd.checking || upd.installing" class="spin">↻</span>
              {{ upd.installing ? t('about.updating') : hasUpdate ? t('about.updateTo', { v: upd.info.latest }) : upd.checking ? t('about.checking') : t('about.check') }}
            </button>
          </div>
          <div v-if="hasUpdate" class="update-box">
            <div class="row wrap">
              <b>{{ t('about.available', { v: upd.info.latest }) }}</b>
              <span v-if="upd.info.published" class="muted small">{{ t('about.published', { date: upd.info.published.slice(0, 10) }) }}</span>
            </div>
            <div v-if="notes" class="notes">{{ notes }}</div>
            <template v-if="upd.installing">
              <div class="progress mt"><div :style="{ width: dlPct + '%' }"></div></div>
              <div class="muted small mt">
                {{ dlPct >= 100 ? t('about.installing') : t('about.downloading', { done: fmtSize(upd.progress.done), total: fmtSize(upd.progress.total) }) }}
              </div>
            </template>
            <div v-if="!upd.info.asset" class="muted small mt">{{ t('about.manual') }}</div>
            <div class="row wrap mt">
              <button class="sm ghost" @click="BrowserOpenURL(upd.info.url)">{{ t('about.notes') }}</button>
              <button v-if="upd.info.latest !== upd.skipped" class="sm ghost" :disabled="upd.installing" @click="skip">{{ t('about.skip') }}</button>
              <span v-else class="muted small">{{ t('about.skipped') }}</span>
            </div>
          </div>
          <div class="row wrap links">
            <button class="sm ghost" @click="BrowserOpenURL(REPO)">{{ t('about.github') }}</button>
            <button class="sm ghost" @click="BrowserOpenURL(REPO + '/releases')">{{ t('about.releases') }}</button>
          </div>
        </div>
      </div>
    </div>

    <!-- 保存条贴在窗口底部：页面再长、窗口再大，都不用找它在哪 -->
    <div class="save-bar">
      <button class="primary" @click="save" :disabled="busy">{{ t('settings.save') }}</button>
      <span class="muted small grow">{{ t('settings.saveHint') }}</span>
    </div>
    <Notice :notice="notice" />
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, onActivated } from 'vue'
import { useI18n } from 'vue-i18n'
import { api, EventsOn, BrowserOpenURL } from '@/api'
import { useNotice } from '@/composables/useNotice'
import { BLANK_STORAGE } from '@/utils/storageTypes'
import { themeSetting, changeTheme } from '@/utils/theme'
import { contentLabel, contentDesc, fmtSize, releaseNotes } from '@/utils/format'
import { upd, hasUpdate, checkUpdate, installUpdate, skipUpdate, setAutoUpdate } from '@/stores/update'
import logo from '@/assets/logo.png'
import Notice from '@/components/Notice.vue'
import LangSwitch from '@/components/LangSwitch.vue'
import StorageForm from '@/components/StorageForm.vue'
import NavIcon from '@/components/NavIcon.vue'
import Skeleton from '@/components/Skeleton.vue'
import Refreshing from '@/components/Refreshing.vue'
import { resources } from '@/stores/data'

const { t, locale } = useI18n()
const form = ref({ deviceName: '', hasSecret: false })
const st = ref({ ...BLANK_STORAGE })
const scanRoots = ref('')
const busy = ref(false)
const { notice, ok, fail, clear } = useNotice()

const THEMES = ['', 'light', 'dark']

// ---------- 开机自启 ----------
const autostart = ref({ enabled: false, available: false })
const autoBusy = ref(false)
async function toggleAutostart(on) {
  autoBusy.value = true
  try {
    autostart.value = await api.SetAutostart(on)
  } catch (e) {
    fail(String(e))
    autostart.value = await api.GetAutostart()
  } finally {
    autoBusy.value = false
  }
}

// ---------- 检查更新 ----------
const REPO = 'https://github.com/ALFO111111/claude-code-sync'
const notes = computed(() => releaseNotes(upd.info?.notes, locale.value))
const dlPct = computed(() => (upd.progress.total ? Math.min(100, Math.round((upd.progress.done / upd.progress.total) * 100)) : 0))

async function onUpdateClick() {
  clear()
  try {
    if (!hasUpdate.value) {
      const info = await checkUpdate()
      if (info.dev) ok(t('about.devLatest', { v: info.latest }))
      else if (!info.available) ok(t('about.upToDate'))
      return
    }
    if (!upd.info.asset) {
      BrowserOpenURL(upd.info.url) // 这个系统没有安装包：去发布页手动下载
      return
    }
    await installUpdate() // 成功的话程序会自己退出、重新打开新版本
  } catch (e) {
    fail(t(hasUpdate.value ? 'about.updateFailed' : 'about.checkFailed', { err: String(e) }))
  }
}

async function skip() {
  try {
    await skipUpdate()
  } catch (e) {
    fail(String(e))
  }
}

async function toggleAutoUpdate(on) {
  try {
    await setAutoUpdate(on)
  } catch (e) {
    fail(String(e))
  }
}

// ---------- 同步内容 ----------
const contentRes = resources.content
const content = computed(() => contentRes.data)
const contentGroups = computed(() => ['claude', 'codex']
  .map((tool) => ({
    tool,
    items: (content.value?.items || []).filter((it) => (it.tool || 'claude') === tool),
  }))
  .filter((group) => group.items.length))
const contentBusy = ref(false)
async function changeContent(fn) {
  contentBusy.value = true
  clear()
  try {
    await fn()
  } catch (e) {
    fail(String(e))
  } finally {
    contentBusy.value = false
    await contentRes.refresh()
    resources.status.refresh() // 首页的"待选择"提示
  }
}
const toggleContent = (tool, category, on) => changeContent(() => api.SetToolContentEnabled(tool, category, on))
const setOwner = (category, owner) => changeContent(() => api.SetContentOwner(category, owner))

// 决定"数据存在哪"的字段：改了任何一个都等于换了云端仓库（AccessKey 只是换钥匙、路径风格只是换访问方式，都不算）
const LOCATION_FIELDS = {
  type: 'settings.fieldType', provider: 'storage.provider', region: 'storage.region', bucket: 'storage.bucket',
  endpoint: 'storage.endpoint', prefix: 'storage.prefix', path: 'storage.path',
}
const saved = ref({})
const norm = (k, v) => {
  v = (v || '').trim()
  return k === 'prefix' && v && !v.endsWith('/') ? v + '/' : v || (k === 'prefix' ? 'v1/' : '')
}
const changedFields = computed(() =>
  Object.entries(LOCATION_FIELDS)
    .filter(([k]) => norm(k, st.value[k]) !== norm(k, saved.value[k]))
    .map(([, key]) => t(key))
)
const locationChanged = computed(() => changedFields.value.length > 0)

// loadForm 用已保存的设置填表单（只在打开页面和保存成功后调，避免冲掉没保存的输入）
async function loadForm() {
  const s = await api.GetSettings()
  form.value = s
  st.value = { ...BLANK_STORAGE, ...s.storage }
  saved.value = { ...st.value } // 和表单初始值比，免得空字段被误判成"改了"
  scanRoots.value = s.scanRoots.join('\n')
}

async function run(fn, okMsg) {
  busy.value = true
  clear()
  try {
    await fn()
    ok(okMsg)
    return true
  } catch (e) {
    fail(e)
    return false
  } finally {
    busy.value = false
  }
}

const testConn = () => run(() => api.TestStorage(st.value), t('common.connOk'))
const save = async () => {
  if (locationChanged.value) {
    const yes = await api.Confirm(t('settings.changeTitle'),
      t('settings.changeConfirm', { fields: changedFields.value.join(t('common.listSep')) }))
    if (!yes) return
  }
  const done = await run(() => api.SaveSettings({
    deviceName: form.value.deviceName,
    storage: st.value,
    scanRoots: scanRoots.value.split('\n'),
  }), t('settings.saved'))
  if (done) loadForm()
}

// ---------- 修改口令 ----------
const passOpen = ref(false)
const pass = ref({ old: '', new1: '', new2: '' })
const passErr = ref('')
const passOk = ref(false)
const rekeying = ref(false)
const rekey = ref({ done: 0, total: 0 })
const rekeyPct = computed(() => (rekey.value.total ? Math.round((rekey.value.done / rekey.value.total) * 100) : 0))
const canChangePass = computed(() => pass.value.old && pass.value.new1 && pass.value.new1 === pass.value.new2)
let offRekey

function closePass() {
  passOpen.value = false
  pass.value = { old: '', new1: '', new2: '' }
  passErr.value = ''
}

async function changePass() {
  const yes = await api.Confirm(t('settings.passConfirmTitle'), t('settings.passConfirm'))
  if (!yes) return
  rekeying.value = true
  passErr.value = ''
  passOk.value = false
  rekey.value = { done: 0, total: 0 }
  try {
    await api.ChangePassphrase(pass.value.old, pass.value.new1)
    closePass()
    passOk.value = true
  } catch (e) {
    passErr.value = String(e)
  } finally {
    rekeying.value = false
  }
}

// 页面切走不销毁（App.vue 的 KeepAlive）：表单只在第一次打开时读，切回来时没保存的输入还在
onMounted(() => {
  offRekey = EventsOn('rekey-progress', (p) => { rekey.value = p })
  loadForm()
})
// 开机自启可能被 uninstall 等改了，每次进来以注册表里实际的为准
onActivated(async () => {
  contentRes.refresh() // CC Switch 的自动同步开关可能刚在它那边改过
  autostart.value = await api.GetAutostart()
})
onUnmounted(() => offRekey?.())
</script>

<style scoped>
/* 窄：一列，卡片按 order 排（两个 .col 用 display: contents 拆开，卡片直接参与外层排序） */
.settings { display: flex; flex-direction: column; gap: 14px; }
.settings .col { display: contents; }
.settings .card { margin-bottom: 0; }
.general { order: 1; }
.content { order: 2; }
.device { order: 3; }
.storage { order: 4; }
.scan { order: 5; }
.pass { order: 6; }
.about { order: 7; }
.about-head { display: flex; align-items: center; gap: 12px; }
.about-logo { width: 40px; height: 40px; border-radius: 8px; }
.about-name { font-weight: 600; font-size: 15px; }
.update-box { margin-top: 12px; padding: 10px 12px; border: 1px solid var(--border); border-radius: 8px; }
.update-box .row { gap: 8px; align-items: baseline; }
.notes {
  margin-top: 6px; white-space: pre-line; font-size: 12.5px; color: var(--muted);
  display: -webkit-box; -webkit-line-clamp: 5; -webkit-box-orient: vertical; overflow: hidden;
}
.links { gap: 8px; margin-top: 10px; }
.owner { gap: 8px; margin-top: 6px; }
.content-group { margin-top: 12px; }
.content-group + .content-group { padding-top: 2px; border-top: 1px solid var(--border); }
.content-tool { margin: 0 0 4px; font-size: 13px; }
/* 宽：左右两列（侧边栏 180 + 边距后，内容区大约 950px 起），每列自己往下排 */
@media (min-width: 1180px) {
  .settings { flex-direction: row; align-items: flex-start; }
  .settings .col { display: flex; flex-direction: column; gap: 14px; flex: 1; min-width: 0; }
}
.save-bar {
  position: sticky;
  bottom: -20px; /* 抵消内容区的下内边距，贴住窗口底边 */
  display: flex;
  align-items: center;
  gap: 10px;
  margin: 14px 0;
  padding: 12px 0;
  background: var(--bg);
  border-top: 1px solid var(--border);
  z-index: 1;
}
</style>
