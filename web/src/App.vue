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
// A1 找回密码: forgot card state + /reset-password?token=… reset card state
const forgotEmail = ref('')
const forgotSent = ref(false)
const resetToken = ref('')
const resetPw1 = ref('')
const resetPw2 = ref('')
const resetDone = ref(false)
// Public site capabilities (GET /api/v1/site-config): the auth forms follow
// the deployment's real configuration — invite field required-or-absent,
// email field + forgot flow available-or-hidden. Until the fetch resolves
// (or if it fails) the pre-config UI stays = today's behavior.
const siteConf = ref({ invite_required: false, mail_configured: false })
const confLoaded = ref(false)
const inviteFieldShown = computed(() => !confLoaded.value || siteConf.value.invite_required)
const inviteIsRequired = computed(() => confLoaded.value && siteConf.value.invite_required)
const mailEnabled = computed(() => !confLoaded.value || siteConf.value.mail_configured)

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

// Launcher deep links (blessing route parity): PCL derives the skin-management
// and profile pages by swapping the API root's yggdrasil path for
// `user/closet` / `user/profile` (PageLoginProfileSkin.xaml.cs:151,
// ProfileUi.cs:291) — the SPA must honor those URLs or the launcher's "换皮肤 /
// 修改密码" buttons land on the dashboard. Same for HMCL's yggdrasil convention.
const legacyRoutes = {
  '/user/closet': 'closet',
  '/user/profile': 'profile',
  '/user/players': 'players',
  '/user/skinlib': 'skinlib',
  '/user/dashboard': 'dashboard',
  '/skinlib': 'skinlib',
}
// Deep-linked view held for after login (open /user/closet logged-out → login
// card first, then drop straight into the closet).
const pendingView = ref('')

async function boot() {
  // Fresh install: the wizard takes over before anything else (empty-DB init).
  try {
    const r = await fetch('/api/v1/setup/status')
    const d = await r.json()
    if (d.needs_setup) { view.value = 'setup'; return }
  } catch { /* server down or old backend — fall through */ }
  // Site capabilities for the auth cards (best-effort: failure keeps the
  // old always-show UI).
  try { siteConf.value = await api.siteConfig(); confLoaded.value = true } catch { /* old UI */ }
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
  // A1 找回密码: /reset-password?token=… comes back from the mail as a real
  // URL; /forgot-password is the shareable form of the login-card link.
  else if (location.pathname === '/reset-password') {
    resetToken.value = new URLSearchParams(location.search).get('token') || ''
    view.value = 'reset'
  }
  else if (location.pathname === '/forgot-password') view.value = 'forgot'
  else if (!user.value && location.pathname === '/register') view.value = 'register' // landing CTA deep link
  else if (legacyRoutes[location.pathname]) {
    // launcher deep link: logged in → straight there; logged out → login card,
    // resume into the target view after authentication. /skinlib is public:
    // anonymous visitors browse directly (backend answers public textures
    // only) — no login wall.
    if (user.value) view.value = legacyRoutes[location.pathname]
    else if (location.pathname === '/skinlib') view.value = 'skinlib'
    else pendingView.value = legacyRoutes[location.pathname]
  }
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
  // returning from login initiated on the device page keeps the flow alive;
  // a launcher deep link resumes into its target view after authentication
  if (location.pathname === '/device') view.value = 'device'
  else if (pendingView.value) { view.value = pendingView.value; pendingView.value = '' }
  else view.value = 'dashboard'
}

async function doRegister() {
  authErr.value = ''
  // Invite required → fail fast client-side with a clear message instead of
  // letting the server 400 the whole submission.
  if (inviteIsRequired.value && !invitationCode.value.trim()) {
    authErr.value = t('auth.inviteRequiredErr'); return
  }
  try {
    user.value = await api.register(email.value, nickname.value, password.value, invitationCode.value.trim())
  } catch (e) { authErr.value = e.message; return }
  view.value = 'dashboard'
}

// A1 找回密码: forgot always shows the same sent-state (server is
// enumeration-safe); reset validates the two fields client-side first.
async function doForgot() {
  authErr.value = ''
  try {
    await api.forgotPassword(forgotEmail.value.trim())
    forgotSent.value = true
  } catch (e) { authErr.value = e.message }
}

