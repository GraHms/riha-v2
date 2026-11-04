import { useEffect, useState } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { api, errMsg } from '../../lib/api'
import { fmt, parseMT } from '../../lib/format'
import { Avatar, Icon, Spinner } from '../../components/ui'

type Link = { name: string; amount: number; custom_amount: boolean; active: boolean; type: string; livemode: boolean; merchant: { name: string; initials: string; avatar_color: string } }

/** Public payment link: opens a checkout session (asking for the amount when the payer chooses it). */
export default function LinkLanding() {
  const { merchant, slug } = useParams()
  const [sp] = useSearchParams()
  const nav = useNavigate()
  const [link, setLink] = useState<Link | null>(null)
  const [err, setErr] = useState('')
  const [amount, setAmount] = useState('')
  const [busy, setBusy] = useState(false)
  const channel = sp.get('c') || ''

  const open = async (centavos: number) => {
    setBusy(true); setErr('')
    try {
      const r = await api<{ id: string }>(`/public/links/${merchant}/${slug}/sessions`, { body: { amount: centavos, channel } })
      nav('/c/' + r.id, { replace: true })
    } catch (e) { setErr(errMsg(e)); setBusy(false) }
  }

  useEffect(() => {
    api<Link>(`/public/links/${merchant}/${slug}`).then(l => {
      setLink(l)
      if (l.active && !l.custom_amount) open(l.amount)
    }).catch(e => setErr(errMsg(e)))
  }, [merchant, slug]) // eslint-disable-line react-hooks/exhaustive-deps

  const c = parseMT(amount)
  return (
    <div style={{ minHeight: '100%', display: 'flex', justifyContent: 'center', padding: '24px 20px' }}>
      <div style={{ width: '100%', maxWidth: 420, display: 'flex', flexDirection: 'column' }}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 6, height: 30, padding: '0 14px', borderRadius: 10, background: 'var(--paper-2)', fontSize: 13, color: 'var(--text-2)', alignSelf: 'center' }}><Icon name="lock" size={12} stroke={2.4} />pay.riha.co.mz</div>
        {!link && !err && <div style={{ display: 'grid', placeItems: 'center', flex: 1 }}><Spinner dark /></div>}
        {err && !link?.custom_amount && <div style={{ marginTop: 40, textAlign: 'center' }}><div className="display" style={{ fontSize: 26 }}>Link indisponível</div><p className="muted">{err}</p></div>}
        {link && !link.active && <div style={{ marginTop: 40, textAlign: 'center' }}><div className="display" style={{ fontSize: 26 }}>Este link já não está ativo</div><p className="muted">Peça um novo link a {link.merchant.name}.</p></div>}
        {link && link.active && !link.custom_amount && !err && <div style={{ display: 'grid', placeItems: 'center', flex: 1, color: 'var(--muted)', gap: 12 }}><Spinner dark />A abrir o pagamento…</div>}
        {link && link.active && link.custom_amount && (
          <>
            <div className="card" style={{ marginTop: 18, borderRadius: 24, padding: 18 }}>
              <div className="row"><Avatar text={link.merchant.initials} color={link.merchant.avatar_color} size={44} radius={14} font={15} /><div><div style={{ fontWeight: 600, fontSize: 16 }}>{link.merchant.name}</div><div style={{ fontSize: 12.5, color: 'var(--muted)' }}>{link.name}</div></div></div>
            </div>
            <div style={{ textAlign: 'center', marginTop: 30 }}>
              <div style={{ fontSize: 13.5, color: 'var(--muted)' }}>Quanto quer pagar?</div>
              <div style={{ display: 'flex', alignItems: 'baseline', justifyContent: 'center', gap: 8, marginTop: 6 }}>
                <input autoFocus inputMode="decimal" value={amount} onChange={e => setAmount(e.target.value.replace(/[^\d.,]/g, '').slice(0, 10))} placeholder="0" style={{ width: `${Math.max(1, amount.length) * 0.62 + 0.2}em`, minWidth: 60, maxWidth: 280, border: 0, background: 'transparent', textAlign: 'right', fontFamily: 'var(--display)', fontWeight: 700, fontSize: 58, letterSpacing: '-0.045em', padding: 0, caretColor: 'var(--green)' }} />
                <span className="display" style={{ fontSize: 26, color: 'var(--muted-2)' }}>MT</span>
              </div>
              <div style={{ display: 'flex', justifyContent: 'center', gap: 8, marginTop: 16 }}>
                {[100, 250, 500, 1000].map(v => <button key={v} className="btn btn-outline" style={{ height: 36, borderRadius: 999, fontSize: 13.5 }} onClick={() => setAmount(String(v))}>{fmt(v * 100, false)}</button>)}
              </div>
            </div>
            {err && <div className="form-err" style={{ marginTop: 16 }}>{err}</div>}
            <div style={{ flex: 1, minHeight: 30 }} />
            <button className="btn btn-green btn-lg btn-block" disabled={busy || c < 1000} onClick={() => open(c)}>{busy ? <Spinner /> : `Continuar · ${fmt(c)} MT`}</button>
          </>
        )}
      </div>
    </div>
  )
}
