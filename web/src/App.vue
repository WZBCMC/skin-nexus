<script setup>
import { ref, computed, onMounted } from 'vue'
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
// Admin panel entry (blessing parity): only accounts with permission > 0 see
// it — normal users never do. `permission` comes from GET /api/v1/me
// (0 = normal, >0 = admin, -1 = banned).
const isAdmin = computed(() => Number(user.value && user.value.permission) > 0)
// login | register | dashboard | admin | device | apps | union | setup
const view = ref('login')
// Per-card messages (review P1-3): one shared ref rendered in four cards.
const authErr = ref('') // login + register cards
const dataErr = ref('') // top-level data-load banner (review P1-6)
const uploadErr = ref('')
const uploadOk = ref('')
const closetErr = ref('')
const closetOk = ref('')
const playerErr = ref('')
const playerOk = ref('')
const pwErr = ref('')

// login form
const identifier = ref('')
const password = ref('')
const email = ref('')
const nickname = ref('')
const invitationCode = ref('')

// password change (dashboard)
const curPw = ref('')
const newPw = ref('')
const pwDone = ref('')
const pwBusy = ref(false) // in-flight flag: no double submit (review P2-9)

// data
const players = ref([])
const textures = ref([])
const newPlayerName = ref('')
const uploadType = ref('skin')
const uploadModel = ref('default')
const uploadFile = ref(null)
const fileInput = ref(null) // DOM handle so we can clear the stale filename
const uploading = ref(false)

// closet (衣柜)
const closet = ref([])
const closetIds = computed(() => new Set(closet.value.map((c) => c.textureId)))

// 401 on any authenticated call means the session is gone (review P1-6):
// drop to the login form instead of leaving a dead screen behind.
function guard(e) {
  if (e && e.status === 401) {
    user.value = null
    view.value = 'login'
    return true
  }
  return false
}

async function addToCloset(tx) {
  closetErr.value = ''; closetOk.value = ''
  try {
    await api.closetAdd(tx.id)
    const cl = await api.closet()
    closet.value = cl.closet || []
    closetOk.value = t('dashboard.closetAdded')
  } catch (e) { if (!guard(e)) closetErr.value = e.message }
}

async function renameCloset(entry) {
  const next = prompt(t('dashboard.closetRenamePrompt'), entry.itemName || '')
  if (next === null) return
  closetErr.value = ''
  try {
    await api.closetRename(entry.id, next)
    const cl = await api.closet()
    closet.value = cl.closet || []
  } catch (e) { if (!guard(e)) closetErr.value = e.message }
}

async function removeCloset(entry) {
  closetErr.value = ''
  try {
    await api.closetRemove(entry.id)
    closet.value = closet.value.filter((c) => c.id !== entry.id)
  } catch (e) { if (!guard(e)) closetErr.value = e.message }
}

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
  } catch {
    // me() failed = no usable session (review P1-6: NOT a data failure)
    user.value = null
  }
  if (user.value) {
    try {
      await load()
    } catch (e) {
      // load() failed = data error, keep the session (review P1-6)
      if (!guard(e)) dataErr.value = t('common.dataLoadFailed', { message: e.message })
    }
  }
  if (isDevice) view.value = 'device'
  else view.value = user.value ? 'dashboard' : 'login'
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
  const [p, tx, cl] = await Promise.all([api.players(), api.textures(), api.closet()])
  players.value = p.players || []
  textures.value = tx.textures || []
  closet.value = cl.closet || []
}

async function doLogin() {
  authErr.value = ''
  try {
    user.value = await api.login(identifier.value, password.value)
    // login returns the bare {id, nickname} shape; me() carries `permission`
    // (admin-entry gating) — refresh so the entry appears without a reload.
    try { user.value = await api.me() } catch { /* keep login payload */ }
  } catch (e) { authErr.value = e.message; return }
  // returning from login initiated on the device page keeps the flow alive
  view.value = location.pathname === '/device' ? 'device' : 'dashboard'
  try {
    await load()
  } catch (e) { if (!guard(e)) dataErr.value = t('common.dataLoadFailed', { message: e.message }) }
}

async function doRegister() {
  authErr.value = ''
  try {
    user.value = await api.register(email.value, nickname.value, password.value, invitationCode.value.trim())
  } catch (e) { authErr.value = e.message; return }
  view.value = 'dashboard'
  try {
    await load()
  } catch (e) { if (!guard(e)) dataErr.value = t('common.dataLoadFailed', { message: e.message }) }
}

async function doLogout() {
  try {
    await api.logout()
  } catch { /* session may already be gone — still sign out locally */ }
  user.value = null
  view.value = 'login'
}

