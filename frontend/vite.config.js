import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

const src = (p) => fileURLToPath(new URL('./src/' + p, import.meta.url))

export default defineConfig(({ mode }) => ({
  plugins: [vue()],
  base: './',
  resolve: {
    alias: [
      // npm run demo：后端换成编出来的假数据（src/demo），浏览器里就能看界面，截图也不会露出个人信息
      ...(mode === 'demo'
        ? [
            { find: /^.*\/wailsjs\/go\/gui\/App$/, replacement: src('demo/App.js') },
            { find: /^.*\/wailsjs\/runtime\/runtime$/, replacement: src('demo/runtime.js') },
          ]
        : []),
      // @/xxx 指向 src/xxx
      { find: '@', replacement: src('').replace(/[\\/]$/, '') },
    ],
  },
}))
