<script setup>
// AuthorizedAppsPage: lists the caller's live OIDC grants (jti ledger) and
// lets them revoke one grant or sign out everywhere.
// API: GET /api/v1/oidc/tokens, POST /api/v1/oidc/tokens/revoke {jti|all}.
import { ref, onMounted } from 'vue'
import { oidc } from '../api.js'

const tokens = ref([])
const error = ref('')
const success = ref('')
const loading = ref(false)

async function load() {
  error.value = ''
  try {
    const d = await oidc.tokens()
    tokens.value = d.tokens || []
  } catch (e) { error.value = e.message }
}

async function revoke(jti) {
  if (!confirm('吊销该授权？对应启动器将需要重新登录。')) return
  loading.value = true; error.value = ''; success.value = ''
  try {
    const d = await oidc.revokeToken({ jti })
    if (d.error) { error.value = d.error; return }
    success.value = '已吊销（含该应用的刷新令牌）'
    await load()
  } catch (e) { error.value = e.message } finally { loading.value = false }
}

async function revokeAll() {
  if (!confirm('退出所有已登录的应用/启动器？所有令牌立即失效。')) return
  loading.value = true; error.value = ''; success.value = ''
  try {
    const d = await oidc.revokeToken({ all: true })
    if (d.error) { error.value = d.error; return }
    success.value = '已退出全部应用'
    await load()
  } catch (e) { error.value = e.message } finally { loading.value = false }
}

function fmt(t) {
  return new Date(t).toLocaleString()
}

onMounted(load)
</script>

<template>
  <div class="card">
    <div class="row" style="justify-content: space-between;">
      <h2>已授权应用（{{ tokens.length }}）</h2>
      <button class="secondary" :disabled="loading || !tokens.length" @click="revokeAll">全部退出</button>
    </div>
    <div class="error" v-if="error">{{ error }}</div>
    <div class="success" v-if="success">{{ success }}</div>
    <table v-if="tokens.length">
      <tr><th>应用</th><th>权限</th><th>角色</th><th>授权时间</th><th>过期时间</th><th></th></tr>
      <tr v-for="t in tokens" :key="t.jti">
        <td><code style="font-size: 12px;">{{ t.client_id }}</code></td>
        <td class="muted" style="font-size: 12px;">{{ (t.scopes || []).join(', ') }}</td>
        <td>{{ t.selected_pid ? '#' + t.selected_pid : '—' }}</td>
        <td>{{ fmt(t.created_at) }}</td>
        <td>{{ fmt(t.expires_at) }}</td>
        <td><button class="secondary" :disabled="loading" @click="revoke(t.jti)">吊销</button></td>
      </tr>
    </table>
    <div v-else class="muted">当前没有生效的授权。用启动器 OAuth 登录后会出现在这里。</div>
  </div>
</template>
