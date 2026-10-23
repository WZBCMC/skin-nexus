<script setup>
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'

const { t } = useI18n()

const token = ref(localStorage.getItem('admin_token') || '')
// Token input is a FALLBACK only (blessing parity): an admin session
// (me.permission > 0) loads admin data directly with cookies alone — the
// X-Admin-Token row appears only when the session can't reach admin data or
// a load returns 401/403.
const needToken = ref(true)
const overview = ref(null)
const users = ref([])
const unionStatus = ref(null)
const unionError = ref('')
const oidcClients = ref([])
const oidcError = ref('')
const newClient = ref({ client_id: '', name: '', redirect_uris: '', public: true })
const lastSecret = ref('') // shown once after create
const error = ref('')
// user management errors are their own card (review P1-4: setError used to be
// shared between the 用户 card and the 站点设置 card)
const userErr = ref('')

// SEO + appearance (phase 3)
const seo = ref(null)
const seoError = ref('')
const appearance = ref(null)
const appearError = ref('')
const themes = ref([])

// Invitation codes (1:1 live blessing plugin admin surface)
const invFree = ref([])
const invUsed = ref([])
const invError = ref('')
const invNew = ref({ amount: 1, description: '' })

// Settings panel: env-migrated site knobs
const settings = ref([])
const setError = ref('')

async function loadSettings() {
  setError.value = ''
  try {
    const d = await fetchJSON('/api/admin/v1/settings')
    // Bool rows normalize to "1"/"0" on load (review B P2: the checkbox uses
    // true-value="1", so a stored "true"/"on"/"yes" rendered as UNCHECKED and
    // the next save silently wrote the feature back to "0").
    const boolKeys = new Set(['invite_required', 'require_verification'])
    settings.value = (d.settings || []).map((s) =>
      boolKeys.has(s.key)
        ? { ...s, value: ['1', 'true', 'on', 'yes'].includes(String(s.value).toLowerCase()) ? '1' : '0' }
        : s
    )
  } catch (e) { setError.value = e.message }
}

async function loadAppearance() {
  appearError.value = ''
  try {
    appearance.value = await fetchJSON('/api/admin/v1/appearance')
    const th = await fetchJSON('/api/admin/v1/themes')
    themes.value = th.themes || []
  } catch (e) { appearError.value = e.message }
}

async function loadSEO() {
  seoError.value = ''
  try { seo.value = await fetchJSON('/api/admin/v1/seo') } catch (e) { seoError.value = e.message }
}

async function saveSetting(s) {
  setError.value = ''
  try {
    await fetchJSON('/api/admin/v1/settings', {
      method: 'PUT', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ key: s.key, value: s.value }),
    })
    await loadSettings()
  } catch (e) {
    // review P2-10: refetch so the UI doesn't keep a toggled-but-unsaved value
    await loadSettings()
    setError.value = e.message
  }
}

// admin user management
async function adminResetPassword(u) {
  const pw = prompt(`重设 ${u.nickname}（uid ${u.id}）的密码（≥8 位）：`, '')
  if (!pw) return
  userErr.value = ''
  try {
    await fetchJSON(`/api/admin/v1/users/${u.id}/password`, {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ password: pw }),
    })
    alert(`已重设：该用户全部令牌与启动器授权已失效。`)
  } catch (e) { userErr.value = e.message }
}

async function adminDeleteUser(u) {
  if (!confirm(`删除用户 ${u.nickname}（uid ${u.id}）？会话/令牌/衣柜/角色一并删除，纹理与审计保留。不可恢复！`)) return
  userErr.value = ''
  try {
    await fetchJSON(`/api/admin/v1/users/${u.id}`, { method: 'DELETE' })
    users.value = users.value.filter((x) => x.id !== u.id)
  } catch (e) { userErr.value = e.message }
}

async function fetchJSON(path, opts) {
  const headers = { ...(opts && opts.headers) }
  // X-Admin-Token only when a token exists — session-admin calls work with
  // cookies alone (same-origin fetch sends them by default, no credentials
  // option needed).
  if (token.value) headers['X-Admin-Token'] = token.value
  const res = await fetch(path, { ...opts, headers })
  const data = await res.json().catch(() => ({}))
  if (!res.ok) {
    // 401/403 = admin access lost (expired session / revoked permission /
    // wrong token) — fall back to the token card.
    if (res.status === 401 || res.status === 403) needToken.value = true
    throw new Error(data.message || data.error || `HTTP ${res.status}`)
  }
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
  // SEO
  await loadSEO()
  // Appearance + themes
  await loadAppearance()
  // Invitation codes
  invError.value = ''
  try {
    const d = await fetchJSON('/api/admin/v1/invitation-codes')
    invFree.value = d.free || []
    invUsed.value = d.used || []
  } catch (e) { invError.value = e.message }
  // Settings panel
  await loadSettings()
}

