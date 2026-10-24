<script setup>
// DashboardPage (A 级批次): 仪表盘 —— 公告（Markdown 双层消毒之客户端层：
// marked 渲染 + DOMPurify 消毒后才 v-html；服务端已 bluemonday 一层）、
// 使用情况（角色 n/limit + 存储字节）、启动器快速配置（API 地址复制/
// 可拖拽链接/HMCL PCL 引导）、上传、纹理列表。
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { api, uploadTexture } from '../api.js'
import { guard401 } from '../session.js'

const { t } = useI18n()

const me = ref(null)
const textures = ref([])
const closetIds = ref(new Set())
const announcement = ref('')

const uploadType = ref('skin')
const uploadModel = ref('default')
const uploadFile = ref(null)
const fileInput = ref(null)
const uploading = ref(false)
const uploadErr = ref('')
const uploadOk = ref('')

// ---- announcement: client sanitization layer ----
// marked renders the RAW markdown; DOMPurify then strips anything hostile.
// The server html field exists for API consumers — the SPA renders its own
// copy so both layers (拍板双层消毒) always run in the browser path.
const announcementHtml = computed(() => {
  if (!announcement.value) return ''
  const raw = marked.parse(announcement.value)
  return DOMPurify.sanitize(raw)
})

// ---- usage card ----
const usageText = computed(() => {
  if (!me.value) return ''
  const n = me.value.playerCount ?? 0
  const limit = me.value.playerLimit ?? '—'
  const bytes = me.value.storageBytes ?? 0
  return t('dashboard.usageLine', { n, limit, storage: formatBytes(bytes) })
})

function formatBytes(b) {
  if (b < 1024) return `${b} B`
  const units = ['KB', 'MB', 'GB']
  let v = b
  let i = -1
  do { v /= 1024; i++ } while (v >= 1024 && i < units.length - 1)
  return `${v.toFixed(1)} ${units[i]}`
}

// ---- launcher quick config ----
const aliUrl = `${location.origin}/api/yggdrasil`
const copyState = ref('') // '' | 'ok' | 'fail'

async function copyAli() {
  try {
    await navigator.clipboard.writeText(aliUrl)
    copyState.value = 'ok'
  } catch { copyState.value = 'fail' }
  setTimeout(() => { copyState.value = '' }, 2000)
}

async function load() {
  try {
    const [u, tx, cl] = await Promise.all([api.me(), api.textures(), api.closet()])
    me.value = u
    textures.value = tx.textures || []
    closetIds.value = new Set((cl.closet || []).map((c) => c.textureId))
  } catch (e) { if (!guard401(e)) uploadErr.value = e.message }
}

async function loadAnnouncement() {
  try {
    const d = await fetch('/api/v1/announcement').then((r) => r.json())
    announcement.value = d.announcement || ''
  } catch { announcement.value = '' }
}

async function addToCloset(tx) {
  uploadErr.value = ''; uploadOk.value = ''
  try {
    await api.closetAdd(tx.id)
    const cl = await api.closet()
    closetIds.value = new Set((cl.closet || []).map((c) => c.textureId))
  } catch (e) { if (!guard401(e)) uploadErr.value = e.message }
}

async function doUpload() {
  if (!uploadFile.value) { uploadErr.value = t('dashboard.pickPng'); return }
  uploadErr.value = ''; uploadOk.value = ''; uploading.value = true
  try {
    await uploadTexture(uploadType.value, uploadFile.value, uploadType.value === 'skin' ? uploadModel.value : '', '')
    uploadOk.value = t('dashboard.uploadSuccess')
    uploadFile.value = null
    if (fileInput.value) fileInput.value.value = ''
    await load()
  } catch (e) { if (!guard401(e)) uploadErr.value = e.message } finally { uploading.value = false }
}

function onPick(e) { uploadFile.value = e.target.files[0] || null }

onMounted(() => { load(); loadAnnouncement() })
</script>

