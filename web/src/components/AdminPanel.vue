<script setup>
// AdminPanel shell (B1 refactor, 2026-10-24): sidebar + per-domain views.
// The old single-file card flow moved to components/admin/*; App.vue still
// mounts this at view === 'admin' (routing untouched).
import { ref, shallowRef, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'
import { token, needToken, fetchJSON, persistToken } from '../adminApi.js'
import DashboardView from './admin/DashboardView.vue'
import UsersView from './admin/UsersView.vue'
import PlayersView from './admin/PlayersView.vue'
import InvitationView from './admin/InvitationView.vue'
import SettingsView from './admin/SettingsView.vue'
import StatusView from './admin/StatusView.vue'
import UnionView from './admin/UnionView.vue'

const { t } = useI18n()

// Sidebar navigation (blessing 管理面板 order). Players stays a placeholder
// until B3 lands the management table.
const nav = [
  { id: 'dashboard', labelKey: 'admin.navDashboard', comp: DashboardView },
  { id: 'users', labelKey: 'admin.navUsers', comp: UsersView },
  { id: 'players', labelKey: 'admin.navPlayers', comp: PlayersView },
  { id: 'invitation', labelKey: 'admin.navInvitation', comp: InvitationView },
  { id: 'settings', labelKey: 'admin.navSettings', comp: SettingsView },
  { id: 'status', labelKey: 'admin.navStatus', comp: StatusView },
  { id: 'union', labelKey: 'admin.navUnion', comp: UnionView },
]
const current = ref('dashboard')
const currentComp = shallowRef(DashboardView)

function go(id) {
  const item = nav.find((n) => n.id === id)
  if (item) {
    current.value = id
    currentComp.value = item.comp
  }
}

// The load button re-runs the current view's data fetch: views fetch their
// own data on mount, so remounting via a key bump is the reload contract.
const reloadTick = ref(0)
// Views mount (and fetch) only once admin access is established — a session
// with permission > 0 or a persisted/entered token. Otherwise the shell
// shows just the sidebar + token card (no 403 spam from every view).
const ready = ref(false)

async function submitToken() {
  persistToken()
  if (token.value) ready.value = true
  reloadTick.value++
}

onMounted(async () => {
  // Session-admin detection: permission > 0 means the session alone reaches
  // admin data — no token ceremony needed.
  try {
    const me = await api.me()
    if (Number(me.permission) > 0) needToken.value = false
  } catch { /* no usable session — the token fallback stays visible */ }
  if (!needToken.value || token.value) {
    ready.value = true
    reloadTick.value++
  }
})
</script>

<template>
  <div class="admin-shell">
    <aside class="card admin-side">
      <h2 style="margin-top: 0;">{{ t('admin.title') }}</h2>
      <!-- X-Admin-Token fallback row: only when the session can't reach admin data -->
      <div v-if="needToken" class="admin-token">
        <input v-model="token" type="password" :placeholder="t('admin.tokenPlaceholder')" />
        <button @click="submitToken">{{ t('admin.load') }}</button>
      </div>
      <nav class="admin-nav">
        <button v-for="n in nav" :key="n.id" class="admin-nav-item"
                :class="{ active: current === n.id }" @click="go(n.id)">
          {{ t(n.labelKey) }}
        </button>
      </nav>
    </aside>
    <main class="admin-main">
      <!-- key bump remounts the active view => its onMounted refetches -->
      <component v-if="ready" :is="currentComp" :key="`${current}-${reloadTick}`" @go="go" />
      <div v-else class="card muted">{{ t('admin.enterTokenHint') }}</div>
    </main>
  </div>
</template>

<style scoped>
.admin-shell { display: flex; gap: 16px; align-items: flex-start; }
.admin-side { position: sticky; top: 16px; min-width: 180px; }
.admin-nav { display: flex; flex-direction: column; gap: 6px; margin-top: 12px; }
.admin-nav-item {
  text-align: left; background: none; border: 1px solid transparent;
  padding: 8px 10px; border-radius: 8px; cursor: pointer; font-size: 14px;
  color: var(--fg, inherit);
}
.admin-nav-item:hover { border-color: var(--border, #8884); }
.admin-nav-item.active { border-color: var(--accent, #4a9eff); font-weight: 600; }
.admin-token { display: flex; gap: 8px; }
.admin-token input { min-width: 0; flex: 1; }
.admin-main { flex: 1; min-width: 0; }
@media (max-width: 720px) {
  .admin-shell { flex-direction: column; }
  .admin-side { position: static; width: 100%; }
  .admin-nav { flex-direction: row; flex-wrap: wrap; }
}
</style>
