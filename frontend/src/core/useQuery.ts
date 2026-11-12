import { useCallback, useEffect, useRef, useState } from 'react'
import { errMsg } from './model'
import type { Gateway } from './ports'
import { useGateway } from './gateway'

export function useQuery<T>(fetcher: ((g: Gateway) => Promise<T>) | null, deps: unknown[] = [], pollMs = 0) {
  const g = useGateway()
  const [data, setData] = useState<T | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const seq = useRef(0)
  const fn = useRef(fetcher)
  fn.current = fetcher
  const load = useCallback(async (quiet = false) => {
    if (!fn.current) return
    const n = ++seq.current
    if (!quiet) setLoading(true)
    try { const d = await fn.current(g); if (n === seq.current) { setData(d); setError(null) } }
    catch (e) { if (n === seq.current) setError(errMsg(e)) }
    finally { if (n === seq.current) setLoading(false) }
  }, [g])
  useEffect(() => { load() }, [load, ...deps]) // eslint-disable-line react-hooks/exhaustive-deps
  // A tab nobody is looking at doesn't poll; it catches up when seen again.
  useEffect(() => {
    if (!pollMs) return
    const id = setInterval(() => { if (!document.hidden) load(true) }, pollMs)
    const seen = () => { if (!document.hidden) load(true) }
    document.addEventListener('visibilitychange', seen)
    return () => { clearInterval(id); document.removeEventListener('visibilitychange', seen) }
  }, [load, pollMs])
  return { data, error, loading, reload: () => load(true), setData }
}
