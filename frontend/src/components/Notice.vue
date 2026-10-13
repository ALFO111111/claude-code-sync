<template>
  <div v-if="notice" class="card">
    <div :class="notice.error ? 'err' : 'ok'">{{ notice.text }}</div>
  </div>
</template>

<script setup>
// 操作结果提示卡片。notice 为 null 时不显示；状态用 composables/useNotice 管理。
defineProps({
  notice: { type: Object, default: null }, // { text, error }
})
</script>
