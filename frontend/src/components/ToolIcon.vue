<template>
  <span class="tool-icon" :class="{ off }" :style="{ background: look.color, width: px, height: px, fontSize: fs }">{{ look.mark }}</span>
</template>

<script setup>
// 工具的小图标：一个带颜色的圆角方块，里面一两个字符（颜色、字符见 stores/tools.js 的 toolLook）。
import { computed } from 'vue'
import { toolLook } from '@/stores/tools'

const props = defineProps({
  tool: { type: [String, Object], required: true },
  size: { type: Number, default: 18 },
  off: Boolean, // 本机没装这个工具：变灰
})
const look = computed(() => toolLook(props.tool))
const px = computed(() => props.size + 'px')
const fs = computed(() => Math.round(props.size * (look.value.mark.length > 1 ? 0.48 : 0.6)) + 'px')
</script>

<style scoped>
.tool-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: none;
  border-radius: 5px;
  color: #fff;
  font-weight: 700;
  font-family: ui-monospace, Consolas, monospace;
  letter-spacing: -0.5px;
  line-height: 1;
}
.tool-icon.off { filter: grayscale(1); opacity: 0.5; }
</style>
