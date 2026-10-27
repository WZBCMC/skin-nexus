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
    // 失败可见（2026-10-27 逻辑修复：原静默 return）：按钮旁浮出 3 秒提示
    const tip = (msg) => {
      let el = document.getElementById('like-tip')
      if (!el) {
        el = document.createElement('span')
        el.id = 'like-tip'
        el.style.cssText = 'color:#d9564c;font-size:13px;margin-left:8px'
        btn.parentElement.appendChild(el)
      }
      el.textContent = msg
      setTimeout(() => el.remove(), 3000)
    }
    try {
      const res = await fetch(`/api/v1/skinlib/${tid}/like`, {
        method: liked ? 'DELETE' : 'POST',
        headers: { 'Content-Type': 'application/json' },
      })
      // 401：带 next 回跳，登录后回到本详情页（原直接 /login 丢失上下文）
      if (res.status === 401) {
        location.href = '/login?next=' + encodeURIComponent(location.pathname)
        return
      }
      if (!res.ok) { tip('操作失败，请稍后重试'); return }
      const data = await res.json().catch(() => null)
      btn.setAttribute('data-liked', liked ? '0' : '1')
      btn.classList.toggle('liked', !liked)
      if (data && typeof data.likes === 'number' && countEl) {
        countEl.textContent = String(data.likes)
      }
    } catch { tip('网络异常，请稍后重试') }
    finally { btn.disabled = false }
  })
}

upgradeViewer()
wireLikeButton()
