<script setup>
// AccessLogsView (2026-10-26 观测面): web/launcher logins, Yggdrasil
// server traffic and Union traffic in one filterable page — the admin's
// "what is happening right now" view. Read-only; retention is server-side
// (90 days).
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { fetchJSON } from '../../adminApi.js'

const { t } = useI18n()

const logs = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = 50
const kind = ref('')
const query = ref('')
const error = ref('')

const KINDS = [
  'web_login', 'web_register',
  'ygg_authenticate', 'ygg_refresh', 'ygg_validate',
  'ygg_join', 'ygg_hasjoined', 'ygg_profile',
  'union_in', 'union_out',
]

async function load() {
  error.value = ''
  try {
    const qs = new URLSearchParams({ page: String(page.value) })
    if (kind.value) qs.set('kind', kind.value)
    if (query.value.trim()) qs.set('q', query.value.trim())
    const d = await fetchJSON(`/api/admin/v1/access-logs?${qs}`)
    logs.value = d.logs || []
    total.value = d.total || 0
  } catch (e) { error.value = e.message }
}

function search() { page.value = 1; load() }

function turnPage(delta) {
  const next = page.value + delta
  if (next < 1 || (next - 1) * pageSize >= total.value) return
  page.value = next
  load()
}
const pageMax = () => Math.max(1, Math.ceil(total.value / pageSize))

// createdAt arrives RFC3339 (UTC); render in the operator's locale.
function fmtTime(s) {
  if (!s) return '—'
  const d = new Date(s)
  return Number.isNaN(d.getTime()) ? s : d.toLocaleString()
}

// Outcome badge tone: ok green, miss muted, denied amber, fail/error red.
function outcomeClass(o) {
  return { ok: 'oc-ok', miss: 'oc-miss', denied: 'oc-warn' }[o] || 'oc-bad'
}

onMounted(load)
</script>

<template>
  <div class="card">
    <h2 style="margin-top: 0;">{{ t('admin.accessTitle') }}</h2>
    <p class="muted" style="margin-top: -6px;">{{ t('admin.accessHint') }}</p>
    <div class="row">
      <select v-model="kind" @change="search" style="max-width: 200px;">
        <option value="">{{ t('admin.accessKindAll') }}</option>
        <option v-for="k in KINDS" :key="k" :value="k">{{ t(`admin.kind.${k}`) }}</option>
      </select>
      <input v-model="query" :placeholder="t('admin.accessSearchPlaceholder')" style="max-width: 260px;"
             @keyup.enter="search" />
      <button class="secondary" @click="search">{{ t('admin.search') }}</button>
      <span class="muted">{{ t('admin.accessTotal', { total }) }}</span>
    </div>
    <div class="error" v-if="error">{{ error }}</div>
    <div class="table-scroll" v-if="logs.length">
    <table class="logs-table" style="margin-top: 10px;">
      <tr>
        <th>{{ t('admin.colTime') }}</th><th>{{ t('admin.colKind') }}</th>
        <th>{{ t('admin.colOutcome') }}</th><th>{{ t('admin.colWho') }}</th>
        <th>IP</th><th>UA</th><th>{{ t('admin.colDetail') }}</th>
      </tr>
      <tr v-for="row in logs" :key="row.id">
        <td class="nowrap muted">{{ fmtTime(row.createdAt) }}</td>
        <td class="nowrap">{{ t(`admin.kind.${row.kind}`) }}</td>
        <td class="nowrap"><span class="oc" :class="outcomeClass(row.outcome)">{{ t(`admin.outcome.${row.outcome}`) }}</span></td>
        <td class="nowrap who">{{ row.who || '—' }} <span class="muted" v-if="row.uid">#{{ row.uid }}</span></td>
        <td class="nowrap"><code class="ip">{{ row.ip || '—' }}</code></td>
        <td class="ua">{{ row.ua || '—' }}</td>
        <td class="detail">{{ row.detail || '—' }}</td>
      </tr>
    </table>
    </div>
    <p class="muted" v-else>{{ t('admin.accessEmpty') }}</p>
    <div class="row" style="margin-top: 12px;" v-if="total > pageSize">
      <button class="secondary" :disabled="page <= 1" @click="turnPage(-1)">‹ {{ t('admin.prevPage') }}</button>
      <span class="muted">{{ page }} / {{ pageMax() }}</span>
      <button class="secondary" :disabled="page >= pageMax()" @click="turnPage(1)">{{ t('admin.nextPage') }} ›</button>
    </div>
  </div>
</template>

<style scoped>
/* narrow viewports scroll the table instead of crushing columns */
.table-scroll { overflow-x: auto; }
.logs-table { min-width: 960px; }
.logs-table th:nth-child(1), .logs-table td:nth-child(1) { width: 150px; }
.logs-table th:nth-child(2), .logs-table td:nth-child(2) { width: 110px; }
.logs-table th:nth-child(3), .logs-table td:nth-child(3) { width: 70px; }
.logs-table th:nth-child(4), .logs-table td:nth-child(4) { width: 140px; }
.logs-table th:nth-child(5), .logs-table td:nth-child(5) { width: 120px; }
.logs-table th:nth-child(6), .logs-table td:nth-child(6) { max-width: 220px; }
.logs-table td.detail { font-family: var(--mono, monospace); font-size: 12px; }
.nowrap { white-space: nowrap; }
.who { font-weight: 600; }
.ip { font-size: 12px; }
.ua { font-size: 12px; word-break: break-all; }
.oc { display: inline-block; padding: 1px 8px; border-radius: 999px; font-size: 12px; border: 1px solid transparent; }
.oc-ok { color: #2e9e5b; border-color: #2e9e5b66; }
.oc-miss { color: var(--muted, #8b93a1); border-color: #8884; }
.oc-warn { color: #b98900; border-color: #b9890066; }
.oc-bad { color: var(--danger, #d64545); border-color: #d6454566; }
</style>
