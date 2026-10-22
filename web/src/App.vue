<script setup>
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api, uploadTexture } from './api.js'
import SkinPreview from './components/SkinPreview.vue'
import AdminPanel from './components/AdminPanel.vue'
import DevicePage from './pages/DevicePage.vue'
import AuthorizedAppsPage from './pages/AuthorizedAppsPage.vue'
import UnionBindPage from './pages/UnionBindPage.vue'
import SetupPage from './pages/SetupPage.vue'
import { setLocale } from './i18n.js'

const { t, locale } = useI18n()

const user = ref(null)
// login | register | dashboard | admin | device | apps | union | setup
const view = ref('login')
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
  // Fresh install: the wizard takes over before anything else (empty-DB init).
  try {
    const r = await fetch('/api/v1/setup/status')
    const d = await r.json()
    if (d.needs_setup) { view.value = 'setup'; return }
  } catch { /* server down or old backend — fall through */ }
  // Launcher verification page: /device?user_code=… is path-probed so the
  // minimal /device.html fallback and the SPA share one URL contract.
  const isDevice = location.pathname === '/device'
  try {
    user.value = await api.me()
    await load()
  } catch {
    view.value = 'login'
  }
  if (isDevice) view.value = 'device'
}

async function goApps() {
  view.value = 'apps'
}

function backToDashboard() {
  // restore the site root URL when leaving the device page
  if (location.pathname === '/device') history.replaceState(null, '', '/')
  view.value = 'dashboard'
}

function onLocaleChange(e) {
  setLocale(e.target.value)
}

async function load() {
  const [p, tx] = await Promise.all([api.players(), api.textures()])
  players.value = p.players || []
  textures.value = tx.textures || []
}

