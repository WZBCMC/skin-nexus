<script setup>
import { ref, watch, onMounted } from 'vue'

const props = defineProps({ url: { type: String, default: '' } })
const canvas = ref(null)
let viewer = null

async function ensureViewer() {
  if (viewer || !canvas.value) return
  const skinview3d = await import('skinview3d')
  viewer = new skinview3d.SkinViewer({
    canvas: canvas.value,
    width: 128,
    height: 192,
    zoom: 0.7,
  })
  viewer.autoRotate = true
}

async function apply() {
  await ensureViewer()
  if (!viewer) return
  if (props.url) {
    try { await viewer.loadSkin(props.url) } catch { /* broken url */ }
  }
}

onMounted(apply)
watch(() => props.url, apply)
</script>

<template>
  <canvas ref="canvas" style="background: transparent;" />
</template>