<template>
  <!-- announcement (Markdown, double-sanitized) -->
  <div class="card" v-if="announcement">
    <h2>{{ t('dashboard.announcement') }}</h2>
    <div class="md" v-html="announcementHtml"></div>
  </div>

  <!-- usage + launcher quick config -->
  <div class="card">
    <h2>{{ t('dashboard.usageTitle') }}</h2>
    <p style="margin: 4px 0 12px;">{{ usageText }}</p>
    <h3>{{ t('dashboard.launcherTitle') }}</h3>
    <p class="muted" style="margin: 4px 0 8px;">{{ t('dashboard.launcherHint') }}</p>
    <div class="row">
      <code class="ali">{{ aliUrl }}</code>
      <button class="secondary" @click="copyAli">{{ copyState === 'ok' ? t('dashboard.copied') : copyState === 'fail' ? t('dashboard.copyFail') : t('dashboard.copy') }}</button>
      <!-- draggable link: HMCL/PCL accept a dragged authlib-injector URL -->
      <a class="drag-btn" :href="aliUrl" draggable="true">{{ t('dashboard.dragBtn') }}</a>
    </div>
    <p class="muted" style="margin-top: 8px; font-size: 12px;">{{ t('dashboard.launcherTutor') }}</p>
  </div>

  <!-- upload -->
  <div class="card">
    <h2>{{ t('dashboard.uploadTitle') }}</h2>
    <div class="row">
      <select v-model="uploadType" style="width: 120px;">
        <option value="skin">{{ t('dashboard.typeSkin') }}</option>
        <option value="cape">{{ t('dashboard.typeCape') }}</option>
      </select>
      <select v-model="uploadModel" style="width: 140px;" :disabled="uploadType !== 'skin'">
        <option value="default">{{ t('dashboard.modelDefault') }}</option>
        <option value="slim">{{ t('dashboard.modelSlim') }}</option>
      </select>
      <input type="file" accept="image/png" ref="fileInput" @change="onPick" style="width: auto;" />
      <button :disabled="uploading" @click="doUpload">{{ uploading ? t('dashboard.uploading') : t('dashboard.upload') }}</button>
    </div>
    <div class="error" v-if="uploadErr">{{ uploadErr }}</div>
    <div class="success" v-if="uploadOk">{{ uploadOk }}</div>
    <div class="muted" style="margin-top: 8px;">{{ t('dashboard.uploadHint') }}</div>
  </div>

  <!-- textures -->
  <div class="card">
    <h2>{{ t('dashboard.texturesTitle', { count: textures.length }) }}</h2>
    <table v-if="textures.length">
      <tr><th>{{ t('dashboard.colType') }}</th><th>{{ t('dashboard.colModel') }}</th><th>{{ t('dashboard.colPreview') }}</th><th>{{ t('dashboard.colHash') }}</th><th></th></tr>
      <tr v-for="tx in textures" :key="tx.id">
        <td>{{ tx.type }}</td>
        <td>{{ tx.model || '—' }}</td>
        <td><img :src="tx.url" style="height: 48px; image-rendering: pixelated;" /></td>
        <td class="muted">{{ tx.hash.slice(0, 12) }}…</td>
        <td>
          <button class="secondary" v-if="!closetIds.has(tx.id)" @click="addToCloset(tx)">{{ t('dashboard.closetAdd') }}</button>
          <span class="muted" v-else style="font-size: 12px;">{{ t('dashboard.closetIn') }}</span>
        </td>
      </tr>
    </table>
    <div v-else class="muted">{{ t('dashboard.texturesEmpty') }}</div>
  </div>
</template>

<style scoped>
.md { margin-top: 8px; line-height: 1.65; }
.md :deep(h1), .md :deep(h2), .md :deep(h3) { margin: 12px 0 6px; }
.md :deep(p) { margin: 6px 0; }
.md :deep(ul), .md :deep(ol) { margin: 6px 0; padding-left: 22px; }
.md :deep(a) { color: var(--accent); }
.md :deep(code) { background: var(--bg); padding: 1px 5px; border-radius: 4px; font-size: 13px; }
.md :deep(img) { max-width: 100%; border-radius: 6px; }
h3 { font-size: 14px; margin: 14px 0 4px; color: var(--muted); font-weight: 600; }
.ali { background: var(--bg); border: 1px solid var(--border); padding: 7px 10px; border-radius: 6px; font-size: 13px; user-select: all; }
.drag-btn {
  display: inline-block; padding: 7px 14px; border: 1px dashed var(--accent);
  color: var(--accent); border-radius: 6px; text-decoration: none; font-size: 13px;
}
</style>
