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

  async request<T>(path: string, opts: { method?: string; body?: unknown; form?: FormData; timeoutMs?: number; signal?: AbortSignal } = {}): Promise<T> {
    const headers: Record<string, string> = { 'X-Requested-With': 'riha', 'X-Riha-Mode': this.mode }
    let body: BodyInit | undefined
    if (opts.form) body = opts.form
    else if (opts.body !== undefined) { headers['Content-Type'] = 'application/json'; body = JSON.stringify(opts.body) }
    // A phone drops requests when the page goes to the background, as when a
    // customer leaves to enter their PIN: the browser's own message means nothing to them.
    // A network cut can leave a request open with no error at all, so it must end by itself.
    const abort = new AbortController()
    const timer = setTimeout(() => abort.abort(), opts.timeoutMs ?? (opts.form ? 120_000 : 25_000))
    opts.signal?.addEventListener('abort', () => abort.abort())
    let res: Response
    let text: string
    try {
      res = await fetch(this.base + path, { method: opts.method || (body ? 'POST' : 'GET'), headers, body, credentials: 'same-origin', signal: abort.signal })
      text = await res.text()
    } catch {
      clearTimeout(timer)
      throw { code: 'network_error', message: 'Sem ligação. Verifique a internet e tente de novo.', status: 0 } satisfies ApiError
    }
    clearTimeout(timer)
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