async function doReset() {
  authErr.value = ''
  if (resetPw1.value !== resetPw2.value) { authErr.value = t('auth.resetMismatch'); return }
  try {
    await api.resetPassword(resetToken.value, resetPw1.value)
    resetDone.value = true
  } catch (e) { authErr.value = e.message }
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
        <template v-else>
          <button class="secondary" @click="view = 'skinlib'">{{ t('nav.skinlib') }}</button>
          <button class="secondary" @click="view = 'login'">{{ t('common.login') }}</button>
          <button @click="view = 'register'">{{ t('common.register') }}</button>
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
      <div style="margin-top: 8px; text-align: right;" v-if="mailEnabled">
        <a href="/forgot-password" class="muted" @click.prevent="forgotSent = false; view = 'forgot'">{{ t('auth.forgotLink') }}</a>
      </div>
      <div class="error" v-if="authErr">{{ authErr }}</div>
      <div class="row" style="margin-top: 16px;">
        <button @click="doLogin">{{ t('common.login') }}</button>
        <button class="secondary" @click="view = 'register'">{{ t('common.register') }}</button>
      </div>
    </div>

    <!-- REGISTER -->
    <div v-else-if="view === 'register'" class="card" style="max-width: 420px;">
      <h2>{{ t('auth.registerTitle') }}</h2>
      <label>{{ t('auth.nickname') }}</label>
      <input v-model="nickname" />
      <template v-if="mailEnabled">
        <label>{{ t('auth.email') }}</label>
        <input v-model="email" type="email" />
      </template>
      <label>{{ t('auth.passwordMin') }}</label>
      <input v-model="password" type="password" @keyup.enter="doRegister" />
      <template v-if="inviteFieldShown">
        <label :class="{ required: inviteIsRequired }">{{ inviteIsRequired ? t('auth.invitationCodeRequired') : t('auth.invitationCode') }}</label>
        <input v-model="invitationCode" @keyup.enter="doRegister" />
      </template>
      <div class="error" v-if="authErr">{{ authErr }}</div>
      <div class="row" style="margin-top: 16px;">
        <button @click="doRegister">{{ t('common.register') }}</button>
        <button class="secondary" @click="view = 'login'">{{ t('auth.toLogin') }}</button>
      </div>
    </div>

    <!-- FORGOT PASSWORD (A1 找回密码) -->
    <div v-else-if="view === 'forgot'" class="card" style="max-width: 420px;">
      <h2>{{ t('auth.forgotTitle') }}</h2>
      <!-- mail unconfigured → the flow is honestly unavailable (direct
           navigation to /forgot-password lands here too) -->
      <template v-if="confLoaded && !siteConf.mail_configured">
        <p class="muted">{{ t('auth.mailDisabled') }}</p>
        <div class="row" style="margin-top: 16px;">
          <button class="secondary" @click="view = 'login'">{{ t('auth.toLogin') }}</button>
        </div>
      </template>
      <template v-else-if="!forgotSent">
        <label>{{ t('auth.forgotEmail') }}</label>
        <input v-model="forgotEmail" type="email" autocomplete="email" @keyup.enter="doForgot" />
        <div class="error" v-if="authErr">{{ authErr }}</div>
        <div class="row" style="margin-top: 16px;">
          <button @click="doForgot">{{ t('auth.forgotSubmit') }}</button>
          <button class="secondary" @click="view = 'login'">{{ t('auth.toLogin') }}</button>
        </div>
      </template>
      <template v-else>
        <p class="muted">{{ t('auth.forgotSent') }}</p>
        <div class="row" style="margin-top: 16px;">
          <button class="secondary" @click="view = 'login'">{{ t('auth.toLogin') }}</button>
        </div>
      </template>
    </div>

    <!-- RESET PASSWORD (A1, /reset-password?token=…) -->
    <div v-else-if="view === 'reset'" class="card" style="max-width: 420px;">
      <h2>{{ t('auth.resetTitle') }}</h2>
      <template v-if="resetDone">
        <p class="muted">{{ t('auth.resetDone') }}</p>
        <div class="row" style="margin-top: 16px;">
          <button @click="view = 'login'">{{ t('common.login') }}</button>
        </div>
      </template>
      <template v-else-if="resetToken">
        <label>{{ t('auth.resetNew') }}</label>
        <input v-model="resetPw1" type="password" autocomplete="new-password" />
        <label>{{ t('auth.resetConfirm') }}</label>
        <input v-model="resetPw2" type="password" autocomplete="new-password" @keyup.enter="doReset" />
        <div class="error" v-if="authErr">{{ authErr }}</div>
        <div class="row" style="margin-top: 16px;">
          <button @click="doReset">{{ t('auth.resetSubmit') }}</button>
          <button class="secondary" @click="view = 'login'">{{ t('auth.toLogin') }}</button>
        </div>
      </template>
      <template v-else>
        <p class="error">{{ t('auth.resetInvalid') }}</p>
        <div class="row" style="margin-top: 16px;">
          <button v-if="mailEnabled" @click="forgotSent = false; view = 'forgot'">{{ t('auth.forgotSubmit') }}</button>
          <button class="secondary" @click="view = 'login'">{{ t('auth.toLogin') }}</button>
        </div>
      </template>
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

    <!-- public skin library: anonymous visitors browse without a session -->
    <template v-else-if="!user && view === 'skinlib'">
      <SkinlibPage />
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
.user-side { position: sticky; top: 16px; min-width: 170px; padding: 16px; }
.user-main { flex: 1; min-width: 0; }
.nav-group + .nav-group { margin-top: 16px; }
.nav-title {
  font-size: 12px; color: var(--muted); font-weight: 600;
  margin: 0 0 8px; padding-left: 12px;
}
.nav-item {
  display: block; width: 100%; text-align: left;
  background: none; border: 1px solid transparent;
  padding: 8px 12px; border-radius: 8px; cursor: pointer;
  font-size: var(--fs-s); color: inherit; margin-bottom: 2px;
}
.nav-item:hover { border-color: var(--border); }
.nav-item.active { border-color: var(--accent); font-weight: 600; }
/* required invite label on the register card */
label.required { color: var(--accent); }
@media (max-width: 720px) {
  .user-shell { flex-direction: column; }
  .user-side { position: static; width: 100%; }
  .nav-group { display: flex; flex-wrap: wrap; gap: 4px; align-items: center; }
  .nav-title { width: 100%; margin-bottom: 2px; }
  .nav-item { width: auto; margin-bottom: 0; }
}
</style>
