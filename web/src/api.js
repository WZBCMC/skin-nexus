// Thin fetch wrapper: same-origin cookies, JSON in/out.
async function req(method, path, body) {
  const opts = { method, headers: {} }
  if (body !== undefined) {
    opts.headers['Content-Type'] = 'application/json'
    opts.body = JSON.stringify(body)
  }
  const res = await fetch(path, opts)
  if (res.status === 204) return null
  const data = await res.json().catch(() => ({}))
  if (!res.ok) throw new Error(data.message || `HTTP ${res.status}`)
  return data
}

export const api = {
  me: () => req('GET', '/api/v1/me'),
  login: (identifier, password) => req('POST', '/api/v1/login', { identifier, password }),
  register: (email, nickname, password) => req('POST', '/api/v1/register', { email, nickname, password }),
  logout: () => req('POST', '/api/v1/logout'),
  players: () => req('GET', '/api/v1/me/players'),
  createPlayer: (name) => req('POST', '/api/v1/me/players', { name }),
  textures: () => req('GET', '/api/v1/me/textures'),
  setSkin: (pid, textureId) => req('PUT', `/api/v1/me/players/${pid}/skin`, { textureId }),
  setCape: (pid, textureId) => req('PUT', `/api/v1/me/players/${pid}/cape`, { textureId }),
}

export async function uploadTexture(type, file, model, description) {
  const fd = new FormData()
  fd.append('type', type)
  fd.append('file', file)
  if (model) fd.append('model', model)
  if (description) fd.append('description', description)
  const res = await fetch('/api/v1/me/textures', { method: 'POST', body: fd })
  const data = await res.json().catch(() => ({}))
  if (!res.ok) throw new Error(data.message || `HTTP ${res.status}`)
  return data
}
