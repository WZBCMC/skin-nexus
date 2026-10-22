<script setup>
// UnionBindPage: MUA Union member-side profile management — server list,
// bind (get 2-min token) / bindto / unbind / UUID remap.
// All endpoints exist only when SKIN_UNION_ENABLED; otherwise the API 404s
// and this page shows a friendly notice (zero surface for non-alliance sites).
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { api, union } from '../api.js'

const { t } = useI18n()

const players = ref([])
const servers = ref([])
const serverVersion = ref('')
const error = ref('')
const success = ref('')
const loading = ref(false)

// bind token workflow
const bindToken = ref('')
const bindTokenFor = ref('')
const bindTokenExpire = ref(0)
const bindtoToken = ref('')
const selectedPID = ref(null)

// remap workflow
const remapTarget = ref('')

const bindTokenAlive = computed(() =>
  bindToken.value && Date.now() < bindTokenExpire.value)

const selectedPlayer = computed(() =>
  players.value.find(p => playerUUID(p) === selectedPID.value) || null)

async function load() {
  error.value = ''; success.value = ''
  try {
    players.value = (await api.players()).players || []
  } catch (e) { error.value = e.message; return }
  try {
    const d = await union.servers()
    servers.value = d.servers || []
    serverVersion.value = d.version || ''
  } catch (e) {
    // disabled module -> friendly notice, not an error storm
    servers.value = []
    error.value = e.message.includes('404') ? t('union.disabled') : e.message
  }
}

function playerUUID(p) { return p.UUID || p.uuid || '' }
function playerName(p) { return p.Name || p.name || '' }

async function doBind() {
  const p = selectedPlayer.value
  if (!p) return
  loading.value = true; error.value = ''; success.value = ''
  try {
    const d = await union.bind(playerUUID(p))
    if (d.error) { error.value = d.error; return }
    bindToken.value = d.token
    bindTokenFor.value = playerName(p)
    bindTokenExpire.value = Date.now() + 2 * 60 * 1000 // hub TTL 2min
    success.value = t('union.bindCodeSuccess')
  } catch (e) { error.value = e.message } finally { loading.value = false }
}

async function doBindTo() {
  const p = selectedPlayer.value
  if (!p || !bindtoToken.value.trim()) { error.value = t('union.bindTokenMissing'); return }
  loading.value = true; error.value = ''; success.value = ''
  try {
    const d = await union.bindto(playerUUID(p), bindtoToken.value.trim())
    if (d.error) { error.value = d.error; return }
    success.value = t('union.bindSuccess')
    bindtoToken.value = ''
  } catch (e) { error.value = e.message } finally { loading.value = false }
}

async function doUnbind() {
  const p = selectedPlayer.value
  if (!p) return
  if (!confirm(t('union.unbindConfirm', { name: playerName(p) }))) return
  loading.value = true; error.value = ''; success.value = ''
  try {
    const d = await union.unbind(playerUUID(p))
    if (d.error) { error.value = d.error; return }
    success.value = t('union.unbindDone')
  } catch (e) { error.value = e.message } finally { loading.value = false }
}

async function doRemap() {
  const p = selectedPlayer.value
  if (!p || !remapTarget.value) { error.value = t('union.remapMissing'); return }
  if (remapTarget.value === playerUUID(p)) { error.value = t('union.remapSelf'); return }
  if (!confirm(t('union.remapConfirm', { name: playerName(p), target: remapTarget.value.slice(0, 16) + '…' }))) return
  loading.value = true; error.value = ''; success.value = ''
  try {
    const d = await union.remapuuid(playerUUID(p), remapTarget.value)
    if (d.error) { error.value = d.error; return }
    success.value = t('union.remapDone')
    remapTarget.value = ''
  } catch (e) { error.value = e.message } finally { loading.value = false }
}

onMounted(load)
</script>

<template>
  <div>
    <div class="card">
      <h2>{{ t('union.title') }}</h2>
      <p class="muted">
        {{ t('union.intro') }}
        <template v-if="serverVersion">{{ t('union.serverListVersion', { version: serverVersion }) }}</template>
      </p>
      <div class="error" v-if="error">{{ error }}</div>
      <div class="success" v-if="success">{{ success }}</div>

      <template v-if="players.length">
        <label>{{ t('union.selectProfile') }}</label>
        <select v-model="selectedPID" style="max-width: 320px;">
          <option v-for="p in players" :key="p.id" :value="playerUUID(p)">
            {{ playerName(p) }}（{{ playerUUID(p).slice(0, 16) }}…）
          </option>
        </select>
        <div class="row" style="margin-top: 12px;">
          <button :disabled="loading || !selectedPID" @click="doBind">{{ t('union.genBindCode') }}</button>
          <input v-model="bindtoToken" :placeholder="t('union.bindTokenPlaceholder')" style="max-width: 240px;" />
          <button class="secondary" :disabled="loading || !selectedPID" @click="doBindTo">{{ t('union.bindWithToken') }}</button>
          <button class="secondary" :disabled="loading || !selectedPID" @click="doUnbind">{{ t('union.unbind') }}</button>
        </div>
        <div v-if="bindTokenAlive" class="success" style="margin-top: 8px;">
          {{ t('union.bindCodeTitle', { name: bindTokenFor }) }}：<code>{{ bindToken }}</code>{{ t('union.bindCodeHint') }}
        </div>
      </template>
      <div v-else class="muted">{{ t('union.needProfile') }}</div>
    </div>

    <div class="card" v-if="players.length >= 2">
      <h2>{{ t('union.remapTitle') }}</h2>
      <p class="muted">{{ t('union.remapIntro') }}</p>
      <div class="row">
        <select v-model="selectedPID" style="max-width: 280px;">
          <option v-for="p in players" :key="p.id" :value="playerUUID(p)">
            {{ t('union.remapCurrent', { name: playerName(p) }) }}
          </option>
        </select>
        <select v-model="remapTarget" style="max-width: 280px;">
          <option value="">{{ t('union.remapTarget') }}</option>
          <option v-for="p in players" :key="p.id" :value="playerUUID(p)" :disabled="p === selectedPlayer">
            {{ playerName(p) }}（{{ playerUUID(p).slice(0, 16) }}…）
          </option>
        </select>
        <button class="secondary" :disabled="loading || !remapTarget" @click="doRemap">{{ t('union.remapSubmit') }}</button>
      </div>
    </div>

    <div class="card" v-if="servers.length">
      <h2>{{ t('union.serversTitle', { count: servers.length }) }}</h2>
      <table>
        <tr><th>{{ t('union.colCode') }}</th><th>{{ t('union.colName') }}</th></tr>
        <tr v-for="s in servers" :key="s.code">
          <td><code>{{ s.code }}</code></td>
          <td>{{ s.name }}</td>
        </tr>
      </table>
    </div>
  </div>
</template>
