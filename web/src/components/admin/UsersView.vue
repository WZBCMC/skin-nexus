<script setup>
// UsersView: blessing 管理面板 用户管理 parity (B2) — table with search,
// pagination, and inline edits (email / nickname / verification /
// permission) on top of the P1 password-reset / delete actions.
//
// Permission guard semantics live server-side (adminapi.inlineGuard):
// strictly-lower targets only, no self-edit, ADMIN grant needs SUPER_ADMIN.
// The UI just renders the 403 message.
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { fetchJSON } from '../../adminApi.js'

const { t } = useI18n()

const users = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = 50
const query = ref('')
const error = ref('')
const busy = ref(false) // in-flight guard: no double submit

async function load() {
  error.value = ''
  try {
    const qs = new URLSearchParams({ page: String(page.value) })
    if (query.value.trim()) qs.set('q', query.value.trim())
    const d = await fetchJSON(`/api/admin/v1/users?${qs}`)
    users.value = d.users || []
    total.value = d.total || 0
  } catch (e) { error.value = e.message }
}

function search() {
  page.value = 1
  load()
}

function turnPage(delta) {
  const next = page.value + delta
  if (next < 1 || (next - 1) * pageSize >= total.value) return
  page.value = next
  load()
}

async function adminResetPassword(u) {
  const pw = prompt(t('admin.resetPwPrompt', { name: u.nickname, id: u.id }), '')
  if (!pw) return
  error.value = ''
  try {
    await fetchJSON(`/api/admin/v1/users/${u.id}/password`, {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ password: pw }),
    })
    alert(t('admin.resetPwDone'))
  } catch (e) { error.value = e.message }
}

async function adminDeleteUser(u) {
  if (!confirm(t('admin.deleteConfirm', { name: u.nickname, id: u.id }))) return
  error.value = ''
  try {
    await fetchJSON(`/api/admin/v1/users/${u.id}`, { method: 'DELETE' })
    await load()
  } catch (e) { error.value = e.message }
}

// ---- inline edits ----

async function putUser(id, path, body) {
  if (busy.value) return false
  busy.value = true
  error.value = ''
  try {
    await fetchJSON(`/api/admin/v1/users/${id}/${path}`, {
      method: 'PUT', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    })
    return true
  } catch (e) {
    error.value = e.message
    return false
  } finally {
    busy.value = false
  }
}

// Each edit keeps the old value until the PUT succeeds; on failure the
// table refetches so the UI never shows an unsaved phantom state.
async function saveEmail(u) {
  const prev = u.email
  if (await putUser(u.id, 'email', { email: u.email || '' })) {
    u.email = String(u.email || '').trim()
  } else {
    u.email = prev
    await load()
  }
}

async function saveNickname(u) {
  const prev = u.nickname
  if (await putUser(u.id, 'nickname', { nickname: u.nickname })) {
    u.nickname = String(u.nickname).trim()
  } else {
    u.nickname = prev
    await load()
  }
}

async function toggleVerified(u) {
  const next = Number(u.emailVerified) ? 0 : 1
  if (await putUser(u.id, 'verification', { verified: !!next })) {
    u.emailVerified = next
  } else {
    await load()
  }
}

// Permission select: -1 banned / 0 normal / 1 admin. SUPER_ADMIN (2) is
// never grantable from the UI (blessing parity: setup-created only).
async function changePermission(u, ev) {
  const next = Number(ev.target.value)
  const prev = Number(u.permission)
  if (next === prev) return
  const label = (p) => t(`admin.perm${p}`)
  if (!confirm(t('admin.permChangeConfirm', {
    name: u.nickname, id: u.id, from: label(prev), to: label(next),
  }))) {
    ev.target.value = String(prev)
    return
  }
  if (await putUser(u.id, 'permission', { permission: next })) {
    u.permission = next
  } else {
    ev.target.value = String(prev)
    await load()
  }
}

const pageMax = () => Math.max(1, Math.ceil(total.value / pageSize))

onMounted(load)
</script>

<template>
  <div class="card">
    <h2 style="margin-top: 0;">{{ t('admin.usersTitle') }}</h2>
    <div class="row">
      <input v-model="query" :placeholder="t('admin.userSearchPlaceholder')" style="max-width: 260px;"
             @keyup.enter="search" />
      <button class="secondary" @click="search">{{ t('admin.search') }}</button>
      <span class="muted">{{ t('admin.userTotal', { total }) }}</span>
    </div>
    <div class="error" v-if="error">{{ error }}</div>
    <table v-if="users.length" style="margin-top: 10px;">
      <tr>
        <th>{{ t('admin.colId') }}</th><th>{{ t('admin.colNickname') }}</th><th>{{ t('admin.colEmail') }}</th>
        <th>{{ t('admin.colPermission') }}</th><th>{{ t('admin.colVerified') }}</th>
        <th>{{ t('admin.colAlgo') }}</th><th>{{ t('admin.colCreated') }}</th><th></th>
      </tr>
      <tr v-for="u in users" :key="u.id">
        <td>{{ u.id }}</td>
        <!-- inline edit: blur/enter commits, Esc reverts -->
        <td><input class="cell-input" v-model="u.nickname" @blur="saveNickname(u)"
                   @keyup.escape="load()" /></td>
        <td><input class="cell-input" v-model="u.email" placeholder="—" @blur="saveEmail(u)"
                   @keyup.escape="load()" /></td>
        <td>
          <select class="cell-input" :value="String(u.permission)" @change="changePermission(u, $event)"
                  :disabled="u.id === 1">
            <option value="-1">{{ t('admin.perm-1') }}</option>
            <option value="0">{{ t('admin.perm0') }}</option>
            <option value="1">{{ t('admin.perm1') }}</option>
          </select>
        </td>
        <td>
          <button class="secondary" @click="toggleVerified(u)">
            {{ Number(u.emailVerified) ? '✓' : '—' }}
          </button>
        </td>
        <td><span class="badge" :style="u.passwordAlgo.startsWith('legacy') ? 'border-color: var(--danger); color: var(--danger);' : ''">{{ u.passwordAlgo }}</span></td>
        <td class="muted">{{ (u.createdAt || '').slice(0, 10) }}</td>
        <td style="white-space: nowrap;">
          <button class="secondary" @click="adminResetPassword(u)">{{ t('admin.resetPw') }}</button>
          <button class="secondary" v-if="u.id !== 1" @click="adminDeleteUser(u)" style="margin-left: 6px;">{{ t('admin.delete') }}</button>
        </td>
      </tr>
    </table>
    <div class="row" style="margin-top: 12px;" v-if="total > pageSize">
      <button class="secondary" :disabled="page <= 1" @click="turnPage(-1)">‹ {{ t('admin.prevPage') }}</button>
      <span class="muted">{{ page }} / {{ pageMax() }}</span>
      <button class="secondary" :disabled="page >= pageMax()" @click="turnPage(1)">{{ t('admin.nextPage') }} ›</button>
    </div>
  </div>
</template>

<style scoped>
.cell-input { max-width: 160px; padding: 4px 6px; font-size: 13px; }
</style>
