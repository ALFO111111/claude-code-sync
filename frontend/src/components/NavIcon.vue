<template>
  <svg viewBox="0 0 24 24" :width="size" :height="size" fill="none" stroke="currentColor"
       stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
    <path v-for="(d, i) in PATHS[name]" :key="i" :d="d" />
  </svg>
</template>

<script setup>
// 线条图标（内联 SVG，颜色跟随文字）：侧边栏，以及各页面的小图标。以前用 ⌂ ☰ 这类字符，不同字体下大小粗细不一。
defineProps({
  name: { type: String, required: true },
  size: { type: [Number, String], default: 18 },
})

const PATHS = {
  home: ['M3 10.5 12 3l9 7.5', 'M5 9v11h14V9', 'M10 20v-6h4v6'],
  sessions: ['M4 5h16v11H8l-4 4z', 'M8 9h8', 'M8 12h5'],
  bindings: ['M10 14a4 4 0 0 0 5.7 0l3-3a4 4 0 0 0-5.7-5.7l-1 1', 'M14 10a4 4 0 0 0-5.7 0l-3 3a4 4 0 0 0 5.7 5.7l1-1'],
  // 备份：一个盾牌里一个回退箭头（"出了事能找回来"）
  backups: ['M12 3 4.5 6v5.5c0 4.6 3.2 8.3 7.5 9.5 4.3-1.2 7.5-4.9 7.5-9.5V6z', 'M9 12.5a3.2 3.2 0 1 0 1-2.4', 'M9 8.5v2.2h2.2'],
  // 冲突：两边的修改迎面撞上
  conflicts: ['M3 12h7', 'M7 8.5l3.5 3.5L7 15.5', 'M21 12h-7', 'M17 8.5 13.5 12l3.5 3.5'],
  settings: ['M4 6h10', 'M18 6h2', 'M4 12h4', 'M12 12h8', 'M4 18h12', 'M20 18h0', 'M16 4v4', 'M10 10v4', 'M18 16v4'],
  logs: ['M6 3h9l4 4v14H6z', 'M14 3v5h5', 'M9 12h7', 'M9 16h7'],
  // 页面里用的小图标
  upload: ['M12 16V5', 'M7 10l5-5 5 5', 'M5 19h14'],
  download: ['M12 5v11', 'M7 11l5 5 5-5', 'M5 19h14'],
  merge: ['M6 4v5a6 6 0 0 0 6 6h6', 'M15 12l3 3-3 3', 'M6 20v-4'],
  fork: ['M6 4v6a4 4 0 0 0 4 4h4', 'M18 4v16', 'M6 20v-2', 'M14 11l3 3-3 3'],
  conflict: ['M12 4 3 20h18z', 'M12 10v4', 'M12 17h0'],
  restore: ['M4 12a8 8 0 1 0 2.4-5.7', 'M4 4v4h4'],
  file: ['M6 3h8l4 4v14H6z', 'M14 3v4h4'],
  folder: ['M3 6h6l2 2h10v11H3z'],
  device: ['M4 5h16v11H4z', 'M9 20h6', 'M12 16v4'],
  cloud: ['M7 18a4 4 0 0 1-.6-8A6 6 0 0 1 18 9a4.5 4.5 0 0 1-.5 9z'],
  play: ['M8 5v14l11-7z'],
  copy: ['M8 8h11v11H8z', 'M5 16V5h11'],
  search: ['M11 18a7 7 0 1 0 0-14 7 7 0 0 0 0 14z', 'M20 20l-4-4'],
  key: ['M14 10a4 4 0 1 0-4.2 4L4 20v-3h3v-3h2.5', 'M15.5 8.5h0'],
  check: ['M5 12.5l4.5 4.5L19 7'],
}
</script>
