import { useEffect, useState } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { errMsg, type ApiError } from '../../core/model'
import { useGateway } from '../../core/gateway'
import { fmt, parseMT } from '../../lib/format'
import { Icon, Spinner } from '../../components/ui'
import { LinkHeader, quickAmounts, rangeText, submitWords } from './LinkCard'

type Link = { name: string; amount: number; custom_amount: boolean; active: boolean; type: string; livemode: boolean; details?: string; image_url?: string; suggested_amount: number; min_amount: number; max_amount: number; submit_type?: string; merchant: { name: string; initials: string; avatar_color: string } }

export default function LinkLanding() {
  const g = useGateway()
  const { merchant, slug } = useParams()
  const [sp] = useSearchParams()
  const nav = useNavigate()
  const [link, setLink] = useState<Link | null>(null)
  const [err, setErr] = useState('')
  const [offline, setOffline] = useState(false)
  const [amount, setAmount] = useState('')
  const [busy, setBusy] = useState(false)
  const channel = sp.get('c') || ''

  const open = async (centavos: number) => {
    setBusy(true); setErr('')
    try {
      const r = await g.checkout.openLink(merchant!, slug!, centavos, channel)
      nav('/c/' + r.id, { replace: true })
    } catch (e) { setOffline((e as ApiError).code === 'network_error'); setErr(errMsg(e)); setBusy(false) }
  }

  useEffect(() => {
    g.checkout.link(merchant!, slug!).then(l => {
      setLink(l)
      if (l.custom_amount && l.suggested_amount) setAmount(String(l.suggested_amount / 100))
      if (l.active && !l.custom_amount) open(l.amount)
    }).catch(e => { setOffline((e as ApiError).code === 'network_error'); setErr(errMsg(e)) })
  }, [merchant, slug]) // eslint-disable-line react-hooks/exhaustive-deps

  const c = parseMT(amount)
  const outside = !!link && link.custom_amount && c > 0 && (c < link.min_amount || c > link.max_amount)
  return (
    <div style={{ minHeight: '100%', display: 'flex', justifyContent: 'center', padding: '24px 20px' }}>
      <div style={{ width: '100%', maxWidth: 420, display: 'flex', flexDirection: 'column' }}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 6, height: 30, padding: '0 14px', borderRadius: 10, background: 'var(--paper-2)', fontSize: 13, color: 'var(--text-2)', alignSelf: 'center' }}><Icon name="lock" size={12} stroke={2.4} />rihagateway.com</div>
        {!link && !err && <div style={{ display: 'grid', placeItems: 'center', flex: 1 }}><Spinner dark /></div>}
        {err && !link?.custom_amount && <div style={{ marginTop: 40, textAlign: 'center' }}><div className="display" style={{ fontSize: 26 }}>{offline ? 'Sem ligação' : 'Link indisponível'}</div><p className="muted">{err}</p></div>}
        {link && !link.active && <div style={{ marginTop: 40, textAlign: 'center' }}><div className="display" style={{ fontSize: 26 }}>Este link já não está ativo</div><p className="muted">Peça um novo link a {link.merchant.name}.</p></div>}
        {link && link.active && !link.custom_amount && !err && <div style={{ display: 'grid', placeItems: 'center', flex: 1, color: 'var(--muted)', gap: 12 }}><Spinner dark />A abrir o pagamento…</div>}
        {link && link.active && link.custom_amount && (
          <>
            <div style={{ marginTop: 18 }}><LinkHeader link={link} /></div>
            <div style={{ textAlign: 'center', marginTop: 30 }}>
              <div style={{ fontSize: 13.5, color: 'var(--muted)' }}>{submitWords(link.submit_type).ask}</div>
              <div style={{ display: 'flex', alignItems: 'baseline', justifyContent: 'center', gap: 8, marginTop: 6 }}>
                <input autoFocus inputMode="decimal" value={amount} onChange={e => setAmount(e.target.value.replace(/[^\d.,]/g, '').slice(0, 10))} placeholder="0" style={{ width: `${Math.max(1, amount.length) * 0.62 + 0.2}em`, minWidth: 60, maxWidth: 280, border: 0, background: 'transparent', textAlign: 'right', fontFamily: 'var(--display)', fontWeight: 700, fontSize: 58, letterSpacing: '-0.045em', padding: 0, caretColor: 'var(--green)' }} />
                <span className="display" style={{ fontSize: 26, color: 'var(--muted-2)' }}>MT</span>
              </div>
              <div style={{ display: 'flex', justifyContent: 'center', gap: 8, marginTop: 16 }}>
                {quickAmounts(link.suggested_amount, link.min_amount, link.max_amount).map(v => <button key={v} className={'btn ' + (c === v ? 'btn-ink' : 'btn-outline')} style={{ height: 36, borderRadius: 999, fontSize: 13.5 }} onClick={() => setAmount(String(v / 100))}>{fmt(v, false)}</button>)}
              </div>
              {rangeText(link.min_amount, link.max_amount) && <div style={{ fontSize: 13, color: outside ? 'var(--red-dark)' : 'var(--muted)', marginTop: 12 }}>{rangeText(link.min_amount, link.max_amount)}</div>}
            </div>
            {err && <div className="form-err" style={{ marginTop: 16 }}>{err}</div>}
            <div style={{ flex: 1, minHeight: 30 }} />
            <button className="btn btn-green btn-lg btn-block" disabled={busy || c < 1000 || outside} onClick={() => open(c)}>{busy ? <Spinner /> : `Continuar · ${fmt(c)} MT`}</button>
          </>
        )}
      </div>
    </div>
  )
}
