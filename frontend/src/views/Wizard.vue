<template>
  <div class="wizard">
    <div class="card">
      <div class="wizard-head">
        <h3 style="font-size:18px">{{ t('wizard.welcome') }}</h3>
        <LangSwitch />
      </div>
      <div class="steps">
        <div v-for="i in STEPS" :key="i" class="dot" :class="{ on: step >= i }"></div>
      </div>

      <!-- 第 1 步：设备名 -->
      <template v-if="step === 1">
        <p>{{ t('wizard.step1') }}</p>
        <div class="form-row">
          <label>{{ t('wizard.deviceName') }}</label>
          <input v-model="deviceName" :placeholder="t('settings.deviceExample')" />
        </div>
      </template>

      <!-- 第 2 步：选服务商 -->
      <template v-if="step === 2">
        <p>{{ t('wizard.step2') }}</p>
        <div class="provider-list">
          <div
            v-for="p in PROVIDERS" :key="p.value"
            class="provider" :class="{ on: st.provider === p.value }"
            @click="applyProvider(st, p.value)"
          >
            <div class="name">{{ t(`storage.providers.${p.value}.name`) }}</div>
            <div class="desc">{{ t(`storage.providers.${p.value}.desc`) }}</div>
          </div>
        </div>
        <p class="muted" style="font-size:13px">{{ t('wizard.step2Hint') }}</p>
      </template>

      <!-- 第 3 步：存储参数 -->
      <template v-if="step === 3">
        <p><b>{{ t(`storage.providers.${st.provider}.name`) }}</b></p>
        <StorageForm :st="st" :show-provider="false" />
        <button @click="testConn" :disabled="busy || !storageOK">{{ busy ? t('common.testing') : t('common.testConn') }}</button>
        <span v-if="testOk" class="ok" style="margin-left:10px">{{ t('common.connOk') }}</span>
      </template>

      <!-- 第 4 步：口令 -->
      <template v-if="step === 4">
        <i18n-t keypath="wizard.step4" tag="p">
          <template #same><b>{{ t('wizard.step4Same') }}</b></template>
        </i18n-t>
        <div class="form-row"><label>{{ t('wizard.pass') }}</label><input v-model="pass1" type="password" /></div>
        <div class="form-row"><label>{{ t('wizard.pass2') }}</label><input v-model="pass2" type="password" /></div>
        <div v-if="pass1 && pass2 && pass1 !== pass2" class="err">{{ t('wizard.mismatch') }}</div>
        <p class="muted">{{ t('wizard.finishHint') }}</p>
      </template>

      <div v-if="err" class="err" style="margin-top:10px">{{ err }}</div>

      <div class="actions">
        <button v-if="step > 1" @click="step--" :disabled="busy">{{ t('wizard.prev') }}</button>
        <span v-else></span>
        <button v-if="step < STEPS" class="primary" @click="next" :disabled="!canNext">{{ t('wizard.next') }}</button>
        <button v-else class="primary" @click="finish" :disabled="busy || !canNext">{{ busy ? t('wizard.initializing') : t('wizard.finish') }}</button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api'
import { languageSetting } from '@/i18n'
import { PROVIDERS, DEFAULT_STORAGE, applyProvider, storageReady } from '@/utils/storageTypes'
import LangSwitch from '@/components/LangSwitch.vue'
import StorageForm from '@/components/StorageForm.vue'

const STEPS = 4
const emit = defineEmits(['done'])
const { t } = useI18n()
const step = ref(1)
const deviceName = ref('')
const st = ref({ ...DEFAULT_STORAGE })
const pass1 = ref('')
const pass2 = ref('')
const busy = ref(false)
const err = ref('')
const testOk = ref(false)

const storageOK = computed(() => storageReady(st.value) && !!st.value.accessKeySecret)

const canNext = computed(() => {
  switch (step.value) {
    case 1: return !!deviceName.value.trim()
    case 2: return !!st.value.provider
    case 3: return storageOK.value
    case 4: return !!pass1.value && pass1.value === pass2.value
  }
  return true
})

async function testConn() {
  busy.value = true
  err.value = ''
  testOk.value = false
  try {
    await api.TestStorage(st.value)
    testOk.value = true
  } catch (e) {
    err.value = String(e)
  } finally {
    busy.value = false
  }
}

function next() {
  err.value = ''
  testOk.value = false
  step.value++
}

// finish 初始化（后端顺带装好 hooks 和定时任务）→ 立即同步一次
async function finish() {
  busy.value = true
  err.value = ''
  try {
    await api.Initialize({
      deviceName: deviceName.value.trim(),
      storage: st.value,
      passphrase: pass1.value,
      language: languageSetting.value, // 向导里选的语言一起存进配置
    })
    await api.SyncNow()
    emit('done')
  } catch (e) {
    err.value = String(e)
  } finally {
    busy.value = false
  }
}

onMounted(async () => {
  // 取不到本机用户名时是空串：不预填，只显示灰色示例
  deviceName.value = await api.DefaultDeviceName()
})
</script>
