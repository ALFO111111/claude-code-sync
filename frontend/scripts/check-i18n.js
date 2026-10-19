// 检查界面文字（npm run check-i18n，CI 也跑）：
//  1. zh-CN.js 和 en.js 的 key 完全一致（谁多了、谁少了都报）
//  2. 代码里用到的 key 都存在：t('a.b')、keypath="a.b"、t(`a.${x}.b`)（动态部分只检查前缀）
//  3. 每条文字都能被 vue-i18n 解析（比如误写了 @ { } | 会在运行时报错，这里提前发现）
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join, relative } from 'node:path'
import { fileURLToPath } from 'node:url'
import { createParser } from '@intlify/message-compiler'
import zhCN from '../src/i18n/zh-CN.js'
import en from '../src/i18n/en.js'

const root = fileURLToPath(new URL('..', import.meta.url))
const src = join(root, 'src')
const problems = []

// 把嵌套对象摊平成 { 'a.b.c': '文字' }
function flatten(obj, prefix = '', out = {}) {
  for (const [k, v] of Object.entries(obj)) {
    const key = prefix ? `${prefix}.${k}` : k
    if (v && typeof v === 'object') flatten(v, key, out)
    else out[key] = v
  }
  return out
}

const locales = { 'zh-CN': flatten(zhCN), en: flatten(en) }

// 1. 两边 key 一致
const [a, b] = Object.values(locales)
for (const k of Object.keys(a)) if (!(k in b)) problems.push(`en.js 缺少 ${k}`)
for (const k of Object.keys(b)) if (!(k in a)) problems.push(`zh-CN.js 缺少 ${k}`)

// 3. 每条文字都能解析
const parser = createParser({ onError: (e) => { throw e } })
for (const [lang, msgs] of Object.entries(locales)) {
  for (const [k, v] of Object.entries(msgs)) {
    try {
      parser.parse(v)
    } catch (e) {
      problems.push(`${lang} 的 ${k} 解析失败（${e.message}）: ${JSON.stringify(v)}`)
    }
  }
}

// 2. 代码里用到的 key 都存在
function walk(dir, out = []) {
  for (const name of readdirSync(dir)) {
    const p = join(dir, name)
    if (statSync(p).isDirectory()) walk(p, out)
    else if (/\.(vue|js)$/.test(name) && !p.startsWith(join(src, 'i18n'))) out.push(p)
  }
  return out
}
const keys = Object.keys(a)
const patterns = [
  /\bt\(\s*'([^']+)'/g, // t('a.b')
  /\bt\(\s*`([^`]+)`/g, // t(`a.${x}.b`)
  /keypath="`?([^"`]+)`?"/g, // <i18n-t keypath="a.b">、:keypath="`a.${x}`"
  /\b(?:okKey|failKey)?\s*'((?:bindings|settings|home|sessions|storage|wizard|common)\.[\w.]+)'/g, // 作为参数传递的 key
]
for (const file of walk(src)) {
  const text = readFileSync(file, 'utf8')
  for (const re of patterns) {
    for (const m of text.matchAll(re)) {
      const key = m[1]
      const ok = key.includes('${')
        ? keys.some((k) => k.startsWith(key.slice(0, key.indexOf('${'))))
        : key in a
      if (!ok) problems.push(`${relative(root, file)} 用了不存在的 key: ${key}`)
    }
  }
}

if (problems.length) {
  console.error(problems.join('\n'))
  process.exit(1)
}
console.log(`i18n OK: ${keys.length} 条文字，zh-CN / en 一致`)
