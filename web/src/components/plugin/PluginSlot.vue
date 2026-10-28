<script setup>
// Fixed slot outlet. The backend manifest whitelists slot IDs; this component
// only renders enabled plugins and never accepts arbitrary HTML. Schema slots
// are declared but not rendered yet (no host-rendered slot consumer).
import { computed } from 'vue'
import { slotsFor } from '../../pluginUI.js'
import PluginFrame from './PluginFrame.vue'

const props = defineProps({ slot: { type: String, required: true } })
const items = computed(() =>
  slotsFor(props.slot).filter((s) => s.render && s.render.type === 'iframe' && s.render.src),
)

function slotHeight(s) {
  return Math.max(80, Math.min(600, Number(s.height) || 180))
}
</script>

<template>
  <div v-if="items.length" class="plugin-slots" :data-skin-slot="slot">
    <PluginFrame v-for="s in items" :key="s.plugin + s.id" :plugin="s.plugin"
                 :src="s.render.src" :title="s.id" :height="slotHeight(s)" />
  </div>
</template>