async function invGenerate() {
  invError.value = ''
  try {
    await fetchJSON('/api/admin/v1/invitation-codes', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ amount: invNew.value.amount || 1, description: invNew.value.description }),
    })
    invNew.value = { amount: 1, description: '' }
    const d = await fetchJSON('/api/admin/v1/invitation-codes')
    invFree.value = d.free || []
    invUsed.value = d.used || []
  } catch (e) { invError.value = e.message }
}

async function invEdit(ic) {
  const next = prompt('修改备注（description）：', ic.description || '')
  if (next === null) return
  invError.value = ''
  try {
    await fetchJSON('/api/admin/v1/invitation-codes', {
      method: 'PUT', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ id: ic.id, description: next }),
    })
    const d = await fetchJSON('/api/admin/v1/invitation-codes')
    invFree.value = d.free || []
    invUsed.value = d.used || []
  } catch (e) { invError.value = e.message }
}

async function seoOneClick() {
  seoError.value = ''
  try {
    seo.value = await fetchJSON('/api/admin/v1/seo/oneclick', { method: 'POST' })
  } catch (e) { seoError.value = e.message }
}

async function seoSave() {
  seoError.value = ''
  try {
    await fetchJSON('/api/admin/v1/seo', {
      method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(seo.value),
    })
  } catch (e) {
    // review P2-10: refetch so the UI doesn't keep unsaved values
    await loadSEO()
    seoError.value = e.message
  }
}

async function appearSave() {
  appearError.value = ''
  try {
    await fetchJSON('/api/admin/v1/appearance', {
      method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(appearance.value),
    })
  } catch (e) {
    // review P2-10: refetch so the UI doesn't keep unsaved values
    await loadAppearance()
    appearError.value = e.message
  }
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
  if (!body.client_id) { oidcError.value = t('admin.oidcIdRequired'); return }
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
  if (!confirm(t('admin.oidcDeleteConfirm', { id }))) return
  oidcError.value = ''
  try {
    await fetchJSON(`/api/admin/v1/oidc/clients/${encodeURIComponent(id)}`, { method: 'DELETE' })
    oidcClients.value = oidcClients.value.filter(c => c.client_id !== id)
  } catch (e) { oidcError.value = e.message }
}

async function resumeRotation() {
  if (!confirm(t('admin.unionResumeConfirm'))) return
  try {
    await fetchJSON('/api/admin/v1/union/rotation/resume', { method: 'POST' })
    await load()
  } catch (e) { unionError.value = e.message }
}

onMounted(async () => {
  // Session-admin detection: permission > 0 means the session alone reaches
  // admin data — load directly on mount, no token ceremony.
  try {
    const me = await api.me()
    if (Number(me.permission) > 0) needToken.value = false
  } catch { /* no usable session — the token fallback stays visible */ }
  if (!needToken.value || token.value) await load()
})
</script>

