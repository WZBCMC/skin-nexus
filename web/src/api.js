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
  register: (email, nickname, password, invitationCode) => req('POST', '/api/v1/register', { email, nickname, password, invitationCode }),
  logout: () => req('POST', '/api/v1/logout'),
  players: () => req('GET', '/api/v1/me/players'),
  createPlayer: (name) => req('POST', '/api/v1/me/players', { name }),
  textures: () => req('GET', '/api/v1/me/textures'),
  setSkin: (pid, textureId) => req('PUT', `/api/v1/me/players/${pid}/skin`, { textureId }),
  setCape: (pid, textureId) => req('PUT', `/api/v1/me/players/${pid}/cape`, { textureId }),
}

// OIDC device flow + authorized-apps management.
export const oidc = {
  deviceInfo: (userCode) => req('GET', `/api/v1/oidc/device/info?user_code=${encodeURIComponent(userCode)}`),
  deviceSubmit: (userCode, decision, pid) => req('POST', '/api/v1/oidc/device/submit', { user_code: userCode, decision, pid }),
  tokens: () => req('GET', '/api/v1/oidc/tokens'),
  revokeToken: (body) => req('POST', '/api/v1/oidc/tokens/revoke', body),
}

// MUA Union member surface (routes exist only when the module is enabled).
export const union = {
  servers: () => req('GET', '/api/v1/union/servers'),
  detail: (uuid) => req('GET', `/api/v1/union/detail?uuid=${encodeURIComponent(uuid)}`),
  unmapped: (name) => req('GET', `/api/v1/union/unmapped?name=${encodeURIComponent(name)}`),
  bind: (uuid) => req('POST', '/api/v1/union/bind', { uuid }),
  bindto: (uuid, token) => req('POST', '/api/v1/union/bindto', { uuid, token }),
  unbind: (uuid) => req('POST', '/api/v1/union/unbind', { uuid }),
  remapuuid: (me, target) => req('POST', '/api/v1/union/remapuuid', { me, target }),
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
