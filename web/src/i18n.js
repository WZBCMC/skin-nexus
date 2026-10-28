// i18n: vue-i18n setup. Locale resolution: localStorage.skin_lang ->
// navigator.language -> zh-CN. Fallback is always zh-CN.
import { createI18n } from 'vue-i18n'
import zhCN from './locales/zh-CN.json'
import en from './locales/en.json'

function initialLocale() {
  const saved = localStorage.getItem('skin_lang')
  if (saved && (saved === 'zh-CN' || saved === 'en')) return saved
  const nav = (navigator.language || '').toLowerCase()
  return nav.startsWith('en') ? 'en' : 'zh-CN'
}

export const i18n = createI18n({
  legacy: false,
  locale: initialLocale(),
  fallbackLocale: 'zh-CN',
  messages: { 'zh-CN': zhCN, en },
})

document.documentElement.lang = i18n.global.locale.value

export function setLocale(locale) {
  i18n.global.locale.value = locale
  localStorage.setItem('skin_lang', locale)
  document.documentElement.lang = locale
}
