<script setup>
import { ref, onMounted } from 'vue'

const token = ref(localStorage.getItem('admin_token') || '')
const overview = ref(null)
const users = ref([])
const unionStatus = ref(null)
const unionError = ref('')
const oidcClients = ref([])
const oidcError = ref('')
const newClient = ref({ client_id: '', name: '', redirect_uris: '', public: true })
const lastSecret = ref('') // shown once after create
const error = ref('')

async function fetchJSON(path, opts) {
  const res = await fetch(path, { ...opts, headers: { 'X-Admin-Token': token.value, ...(opts && opts.headers) } })
  const data = await res.json().catch(() => ({}))
  if (!res.ok) throw new Error(data.message || data.error || `HTTP ${res.status}`)
  return data
}

async function load() {
  error.value = ''
  try {
    localStorage.setItem('admin_token', token.value)
    const [o, u] = await Promise.all([
      fetchJSON('/api/admin/v1/overview'),
      fetchJSON('/api/admin/v1/users?page=1'),
    ])
    overview.value = o
    users.value = u.users || []
  } catch (e) { error.value = e.message }
  // Union status: absent (404) when the module is disabled — that's expected.
  unionError.value = ''
  try {
    unionStatus.value = await fetchJSON('/api/admin/v1/union/status')
  } catch (e) {
    unionStatus.value = null
    unionError.value = e.message
  }
  // OIDC clients
  oidcError.value = ''
  try {
    const c = await fetchJSON('/api/admin/v1/oidc/clients')
    oidcClients.value = c.clients || []
  } catch (e) { oidcError.value = e.message }
}

async function createClient() {
  oidcError.value = ''; lastSecret.value = ''
  const body = {
    client_id: newClient.value.client_id.trim(),
    name: newClient.value.name.trim() || newClient.value.client_id.trim(),
    redirect_uris: newClient.value.redirect_uris.split('\n').map(s => s.trim()).filter(Boolean),
    grant_types: ['authorization_code', 'refresh_token'],
    public: newClient.value.public,
  }
  if (!body.client_id) { oidcError.value = 'client_id 必填'; return }
  try {
    const d = await fetchJSON('/api/admin/v1/oidc/clients', {
      method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body),
    })
    if (d.secret) lastSecret.value = d.secret // only visible now — copy it!
    newClient.value = { client_id: '', name: '', redirect_uris: '', public: true }
    const c = await fetchJSON('/api/admin/v1/oidc/clients')
    oidcClients.value = c.clients || []
  } catch (e) { oidcError.value = e.message }
}

async function deleteClient(id) {
  if (!confirm(`删除 OAuth 客户端 ${id}？使用它的应用将无法登录。`)) return
  oidcError.value = ''
  try {
    await fetchJSON(`/api/admin/v1/oidc/clients/${encodeURIComponent(id)}`, { method: 'DELETE' })
    oidcClients.value = oidcClients.value.filter(c => c.client_id !== id)
  } catch (e) { oidcError.value = e.message }
}

async function resumeRotation() {
  if (!confirm('恢复签名钥轮换？（日变更超限后的手动恢复通道）')) return
  try {
    await fetchJSON('/api/admin/v1/union/rotation/resume', { method: 'POST' })
    await load()
  } catch (e) { unionError.value = e.message }
}

onMounted(() => { if (token.value) load() })
</script>

