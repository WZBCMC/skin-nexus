<script setup>
// PluginsView: read-only catalog for admins; enable/disable/reload are
// SUPER_ADMIN-session-only (the backend rejects X-Admin-Token and perm<2).
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { fetchJSON, needToken } from '../../adminApi.js'
import { api } from '../../api.js'
import SchemaForm from '../SchemaForm.vue'

const { t } = useI18n()

const plugins = ref([])
const loadError = ref('')
const actionError = ref('')
const isSuper = ref(false)
const readOnly = ref(true)

const configTarget = ref(null)
const configView = ref(null)
const configValues = ref({})
const configError = ref('')
const configOk = ref('')
const configSaving = ref(false)
const guardsBypassed = ref(false)

async function load() {
  loadError.value = ''
  try {
    const d = await fetchJSON('/api/admin/v1/plugins')
    plugins.value = d.plugins || []
    guardsBypassed.value = !!d.guardsBypassed
  } catch (e) {
    loadError.value = e.message
  }
}

async function detectRole() {
  // Token-only sessions (machine fallback) can list but never mutate.
  if (needToken.value) return
  try {
    const me = await api.me()
    isSuper.value = Number(me.permission) === 2
  } catch { /* stay read-only */ }
}

async function openConfig(p) {
  configTarget.value = p
  configView.value = null
  configValues.value = {}
  configError.value = ''
  configOk.value = ''
  try {
    const d = await fetchJSON(`/api/admin/v1/plugins/${encodeURIComponent(p.name)}/config`)
    configView.value = d
    configValues.value = { ...(d.values || {}) }
  } catch (e) {
    configError.value = e.message
  }
}

async function saveConfig() {
  if (!configTarget.value || !configView.value) return
  configSaving.value = true
  configError.value = ''
  configOk.value = ''
  try {
    const d = await fetchJSON(`/api/admin/v1/plugins/${encodeURIComponent(configTarget.value.name)}/config`, {
      method: 'PUT', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ values: configValues.value }),
    })
    configView.value = d
    configValues.value = { ...(d.values || {}) }
    configOk.value = t('admin.pluginsConfigSaved')
    await load()
  } catch (e) {
    configError.value = e.message
  } finally {
    configSaving.value = false
  }
}

async function rollback(p) {
  actionError.value = ''
  if (!window.confirm(t('admin.pluginsRollbackConfirm', { name: p.title || p.name }))) return
  try {
    await fetchJSON(`/api/admin/v1/plugins/${encodeURIComponent(p.name)}/rollback`, { method: 'POST' })
    await load()
  } catch (e) { actionError.value = e.message }
}

async function uninstall(p) {
  actionError.value = ''
  if (!window.confirm(t('admin.pluginsUninstallConfirm', { name: p.title || p.name }))) return
  try {
    await fetchJSON(`/api/admin/v1/plugins/${encodeURIComponent(p.name)}`, { method: 'DELETE' })
    await load()
  } catch (e) { actionError.value = e.message }
}

async function toggleBypass() {
  actionError.value = ''
  const next = !guardsBypassed.value
  if (next && !window.confirm(t('admin.pluginsGuardBypassConfirm'))) return
  try {
    const d = await fetchJSON('/api/admin/v1/plugins/guards/bypass', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ enabled: next }),
    })
    guardsBypassed.value = !!d.guardsBypassed
  } catch (e) { actionError.value = e.message }
}

async function act(p, action) {
  actionError.value = ''
  if (action === 'enable' && !window.confirm(t('admin.pluginsEnableConfirm'))) return
  try {
    await fetchJSON(`/api/admin/v1/plugins/${encodeURIComponent(p.name)}/${action}`, { method: 'POST' })
    await load()
  } catch (e) {
    actionError.value = e.message
  }
}

function trustText(p) {
  switch (p.trust) {
    case 'verified': return t('admin.trustVerified')
    case 'unknown-key': return t('admin.trustUnknownKey')
    case 'unsigned': return t('admin.trustUnsigned')
    case 'invalid': return t('admin.trustInvalid')
    default: return t('admin.trustUnrecorded')
  }
}

function sourceText(p) {
  switch (p.source) {
    case 'market': return t('admin.sourceMarket')
    case 'upload': return t('admin.sourceUpload')
    case 'manual': return t('admin.sourceManual')
    default: return '—'
  }
}

function provenanceTitle(p) {
  return [p.signer ? `${t('admin.marketSigner')}: ${p.signer}` : '', p.digest ? `${t('admin.marketDigest')}: ${p.digest}` : '']
    .filter(Boolean).join('\n')
}

