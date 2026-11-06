// What a checkout page tells RIHA about the customer's side: the connection, when
// they leave the page (to answer the prompt on their phone) and come back, errors
// in their browser. It goes to the payment's trail, for finding out what happened.
// Nothing here may ever get in the way of paying: every failure is ignored.

type Ev = { name: string; at: number; data: Record<string, unknown> }

let session = ''
let queue: Ev[] = []
let timer: number | undefined
let started = false

function connection() {
  const c = (navigator as any).connection
  if (!c) return {}
  return { connection: c.effectiveType || c.type || '', downlink_mbps: c.downlink, rtt_ms: c.rtt, save_data: !!c.saveData }
}

function flush(final = false) {
  if (!session || !queue.length) return
  const now = Date.now()
  const events = queue.splice(0, 30).map(e => ({ name: e.name, at: Math.max(0, now - e.at), data: e.data }))
  try {
    fetch(`/api/public/checkout/${encodeURIComponent(session)}/telemetry`, {
      method: 'POST', keepalive: final, credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json', 'X-Requested-With': 'riha' }, body: JSON.stringify({ events }),
    }).catch(() => {})
  } catch { /* ignored */ }
}

export function track(name: string, data: Record<string, unknown> = {}) {
  if (!session) return
  queue.push({ name, at: Date.now(), data })
  if (queue.length > 60) queue = queue.slice(-60)
  window.clearTimeout(timer)
  timer = window.setTimeout(() => flush(), name === 'opened' || name === 'submit' ? 300 : 2000)
}

// startTelemetry begins for a checkout, once.
export function startTelemetry(sessionId: string) {
  session = sessionId
  if (started) return
  started = true
  const nav = performance.getEntriesByType?.('navigation')?.[0] as PerformanceNavigationTiming | undefined
  track('opened', {
    ...connection(), ready_ms: Math.round(nav ? nav.domContentLoadedEventEnd : performance.now()), online: navigator.onLine,
    screen: `${screen.width}x${screen.height}`, language: navigator.language, device_memory_gb: (navigator as any).deviceMemory,
  })
  document.addEventListener('visibilitychange', () => {
    track(document.visibilityState === 'hidden' ? 'hidden' : 'visible', connection())
    if (document.visibilityState === 'hidden') flush(true)
  })
  window.addEventListener('offline', () => track('offline'))
  window.addEventListener('online', () => track('online', connection()))
  window.addEventListener('pagehide', () => flush(true))
  window.addEventListener('error', e => track('error', { message: String(e.message || 'erro'), source: String(e.filename || '').split('/').pop(), line: e.lineno }))
  window.addEventListener('unhandledrejection', e => track('error', { message: String((e.reason && (e.reason.message || e.reason)) || 'erro numa promessa') }))
}