<template>
  <div>
    <div class="card">
      <h2>管理面板</h2>
      <div class="row">
        <input v-model="token" type="password" placeholder="X-Admin-Token" style="max-width: 320px;" />
        <button @click="load">加载</button>
      </div>
      <div class="error" v-if="error">{{ error }}</div>
    </div>

    <div class="card" v-if="overview">
      <h2>总览</h2>
      <div class="row">
        <span class="badge">用户 {{ overview.users }}</span>
        <span class="badge">角色 {{ overview.players }}</span>
        <span class="badge">纹理 {{ overview.textures }}</span>
        <span class="badge" :style="overview.weakHashUsers > 0 ? 'border-color: var(--danger); color: var(--danger);' : ''">
          弱哈希余量 {{ overview.weakHashUsers }}
        </span>
        <span class="badge">签名密钥 {{ overview.signKeyFingerprint }}</span>
      </div>
    </div>

    <div class="card" v-if="users.length">
      <h2>用户（最新 50）</h2>
      <table>
        <tr><th>ID</th><th>昵称</th><th>邮箱</th><th>密码算法</th><th>创建</th></tr>
        <tr v-for="u in users" :key="u.id">
          <td>{{ u.id }}</td>
          <td>{{ u.nickname }}</td>
          <td class="muted">{{ u.email || '—' }}</td>
          <td><span class="badge" :style="u.passwordAlgo.startsWith('legacy') ? 'border-color: var(--danger); color: var(--danger);' : ''">{{ u.passwordAlgo }}</span></td>
          <td class="muted">{{ (u.createdAt || '').slice(0, 10) }}</td>
        </tr>
      </table>
    </div>

    <!-- MUA Union module status -->
    <div class="card" v-if="unionStatus">
      <h2>Union 联盟模块</h2>
      <div class="row">
        <span class="badge">模式 {{ unionStatus.mode }}</span>
        <span class="badge">签名钥指纹 {{ unionStatus.key_fingerprint || '—' }}</span>
        <span class="badge">钥版本 {{ unionStatus.private_key_version || '—' }}</span>
        <span class="badge">服务器列表 v{{ unionStatus.server_list_version || '—' }}</span>
        <span class="badge">queryemail {{ unionStatus.queryemail_mode }}</span>
        <span class="badge" :style="unionStatus.rotation_suspended ? 'border-color: var(--danger); color: var(--danger);' : ''">
          轮换{{ unionStatus.rotation_suspended ? '已暂停' : '正常' }}
        </span>
      </div>
      <div class="row" style="margin-top: 10px;" v-if="unionStatus.rotation_suspended">
        <button class="secondary" @click="resumeRotation">恢复签名钥轮换</button>
      </div>
      <div class="muted" style="margin-top: 10px; font-size: 12px;">
        ⚠️ 信任声明：加入 MUA Union 后，本站的纹理签名私钥与联盟全网共享——
        任何成员站都可使用同一签名密钥发布纹理。请仅在加入联盟组织时启用本模块。
      </div>
      <div class="error" v-if="unionError">{{ unionError }}</div>
    </div>

    <!-- OIDC clients -->
    <div class="card">
      <h2>OAuth 客户端（{{ oidcClients.length }}）</h2>
      <div class="row">
        <input v-model="newClient.client_id" placeholder="client_id" style="max-width: 180px;" />
        <input v-model="newClient.name" placeholder="名称" style="max-width: 180px;" />
        <label style="display: flex; align-items: center; gap: 4px; white-space: nowrap;">
          <input type="checkbox" v-model="newClient.public" /> 公开客户端
        </label>
        <button @click="createClient">创建</button>
      </div>
      <textarea v-model="newClient.redirect_uris" placeholder="redirect_uris（每行一个，机密客户端用）" rows="2"
                style="margin-top: 8px; width: 100%;"></textarea>
      <div class="success" v-if="lastSecret" style="margin-top: 8px;">
        客户端密钥（仅显示这一次，请立即保存）：<code>{{ lastSecret }}</code>
      </div>
      <div class="error" v-if="oidcError">{{ oidcError }}</div>
      <table v-if="oidcClients.length" style="margin-top: 10px;">
        <tr><th>client_id</th><th>名称</th><th>类型</th><th>授权类型</th><th>redirect_uris</th><th></th></tr>
        <tr v-for="c in oidcClients" :key="c.client_id">
          <td><code>{{ c.client_id }}</code></td>
          <td>{{ c.name }}</td>
          <td><span class="badge">{{ c.public ? '公开' : '机密' }}</span></td>
          <td class="muted" style="font-size: 12px;">{{ (c.grant_types || []).join(', ') }}</td>
          <td class="muted" style="font-size: 12px;">{{ (c.redirect_uris || []).join(', ') || '—' }}</td>
          <td><button class="secondary" @click="deleteClient(c.client_id)">删除</button></td>
        </tr>
      </table>
    </div>
  </div>
</template>
