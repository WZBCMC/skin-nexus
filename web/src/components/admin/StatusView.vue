<script setup>
// StatusView: read-only runtime facts (运行状态, blessing parity B1) —
// build/Go/MySQL versions, applied-migration watermark, module flags.
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { fetchJSON } from '../../adminApi.js'

const { t } = useI18n()
const status = ref(null)
const error = ref('')

onMounted(async () => {
  error.value = ''
  try {
    status.value = await fetchJSON('/api/admin/v1/status')
  } catch (e) { error.value = e.message }
})
</script>

<template>
  <div class="card">
    <h2 style="margin-top: 0;">{{ t('admin.statusTitle') }}</h2>
    <div class="error" v-if="error">{{ error }}</div>
    <template v-if="status">
      <div class="row">
        <span class="badge">{{ t('admin.statusVersion') }} {{ status.version }}</span>
        <span class="badge">{{ t('admin.statusGo') }} {{ status.goVersion }}</span>
        <span class="badge">{{ t('admin.statusMySQL') }} {{ status.mysqlVersion }}</span>
        <span class="badge">{{ t('admin.statusMigration') }} {{ status.migration }}</span>
      </div>
      <div class="row" style="margin-top: 10px;">
        <span class="badge" :style="status.modules.union ? '' : 'opacity: 0.6;'">
          {{ t('admin.statusUnion') }} {{ status.modules.union ? t('admin.moduleOn') : t('admin.moduleOff') }}
        </span>
        <span class="badge" :style="status.modules.appearance ? '' : 'opacity: 0.6;'">
          {{ t('admin.statusAppearance') }} {{ status.modules.appearance ? t('admin.moduleOn') : t('admin.moduleOff') }}
        </span>
        <span class="badge">{{ t('admin.signKeyFingerprint') }} {{ status.signKeyFingerprint }}</span>
      </div>
    </template>
  </div>
</template>
