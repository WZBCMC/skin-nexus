<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from './api.js'
import AdminPanel from './components/AdminPanel.vue'
import DevicePage from './pages/DevicePage.vue'
import AuthorizedAppsPage from './pages/AuthorizedAppsPage.vue'
import UnionBindPage from './pages/UnionBindPage.vue'
import SetupPage from './pages/SetupPage.vue'
import DashboardPage from './pages/DashboardPage.vue'
import ClosetPage from './pages/ClosetPage.vue'
import SkinlibPage from './pages/SkinlibPage.vue'
import PlayersPage from './pages/PlayersPage.vue'
import ProfilePage from './pages/ProfilePage.vue'
import { setLocale } from './i18n.js'
import { setUnauthorizedHandler } from './session.js'

const { t, locale } = useI18n()

const user = ref(null)
// Admin panel entry (blessing parity): only accounts with permission > 0 see
// it — normal users never do. `permission` comes from GET /api/v1/me
// (0 = normal, >0 = admin, -1 = banned).
const isAdmin = computed(() => Number(user.value && user.value.permission) > 0)
// login | register | setup | device | dashboard | closet | players |
// profile | apps | union | admin
const view = ref('login')
const authErr = ref('') // login + register cards
const dataErr = ref('') // top-level data-load banner (review P1-6)

// login form
const identifier = ref('')
const password = ref('')
const email = ref('')
const nickname = ref('')
const invitationCode = ref('')

// Sidebar nav (blessing parity: 用户中心/管理 两组；浏览组随 skinlib B 级后置).
// device/apps/union are user-center entries too — kept out of the sidebar to
// preserve the launcher-OAuth flow's focused page (they stay reachable from
// the dashboard quick-config copy and the legacy top-row buttons below).
const navGroups = computed(() => {
  if (!user.value) return []
  const groups = [{
    items: [
      { id: 'dashboard', labelKey: 'nav.dashboard' },
      { id: 'skinlib', labelKey: 'nav.skinlib' },
      { id: 'closet', labelKey: 'nav.closet' },
      { id: 'players', labelKey: 'nav.players' },
      { id: 'profile', labelKey: 'nav.profile' },
      { id: 'apps', labelKey: 'nav.apps' },
      { id: 'union', labelKey: 'nav.union' },
    ],
  }]
  if (isAdmin.value) groups.push({ items: [{ id: 'admin', labelKey: 'nav.adminPanel' }] })
  return groups
})

// Session-drop handler: pages call notifyUnauthorized() on 401 → back to login.
setUnauthorizedHandler(() => { user.value = null; view.value = 'login' })

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
  if (isDevice) view.value = 'device'
  else view.value = user.value ? 'dashboard' : 'login'
}

function backToDashboard() {
  // restore the site root URL when leaving the device page
  if (location.pathname === '/device') history.replaceState(null, '', '/')
  view.value = 'dashboard'
}

function onLocaleChange(e) {
  setLocale(e.target.value)
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
}

async function doRegister() {
  authErr.value = ''
  try {
    user.value = await api.register(email.value, nickname.value, password.value, invitationCode.value.trim())
  } catch (e) { authErr.value = e.message; return }
  view.value = 'dashboard'
}

async function doLogout() {
  try {
    await api.logout()
  } catch { /* session may already be gone — still sign out locally */ }
  user.value = null
  view.value = 'login'
}

onMounted(boot)
</script>

<template>
  <div class="container" :class="{ wide: view === 'admin' }">
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

    <!-- logged-in shell: sidebar + page (blessing 用户中心布局) -->
    <template v-else-if="user">
      <div class="user-shell">
        <aside class="card user-side">
          <nav v-for="(g, gi) in navGroups" :key="gi" class="nav-group">
            <div class="nav-title" v-if="g.items.length">{{ gi === 0 ? t('nav.userCenter') : t('nav.manageGroup') }}</div>
            <button v-for="n in g.items" :key="n.id" class="nav-item"
                    :class="{ active: view === n.id }" @click="view = n.id">
              {{ t(n.labelKey) }}
            </button>
          </nav>
        </aside>
        <main class="user-main">
          <DashboardPage v-if="view === 'dashboard'" />
          <SkinlibPage v-else-if="view === 'skinlib'" />
          <ClosetPage v-else-if="view === 'closet'" />
          <PlayersPage v-else-if="view === 'players'" />
          <ProfilePage v-else-if="view === 'profile'" />
          <AuthorizedAppsPage v-else-if="view === 'apps'" />
          <UnionBindPage v-else-if="view === 'union'" />
          <AdminPanel v-else-if="view === 'admin'" />
        </main>
      </div>
    </template>
  </div>
</template>

<style scoped>
.user-shell { display: flex; gap: 16px; align-items: flex-start; }
.user-side { position: sticky; top: 16px; min-width: 170px; padding: 14px; }
.user-main { flex: 1; min-width: 0; }
.nav-group + .nav-group { margin-top: 14px; }
.nav-title {
  font-size: 12px; color: var(--muted); font-weight: 600;
  margin: 0 0 6px; padding-left: 10px;
}
.nav-item {
  display: block; width: 100%; text-align: left;
  background: none; border: 1px solid transparent;
  padding: 8px 10px; border-radius: 8px; cursor: pointer;
  font-size: 14px; color: inherit; margin-bottom: 2px;
}
.nav-item:hover { border-color: var(--border); }
.nav-item.active { border-color: var(--accent); font-weight: 600; }
@media (max-width: 720px) {
  .user-shell { flex-direction: column; }
  .user-side { position: static; width: 100%; }
  .nav-group { display: flex; flex-wrap: wrap; gap: 4px; align-items: center; }
  .nav-title { width: 100%; margin-bottom: 2px; }
  .nav-item { width: auto; margin-bottom: 0; }
}
</style>
