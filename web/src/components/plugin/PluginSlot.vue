<script setup>
// Fixed slot outlet. The backend manifest whitelists slot IDs; this component
// only renders enabled plugins and never accepts arbitrary HTML.
import { computed } from 'vue'
import { slotsFor } from '../../pluginUI.js'
import PluginFrame from './PluginFrame.vue'

const props = defineProps({ slot: { type: String, required: true } })
const items = computed(() => slotsFor(props.slot))
</script>

<template>
  <div v-if="items.length" class="plugin-slots" :data-skin-slot="slot">
    <PluginFrame v-for="s in items" :key="s.plugin + s.id" :plugin="s.plugin"
                 :src="s.render.src" :title="s.id" :height="s.height || 180" />
  </div>
</template>
