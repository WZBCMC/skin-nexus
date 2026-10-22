<script setup>
import { ref, onMounted } from 'vue'

const token = ref(localStorage.getItem('admin_token') || '')
const overview = ref(null)
const users = ref([])
const error = ref('')

async function fetchJSON(path) {
  const res = await fetch(path, { headers: { 'X-Admin-Token': token.value } })
  const data = await res.json().catch(() => ({}))
  if (!res.ok) throw new Error(data.message || `HTTP ${res.status}`)
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
  </div>
</template>