onMounted(async () => {
  await detectRole()
  readOnly.value = !isSuper.value
  await load()
})
</script>

<template>
  <div class="card">
    <h2 style="margin-top: 0;">{{ t('admin.pluginsTitle') }}</h2>
    <p class="muted">{{ t('admin.pluginsHint') }}</p>
    <p class="muted" v-if="readOnly">{{ t('admin.pluginsReadOnly') }}</p>
    <label v-if="isSuper" class="row" style="align-items:center;gap:8px;margin:8px 0;">
      <input type="checkbox" :checked="guardsBypassed" @change="toggleBypass" style="width:auto;" />
      <span class="muted">{{ t('admin.pluginsGuardBypass') }}</span>
    </label>
    <div class="error" v-if="loadError">{{ loadError }}</div>
    <div class="error" v-if="actionError">{{ actionError }}</div>

    <p v-if="!plugins.length" class="muted">{{ t('admin.pluginsEmpty') }}</p>
    <table v-else>
      <thead>
        <tr>
          <th>{{ t('admin.colPluginName') }}</th>
          <th>{{ t('admin.colVersion') }}</th>
          <th>{{ t('admin.colSource') }}</th>
          <th>{{ t('admin.colAuthor') }}</th>
          <th>{{ t('admin.colPerms') }}</th>
          <th>{{ t('admin.colState') }}</th>
          <th>{{ t('admin.colHealth') }}</th>
          <th>{{ t('admin.colRoutes') }}</th>
          <th>{{ t('admin.colError') }}</th>
          <th v-if="!readOnly">{{ t('admin.colActions') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="p in plugins" :key="p.name">
          <td>
            <strong>{{ p.title || p.name }}</strong>
            <div class="muted">{{ p.description }}</div>
            <button class="secondary" style="margin-top: 6px;" @click="openConfig(p)">{{ t('admin.pluginsConfig') }}</button>
          </td>
          <td>{{ p.version }}</td>
          <td :title="provenanceTitle(p)">
            <span class="badge" :style="p.trust && p.trust !== 'verified' ? 'color:var(--danger);border-color:var(--danger)' : ''">{{ trustText(p) }}</span>
            <div class="muted">{{ sourceText(p) }}</div>
          </td>
          <td>{{ p.author || '—' }}</td>
          <td>
            <span class="badge" v-for="perm in p.permissions" :key="perm">{{ perm }}</span>
          </td>
          <td>
            <span class="badge">{{ t('admin.state' + p.state.charAt(0).toUpperCase() + p.state.slice(1)) }}</span>
          </td>
          <td><span class="badge">{{ p.health || p.state }}</span></td>
          <td class="muted">{{ p.routes || 0 }} route · {{ p.uiPages || 0 }} UI</td>
          <td class="muted">{{ p.lastError || '—' }}</td>
          <td v-if="!readOnly" class="nowrap">
            <button v-if="!p.enabled" @click="act(p, 'enable')">{{ t('admin.pluginsEnable') }}</button>
            <button v-else class="secondary" @click="act(p, 'disable')">{{ t('admin.pluginsDisable') }}</button>
            <button class="secondary" style="margin-left: 6px;" @click="act(p, 'reload')">{{ t('admin.pluginsReload') }}</button>
            <button class="secondary" style="margin-left: 6px;" @click="rollback(p)">{{ t('admin.pluginsRollback') }}</button>
            <button class="danger-outline" style="margin-left: 6px;" @click="uninstall(p)">{{ t('admin.pluginsUninstall') }}</button>
          </td>
        </tr>
      </tbody>
    </table>

    <div v-if="configTarget" class="card" style="margin-top: 16px;">
      <div class="row" style="justify-content: space-between; align-items: center;">
        <h3 style="margin: 0;">{{ t('admin.pluginsConfigTitle', { name: configTarget.title || configTarget.name }) }}</h3>
        <button class="secondary" @click="configTarget = null">{{ t('common.close') }}</button>
      </div>
      <p class="muted">{{ t('admin.pluginsConfigHint') }}</p>
      <div class="error" v-if="configError">{{ configError }}</div>
      <div class="success" v-if="configOk">{{ configOk }}</div>
      <template v-if="configView">
        <SchemaForm :schema="configView.schema" v-model="configValues" :disabled="!isSuper || configSaving" />
        <div class="row" style="margin-top: 12px;">
          <button :disabled="!isSuper || configSaving" @click="saveConfig">{{ t('admin.pluginsConfigSave') }}</button>
          <span v-if="!isSuper" class="muted">{{ t('admin.pluginsConfigReadOnly') }}</span>
        </div>
      </template>
    </div>
  </div>
</template>
