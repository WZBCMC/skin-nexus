// Session-wide unauthorized notifier: pages call notifyUnauthorized() when a
// 401 hits their guard; App.vue registered the handler that drops the shell
// back to the login card (review P1-6 semantics, now cross-component).
let handler = null
export function setUnauthorizedHandler(fn) { handler = fn }
export function notifyUnauthorized() { if (handler) handler() }

// Shared 401 guard for page-level catch blocks: true when the error was a
// session-drop (caller should stop its own error toast).
export function guard401(e) {
  if (e && e.status === 401) { notifyUnauthorized(); return true }
  return false
}
