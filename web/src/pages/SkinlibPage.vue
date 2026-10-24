<script setup>
// Skinlib — public texture library (blessing skinlib.index parity).
// Like heart = closet add/remove (legacy semantics). Owner sees rename /
// visibility toggle on their own entries.
import { ref, computed, watch, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api.js'

const { t } = useI18n()

const items = ref([])
const total = ref(0)
const page = ref(1)
const perPage = 20
const filter = ref('skin')
const sort = ref('time')
const keyword = ref('')
const mine = ref(false)
const loading = ref(false)
const err = ref('')
const me = ref(null)

const totalPages = computed(() => Math.max(1, Math.ceil(total.value / perPage)))
const canEdit = (it) => me.value && (me.value.id === it.uploader || me.value.permission > 0)

async function load() {
  loading.value = true
  err.value = ''
  try {
    const data = await api.skinlib({
      filter: filter.value,
      sort: sort.value,
      keyword: keyword.value.trim(),
      page: page.value,
      uploader: mine.value && me.value ? me.value.id : '',
    })
    items.value = data.items || []
    total.value = data.total || 0
  } catch (e) {
    err.value = e.message
  } finally {
    loading.value = false
  }
}

let searchTimer = null
function onSearch() {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(() => { page.value = 1; load() }, 300)
}

function setFilter(f) { filter.value = f; page.value = 1; load() }
function setSort(s) { sort.value = s; page.value = 1; load() }
function toggleMine() { mine.value = !mine.value; page.value = 1; load() }
function goPage(p) {
  if (p < 1 || p > totalPages.value || p === page.value) return
  page.value = p
  load()
}

async function toggleLike(it) {
  try {
    if (it.liked) {
      const res = await api.skinlibUnlike(it.tid)
      it.liked = false
      it.likes = res.likes
    } else {
      const res = await api.skinlibLike(it.tid)
      it.liked = true
      it.likes = res.likes
    }
  } catch (e) {
    err.value = e.message
  }
}

async function togglePublic(it) {
  try {
    await api.skinlibPatch(it.tid, { public: !it.public })
    it.public = !it.public
  } catch (e) {
    err.value = e.message
  }
}

async function renameItem(it) {
  const next = prompt(t('skinlib.renamePrompt'), it.name || '')
  if (next === null) return
  try {
    await api.skinlibPatch(it.tid, { name: next.trim() })
    it.name = next.trim()
  } catch (e) {
    err.value = e.message
  }
}

// Server-rendered preview PNG (avatargen /preview/{tid}: back+front views).
// One <img> per card — a skinview3d canvas per card burns a WebGL context
// each and the grid exceeds the browser's ~16-context budget: oldest
// contexts get dropped (white cards + "Context Lost" + renderer crashes).
// The API hands back a versioned previewURL (?v=rendererVersion) so the
// year-long immutable cache re-fetches after a rasterizer fix.
// Capes have no body render; serve the flat texture instead.
function previewURL(it) {
  return it.type === 'skin' ? (it.previewURL || `/preview/${it.tid}`) : it.textureURL
}

onMounted(async () => {
  try { me.value = await api.me() } catch { me.value = null }
  load()
})
</script>

<template>
  <div class="card">
    <h2 style="margin-top: 0;">{{ t('skinlib.title') }}</h2>
    <p class="muted">{{ t('skinlib.subtitle') }}</p>

    <div class="row" style="flex-wrap: wrap; gap: 8px; align-items: center;">
      <div class="tabs">
        <button :class="{ active: filter === 'skin' }" @click="setFilter('skin')">{{ t('skinlib.tabSkin') }}</button>
        <button :class="{ active: filter === 'cape' }" @click="setFilter('cape')">{{ t('skinlib.tabCape') }}</button>
        <button :class="{ active: filter === 'steve' }" @click="setFilter('steve')">{{ t('skinlib.tabSteve') }}</button>
        <button :class="{ active: filter === 'alex' }" @click="setFilter('alex')">{{ t('skinlib.tabAlex') }}</button>
      </div>
      <input v-model="keyword" :placeholder="t('skinlib.searchPlaceholder')" style="max-width: 220px;" @input="onSearch" />
      <select :value="sort" style="max-width: 140px;" @change="setSort($event.target.value)">
        <option value="time">{{ t('skinlib.sortTime') }}</option>
        <option value="likes">{{ t('skinlib.sortLikes') }}</option>
      </select>
      <button class="secondary" :class="{ 'mine-on': mine }" @click="toggleMine">{{ t('skinlib.mineOnly') }}</button>
    </div>

    <div class="error" v-if="err">{{ err }}</div>

    <p class="muted" v-if="loading">{{ t('common.loading') }}</p>
    <p class="muted" v-else-if="!items.length">{{ t('skinlib.empty') }}</p>

    <div class="skinlib-grid" v-else>
      <div class="skin-card" v-for="it in items" :key="it.tid">
        <div class="skin-card-prev">
          <img v-if="it.type === 'skin'" :src="previewURL(it)" :alt="it.name" class="skin-preview-img" loading="lazy" />
          <img v-else :src="previewURL(it)" :alt="it.name" class="cape-thumb" loading="lazy" />
        </div>
        <div class="skin-card-body">
          <div class="skin-card-name" :title="it.name">
            <span v-if="!it.public" class="lock">🔒</span>
            {{ it.name || `#${it.tid}` }}
          </div>
          <div class="skin-card-meta">
            <span class="badge" :class="it.model === 'slim' ? 'alex' : 'steve'">
              {{ it.model === 'slim' ? 'Alex' : 'Steve' }}
            </span>
            <span class="uploader">{{ it.nickname }}</span>
          </div>
          <div class="skin-card-actions">
            <button class="like-btn" :class="{ liked: it.liked }" @click="toggleLike(it)">
              ♥ {{ it.likes }}
            </button>
            <template v-if="canEdit(it)">
              <button class="ghost" @click="renameItem(it)">{{ t('common.rename') }}</button>
              <button class="ghost" @click="togglePublic(it)">
                {{ it.public ? t('skinlib.makePrivate') : t('skinlib.makePublic') }}
              </button>
            </template>
          </div>
        </div>
      </div>
    </div>

    <div class="row pager" v-if="totalPages > 1">
      <button class="secondary" :disabled="page <= 1" @click="goPage(page - 1)">‹</button>
      <span class="muted">{{ page }} / {{ totalPages }} · {{ t('skinlib.total', { n: total }) }}</span>
      <button class="secondary" :disabled="page >= totalPages" @click="goPage(page + 1)">›</button>
    </div>
  </div>
</template>

<style scoped>
.tabs { display: flex; gap: 6px; }
.tabs button {
  padding: 6px 12px; border-radius: 6px; border: 1px solid var(--border);
  background: transparent; color: var(--text-dim); cursor: pointer; font-size: 13px;
}
.tabs button.active { background: var(--accent); border-color: var(--accent); color: #fff; }
.mine-on { border-color: var(--accent) !important; color: var(--accent) !important; }

.skinlib-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 14px; margin-top: 14px;
}
.skin-card {
  border: 1px solid var(--border); border-radius: 10px; overflow: hidden;
  background: var(--bg); display: flex; flex-direction: column;
}
.skin-card-prev {
  height: 180px; display: flex; align-items: center; justify-content: center;
  background: rgba(255, 255, 255, 0.02);
}
.skin-preview-img { height: 150px; image-rendering: pixelated; object-fit: contain; }
.cape-thumb { max-width: 90%; max-height: 150px; image-rendering: pixelated; }
.skin-card-body { padding: 10px 12px; display: flex; flex-direction: column; gap: 6px; }
.skin-card-name {
  font-weight: 600; font-size: 14px; white-space: nowrap;
  overflow: hidden; text-overflow: ellipsis;
}
.lock { margin-right: 2px; }
.skin-card-meta { display: flex; align-items: center; gap: 6px; font-size: 12px; }
.badge {
  padding: 1px 7px; border-radius: 999px; font-size: 11px; font-weight: 600;
}
.badge.steve { background: rgba(0, 200, 80, 0.18); color: #6f6; }
.badge.alex { background: rgba(255, 180, 0, 0.18); color: #fb3; }
.uploader { color: var(--text-dim); }
.skin-card-actions { display: flex; gap: 6px; align-items: center; flex-wrap: wrap; }
.like-btn {
  border: 1px solid var(--border); background: transparent; color: var(--text-dim);
  border-radius: 6px; padding: 3px 9px; cursor: pointer; font-size: 13px;
}
.like-btn.liked { color: #f66; border-color: #f66; }
.ghost {
  border: none; background: transparent; color: var(--text-dim);
  cursor: pointer; font-size: 12px; padding: 3px 4px;
}
.ghost:hover { color: var(--accent); }
.pager { justify-content: center; margin-top: 16px; gap: 12px; }
</style>
