import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  base: './',
  resolve: {
    // @/xxx 指向 src/xxx
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
})
