<template>
  <!-- 工具不多（≤3）时并排摆开，一眼看到支持哪些；再多就收成下拉，侧边栏放不下 -->
  <div v-if="list.length <= 3" class="tool-switch segmented-tools">
    <button
      v-for="x in list" :key="x.id"
      :class="{ on: x.id === cur.id }" :title="title(x)"
      @click="setTool(x.id)"
    >
      <ToolIcon :tool="x" :size="16" :off="!x.installed" />
      <span class="ellipsis">{{ toolLook(x).short }}</span>
    </button>
  </div>
  <div v-else class="tool-switch dropdown" @keydown.esc="open = false">
    <button class="current" @click="open = !open">
      <ToolIcon :tool="cur" :size="18" :off="cur.installed === false" />
      <span class="grow ellipsis">{{ cur.name }}</span>
      <NavIcon name="chevron" :size="14" />
    </button>
    <div v-if="open" class="backdrop" @click="open = false"></div>
    <div v-if="open" class="menu">
      <button v-for="x in list" :key="x.id" :class="{ on: x.id === cur.id }" :title="title(x)" @click="pick(x.id)">
        <ToolIcon :tool="x" :size="16" :off="!x.installed" />
        <span class="grow ellipsis" :class="{ muted: !x.installed }">{{ x.name }}</span>
      </button>
    </div>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { tools, currentTool, setTool, toolLook } from '@/stores/tools'
import ToolIcon from '@/components/ToolIcon.vue'
import NavIcon from '@/components/NavIcon.vue'

const { t } = useI18n()
const list = computed(() => tools.value)
const cur = computed(() => currentTool.value)
const open = ref(false)
const title = (x) => (x.installed ? x.name : `${x.name}（${t('tools.notInstalled')}）`)
function pick(id) {
  setTool(id)
  open.value = false
}
</script>

<style scoped>
.tool-switch { margin: 0 2px 6px; }
.segmented-tools {
  display: flex;
  gap: 2px;
  padding: 3px;
  border-radius: 9px;
  background: var(--bg);
  border: 1px solid var(--border);
}
.segmented-tools button {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
  padding: 5px 4px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--muted);
  font-size: 12px;
  cursor: pointer;
}
.segmented-tools button:hover { color: var(--text); }
.segmented-tools button.on { background: var(--panel); color: var(--text); font-weight: 600; box-shadow: 0 1px 2px rgba(0, 0, 0, 0.12); }

.dropdown { position: relative; }
.dropdown .current,
.dropdown .menu button {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 7px 10px;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: var(--bg);
  color: var(--text);
  font-size: 13px;
  cursor: pointer;
  text-align: left;
}
.dropdown .menu {
  position: absolute;
  top: calc(100% + 4px);
  left: 0;
  right: 0;
  z-index: 20;
  padding: 4px;
  border-radius: 9px;
  border: 1px solid var(--border);
  background: var(--panel);
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.15);
}
.dropdown .menu button { border: none; background: transparent; }
.backdrop { position: fixed; inset: 0; z-index: 19; } /* 点菜单外面任何地方就收起 */
.dropdown .menu button:hover { background: var(--bg); }
.dropdown .menu button.on { background: var(--accent-soft); color: var(--accent-text); }
</style>
