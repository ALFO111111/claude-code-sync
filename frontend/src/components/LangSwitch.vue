<template>
  <select :value="languageSetting" :title="t('lang.label')" @change="change($event.target.value)">
    <option value="">{{ t('lang.system') }}</option>
    <!-- 语言名用它自己的写法，不翻译：看不懂当前界面的人也能认出自己的语言 -->
    <option value="zh-CN">简体中文</option>
    <option value="en">English</option>
  </select>
</template>

<script setup>
// 界面语言下拉框（设置页、向导右上角）。切换立即生效，后端存进配置并刷新托盘菜单。
import { useI18n } from 'vue-i18n'
import { languageSetting, changeLanguage } from '@/i18n'

const { t } = useI18n()

async function change(lang) {
  try {
    await changeLanguage(lang)
  } catch {
    // 后端保存失败也不影响这次显示；下次启动按配置文件里的为准
  }
}
</script>
