<template>
  <span v-if="on" class="refreshing" :title="t('common.refreshing')"><span class="spin">↻</span></span>
</template>

<script setup>
// 后台刷新中的小圈：已经有数据、正在悄悄更新时放在卡片标题旁边，不挡内容。
import { useI18n } from 'vue-i18n'

defineProps({ on: { type: Boolean, default: false } })
const { t } = useI18n()
</script>

<style scoped>
.refreshing { color: var(--muted); font-size: 13px; margin-left: 6px; font-weight: 400; }
</style>
