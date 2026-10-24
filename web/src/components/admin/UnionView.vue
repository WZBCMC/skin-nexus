<script setup>
// UnionView: MUA Union module status + rotation resume + ops buttons (B5.3)
// + union blacklist proxy (B5.4). The module-disabled case shows an empty
// state. Release/Delete are irreversible-ish — both double-confirm, delete
// is permanent (hub erases the record).
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { fetchJSON } from '../../adminApi.js'

const { t } = useI18n()
const unionStatus = ref(null)
const error = ref('')
const opResult = ref('')
const opBusy = ref(false)

// blacklist
const blRows = ref([])
const blTotal = ref(0)
const blPage = ref(1)
const blQ = ref('')
const blEmail = ref('')
const blReason = ref('')

async function load() {
  error.value = ''
  // Absent (404) when the module is disabled — that's expected.
  try {
    unionStatus.value = await fetchJSON('/api/admin/v1/union/status')
  } catch (e) {
    unionStatus.value = null
    error.value = e.message
  }
}

async function resumeRotation() {
  if (!confirm(t('admin.unionResumeConfirm'))) return
  try {
    await fetchJSON('/api/admin/v1/union/rotation/resume', { method: 'POST' })
    await load()
  } catch (e) { error.value = e.message }
}

async function runOp(name, path, confirmMsg) {
  if (confirmMsg && !confirm(confirmMsg)) return
  opBusy.value = true
  opResult.value = ''
  error.value = ''
  try {
    const d = await fetchJSON(path, { method: 'POST' })
    opResult.value = t('admin.unionOpDone', { name: t(name), detail: JSON.stringify(d).slice(0, 400) })
    await load()
  } catch (e) {
    error.value = e.message
  } finally {
    opBusy.value = false
  }
}

async function loadBlacklist() {
  error.value = ''
  try {
    const d = await fetchJSON(`/api/admin/v1/union/blacklist?q=${encodeURIComponent(blQ.value)}&page=${blPage.value}`)
    // hub returns Laravel pagination JSON; tolerate both shaped answers
    blRows.value = d.data || d.items || []
    blTotal.value = d.total || (d.data ? d.data.length : 0)
  } catch (e) {
    blRows.value = []
    error.value = e.message
  }
}

async function blAdd() {
  if (!blEmail.value.trim()) return
  error.value = ''
  try {
    await fetchJSON('/api/admin/v1/union/blacklist', {
      method: 'POST',
      body: JSON.stringify({ email: blEmail.value.trim(), reason: blReason.value.trim() }),
    })
    blEmail.value = ''
    blReason.value = ''
    await loadBlacklist()
  } catch (e) { error.value = e.message }
}

async function blRelease(row) {
  if (!confirm(t('admin.unionBlReleaseConfirm', { email: row.email || row.id }))) return
  error.value = ''
  try {
    await fetchJSON(`/api/admin/v1/union/blacklist/${encodeURIComponent(row.id)}/invalidate`, { method: 'PUT' })
    await loadBlacklist()
  } catch (e) { error.value = e.message }
}

async function blDelete(row) {
  if (!confirm(t('admin.unionBlDeleteConfirm1', { email: row.email || row.id }))) return
  if (!confirm(t('admin.unionBlDeleteConfirm2'))) return
  error.value = ''
  try {
    await fetchJSON(`/api/admin/v1/union/blacklist/${encodeURIComponent(row.id)}`, { method: 'DELETE' })
    await loadBlacklist()
  } catch (e) { error.value = e.message }
}

onMounted(async () => {
  await load()
  if (unionStatus.value) await loadBlacklist()
})
</script>