async function doChangePassword() {
  if (pwBusy.value) return // in-flight: ignore repeat clicks (review P2-9)
  pwErr.value = ''; pwDone.value = ''
  pwBusy.value = true
  try {
    await api.changePassword(curPw.value, newPw.value)
    pwDone.value = t('auth.pwChanged')
    curPw.value = ''; newPw.value = ''
  } catch (e) {
    pwDone.value = ''
    if (!guard(e)) pwErr.value = e.message
  } finally {
    pwBusy.value = false
  }
}

async function addPlayer() {
  playerErr.value = ''
  try {
    await api.createPlayer(newPlayerName.value)
    newPlayerName.value = ''
    await load()
  } catch (e) { if (!guard(e)) playerErr.value = e.message }
}

async function doUpload() {
  if (!uploadFile.value) { uploadErr.value = t('dashboard.pickPng'); return }
  uploadErr.value = ''; uploadOk.value = ''; uploading.value = true
  try {
    await uploadTexture(uploadType.value, uploadFile.value, uploadType.value === 'skin' ? uploadModel.value : '', '')
    uploadOk.value = t('dashboard.uploadSuccess')
    uploadFile.value = null
    // review P3-17: clear the DOM value too, or a second 上传 press shows
    // 「请选择 PNG 文件」 next to a stale filename.
    if (fileInput.value) fileInput.value.value = ''
    await load()
  } catch (e) { if (!guard(e)) uploadErr.value = e.message } finally { uploading.value = false }
}

function onPick(e) { uploadFile.value = e.target.files[0] || null }

// currentTid echoes the player's mounted texture in the <select> (review B
// P2: without a :value binding the picker always showed 无皮肤 — users then
// "re-set" nothing or cleared a working skin by accident). Matches the
// served URL's hash suffix against the texture list.
function currentTid(p, kind) {
  const url = (kind === 'skin' ? (p.SkinURL || p.skinurl) : (p.CapeURL || p.capeurl)) || ''
  if (!url) return ''
  const tx = textures.value.find(x => url.endsWith(x.hash))
  return tx ? String(tx.id) : ''
}

async function setSkin(p, e) {
  const v = parseInt(e.target.value || '0', 10)
  playerErr.value = ''; playerOk.value = ''
  try {
    await api.setSkin(p.ID ?? p.id, v)
    playerOk.value = t('dashboard.skinUpdated')
  } catch (err) {
    if (!guard(err)) playerErr.value = err.message
  } finally {
    // review P2-7: a failing reload must not escape the change handler
    try { await load() } catch (err) { if (!guard(err)) playerErr.value = err.message }
  }
}

