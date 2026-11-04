import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from 'react'
import { useGateway } from '../core/gateway'
import type { Me } from '../core/model'

export type { Me, Merchant } from '../core/model'

type Ctx = {
  me: Me | null; loading: boolean; refresh: () => Promise<Me | null>; setMe: (m: Me | null) => void
  live: boolean; setLive: (v: boolean) => void; logout: () => Promise<void>
  toast: (msg: string, err?: boolean) => void
}
const C = createContext<Ctx>(null as unknown as Ctx)
export const useSession = () => useContext(C)

export function SessionProvider({ children }: { children: ReactNode }) {
  const g = useGateway()
  const [me, setMe] = useState<Me | null>(null)
  const [loading, setLoading] = useState(true)
  const [live, setLiveState] = useState(g.mode.get() === 'live')
  const [t, setT] = useState<{ msg: string; err?: boolean } | null>(null)

  const refresh = useCallback(async () => {
    try { const m = await g.auth.me(); setMe(m); return m } catch { setMe(null); return null } finally { setLoading(false) }
  }, [g])
  useEffect(() => { refresh() }, [refresh])
  useEffect(() => g.onUnauthorized(() => setMe(null)), [g])
  useEffect(() => { if (!t) return; const id = setTimeout(() => setT(null), 2600); return () => clearTimeout(id) }, [t])

  const setLive = (v: boolean) => { g.mode.set(v ? 'live' : 'test'); setLiveState(v) }
  const logout = async () => { try { await g.auth.logout() } finally { setMe(null) } }
  const toast = (msg: string, err?: boolean) => setT({ msg, err })

  return (
    <C.Provider value={{ me, loading, refresh, setMe, live, setLive, logout, toast }}>
      {children}
      {t && <div className={'toast' + (t.err ? ' err' : '')} role="status"><span className="dot" />{t.msg}</div>}
    </C.Provider>
  )
}
