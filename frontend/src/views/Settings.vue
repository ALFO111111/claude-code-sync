<template>
  <div style="max-width:640px">
    <div class="card">
      <h3>{{ t('settings.device') }}</h3>
      <div class="form-row">
        <label>{{ t('settings.deviceName') }}</label>
        <input v-model="form.deviceName" :placeholder="t('settings.deviceExample')" />
      </div>
      <div class="form-row">
        <label>{{ t('lang.label') }}</label>
        <LangSwitch />
      </div>
    </div>

    <div class="card">
      <h3>{{ t('settings.storage') }}</h3>
      <StorageForm v-if="st.type === 's3'" :st="st" :has-secret="form.hasSecret" />
      <!-- 本地文件夹只给开发测试用，界面不提供，但已保存的要能看到、能改 -->
      <div v-else class="form-row">
        <label>{{ t('storage.testOnly', { type: st.type }) }} · {{ t('storage.path') }}</label>
        <input v-model.trim="st.path" />
      </div>
      <div class="form-row"><label>{{ t('storage.prefix') }}</label><input v-model.trim="st.prefix" placeholder="v1/" /></div>
      <div v-if="locationChanged" class="warn-box">
        <i18n-t keypath="settings.locationWarn" tag="span">
          <template #fields>{{ changedFields.join(t('common.listSep')) }}</template>
          <template #repo><b>{{ t('settings.newRepo') }}</b></template>
        </i18n-t>
      </div>
      <button @click="testConn" :disabled="busy">{{ t('common.testConn') }}</button>
      <div class="muted" style="margin-top:8px; font-size:13px">{{ t('settings.passHint') }}</div>
    </div>

    <div class="card">
      <h3>{{ t('settings.scanRoots') }}</h3>
      <div class="form-row">
        <label>{{ t('settings.scanRootsHint') }}</label>
        <textarea v-model="scanRoots" rows="3" placeholder="D:\code"></textarea>
      </div>
    </div>

    <div style="margin-bottom:16px">
      <button class="primary" @click="save" :disabled="busy">{{ t('settings.save') }}</button>
      <span class="muted" style="margin-left:8px; font-size:13px">{{ t('settings.saveHint') }}</span>
    </div>

    <Notice :notice="notice" />
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api'
import { useNotice } from '@/composables/useNotice'
import { BLANK_STORAGE } from '@/utils/storageTypes'
import Notice from '@/components/Notice.vue'
import LangSwitch from '@/components/LangSwitch.vue'
import StorageForm from '@/components/StorageForm.vue'

const { t } = useI18n()
const form = ref({ deviceName: '', hasSecret: false })
const st = ref({ ...BLANK_STORAGE })
const scanRoots = ref('')
const busy = ref(false)
const { notice, ok, fail, clear } = useNotice()

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

onMounted(loadForm)
</script>
