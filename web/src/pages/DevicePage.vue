<script setup>
// DevicePage: RFC 8628 verification page (SPA). Reached at /device?user_code=…
// (launcher verification_uri). Without a code it shows an input box for the
// bare verification_uri case. Submit is the JSON endpoint /api/v1/oidc/device/submit.
import { ref, onMounted } from 'vue'
import { oidc } from '../api.js'

const userCode = ref(new URLSearchParams(location.search).get('user_code') || '')
const inputCode = ref('')
const info = ref(null)
const error = ref('')
const selectedPID = ref(0)
const done = ref('')
const loading = ref(false)

async function loadInfo() {
  error.value = ''; done.value = ''
  if (!userCode.value) { info.value = null; return }
  try {
    const d = await oidc.deviceInfo(userCode.value)
    if (d.error) { error.value = d.error; info.value = null; return }
    info.value = d
    selectedPID.value = d.default_pid || (d.players && d.players[0] ? d.players[0].id : 0)
  } catch (e) { error.value = e.message; info.value = null }
}

function useCode() {
  const c = (inputCode.value || '').trim().toUpperCase()
  if (!c) return
  // keep the URL shareable/refreshable
  history.replaceState(null, '', '/device?user_code=' + encodeURIComponent(c))
  userCode.value = c
  loadInfo()
}

async function submit(decision) {
  loading.value = true; error.value = ''
  try {
    const d = await oidc.deviceSubmit(userCode.value, decision, selectedPID.value)
    if (d.error) { error.value = d.error; return }
    done.value = decision === 'approve'
      ? '已授权，可返回启动器继续登录。'
      : '已拒绝，可返回启动器。'
    info.value = null
  } catch (e) {
    error.value = e.message.includes('401') ? '请先登录皮肤站账号后再批准' : e.message
  } finally { loading.value = false }
}

onMounted(loadInfo)
</script>

<template>
  <div class="card" style="max-width: 480px;">
    <h2>设备授权</h2>

    <!-- done -->
    <div v-if="done" class="success">{{ done }}</div>

    <!-- no user_code: manual input -->
    <template v-else-if="!userCode">
      <p class="muted">启动器显示了一个用户码？在此输入以继续。</p>
      <div class="row">
        <input v-model="inputCode" placeholder="XXXX-XXXX" style="max-width: 160px; text-transform: uppercase;"
               @keyup.enter="useCode" />
        <button @click="useCode">继续</button>
      </div>
    </template>

    <!-- pending info -->
    <template v-else-if="info">
      <p>用户码：<b style="font-size: 18px; letter-spacing: 2px;">{{ info.user_code }}</b></p>
      <p>请求方：<b>{{ info.client_name || info.client_id }}</b></p>
      <p class="muted">请求权限：{{ (info.scopes || []).join('、') }}</p>
      <template v-if="info.players && info.players.length">
        <label>选择要授权的角色：</label>
        <div v-for="p in info.players" :key="p.id" style="margin: 4px 0;">
          <label style="display: flex; gap: 8px; align-items: center; cursor: pointer;">
            <input type="radio" :value="p.id" v-model="selectedPID" />
            <strong>{{ p.name }}</strong>
            <span class="muted" style="font-size: 11px;">{{ (p.uuid || '').slice(0, 16) }}…</span>
          </label>
        </div>
      </template>
      <div v-else class="error">账号下没有可选角色，请先在个人页创建角色。</div>
      <div class="error" v-if="error">{{ error }}</div>
      <div class="row" style="margin-top: 14px;">
        <button :disabled="loading || !selectedPID" @click="submit('approve')">同意</button>
        <button class="secondary" :disabled="loading" @click="submit('deny')">拒绝</button>
      </div>
    </template>

    <!-- error / no info -->
    <div v-else>
      <div class="error" v-if="error">{{ error }}</div>
      <div class="row">
        <input v-model="inputCode" placeholder="XXXX-XXXX" style="max-width: 160px; text-transform: uppercase;"
               @keyup.enter="useCode" />
        <button @click="useCode">查询</button>
      </div>
    </div>
  </div>
</template>