async function doLogin() {
  error.value = ''
  try {
    user.value = await api.login(identifier.value, password.value)
    // returning from login initiated on the device page keeps the flow alive
    view.value = location.pathname === '/device' ? 'device' : 'dashboard'
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
  if (!uploadFile.value) { error.value = t('dashboard.pickPng'); return }
  error.value = ''; success.value = ''; uploading.value = true
  try {
    await uploadTexture(uploadType.value, uploadFile.value, uploadType.value === 'skin' ? uploadModel.value : '', '')
    success.value = t('dashboard.uploadSuccess')
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
        <h1>{{ t('site.name') }} <span class="muted">{{ t('site.subtitle') }}</span></h1>
        <div class="muted">{{ t('site.tagline') }}</div>
      </div>
      <div class="row">
        <select :value="locale" @change="onLocaleChange" style="width: 110px;">
          <option value="zh-CN">中文</option>
          <option value="en">English</option>
        </select>
        <template v-if="user">
          <span class="badge">{{ user.nickname }}</span>
          <button v-if="view === 'dashboard'" class="secondary" @click="goApps">{{ t('nav.apps') }}</button>
          <button v-if="view === 'dashboard'" class="secondary" @click="view = 'union'">{{ t('nav.union') }}</button>
          <button v-if="view === 'apps' || view === 'union'" class="secondary" @click="view = 'dashboard'">{{ t('common.back') }}</button>
          <button class="secondary" @click="view = view === 'admin' ? 'dashboard' : 'admin'">
            {{ view === 'admin' ? t('common.back') : t('common.admin') }}
          </button>
          <button class="secondary" @click="doLogout">{{ t('common.logout') }}</button>
        </template>
      </div>
    </div>

    <!-- LOGIN -->
    <div v-if="view === 'login'" class="card" style="max-width: 420px;">
      <h2>{{ t('auth.loginTitle') }}</h2>
      <label>{{ t('auth.identifier') }}</label>
      <input v-model="identifier" autocomplete="username" />
      <label>{{ t('auth.password') }}</label>
      <input v-model="password" type="password" autocomplete="current-password" @keyup.enter="doLogin" />
      <div class="error" v-if="error">{{ error }}</div>
      <div class="row" style="margin-top: 14px;">
        <button @click="doLogin">{{ t('common.login') }}</button>
        <button class="secondary" @click="view = 'register'">{{ t('common.register') }}</button>
      </div>
    </div>

    <!-- REGISTER -->
    <div v-else-if="view === 'register'" class="card" style="max-width: 420px;">
      <h2>{{ t('auth.registerTitle') }}</h2>
      <label>{{ t('auth.nickname') }}</label>
      <input v-model="nickname" />
      <label>{{ t('auth.email') }}</label>
      <input v-model="email" type="email" />
      <label>{{ t('auth.passwordMin') }}</label>
      <input v-model="password" type="password" @keyup.enter="doRegister" />
      <div class="error" v-if="error">{{ error }}</div>
      <div class="row" style="margin-top: 14px;">
        <button @click="doRegister">{{ t('common.register') }}</button>
        <button class="secondary" @click="view = 'login'">{{ t('auth.toLogin') }}</button>
      </div>
    </div>

    <!-- SETUP wizard (first run) -->
    <template v-else-if="view === 'setup'">
      <SetupPage />
    </template>

    <!-- DEVICE verification (launcher OAuth) -->
    <template v-else-if="view === 'device'">
      <div v-if="!user" class="card" style="max-width: 420px;">
        <h2>{{ t('device.needLogin') }}</h2>
        <p class="muted">{{ t('device.needLoginHint') }}</p>
        <button @click="view = 'login'">{{ t('device.goLogin') }}</button>
      </div>
      <template v-else>
        <DevicePage />
        <div class="row" style="margin-top: 12px;">
          <button class="secondary" @click="backToDashboard">{{ t('nav.backToDashboard') }}</button>
        </div>
      </template>
    </template>

    <!-- AUTHORIZED APPS -->
    <template v-else-if="view === 'apps'">
      <AuthorizedAppsPage />
    </template>

    <!-- UNION BINDING -->
    <template v-else-if="view === 'union'">
      <UnionBindPage />
    </template>

    <!-- DASHBOARD -->
    <template v-else-if="view === 'admin'">
      <AdminPanel />
    </template>

    <template v-else>
      <!-- upload -->
      <div class="card">
        <h2>{{ t('dashboard.uploadTitle') }}</h2>
        <div class="row">
          <select v-model="uploadType" style="width: 120px;">
            <option value="skin">{{ t('dashboard.typeSkin') }}</option>
            <option value="cape">{{ t('dashboard.typeCape') }}</option>
          </select>
          <select v-model="uploadModel" style="width: 140px;" :disabled="uploadType !== 'skin'">
            <option value="default">{{ t('dashboard.modelDefault') }}</option>
            <option value="slim">{{ t('dashboard.modelSlim') }}</option>
          </select>
          <input type="file" accept="image/png" @change="onPick" style="width: auto;" />
          <button :disabled="uploading" @click="doUpload">{{ uploading ? t('dashboard.uploading') : t('dashboard.upload') }}</button>
        </div>
        <div class="error" v-if="error">{{ error }}</div>
        <div class="success" v-if="success">{{ success }}</div>
        <div class="muted" style="margin-top: 8px;">{{ t('dashboard.uploadHint') }}</div>
      </div>

      <!-- textures -->
      <div class="card">
        <h2>{{ t('dashboard.texturesTitle', { count: textures.length }) }}</h2>
        <table v-if="textures.length">
          <tr><th>{{ t('dashboard.colType') }}</th><th>{{ t('dashboard.colModel') }}</th><th>{{ t('dashboard.colPreview') }}</th><th>{{ t('dashboard.colHash') }}</th></tr>
          <tr v-for="t in textures" :key="t.id">
            <td>{{ t.type }}</td>
            <td>{{ t.model || '—' }}</td>
            <td><img :src="t.url" style="height: 48px; image-rendering: pixelated;" /></td>
            <td class="muted">{{ t.hash.slice(0, 12) }}…</td>
          </tr>
        </table>
        <div v-else class="muted">{{ t('dashboard.texturesEmpty') }}</div>
      </div>

      <!-- players -->
      <div class="card">
        <h2>{{ t('dashboard.playersTitle', { count: players.length }) }}</h2>
        <div class="row">
          <input v-model="newPlayerName" :placeholder="t('dashboard.newPlayerName')" style="max-width: 200px;" />
          <button @click="addPlayer">{{ t('dashboard.createPlayer') }}</button>
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
              <option value="">{{ t('dashboard.noSkin') }}</option>
              <option v-for="tx in textures.filter(x => x.type === 'skin')" :key="tx.id" :value="tx.id">{{ tx.hash.slice(0, 10) }}…</option>
            </select>
            <select style="width: 200px;" @change="setCape(p, $event)">
              <option value="">{{ t('dashboard.noCape') }}</option>
              <option v-for="tx in textures.filter(x => x.type === 'cape')" :key="tx.id" :value="tx.id">{{ tx.hash.slice(0, 10) }}…</option>
            </select>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>