<template>
  <div>
    <div class="card">
      <h2>{{ t('admin.title') }}</h2>
      <div class="row">
        <!-- X-Admin-Token fallback row: only when the session can't reach admin data -->
        <input v-if="needToken" v-model="token" type="password" :placeholder="t('admin.tokenPlaceholder')" style="max-width: 320px;" />
        <button @click="load">{{ t('admin.load') }}</button>
      </div>
      <div class="error" v-if="error">{{ error }}</div>
    </div>

    <div class="card" v-if="overview">
      <h2>{{ t('admin.overviewTitle') }}</h2>
      <div class="row">
        <span class="badge">{{ t('admin.users') }} {{ overview.users }}</span>
        <span class="badge">{{ t('admin.players') }} {{ overview.players }}</span>
        <span class="badge">{{ t('admin.textures') }} {{ overview.textures }}</span>
        <span class="badge" :style="overview.weakHashUsers > 0 ? 'border-color: var(--danger); color: var(--danger);' : ''">
          {{ t('admin.weakHash') }} {{ overview.weakHashUsers }}
        </span>
        <span class="badge">{{ t('admin.signKeyFingerprint') }} {{ overview.signKeyFingerprint }}</span>
      </div>
    </div>

    <div class="card" v-if="users.length">
      <h2>{{ t('admin.usersTitle') }}</h2>
      <div class="error" v-if="userErr">{{ userErr }}</div>
      <table>
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

    <!-- MUA Union module status -->
    <div class="card" v-if="unionStatus">
      <h2>{{ t('admin.unionTitle') }}</h2>
      <div class="row">
        <span class="badge">{{ t('admin.unionMode') }} {{ unionStatus.mode }}</span>
        <span class="badge">{{ t('admin.unionKeyFp') }} {{ unionStatus.key_fingerprint || '—' }}</span>
        <span class="badge">{{ t('admin.unionKeyVersion') }} {{ unionStatus.private_key_version || '—' }}</span>
        <span class="badge">{{ t('admin.unionServerListVersion', { version: unionStatus.server_list_version || '—' }) }}</span>
        <span class="badge">{{ t('admin.unionQueryEmail') }} {{ unionStatus.queryemail_mode }}</span>
        <span class="badge" :style="unionStatus.rotation_suspended ? 'border-color: var(--danger); color: var(--danger);' : ''">
          {{ unionStatus.rotation_suspended ? t('admin.unionRotationSuspended') : t('admin.unionRotationOk') }}
        </span>
      </div>
      <div class="row" style="margin-top: 10px;" v-if="unionStatus.rotation_suspended">
        <button class="secondary" @click="resumeRotation">{{ t('admin.unionResumeRotation') }}</button>
      </div>
      <div class="muted" style="margin-top: 10px; font-size: 12px;">
        {{ t('admin.unionTrustNotice') }}
      </div>
      <div class="error" v-if="unionError">{{ unionError }}</div>
    </div>

    <!-- OIDC clients -->
    <div class="card">
      <h2>{{ t('admin.oidcClientsTitle', { count: oidcClients.length }) }}</h2>
      <div class="row">
        <input v-model="newClient.client_id" :placeholder="t('admin.oidcClientId')" style="max-width: 180px;" />
        <input v-model="newClient.name" :placeholder="t('admin.oidcClientName')" style="max-width: 180px;" />
        <label style="display: flex; align-items: center; gap: 4px; white-space: nowrap;">
          <input type="checkbox" v-model="newClient.public" /> {{ t('admin.oidcPublicCheckbox') }}
        </label>
        <button @click="createClient">{{ t('admin.oidcCreate') }}</button>
      </div>
      <textarea v-model="newClient.redirect_uris" :placeholder="t('admin.oidcRedirectPlaceholder')" rows="2"
                style="margin-top: 8px; width: 100%;"></textarea>
      <div class="success" v-if="lastSecret" style="margin-top: 8px;">
        {{ t('admin.oidcSecretOnce') }}<code>{{ lastSecret }}</code>
      </div>
      <div class="error" v-if="oidcError">{{ oidcError }}</div>
      <table v-if="oidcClients.length" style="margin-top: 10px;">
        <tr><th>{{ t('admin.oidcClientId') }}</th><th>{{ t('admin.oidcClientName') }}</th><th>{{ t('admin.oidcClientType') }}</th><th>{{ t('admin.oidcClientGrants') }}</th><th>{{ t('admin.oidcClientRedirects') }}</th><th></th></tr>
        <tr v-for="c in oidcClients" :key="c.client_id">
          <td><code>{{ c.client_id }}</code></td>
          <td>{{ c.name }}</td>
          <td><span class="badge">{{ c.public ? t('admin.oidcPublic') : t('admin.oidcConfidential') }}</span></td>
          <td class="muted" style="font-size: 12px;">{{ (c.grant_types || []).join(', ') }}</td>
          <td class="muted" style="font-size: 12px;">{{ (c.redirect_uris || []).join(', ') || '—' }}</td>
          <td><button class="secondary" @click="deleteClient(c.client_id)">{{ t('admin.oidcDelete') }}</button></td>
        </tr>
      </table>
    </div>

    <!-- SEO -->
    <div class="card" v-if="seo">
      <h2>SEO 配置</h2>
      <div class="row">
        <button class="secondary" @click="seoOneClick">一键优化（填充默认值）</button>
        <button @click="seoSave">保存 SEO</button>
      </div>
      <label>站点标题</label>
      <input v-model="seo.title" />
      <label>meta description</label>
      <textarea v-model="seo.description" rows="2"></textarea>
      <label>meta keywords（逗号分隔）</label>
      <input v-model="seo.keywords" />
      <label>OG 分享图 URL</label>
      <input v-model="seo.og_image" />
      <label>canonical 基址</label>
      <input v-model="seo.canonical" />
      <div class="row" style="margin-top: 8px;">
        <label style="display: flex; align-items: center; gap: 4px;">
          <input type="checkbox" v-model="seo.sitemap" /> 启用 sitemap.xml
        </label>
        <label style="display: flex; align-items: center; gap: 4px;">
          <input type="checkbox" v-model="seo.noindex" /> 整站 noindex（预发）
        </label>
      </div>
      <label style="margin-top: 8px;">robots.txt 内容</label>
      <textarea v-model="seo.robots" rows="3"></textarea>
      <div class="error" v-if="seoError">{{ seoError }}</div>
    </div>

    <!-- Appearance: themes + custom CSS/JS -->
    <div class="card" v-if="appearance">
      <h2>外观（主题 / 自定义 CSS / JS）</h2>
      <label>启用主题包</label>
      <select v-model="appearance.active_theme" style="max-width: 320px;">
        <option value="">无（默认样式）</option>
        <option v-for="th in themes" :key="th.id" :value="th.id">{{ th.name }}（{{ th.version }}）</option>
      </select>
      <div class="muted" v-if="!themes.length" style="font-size: 12px;">
        将主题包放入 data/themes/&lt;id&gt;/（含 theme.json），刷新即识别。
      </div>
      <label style="margin-top: 10px;">自定义 CSS</label>
      <textarea v-model="appearance.custom_css" rows="5" placeholder=".card { border-radius: 16px; }"></textarea>
      <label>自定义 JS</label>
      <textarea v-model="appearance.custom_js" rows="4" placeholder="console.log('hello')"></textarea>
      <div class="muted" style="font-size: 12px; margin-top: 6px;">
        ⚠️ 自定义 JS 与主题 JS 以管理员信任面原样注入（与 blessing 行为一致）——请只粘贴自己信任的代码。
      </div>
      <div class="row" style="margin-top: 10px;">
        <button @click="appearSave">保存外观</button>
      </div>
      <div class="error" v-if="appearError">{{ appearError }}</div>
    </div>

    <!-- 邀请码（1:1 线上 blessing 插件管理面） -->
    <div class="card">
      <h2>邀请码（未用 {{ invFree.length }} / 已用 {{ invUsed.length }}）</h2>
      <div class="row">
        <input v-model.number="invNew.amount" type="number" min="1" max="1000" placeholder="数量" style="max-width: 100px;" />
        <input v-model="invNew.description" placeholder="备注（description，如 姓名+学号，选填）" style="max-width: 320px;" />
        <button @click="invGenerate">生成</button>
      </div>
      <div class="error" v-if="invError">{{ invError }}</div>
      <table v-if="invFree.length" style="margin-top: 10px;">
        <tr><th>ID</th><th>邀请码</th><th>备注</th><th>生成时间</th><th></th></tr>
        <tr v-for="ic in invFree" :key="ic.id">
          <td>{{ ic.id }}</td>
          <td><code>{{ ic.code }}</code></td>
          <td class="muted">{{ ic.description || '—' }}</td>
          <td class="muted">{{ (ic.generated_at || '').slice(0, 19).replace('T', ' ') }}</td>
          <td><button class="secondary" @click="invEdit(ic)">改备注</button></td>
        </tr>
      </table>
      <table v-if="invUsed.length" style="margin-top: 10px;">
        <tr><th>ID</th><th>邀请码</th><th>备注</th><th>使用者 UID</th><th>使用时间</th></tr>
        <tr v-for="ic in invUsed" :key="ic.id">
          <td>{{ ic.id }}</td>
          <td><code>{{ ic.code }}</code></td>
          <td class="muted">{{ ic.description || '—' }}</td>
          <td>{{ ic.used_by }}</td>
          <td class="muted">{{ (ic.used_at || '').slice(0, 19).replace('T', ' ') }}</td>
        </tr>
      </table>
    </div>

    <!-- 站点设置（env 迁 UI） -->
    <div class="card">
      <h2>站点设置（原环境变量，管理员可改）</h2>
      <div class="error" v-if="setError">{{ setError }}</div>
      <div class="row" v-for="s in settings" :key="s.key" style="margin-top: 10px; align-items: center;">
        <!-- Chinese-first label: server-provided desc is primary, the raw key
             demoted to a small muted hint -->
        <label style="width: 240px; font-weight: 600; font-size: 13px;">
          {{ s.desc }}
          <span class="muted" style="display: block; font-weight: 400; font-size: 11px;">{{ s.key }}</span>
        </label>
        <input v-if="s.key === 'invite_required' || s.key === 'require_verification'"
               type="checkbox" v-model="s.value" true-value="1" false-value="0" style="width: auto;" />
        <textarea v-else-if="s.key === 'skin_domains'" v-model="s.value" style="max-width: 360px; min-height: 38px;"></textarea>
        <input v-else v-model="s.value" style="max-width: 300px;" :placeholder="s.default" />
        <button @click="saveSetting(s)">保存</button>
        <span class="muted" style="font-size: 11px;">{{ s.restart ? '重启后生效' : '立即生效' }}</span>
      </div>
    </div>
  </div>
</template>