<template>
  <div>
    <div class="card" v-if="unionStatus">
      <h2 style="margin-top: 0;">{{ t('admin.unionTitle') }}</h2>
      <div class="row">
        <span class="badge">{{ t('admin.unionMode') }} {{ unionStatus.mode }}</span>
        <span class="badge">{{ t('admin.unionKeyFp') }} {{ unionStatus.key_fingerprint || '—' }}</span>
        <span class="badge">{{ t('admin.unionKeyVersion') }} {{ unionStatus.private_key_version || '—' }}</span>
        <span class="badge">{{ t('admin.unionServerListVersion', { version: unionStatus.server_list_version || '—' }) }}</span>
        <span class="badge">{{ t('admin.unionQueryEmail') }} {{ unionStatus.queryemail_mode }}</span>
        <span class="badge" :style="unionStatus.rotation_suspended ? 'border-color: var(--danger); color: var(--danger);' : ''">
          {{ unionStatus.rotation_suspended ? t('admin.unionRotationSuspended') : t('admin.unionRotationOk') }}
        </span>
      </div>
      <div class="row" style="margin-top: 10px;" v-if="unionStatus.rotation_suspended">
        <button class="secondary" @click="resumeRotation">{{ t('admin.unionResumeRotation') }}</button>
      </div>

      <!-- B5.3 ops buttons -->
      <div class="row" style="margin-top: 10px;">
        <button class="secondary" :disabled="opBusy" @click="runOp('admin.unionOpServerList', '/api/admin/v1/union/ops/serverlist')">
          {{ t('admin.unionOpServerList') }}
        </button>
        <button class="secondary" :disabled="opBusy" @click="runOp('admin.unionOpKeyUpdate', '/api/admin/v1/union/ops/key-update', t('admin.unionOpKeyUpdateConfirm'))">
          {{ t('admin.unionOpKeyUpdate') }}
        </button>
        <button class="secondary" :disabled="opBusy" @click="runOp('admin.unionOpRosterSync', '/api/admin/v1/union/ops/roster-sync')">
          {{ t('admin.unionOpRosterSync') }}
        </button>
        <button class="secondary" :disabled="opBusy" @click="runOp('admin.unionOpDiagnose', '/api/admin/v1/union/ops/diagnose')">
          {{ t('admin.unionOpDiagnose') }}
        </button>
      </div>
      <div class="success" v-if="opResult">{{ opResult }}</div>

      <div class="muted" style="margin-top: 10px; font-size: 12px;">
        {{ t('admin.unionTrustNotice') }}
      </div>
      <div class="error" v-if="error">{{ error }}</div>
    </div>
    <div class="card" v-else>
      <h2 style="margin-top: 0;">{{ t('admin.unionTitle') }}</h2>
      <p class="muted">{{ t('admin.unionDisabled') }}</p>
    </div>

    <!-- B5.4 blacklist -->
    <div class="card" v-if="unionStatus">
      <h2 style="margin-top: 0;">{{ t('admin.unionBlTitle') }}</h2>
      <div class="row">
        <input v-model="blQ" :placeholder="t('admin.unionBlSearch')" style="max-width: 220px;" @keyup.enter="blPage = 1; loadBlacklist()" />
        <button class="secondary" @click="blPage = 1; loadBlacklist()">{{ t('admin.unionBlSearchBtn') }}</button>
      </div>
      <table v-if="blRows.length" class="bl-table" style="margin-top: 10px;">
        <tr>
          <th>ID</th><th>{{ t('admin.unionBlEmail') }}</th><th>{{ t('admin.unionBlSource') }}</th>
          <th>{{ t('admin.unionBlReason') }}</th><th>{{ t('admin.unionBlCreated') }}</th><th></th>
        </tr>
        <tr v-for="row in blRows" :key="row.id">
          <td>{{ row.id }}</td>
          <td>{{ row.email || '—' }}</td>
          <td>{{ row.source || '—' }}</td>
          <td>{{ row.reason || '—' }}</td>
          <td class="muted">{{ row.created_at || '—' }}</td>
          <td>
            <button class="secondary" @click="blRelease(row)">{{ t('admin.unionBlRelease') }}</button>
            <button class="secondary" @click="blDelete(row)" style="margin-left: 6px; border-color: var(--danger); color: var(--danger);">
              {{ t('admin.unionBlDelete') }}
            </button>
          </td>
        </tr>
      </table>
      <div v-else class="muted">{{ t('admin.unionBlEmpty') }}</div>
      <div class="row" style="margin-top: 12px;">
        <input v-model="blEmail" :placeholder="t('admin.unionBlEmail')" style="max-width: 220px;" />
        <input v-model="blReason" :placeholder="t('admin.unionBlReason')" style="max-width: 260px;" />
        <button class="secondary" @click="blAdd">{{ t('admin.unionBlAdd') }}</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* Blacklist table: fixed layout so the long free-text reason column cannot
   push created_at/actions off the card edge (2026-10-24 UI fix). */
.bl-table {
  table-layout: fixed;
  width: 100%;
}
.bl-table th:nth-child(1), .bl-table td:nth-child(1) { width: 44px; }
.bl-table th:nth-child(2), .bl-table td:nth-child(2) { width: 170px; }
.bl-table th:nth-child(3), .bl-table td:nth-child(3) { width: 64px; }
.bl-table th:nth-child(5), .bl-table td:nth-child(5) { width: 150px; }
.bl-table th:nth-child(6), .bl-table td:nth-child(6) { width: 130px; }
.bl-table td {
  word-break: break-all;
  vertical-align: top;
}
</style>
