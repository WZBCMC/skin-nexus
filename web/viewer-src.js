// viewer-src.js — fixed-name entry (/assets/viewer.js) for the
// server-rendered texture detail page (texturepage.go).
//
// Progressive enhancement contract: the page ships a complete static
// <img> preview that crawlers and no-JS visitors read; this module upgrades
// it into an auto-rotating skinview3d canvas and wires the like toggle.
// Nothing here is required to understand the page — SEO content lives in
// the server-rendered HTML, not in this file.
//
// skinview3d is dynamically imported only when a canvas is actually present
// (i.e. a skin, not a cape) and WebGL is available, so the ~500 KB chunk
// never loads on cape pages or legacy browsers.

async function upgradeViewer() {
  const wrap = document.getElementById('tex-viewer')
  const canvas = wrap && wrap.querySelector('canvas[data-skinview3d]')
  if (!wrap || !canvas) return

  // WebGL probe: without it skinview3d throws — keep the static preview.
  try {
    if (!canvas.getContext('webgl2') && !canvas.getContext('webgl')) return
  } catch { return }

  try {
    const skinview3d = await import('skinview3d')
    const viewer = new skinview3d.SkinViewer({
      canvas,
      width: Math.min(320, wrap.clientWidth || 320),
      height: 420,
      zoom: 0.9,
      autoRotate: true,
    })
    const model = canvas.dataset.model === 'slim' ? 'slim' : 'default'
    await viewer.loadSkin(canvas.dataset.texture, { model })
    // Only hide the static preview once the first frame is actually up —
    // a failed load leaves the <img> in place instead of a blank box.
    wrap.classList.add('ready')
  } catch {
    /* keep static preview */
  }
}

function wireLikeButton() {
  const btn = document.querySelector('[data-like]')
  if (!btn) return
  const tid = btn.getAttribute('data-like')
  const countEl = document.getElementById('like-count')
  btn.addEventListener('click', async () => {
    const liked = btn.getAttribute('data-liked') === '1'
    btn.disabled = true
    try {
      const res = await fetch(`/api/v1/skinlib/${tid}/like`, {
        method: liked ? 'DELETE' : 'POST',
        headers: { 'Content-Type': 'application/json' },
      })
      if (res.status === 401) { location.href = '/login'; return }
      if (!res.ok) return
      const data = await res.json().catch(() => null)
      btn.setAttribute('data-liked', liked ? '0' : '1')
      btn.classList.toggle('liked', !liked)
      if (data && typeof data.likes === 'number' && countEl) {
        countEl.textContent = String(data.likes)
      }
    } catch { /* offline — ignore */ }
    finally { btn.disabled = false }
  })
}

upgradeViewer()
wireLikeButton()
