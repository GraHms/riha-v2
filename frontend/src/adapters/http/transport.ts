import type { ApiError, Mode } from '../../core/model'

export class Transport {
  private mode: Mode
  private listeners = new Set<() => void>()

  private base: string
  private storage: Pick<Storage, 'getItem' | 'setItem'>

  constructor(base = '/api', storage: Pick<Storage, 'getItem' | 'setItem'> = localStorage) {
    this.base = base
    this.storage = storage
    this.mode = (storage.getItem('riha_mode') as Mode) || 'live'
  }

  getMode = () => this.mode
  setMode = (m: Mode) => { this.mode = m; this.storage.setItem('riha_mode', m) }
  onUnauthorized = (fn: () => void) => { this.listeners.add(fn); return () => { this.listeners.delete(fn) } }
  url = (path: string) => this.base + path

  async request<T>(path: string, opts: { method?: string; body?: unknown; form?: FormData } = {}): Promise<T> {
    const headers: Record<string, string> = { 'X-Requested-With': 'riha', 'X-Riha-Mode': this.mode }
    let body: BodyInit | undefined
    if (opts.form) body = opts.form
    else if (opts.body !== undefined) { headers['Content-Type'] = 'application/json'; body = JSON.stringify(opts.body) }
    const res = await fetch(this.base + path, { method: opts.method || (body ? 'POST' : 'GET'), headers, body, credentials: 'same-origin' })
    const text = await res.text()
    let data: unknown = null
    try { data = text ? JSON.parse(text) : null } catch { data = null }
    if (!res.ok) {
      const e: ApiError = { code: 'error', message: 'Algo correu mal. Tente de novo.', ...((data as { error?: object })?.error || {}), status: res.status }
      if (res.status === 401 && !path.startsWith('/auth/') && !path.startsWith('/public/')) this.listeners.forEach(f => f())
      throw e
    }
    return data as T
  }
}
