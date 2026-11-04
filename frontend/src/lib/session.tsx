import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from 'react'
import { api, getMode, setModeHeader } from './api'

export type Merchant = {
  id: string; slug: string; name: string; legal_name: string; business_type: string; nuit: string; phone: string; email: string
  city: string; owner_name: string; status: string; fee_bps: number; fee_negotiated: boolean; fee_label: string
  statement_descriptor: string; avatar_color: string; success_url: string; cancel_url: string; sms_notifications: boolean
  approved_at: string | null; kyc_submitted_at: string | null; review_note: string; expected_volume: string; created_at: string
}
export type Me = { user: { id: string; email: string; name: string; role: string; title: string }; merchant?: Merchant; default_fee_bps: number }

type Ctx = {
  me: Me | null; loading: boolean; refresh: () => Promise<Me | null>; setMe: (m: Me | null) => void
  live: boolean; setLive: (v: boolean) => void; logout: () => Promise<void>
  toast: (msg: string, err?: boolean) => void
}
const C = createContext<Ctx>(null as unknown as Ctx)
export const useSession = () => useContext(C)

export function SessionProvider({ children }: { children: ReactNode }) {
  const [me, setMe] = useState<Me | null>(null)
  const [loading, setLoading] = useState(true)
  const [live, setLiveState] = useState(getMode() === 'live')
  const [t, setT] = useState<{ msg: string; err?: boolean } | null>(null)

  const refresh = useCallback(async () => {
    try { const m = await api<Me>('/auth/me'); setMe(m); return m } catch { setMe(null); return null } finally { setLoading(false) }
  }, [])
  useEffect(() => { refresh() }, [refresh])
  useEffect(() => {
    const h = () => setMe(null)
    window.addEventListener('riha:unauthorized', h)
    return () => window.removeEventListener('riha:unauthorized', h)
  }, [])
  useEffect(() => { if (!t) return; const id = setTimeout(() => setT(null), 2600); return () => clearTimeout(id) }, [t])

  const setLive = (v: boolean) => { setModeHeader(v ? 'live' : 'test'); setLiveState(v) }
  const logout = async () => { try { await api('/auth/logout', { method: 'POST' }) } finally { setMe(null) } }
  const toast = (msg: string, err?: boolean) => setT({ msg, err })

  return (
    <C.Provider value={{ me, loading, refresh, setMe, live, setLive, logout, toast }}>
      {children}
      {t && <div className={'toast' + (t.err ? ' err' : '')} role="status"><span className="dot" />{t.msg}</div>}
    </C.Provider>
  )
}
