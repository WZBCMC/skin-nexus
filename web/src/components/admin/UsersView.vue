<script setup>
// UsersView: admin user table (B1 moved it out of the old card flow;
// B2 adds search / pagination / inline editing on top of this base).
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { fetchJSON } from '../../adminApi.js'

const { t } = useI18n()

const users = ref([])
const error = ref('')

async function load() {
  error.value = ''
  try {
    const d = await fetchJSON('/api/admin/v1/users?page=1')
    users.value = d.users || []
  } catch (e) { error.value = e.message }
}

async function adminResetPassword(u) {
  const pw = prompt(`重设 ${u.nickname}（uid ${u.id}）的密码（≥8 位）：`, '')
  if (!pw) return
  error.value = ''
  try {
    await fetchJSON(`/api/admin/v1/users/${u.id}/password`, {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ password: pw }),
    })
    alert(`已重设：该用户全部令牌与启动器授权已失效。`)
  } catch (e) { error.value = e.message }
}

async function adminDeleteUser(u) {
  if (!confirm(`删除用户 ${u.nickname}（uid ${u.id}）？会话/令牌/衣柜/角色一并删除，纹理与审计保留。不可恢复！`)) return
  error.value = ''
  try {
    await fetchJSON(`/api/admin/v1/users/${u.id}`, { method: 'DELETE' })
    users.value = users.value.filter((x) => x.id !== u.id)
  } catch (e) { error.value = e.message }
}

onMounted(load)
</script>

<template>
  <div class="card">
    <h2 style="margin-top: 0;">{{ t('admin.usersTitle') }}</h2>
    <div class="error" v-if="error">{{ error }}</div>
    <table v-if="users.length">
      <tr><th>{{ t('admin.colId') }}</th><th>{{ t('admin.colNickname') }}</th><th>{{ t('admin.colEmail') }}</th><th>{{ t('admin.colAlgo') }}</th><th>{{ t('admin.colCreated') }}</th><th></th></tr>
      <tr v-for="u in users" :key="u.id">
        <td>{{ u.id }}</td>
        <td>{{ u.nickname }}</td>
        <td class="muted">{{ u.email || '—' }}</td>
        <td><span class="badge" :style="u.passwordAlgo.startsWith('legacy') ? 'border-color: var(--danger); color: var(--danger);' : ''">{{ u.passwordAlgo }}</span></td>
        <td class="muted">{{ (u.createdAt || '').slice(0, 10) }}</td>
        <td>
          <button class="secondary" @click="adminResetPassword(u)">改密</button>
          <button class="secondary" v-if="u.id !== 1" @click="adminDeleteUser(u)" style="margin-left: 6px;">删除</button>
        </td>
      </tr>
    </table>
  </div>
</template>
