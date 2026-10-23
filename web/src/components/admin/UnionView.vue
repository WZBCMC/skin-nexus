<script setup>
// UnionView: MUA Union module status + rotation resume (moved out of the
// old card flow in B1; the module-disabled case shows an empty state now
// instead of silently hiding the card).
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { fetchJSON } from '../../adminApi.js'

const { t } = useI18n()
const unionStatus = ref(null)
const error = ref('')

async function load() {
  error.value = ''
  // Absent (404) when the module is disabled — that's expected.
  try {
    unionStatus.value = await fetchJSON('/api/admin/v1/union/status')
  } catch (e) {
    unionStatus.value = null
    error.value = e.message
  }
}

async function resumeRotation() {
  if (!confirm(t('admin.unionResumeConfirm'))) return
  try {
    await fetchJSON('/api/admin/v1/union/rotation/resume', { method: 'POST' })
    await load()
  } catch (e) { error.value = e.message }
}

onMounted(load)
</script>

<template>
  <div>
    <div class="card" v-if="unionStatus">
      <h2 style="margin-top: 0;">{{ t('admin.unionTitle') }}</h2>
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
      <div class="error" v-if="error">{{ error }}</div>
    </div>
    <div class="card" v-else>
      <h2 style="margin-top: 0;">{{ t('admin.unionTitle') }}</h2>
      <p class="muted">{{ t('admin.unionDisabled') }}</p>
    </div>
  </div>
</template>
