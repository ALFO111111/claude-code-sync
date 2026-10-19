<template>
  <div class="skeleton" aria-hidden="true">
    <div v-for="i in lines" :key="i" class="bar" :style="{ width: i === lines ? '55%' : '100%' }"></div>
  </div>
</template>

<script setup>
// 骨架占位：数据第一次还没拉到时，用几条灰条代替"加载中…"，内容出来时版面不跳。之后再进页面直接显示缓存，不会再看到它。
defineProps({ lines: { type: Number, default: 3 } })
</script>

<style scoped>
.bar {
  height: 12px;
  border-radius: 6px;
  margin: 10px 0;
  background: linear-gradient(90deg, var(--panel-2) 25%, var(--border) 50%, var(--panel-2) 75%);
  background-size: 200% 100%;
  animation: shimmer 1.2s ease-in-out infinite;
}
@keyframes shimmer { to { background-position: -200% 0; } }
</style>
