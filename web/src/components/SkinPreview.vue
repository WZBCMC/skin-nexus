<script setup>
// SkinPreview — small auto-rotating skinview3d canvas (PlayersPage rows,
// Dashboard upload preview).
// v2 (2026-10-26 批注): modelType prop (default/slim) hot-swaps the player
// model; WebGL probe keeps a static fallback viable on gl-less browsers;
// dispose on unmount so the context budget stays honest (grid pages must
// not accumulate live WebGL contexts).
import { ref, watch, onMounted, onBeforeUnmount } from 'vue'

const props = defineProps({
  url: { type: String, default: '' },
  modelType: { type: String, default: 'default' },
  width: { type: Number, default: 128 },
  height: { type: Number, default: 192 },
})
const emit = defineEmits(['gl-unavailable'])
const canvas = ref(null)
let viewer = null

function webglOK() {
  try {
    const probe = document.createElement('canvas')
    return !!(probe.getContext('webgl2') || probe.getContext('webgl'))
  } catch { return false }
}

async function ensureViewer() {
  if (viewer || !canvas.value) return
  if (!webglOK()) { emit('gl-unavailable'); return }
  const skinview3d = await import('skinview3d')
  viewer = new skinview3d.SkinViewer({
    canvas: canvas.value,
    width: props.width,
    height: props.height,
    zoom: 0.7,
  })
  viewer.autoRotate = true
  viewer.playerModelType = props.modelType === 'slim' ? 'slim' : 'default'
}

async function apply() {
  await ensureViewer()
  if (!viewer) return
  if (props.url) {
    try { await viewer.loadSkin(props.url) } catch { /* broken url */ }
  }
}

function applyModel() {
  if (viewer) viewer.playerModelType = props.modelType === 'slim' ? 'slim' : 'default'
}

onMounted(apply)
watch(() => props.url, apply)
watch(() => props.modelType, applyModel)
onBeforeUnmount(() => { if (viewer) { viewer.dispose(); viewer = null } })
</script>

<template>
  <canvas ref="canvas" style="background: transparent;" />
</template>
