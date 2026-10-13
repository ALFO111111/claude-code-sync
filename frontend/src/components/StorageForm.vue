<template>
  <div>
    <div v-if="showProvider" class="form-row">
      <label>{{ t('storage.provider') }}</label>
      <select :value="st.provider" @change="setProvider($event.target.value)">
        <option v-for="p in PROVIDERS" :key="p.value" :value="p.value">{{ t(`storage.providers.${p.value}.name`) }}</option>
      </select>
    </div>
    <div class="form-row">
      <label>{{ provider.region ? t('storage.region') : t('storage.regionOptional') }}</label>
      <input v-model.trim="st.region" :placeholder="provider.region || (st.provider === 'r2' ? 'auto' : 'us-east-1')" />
    </div>
    <div class="form-row">
      <label>{{ needsEndpoint(provider) ? t('storage.endpoint') : t('storage.endpointAuto') }}</label>
      <input v-model.trim="st.endpoint" :placeholder="endpointPlaceholder" />
    </div>
    <div class="form-row"><label>{{ t('storage.bucket') }}</label><input v-model.trim="st.bucket" /></div>
    <div class="form-row"><label>AccessKeyId</label><input v-model.trim="st.accessKeyId" /></div>
    <div class="form-row">
      <label>AccessKeySecret</label>
      <input v-model="st.accessKeySecret" type="password" :placeholder="hasSecret ? t('storage.secretKept') : ''" />
    </div>
    <div v-if="needsEndpoint(provider)" class="form-row">
      <label class="check"><input v-model="st.pathStyle" type="checkbox" /> {{ t('storage.pathStyle') }}</label>
    </div>
    <div class="muted" style="font-size:13px; margin-bottom:12px">
      <a href="#" @click.prevent="BrowserOpenURL(storageGuideURL(locale))">{{ t('storage.guide') }} ↗</a>
    </div>
  </div>
</template>

<script setup>
// 云端存储参数表单（S3 兼容）：向导第 3 步和设置页共用。直接改传进来的 st 对象。
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { BrowserOpenURL } from '@/api'
import { PROVIDERS, providerOf, needsEndpoint, applyProvider, storageGuideURL } from '@/utils/storageTypes'

const props = defineProps({
  st: { type: Object, required: true }, // storage.Config
  hasSecret: { type: Boolean, default: false }, // 设置页：Secret 已保存，留空不改
  showProvider: { type: Boolean, default: true }, // 向导第 2 步已经选过服务商，第 3 步不再显示
})
const { t, locale } = useI18n()

const provider = computed(() => providerOf(props.st.provider))

// 自动生成的 Endpoint 当灰色提示，让用户知道留空会连哪里
const endpointPlaceholder = computed(() => {
  const p = provider.value
  return p.endpoint ? p.endpoint(props.st.region || p.region) : p.endpointHint
})

const setProvider = (value) => applyProvider(props.st, value)
</script>
