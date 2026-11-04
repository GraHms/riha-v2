// Thin client for the Riha dashboard API (cookie session + X-Riha-Mode).
export type ApiError = { type?: string; code: string; message: string; param?: string; request_id?: string; status: number }

let mode: 'live' | 'test' = (localStorage.getItem('riha_mode') as 'live' | 'test') || 'live'
export const getMode = () => mode
export const setModeHeader = (m: 'live' | 'test') => { mode = m; localStorage.setItem('riha_mode', m) }

export async function api<T = any>(path: string, opts: { method?: string; body?: unknown; form?: FormData } = {}): Promise<T> {
  const headers: Record<string, string> = { 'X-Requested-With': 'riha', 'X-Riha-Mode': mode }
  let body: BodyInit | undefined
  if (opts.form) body = opts.form
  else if (opts.body !== undefined) { headers['Content-Type'] = 'application/json'; body = JSON.stringify(opts.body) }
  const res = await fetch('/api' + path, { method: opts.method || (body ? 'POST' : 'GET'), headers, body, credentials: 'same-origin' })
  const text = await res.text()
  let data: any = null
  try { data = text ? JSON.parse(text) : null } catch { data = null }
  if (!res.ok) {
    const e: ApiError = { code: 'error', message: 'Algo correu mal. Tente de novo.', ...(data?.error || {}), status: res.status }
    if (res.status === 401 && !path.startsWith('/auth/') && !path.startsWith('/public/')) window.dispatchEvent(new Event('riha:unauthorized'))
    throw e
  }
  return data as T
}

export const errMsg = (e: unknown) => (e as ApiError)?.message || 'Algo correu mal. Tente de novo.'
