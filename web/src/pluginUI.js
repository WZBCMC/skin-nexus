// Plugin UI registry: one fetch per page load, merged into vue-i18n under
// plugin.<name>.* so plugin pages/widgets never edit the static locale files.
import { reactive } from 'vue'
import { i18n } from './i18n.js'

export const pluginUI = reactive({ plugins: [] })

export async function loadPluginUI() {
  try {
    const r = await fetch('/api/v1/plugins/ui')
    if (!r.ok) return
    const d = await r.json()
    pluginUI.plugins = d.plugins || []
    for (const p of pluginUI.plugins) {
      for (const [locale, kv] of Object.entries(p.i18n || {})) {
        i18n.global.mergeLocaleMessage(locale, { plugin: { [p.plugin]: kv } })
      }
    }
  } catch { /* plugin UI is best-effort */ }
}

export function pages(area) {
  const out = []
  for (const p of pluginUI.plugins) {
    for (const page of (p.pages || [])) {
      if (page.area === area) out.push({ ...page, plugin: p.plugin })
    }
  }
  return out
}

export function navPages(area) {
  return pages(area).filter((p) => p.nav && p.nav.label)
}

export function pageByPath(path) {
  const want = '/' + String(path || '').replace(/^\/+/, '')
  for (const p of pluginUI.plugins) {
    for (const page of (p.pages || [])) {
      if (page.path === want) return { ...page, plugin: p.plugin }
    }
  }
  return null
}

export function slotsFor(id) {
  const out = []
  for (const p of pluginUI.plugins) {
    for (const slot of (p.slots || [])) {
      if (slot.id === id) out.push({ ...slot, plugin: p.plugin })
    }
  }
  return out
}

export function assetURL(plugin, src) {
  const rel = String(src || '').replace(/^\/+/, '')
  return `/plugin-assets/${encodeURIComponent(plugin)}/${rel.split('/').map(encodeURIComponent).join('/')}`
}
