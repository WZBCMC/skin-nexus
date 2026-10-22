<script setup>
// UnionBindPage: MUA Union member-side profile management — server list,
// bind (get 2-min token) / bindto / unbind / UUID remap.
// All endpoints exist only when SKIN_UNION_ENABLED; otherwise the API 404s
// and this page shows "联盟功能未启用" (zero surface for non-alliance sites).
import { ref, onMounted, computed } from 'vue'
import { api, union } from '../api.js'

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
    error.value = e.message.includes('404') ? '联盟功能未启用' : e.message
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
    success.value = '绑定码已生成（2 分钟内有效），请到联盟侧或对应服务器完成绑定'
  } catch (e) { error.value = e.message } finally { loading.value = false }
}

async function doBindTo() {
  const p = selectedPlayer.value
  if (!p || !bindtoToken.value.trim()) { error.value = '请填写联盟侧返回的绑定令牌'; return }
  loading.value = true; error.value = ''; success.value = ''
  try {
    const d = await union.bindto(playerUUID(p), bindtoToken.value.trim())
    if (d.error) { error.value = d.error; return }
    success.value = '绑定成功'
    bindtoToken.value = ''
  } catch (e) { error.value = e.message } finally { loading.value = false }
}

async function doUnbind() {
  const p = selectedPlayer.value
  if (!p) return
  if (!confirm(`解除 ${playerName(p)} 的联盟绑定？`)) return
  loading.value = true; error.value = ''; success.value = ''
  try {
    const d = await union.unbind(playerUUID(p))
    if (d.error) { error.value = d.error; return }
    success.value = '已解绑'
  } catch (e) { error.value = e.message } finally { loading.value = false }
}

async function doRemap() {
  const p = selectedPlayer.value
  if (!p || !remapTarget.value) { error.value = '请选择要修正到的目标角色'; return }
  if (remapTarget.value === playerUUID(p)) { error.value = '目标不能是当前角色自己'; return }
  if (!confirm(`将 ${playerName(p)} 的联盟 UUID 修正为 ${remapTarget.value.slice(0, 16)}… ？`)) return
  loading.value = true; error.value = ''; success.value = ''
  try {
    const d = await union.remapuuid(playerUUID(p), remapTarget.value)
    if (d.error) { error.value = d.error; return }
    success.value = 'UUID 修正请求已提交'
    remapTarget.value = ''
  } catch (e) { error.value = e.message } finally { loading.value = false }
}

const selectedPlayer = computed(() =>
  players.value.find(p => playerUUID(p) === selectedPID.value) || null)

onMounted(load)
</script>

<template>
  <div>
    <div class="card">
      <h2>Union 联盟绑定</h2>
      <p class="muted">
        MUA Union 成员站功能：绑定角色后可在联盟服务器间同步皮肤与登录状态。
        <template v-if="serverVersion">（服务器列表 v{{ serverVersion }}）</template>
      </p>
      <div class="error" v-if="error">{{ error }}</div>
      <div class="success" v-if="success">{{ success }}</div>

      <template v-if="players.length">
        <label>选择角色</label>
        <select v-model="selectedPID" style="max-width: 320px;">
          <option v-for="p in players" :key="p.id" :value="playerUUID(p)">
            {{ playerName(p) }}（{{ playerUUID(p).slice(0, 16) }}…）
          </option>
        </select>
        <div class="row" style="margin-top: 12px;">
          <button :disabled="loading || !selectedPID" @click="doBind">生成绑定码</button>
          <input v-model="bindtoToken" placeholder="联盟侧绑定令牌" style="max-width: 240px;" />
          <button class="secondary" :disabled="loading || !selectedPID" @click="doBindTo">用令牌绑定</button>
          <button class="secondary" :disabled="loading || !selectedPID" @click="doUnbind">解除绑定</button>
        </div>
        <div v-if="bindTokenAlive" class="success" style="margin-top: 8px;">
          绑定码（{{ bindTokenFor }}）：<code>{{ bindToken }}</code> — 请在 2 分钟内在联盟侧使用
        </div>
      </template>
      <div v-else class="muted">先在个人页创建角色。</div>
    </div>

    <div class="card" v-if="players.length >= 2">
      <h2>UUID 修正</h2>
      <p class="muted">
        1.19+ 正版会话与外置登录的 UUID 不一致时，需要在联盟侧做一致性绑定；
        若绑定到了错误角色，在此发起修正（两个角色都必须属于当前账号）。
      </p>
      <div class="row">
        <select v-model="selectedPID" style="max-width: 280px;">
          <option v-for="p in players" :key="p.id" :value="playerUUID(p)">
            当前：{{ playerName(p) }}
          </option>
        </select>
        <select v-model="remapTarget" style="max-width: 280px;">
          <option value="">修正到…</option>
          <option v-for="p in players" :key="p.id" :value="playerUUID(p)" :disabled="p === selectedPlayer">
            {{ playerName(p) }}（{{ playerUUID(p).slice(0, 16) }}…）
          </option>
        </select>
        <button class="secondary" :disabled="loading || !remapTarget" @click="doRemap">提交修正</button>
      </div>
    </div>

    <div class="card" v-if="servers.length">
      <h2>联盟服务器列表（{{ servers.length }}）</h2>
      <table>
        <tr><th>代码</th><th>名称</th></tr>
        <tr v-for="s in servers" :key="s.code">
          <td><code>{{ s.code }}</code></td>
          <td>{{ s.name }}</td>
        </tr>
      </table>
    </div>
  </div>
</template>
