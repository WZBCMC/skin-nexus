<script setup>
import { ref, onMounted, nextTick } from 'vue'
import { api, uploadTexture } from './api.js'
import SkinPreview from './components/SkinPreview.vue'

const user = ref(null)
const view = ref('login') // login | register | dashboard
const error = ref('')
const success = ref('')

// login form
const identifier = ref('')
const password = ref('')
const email = ref('')
const nickname = ref('')

// data
const players = ref([])
const textures = ref([])
const newPlayerName = ref('')
const uploadType = ref('skin')
const uploadModel = ref('default')
const uploadFile = ref(null)
const uploading = ref(false)

async function boot() {
  try { user.value = await api.me(); await load() } catch { view.value = 'login' }
}

async function load() {
  const [p, t] = await Promise.all([api.players(), api.textures()])
  players.value = p.players || []
  textures.value = t.textures || []
}

async function doLogin() {
  error.value = ''
  try {
    user.value = await api.login(identifier.value, password.value)
    view.value = 'dashboard'
    await load()
  } catch (e) { error.value = e.message }
}

async function doRegister() {
  error.value = ''
  try {
    user.value = await api.register(email.value, nickname.value, password.value)
    view.value = 'dashboard'
    await load()
  } catch (e) { error.value = e.message }
}

async function doLogout() {
  await api.logout()
  user.value = null
  view.value = 'login'
}

async function addPlayer() {
  error.value = ''
  try {
    await api.createPlayer(newPlayerName.value)
    newPlayerName.value = ''
    await load()
  } catch (e) { error.value = e.message }
}

async function doUpload() {
  if (!uploadFile.value) { error.value = '请选择 PNG 文件'; return }
  error.value = ''; success.value = ''; uploading.value = true
  try {
    await uploadTexture(uploadType.value, uploadFile.value, uploadType.value === 'skin' ? uploadModel.value : '', '')
    success.value = '上传成功'
    uploadFile.value = null
    await load()
  } catch (e) { error.value = e.message } finally { uploading.value = false }
}

function onPick(e) { uploadFile.value = e.target.files[0] || null }

async function setSkin(p, e) {
  const v = parseInt(e.target.value || '0', 10)
  await api.setSkin(p.ID ?? p.id, v)
  await load()
}

async function setCape(p, e) {
  const v = parseInt(e.target.value || '0', 10)
  await api.setCape(p.ID ?? p.id, v)
  await load()
}

onMounted(boot)
</script>

<template>
  <div class="container">
    <div class="row" style="justify-content: space-between; margin-bottom: 16px;">
      <div>
        <h1>SkinNexus <span class="muted">肤枢</span></h1>
        <div class="muted">Minecraft 皮肤站 · Yggdrasil 外置登录</div>
      </div>
      <div v-if="user" class="row">
        <span class="badge">{{ user.nickname }}</span>
        <button class="secondary" @click="doLogout">退出</button>
      </div>
    </div>

    <!-- LOGIN -->
    <div v-if="view === 'login'" class="card" style="max-width: 420px;">
      <h2>登录</h2>
      <label>邮箱或昵称</label>
      <input v-model="identifier" autocomplete="username" />
      <label>密码</label>
      <input v-model="password" type="password" autocomplete="current-password" @keyup.enter="doLogin" />
      <div class="error" v-if="error">{{ error }}</div>
      <div class="row" style="margin-top: 14px;">
        <button @click="doLogin">登录</button>
        <button class="secondary" @click="view = 'register'">注册</button>
      </div>
    </div>

    <!-- REGISTER -->
    <div v-else-if="view === 'register'" class="card" style="max-width: 420px;">
      <h2>注册</h2>
      <label>昵称（MC 风格）</label>
      <input v-model="nickname" />
      <label>邮箱（可留空）</label>
      <input v-model="email" type="email" />
      <label>密码（≥8 位）</label>
      <input v-model="password" type="password" @keyup.enter="doRegister" />
      <div class="error" v-if="error">{{ error }}</div>
      <div class="row" style="margin-top: 14px;">
        <button @click="doRegister">注册</button>
        <button class="secondary" @click="view = 'login'">返回登录</button>
      </div>
    </div>

    <!-- DASHBOARD -->
    <template v-else>
      <!-- upload -->
      <div class="card">
        <h2>上传纹理</h2>
        <div class="row">
          <select v-model="uploadType" style="width: 120px;">
            <option value="skin">皮肤</option>
            <option value="cape">披风</option>
          </select>
          <select v-model="uploadModel" style="width: 140px;" :disabled="uploadType !== 'skin'">
            <option value="default">default (Steve)</option>
            <option value="slim">slim (Alex)</option>
          </select>
          <input type="file" accept="image/png" @change="onPick" style="width: auto;" />
          <button :disabled="uploading" @click="doUpload">{{ uploading ? '上传中…' : '上传' }}</button>
        </div>
        <div class="error" v-if="error">{{ error }}</div>
        <div class="success" v-if="success">{{ success }}</div>
        <div class="muted" style="margin-top: 8px;">皮肤 64×64 / 64×32，披风 64×32 / 22×17（或等比放大）</div>
      </div>

      <!-- textures -->
      <div class="card">
        <h2>我的纹理（{{ textures.length }}）</h2>
        <table v-if="textures.length">
          <tr><th>类型</th><th>模型</th><th>预览</th><th>hash</th></tr>
          <tr v-for="t in textures" :key="t.id">
            <td>{{ t.type }}</td>
            <td>{{ t.model || '—' }}</td>
            <td><img :src="t.url" style="height: 48px; image-rendering: pixelated;" /></td>
            <td class="muted">{{ t.hash.slice(0, 12) }}…</td>
          </tr>
        </table>
        <div v-else class="muted">还没有纹理，先上传一个吧</div>
      </div>

      <!-- players -->
      <div class="card">
        <h2>我的角色（{{ players.length }}）</h2>
        <div class="row">
          <input v-model="newPlayerName" placeholder="新角色名" style="max-width: 200px;" />
          <button @click="addPlayer">创建角色</button>
        </div>
        <div class="error" v-if="error">{{ error }}</div>
        <div v-for="p in players" :key="p.id" class="row" style="margin-top: 12px; border-top: 1px solid var(--border); padding-top: 12px;">
          <div style="width: 120px;">
            <strong>{{ p.Name || p.name }}</strong>
            <div class="muted" style="font-size: 11px;">{{ (p.UUID || p.uuid || '').slice(0, 16) }}…</div>
          </div>
          <SkinPreview :url="p.SkinURL || p.skinurl" />
          <div class="row">
            <select style="width: 220px;" @change="setSkin(p, $event)">
              <option value="">无皮肤</option>
              <option v-for="t in textures.filter(t => t.type === 'skin')" :key="t.id" :value="t.id">{{ t.hash.slice(0, 10) }}…</option>
            </select>
            <select style="width: 200px;" @change="setCape(p, $event)">
              <option value="">无披风</option>
              <option v-for="t in textures.filter(t => t.type === 'cape')" :key="t.id" :value="t.id">{{ t.hash.slice(0, 10) }}…</option>
            </select>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>
