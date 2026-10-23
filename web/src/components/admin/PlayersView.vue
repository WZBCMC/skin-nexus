<script setup>
// PlayersView: placeholder until B3 lands the player management table
// (list / swap texture / rename / delete, blessing 玩家管理 parity).
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
</script>

<template>
  <div class="card">
    <h2 style="margin-top: 0;">{{ t('admin.playersTitle') }}</h2>
    <p class="muted">{{ t('admin.playersPlaceholder') }}</p>
  </div>
</template>
