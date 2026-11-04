import { useCallback, useEffect, useRef, useState } from 'react'
import { api, errMsg } from './api'

/** Fetch JSON from the dashboard API; re-fetches when deps change. */
export function useData<T = any>(path: string | null, deps: unknown[] = [], pollMs = 0) {
  const [data, setData] = useState<T | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const seq = useRef(0)
  const load = useCallback(async (quiet = false) => {
    if (!path) return
    const n = ++seq.current
    if (!quiet) setLoading(true)
    try { const d = await api<T>(path); if (n === seq.current) { setData(d); setError(null) } }
    catch (e) { if (n === seq.current) setError(errMsg(e)) }
    finally { if (n === seq.current) setLoading(false) }
  }, [path]) // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => { load() }, [load, ...deps]) // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => {
    if (!pollMs) return
    const id = setInterval(() => load(true), pollMs)
    return () => clearInterval(id)
  }, [load, pollMs])
  return { data, error, loading, reload: () => load(true), setData }
}