async function setCape(p, e) {
  const v = parseInt(e.target.value || '0', 10)
  playerErr.value = ''; playerOk.value = ''
  try {
    await api.setCape(p.ID ?? p.id, v)
    playerOk.value = t('dashboard.skinUpdated')
  } catch (err) {
    if (!guard(err)) playerErr.value = err.message
  } finally {
    try { await load() } catch (err) { if (!guard(err)) playerErr.value = err.message }
  }
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
          <button v-if="isAdmin" class="secondary" @click="view = view === 'admin' ? 'dashboard' : 'admin'">
            {{ view === 'admin' ? t('common.back') : t('common.admin') }}
          </button>
          <button class="secondary" @click="doLogout">{{ t('common.logout') }}</button>
        </template>
      </div>
    </div>

    <!-- data-load failure banner: stays visible without dropping to login -->
    <div class="error" v-if="dataErr">{{ dataErr }}</div>

    <!-- LOGIN -->
    <div v-if="view === 'login'" class="card" style="max-width: 420px;">
      <h2>{{ t('auth.loginTitle') }}</h2>
      <label>{{ t('auth.identifier') }}</label>
      <input v-model="identifier" autocomplete="username" />
      <label>{{ t('auth.password') }}</label>
      <input v-model="password" type="password" autocomplete="current-password" @keyup.enter="doLogin" />
      <div class="error" v-if="authErr">{{ authErr }}</div>
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
      <label>{{ t('auth.invitationCode') }}</label>
      <input v-model="invitationCode" @keyup.enter="doRegister" />
      <div class="error" v-if="authErr">{{ authErr }}</div>
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
          <input type="file" accept="image/png" ref="fileInput" @change="onPick" style="width: auto;" />
          <button :disabled="uploading" @click="doUpload">{{ uploading ? t('dashboard.uploading') : t('dashboard.upload') }}</button>
        </div>
        <div class="error" v-if="uploadErr">{{ uploadErr }}</div>
        <div class="success" v-if="uploadOk">{{ uploadOk }}</div>
        <div class="muted" style="margin-top: 8px;">{{ t('dashboard.uploadHint') }}</div>
      </div>

      <!-- textures -->
      <div class="card">
        <h2>{{ t('dashboard.texturesTitle', { count: textures.length }) }}</h2>
        <table v-if="textures.length">
          <tr><th>{{ t('dashboard.colType') }}</th><th>{{ t('dashboard.colModel') }}</th><th>{{ t('dashboard.colPreview') }}</th><th>{{ t('dashboard.colHash') }}</th><th></th></tr>
          <tr v-for="tx in textures" :key="tx.id">
            <td>{{ tx.type }}</td>
            <td>{{ tx.model || '—' }}</td>
            <td><img :src="tx.url" style="height: 48px; image-rendering: pixelated;" /></td>
            <td class="muted">{{ tx.hash.slice(0, 12) }}…</td>
            <td>
              <button class="secondary" v-if="!closetIds.has(tx.id)" @click="addToCloset(tx)">{{ t('dashboard.closetAdd') }}</button>
              <span class="muted" v-else style="font-size: 12px;">{{ t('dashboard.closetIn') }}</span>
            </td>
          </tr>
        </table>
        <div v-else class="muted">{{ t('dashboard.texturesEmpty') }}</div>
      </div>

      <!-- closet -->
      <div class="card">
        <h2>{{ t('dashboard.closetTitle', { count: closet.length }) }}</h2>
        <table v-if="closet.length">
          <tr><th>{{ t('dashboard.colPreview') }}</th><th>{{ t('dashboard.colType') }}</th><th>{{ t('dashboard.closetName') }}</th><th></th></tr>
          <tr v-for="c in closet" :key="c.id">
            <td><img v-if="c.url" :src="c.url" style="height: 48px; image-rendering: pixelated;" /><span v-else class="muted">—</span></td>
            <td>{{ c.type || '—' }}</td>
            <td>{{ c.itemName || '—' }}</td>
            <td>
              <button class="secondary" @click="renameCloset(c)">{{ t('dashboard.closetRename') }}</button>
              <button class="secondary" @click="removeCloset(c)" style="margin-left: 6px;">{{ t('dashboard.closetRemove') }}</button>
            </td>
          </tr>
        </table>
        <div v-else class="muted">{{ t('dashboard.closetEmpty') }}</div>
        <div class="error" v-if="closetErr">{{ closetErr }}</div>
        <div class="success" v-if="closetOk">{{ closetOk }}</div>
      </div>

      <!-- players -->
      <div class="card">
        <h2>{{ t('dashboard.playersTitle', { count: players.length }) }}</h2>
        <div class="row">
          <input v-model="newPlayerName" :placeholder="t('dashboard.newPlayerName')" style="max-width: 200px;" />
          <button @click="addPlayer">{{ t('dashboard.createPlayer') }}</button>
        </div>
        <div class="error" v-if="playerErr">{{ playerErr }}</div>
        <div class="success" v-if="playerOk">{{ playerOk }}</div>
        <div v-for="p in players" :key="p.id" class="row" style="margin-top: 12px; border-top: 1px solid var(--border); padding-top: 12px;">
          <div style="width: 120px;">
            <strong>{{ p.Name || p.name }}</strong>
            <div class="muted" style="font-size: 11px;">{{ (p.UUID || p.uuid || '').slice(0, 16) }}…</div>
          </div>
          <SkinPreview :url="p.SkinURL || p.skinurl" />
          <div class="row">
            <select style="width: 220px;" :value="currentTid(p, 'skin')" @change="setSkin(p, $event)">
              <option value="">{{ t('dashboard.noSkin') }}</option>
              <option v-for="tx in textures.filter(x => x.type === 'skin')" :key="tx.id" :value="String(tx.id)">{{ tx.hash.slice(0, 10) }}…</option>
            </select>
            <select style="width: 200px;" :value="currentTid(p, 'cape')" @change="setCape(p, $event)">
              <option value="">{{ t('dashboard.noCape') }}</option>
              <option v-for="tx in textures.filter(x => x.type === 'cape')" :key="tx.id" :value="String(tx.id)">{{ tx.hash.slice(0, 10) }}…</option>
            </select>
          </div>
        </div>
      </div>

      <!-- password change -->
      <div class="card">
        <h2>{{ t('auth.pwTitle') }}</h2>
        <div class="row">
          <input v-model="curPw" type="password" :placeholder="t('auth.pwCurrent')" style="max-width: 220px;" />
          <input v-model="newPw" type="password" :placeholder="t('auth.pwNew')" style="max-width: 220px;" @keyup.enter="doChangePassword" />
          <button :disabled="pwBusy" @click="doChangePassword">{{ t('auth.pwSubmit') }}</button>
        </div>
        <div class="error" v-if="pwErr">{{ pwErr }}</div>
        <div class="success" v-if="pwDone">{{ pwDone }}</div>
        <div class="muted" style="margin-top: 8px; font-size: 12px;">{{ t('auth.pwHint') }}</div>
      </div>
    </template>
  </div>
</template>
