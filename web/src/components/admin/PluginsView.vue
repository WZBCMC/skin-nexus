<script setup>
// PluginsView: read-only catalog for admins; enable/disable/reload are
// SUPER_ADMIN-session-only (the backend rejects X-Admin-Token and perm<2).
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { fetchJSON, needToken } from '../../adminApi.js'
import { api } from '../../api.js'

const { t } = useI18n()

const plugins = ref([])
const loadError = ref('')
const actionError = ref('')
const isSuper = ref(false)
const readOnly = ref(true)

async function load() {
  loadError.value = ''
  try {
    const d = await fetchJSON('/api/admin/v1/plugins')
    plugins.value = d.plugins || []
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
    <div class="error" v-if="loadError">{{ loadError }}</div>
    <div class="error" v-if="actionError">{{ actionError }}</div>

    <p v-if="!plugins.length" class="muted">{{ t('admin.pluginsEmpty') }}</p>
    <table v-else>
      <thead>
        <tr>
          <th>{{ t('admin.colPluginName') }}</th>
          <th>{{ t('admin.colVersion') }}</th>
          <th>{{ t('admin.colAuthor') }}</th>
          <th>{{ t('admin.colPerms') }}</th>
          <th>{{ t('admin.colState') }}</th>
          <th>{{ t('admin.colError') }}</th>
          <th v-if="!readOnly">{{ t('admin.colActions') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="p in plugins" :key="p.name">
          <td>
            <strong>{{ p.title || p.name }}</strong>
            <div class="muted">{{ p.description }}</div>
          </td>
          <td>{{ p.version }}</td>
          <td>{{ p.author || '—' }}</td>
          <td>
            <span class="badge" v-for="perm in p.permissions" :key="perm">{{ perm }}</span>
          </td>
          <td>
            <span class="badge">{{ t('admin.state' + p.state.charAt(0).toUpperCase() + p.state.slice(1)) }}</span>
          </td>
          <td class="muted">{{ p.lastError || '—' }}</td>
          <td v-if="!readOnly" class="nowrap">
            <button v-if="!p.enabled" @click="act(p, 'enable')">{{ t('admin.pluginsEnable') }}</button>
            <button v-else class="secondary" @click="act(p, 'disable')">{{ t('admin.pluginsDisable') }}</button>
            <button class="secondary" style="margin-left: 6px;" @click="act(p, 'reload')">{{ t('admin.pluginsReload') }}</button>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
