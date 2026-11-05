import { useEffect, useRef, useState } from 'react'
import { useParams } from 'react-router-dom'
import { Icon, MethodIcon, Spinner } from '../../../components/ui'
import { useGateway } from '../../../core/gateway'
import { errMsg } from '../../../core/model'
import { METHOD_NAME } from '../../../lib/format'
import { walletPrompt } from '../../../lib/walletPrompt'
import { money, MonthStrip } from '../../app/subs/common'
import { Center, MerchantHead, PayFrame } from './PayFrame'

const WALLET_TIMEOUT = 180
const cents = (c: number) => money(c) + ',' + String(Math.abs(c) % 100).padStart(2, '0')

// The invoice's payment names its method for display, as in M-Pesa.
const walletKey = (name: string) => Object.keys(METHOD_NAME).find(k => k === name || METHOD_NAME[k] === name) ?? name

export default function Invoice() {
  const { token } = useParams()
  const g = useGateway()
  const [d, setD] = useState<any>(null)
  const [loadErr, setLoadErr] = useState('')
  const [offline, setOffline] = useState(false)
  // 'checking': the answer to "pay" was lost, so we don't yet know if a prompt went out.
  const [view, setView] = useState<'pay' | 'checking' | 'confirm' | 'done' | 'manage'>('pay')
  const [change, setChange] = useState(false)
  const [method, setMethod] = useState('mpesa')
  const [phone, setPhone] = useState('')
  const [card, setCard] = useState({ number: '', exp: '', cvc: '' })
  const [accept, setAccept] = useState(false)
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')
  const [receipt, setReceipt] = useState(false)
  const [tick, setTick] = useState(0)
  const start = useRef(0)
  const paidOnly = useRef('')
  const lostPay = useRef<{ before: string | null } | null>(null)

  // The USSD prompt cuts the phone's data, so a failed check while waiting means nothing.
  const load = async (quiet = false) => {
    for (let attempt = 0; ; attempt++) {
      try { const x = await g.publicSubs.invoice(token!); setD(x); return x } catch (e: any) {
        if (quiet) return null
        if (e?.code === 'network_error' && attempt < 2) { await new Promise(r => setTimeout(r, 1500)); continue }
        setOffline(e?.code === 'network_error'); setLoadErr(errMsg(e)); return null
      }
    }
  }
  useEffect(() => {
    load().then(x => {
      if (!x) return
      if ((x.coming_soon || []).includes(x.last_method)) { setMethod('mpesa'); setChange(true) } else setMethod(x.last_method || 'mpesa')
      if (x.payment?.status === 'processing') { start.current = Date.now(); setView('confirm') }
      else if (x.lines.length === 0 && x.paid.length) setView('done')
    })
  }, [token]) // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => {
    if (view !== 'confirm' && view !== 'checking') return
    let running = false
    const check = async () => {
      if (running) return
      running = true
      setTick(v => v + 1)
      const x = await load(true)
      running = false
      if (!x) return
      const lost = lostPay.current
      if (lost) {
        lostPay.current = null
        if ((x.payment?.id ?? null) === lost.before) {
          setView('pay'); setErr('O pedido não chegou ao M-Pesa. Nada foi cobrado. Tente de novo.')
          return
        }
        start.current = Date.now(); setView('confirm')
      }
      if (!x.payment) return
      if (x.payment.status === 'succeeded') setView(x.lines.length === 0 ? 'done' : 'pay')
      else if (x.payment.status === 'failed') { setView('pay'); setErr(x.payment.failure_message || 'O pagamento falhou.') }
    }
    const t = setInterval(check, 1500)
    document.addEventListener('visibilitychange', check)
    window.addEventListener('online', check)
    return () => { clearInterval(t); document.removeEventListener('visibilitychange', check); window.removeEventListener('online', check) }
  }, [view]) // eslint-disable-line react-hooks/exhaustive-deps

  if (loadErr) return <Center><div className="display" style={{ fontSize: 24 }}>{offline ? 'Sem ligação' : 'Link indisponível'}</div><p className="muted">{loadErr}</p>{offline && <button className="btn btn-ink" style={{ marginTop: 12 }} onClick={() => window.location.reload()}>Tentar de novo</button>}</Center>
  if (!d) return <Center><Spinner dark /></Center>

  const lines: any[] = d.lines
  const cur = lines.find(l => l.current) || lines[0]
  const debts = lines.filter(l => l !== cur)
  const oldest = lines.slice().filter(l => l.past_due).pop()
  const sub = [d.member.name, ...Object.entries(d.member.fields || {}).filter(([, v]) => v !== d.member.name).map(([, v]) => v)].join(' · ')
  const usingLast = !change && !!d.phone_masked && method !== 'card'

  const pay = async (ids?: string[]) => {
    setBusy(true); setErr('')
    try {
      const body: any = { method, accept, charge_ids: ids || [] }
      if (method === 'card') Object.assign(body, { card_number: card.number, card_expiry: card.exp, card_cvc: card.cvc })
      else if (usingLast) body.use_last = true
      else body.phone = phone.replace(/\D/g, '')
      paidOnly.current = ids?.length === 1 ? lines.find(l => l.id === ids[0])?.label || '' : ''
      const x = await g.publicSubs.pay(token!, body)
      setD(x)
      if (x.payment?.status === 'processing') { start.current = Date.now(); setTick(0); setView('confirm') }
      else if (x.payment?.status === 'succeeded') setView(x.lines.length === 0 ? 'done' : 'pay')
      else if (x.payment?.status === 'failed') setErr(x.payment.failure_message || 'O pagamento falhou.')
    } catch (e: any) {
      if (e?.code === 'network_error') {
        // The prompt may have gone out and cut the connection before the answer came back.
        lostPay.current = { before: d.payment?.id ?? null }
        setView('checking')
      } else setErr(errMsg(e))
    } finally { setBusy(false) }
  }

  if (view === 'checking') return <PayFrame test={d.test_mode}><Center><Spinner dark /><p className="muted" style={{ marginTop: 14 }}>Se recebeu um pedido no telemóvel, responda ao pedido. Estamos a confirmar.</p></Center></PayFrame>

  if (view === 'confirm' && d.payment) {
    const elapsed = start.current ? Math.floor((Date.now() - start.current) / 1000) : 0
    const left = Math.max(0, WALLET_TIMEOUT - elapsed)
    void tick
    return (
      <PayFrame test={d.test_mode}>
        <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', textAlign: 'center', paddingTop: 40 }}>
          <div style={{ width: 210, padding: 16, borderRadius: 16, background: '#2B2F2D', color: '#fff', textAlign: 'left', boxShadow: '0 24px 40px -18px rgba(0,0,0,.6)' }}>
            <div style={{ fontSize: 12, lineHeight: 1.5, color: '#E6E9E7' }}>{walletPrompt(walletKey(d.payment.method)).screen(cents(d.payment.amount))}</div>
          </div>
          <h1 className="display" style={{ margin: '26px 0 0', fontSize: 26 }}>{walletPrompt(walletKey(d.payment.method)).title}</h1>
          <p style={{ margin: '8px 0 0', fontSize: 15, color: 'var(--muted)', lineHeight: 1.5 }}>{walletPrompt(walletKey(d.payment.method)).steps(<b>o seu número</b>)}</p>
          <div className="card" style={{ width: '100%', marginTop: 22, padding: 16, textAlign: 'left' }}>
            <div className="between" style={{ fontSize: 13.5 }}><span style={{ display: 'flex', alignItems: 'center', gap: 8, fontWeight: 600 }}><span style={{ width: 8, height: 8, borderRadius: '50%', background: 'var(--sun)' }} />A aguardar confirmação</span><span className="mono muted">{Math.floor(left / 60)}:{String(left % 60).padStart(2, '0')}</span></div>
            <div style={{ height: 6, borderRadius: 3, background: '#EEECE5', marginTop: 12, overflow: 'hidden' }}><div style={{ width: `${Math.min(100, (elapsed / WALLET_TIMEOUT) * 100)}%`, height: '100%', background: 'var(--green)', transition: 'width 1s linear' }} /></div>
          </div>
          <button className="btn btn-ghost" style={{ marginTop: 16 }} onClick={() => setView('pay')}>Mudar método de pagamento</button>
        </div>
      </PayFrame>
    )
  }

  if (view === 'manage') {
    return (
      <PayFrame test={d.test_mode}>
        <button className="icon-btn" onClick={() => setView(lines.length ? 'pay' : 'done')} aria-label="Voltar" style={{ marginTop: 12 }}><Icon name="arrowLeft" size={18} /></button>
        <MerchantHead m={d.merchant} sub={sub} />
        <h1 className="display" style={{ fontSize: 24, margin: '20px 0 10px' }}>A sua assinatura</h1>
        {d.subscriptions.map((s: any) => (
          <div key={s.name} className="card" style={{ padding: 16, marginBottom: 8 }}>
            <div className="between"><b>{s.name}</b><span className="tnum">{money(s.amount)} MT</span></div>
            <div style={{ fontSize: 13, color: 'var(--muted)', marginTop: 4 }}>{s.schedule}{s.next_due && ` · próximo ${s.next_due}`}</div>
            <div style={{ marginTop: 10 }}><MonthStrip mask={s.months} size={20} /></div>
          </div>
        ))}
        <div style={{ fontSize: 13, color: 'var(--muted)', lineHeight: 1.5, marginTop: 8 }}>Recebe cada link por {d.channel === 'whatsapp' ? 'WhatsApp' : 'SMS'} no {d.phone_masked}. Para pausar, mudar ou cancelar, fale com {d.merchant.name}.</div>
      </PayFrame>
    )
  }

  if (view === 'done' || lines.length === 0) {
    const first = d.paid[0]
    return (
      <PayFrame test={d.test_mode}>
        <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', textAlign: 'center', paddingTop: 36 }}>
          <div style={{ width: 88, height: 88, borderRadius: '50%', background: 'var(--green)', display: 'grid', placeItems: 'center', boxShadow: '0 0 0 12px #E2F3DE,0 0 0 26px #EEF6EB' }}><Icon name="check" size={40} stroke={2.6} color="#fff" /></div>
          <h1 className="display" style={{ margin: '36px 0 0', fontSize: 28 }}>{first ? `${first.label.split(' ')[0]} pago` : 'Tudo pago'}</h1>
          {first && <p style={{ margin: '6px 0 0', fontSize: 15, color: 'var(--muted)' }}>{cents(d.paid.reduce((a: number, p: any) => a + p.amount, 0))} MT · {d.merchant.name}</p>}
        </div>
        {d.next_due && (
          <div className="card" style={{ marginTop: 24, padding: 18 }}>
            <div style={{ fontSize: 13, color: 'var(--muted)' }}>Próximo pagamento</div>
            <div className="display" style={{ fontSize: 22, marginTop: 4 }}>{d.next_due}</div>
            {d.skipped.length > 0 && <div style={{ fontSize: 13.5, marginTop: 8 }}>{d.skipped.join(' e ')} não {d.skipped.length === 1 ? 'é cobrado' : 'são cobrados'}.</div>}
            <div style={{ fontSize: 13.5, color: 'var(--muted)', marginTop: 6 }}>Recebe o link por {d.channel === 'whatsapp' ? 'WhatsApp' : 'SMS'} no {d.phone_masked}.</div>
          </div>
        )}
        {receipt && first && (
          <div className="card" style={{ marginTop: 10, padding: '4px 18px' }}>
            {d.paid.map((p: any, i: number) => <div key={i} className="between" style={{ padding: '11px 0', borderBottom: '1px solid var(--line-3)', fontSize: 14 }}><span>{p.plan_name} · {p.label}</span><b className="tnum">{cents(p.amount)}</b></div>)}
            {d.payment && <div className="between" style={{ padding: '11px 0', borderBottom: '1px solid var(--line-3)', fontSize: 14 }}><span className="muted">Referência</span><span className="mono">{d.payment.reference}</span></div>}
            <div className="between" style={{ padding: '11px 0', fontSize: 14 }}><span className="muted">Método</span><span>{first.outside ? 'Pago fora da RIHA' : `${first.method} · ${first.paid_at}`}</span></div>
          </div>
        )}
        <div style={{ flex: 1, minHeight: 24 }} />
        <div style={{ display: 'flex', gap: 8, marginTop: 20 }}>
          <button className="btn btn-outline btn-lg" style={{ flex: 1 }} onClick={() => setReceipt(!receipt)}>Recibo</button>
          <button className="btn btn-ink btn-lg" style={{ flex: 1 }} onClick={() => setView('manage')}>Gerir assinatura</button>
        </div>
      </PayFrame>
    )
  }

  const total = lines.reduce((a, l) => a + l.amount, 0)
  return (
    <PayFrame test={d.test_mode}>
      <MerchantHead m={d.merchant} sub={sub}>
        <div style={{ borderTop: '1px dashed #E1DFD7', marginTop: 14, paddingTop: 12 }}>
          {[cur, ...debts].map(l => (
            <div key={l.id} className="between" style={{ padding: '6px 0', fontSize: 14 }}>
              <span>{l.past_due ? <><b style={{ color: 'var(--sun-text)' }}>{l.label.split(' ')[0]}</b> em dívida</> : <><b>{l.plan_name.split(' · ')[0]} de {l.label.split(' ')[0].toLowerCase()}</b><span style={{ display: 'block', fontSize: 12.5, color: 'var(--muted)' }}>Vence {l.due}{lines.length > 1 && l.plan_name.includes('·') ? ` · ${l.plan_name.split(' · ').slice(1).join(' · ')}` : ''}</span></>}</span>
              <span className="tnum" style={{ fontWeight: 600 }}>{cents(l.amount)}</span>
            </div>
          ))}
          {lines.length > 1 && <div className="between" style={{ borderTop: '1px solid var(--line-3)', marginTop: 6, paddingTop: 10 }}><b>Total</b><b className="display tnum" style={{ fontSize: 22 }}>{cents(total)} MT</b></div>}
          {lines.length === 1 && <div className="display tnum" style={{ fontSize: 30, textAlign: 'right', marginTop: 4 }}>{cents(total)} <span style={{ fontSize: 14, color: 'var(--muted)', fontFamily: 'var(--sans)' }}>MT</span></div>}
        </div>
      </MerchantHead>
      {paidOnly.current && <div style={{ marginTop: 12, padding: '10px 14px', borderRadius: 12, background: 'var(--green-bg)', color: 'var(--green-dark)', fontSize: 13.5, fontWeight: 600 }}>{paidOnly.current} pago. Falta o resto.</div>}
      <div className="between" style={{ marginTop: 22 }}><b style={{ fontSize: 16 }}>Pagar com</b>{d.phone_masked && <button className="link-btn" onClick={() => setChange(!change)}>{change ? (d.has_last ? 'Usar o último' : 'Usar o meu número') : 'Mudar'}</button>}</div>
      {usingLast ? (
        <div className="card" style={{ marginTop: 10, padding: '14px 16px', display: 'flex', gap: 12, alignItems: 'center', border: '2px solid var(--ink)' }}>
          <MethodIcon method={method} />
          <span style={{ flex: 1 }}><b style={{ display: 'block' }}>{METHOD_NAME[method]}</b><span style={{ fontSize: 12.5, color: 'var(--muted)' }}>{d.phone_masked} · {d.has_last ? 'usado da última vez' : 'o número que indicou'}</span></span>
          <span style={{ width: 20, height: 20, borderRadius: '50%', border: '6px solid var(--ink)' }} />
        </div>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8, marginTop: 10 }}>
          {['mpesa', 'emola', 'bim', 'card'].filter(m => !(d.coming_soon || []).includes(m)).map(m => (
            <button key={m} onClick={() => setMethod(m)} className="card" style={{ padding: '12px 14px', display: 'flex', gap: 12, alignItems: 'center', textAlign: 'left', border: method === m ? '2px solid var(--ink)' : '1px solid var(--line)', margin: method === m ? 0 : 1 }}>
              <MethodIcon method={m} /><span style={{ flex: 1, fontWeight: 600 }}>{METHOD_NAME[m]}</span><span style={{ width: 20, height: 20, borderRadius: '50%', border: method === m ? '6px solid var(--ink)' : '1.5px solid #C9C7BE' }} />
            </button>
          ))}
          {(d.coming_soon || []).map((m: string) => (
            <div key={m} className="card" aria-disabled="true" style={{ padding: '12px 14px', display: 'flex', gap: 12, alignItems: 'center', opacity: 0.55 }}>
              <MethodIcon method={m} /><span style={{ flex: 1, fontWeight: 600 }}>{METHOD_NAME[m]}</span><span style={{ fontSize: 12.5, color: 'var(--muted)' }}>Em breve</span>
            </div>
          ))}
          {method !== 'card' && <div className="phone-input"><span className="cc">+258</span><input inputMode="tel" value={phone} onChange={e => setPhone(e.target.value)} placeholder={method === 'emola' ? '86 123 4567' : method === 'bim' ? '82 123 4567' : '84 123 4567'} aria-label="Número" /></div>}
          {method === 'card' && <div className="grid-2" style={{ gap: 8 }}>
            <input className="input" style={{ gridColumn: '1/-1' }} inputMode="numeric" placeholder="Número do cartão" value={card.number} onChange={e => setCard({ ...card, number: e.target.value })} />
            <input className="input" placeholder="MM / AA" value={card.exp} onChange={e => setCard({ ...card, exp: e.target.value })} />
            <input className="input" placeholder="CVC" inputMode="numeric" value={card.cvc} onChange={e => setCard({ ...card, cvc: e.target.value })} />
          </div>}
        </div>
      )}
      {method !== 'card' && <div style={{ fontSize: 12.5, color: 'var(--muted)', marginTop: 8 }}>{walletPrompt(method).before}</div>}
      {d.needs_consent && (
        <label style={{ display: 'flex', gap: 12, alignItems: 'flex-start', padding: 14, borderRadius: 16, background: '#fff', border: '1px solid var(--line)', marginTop: 14, cursor: 'pointer' }}>
          <input type="checkbox" checked={accept} onChange={e => setAccept(e.target.checked)} style={{ width: 20, height: 20, marginTop: 2, accentColor: 'var(--green)', flex: 'none' }} />
          <span style={{ fontSize: 13.5, lineHeight: 1.5, color: 'var(--text-2)' }}>{d.agreements.map((t: string, i: number) => <span key={i} style={{ display: 'block', marginTop: i ? 6 : 0 }}>{t}</span>)}</span>
        </label>
      )}
      {err && <div className="form-err" style={{ marginTop: 14 }} role="alert">{err}</div>}
      <div style={{ flex: 1, minHeight: 20 }} />
      <button className="btn btn-green btn-lg btn-block" style={{ marginTop: 18 }} disabled={busy || (d.needs_consent && !accept)} onClick={() => pay()}>{busy ? <Spinner /> : <><Icon name="lock" size={16} stroke={2.2} />Pagar {cents(total)} MT</>}</button>
      {oldest && lines.length > 1 && <button className="btn btn-ghost" style={{ marginTop: 6 }} disabled={busy} onClick={() => pay([oldest.id])}>Pagar só {oldest.label.split(' ')[0].toLowerCase()} · {cents(oldest.amount)} MT</button>}
      <button className="btn btn-ghost" style={{ marginTop: 2, fontSize: 13 }} onClick={() => setView('manage')}>Ver assinatura</button>
    </PayFrame>
  )
}
