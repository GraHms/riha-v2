import { useEffect, useRef, useState } from 'react'
import { SUPPORT_PHONE, SUPPORT_TEL } from '../../lib/support'
import { useParams } from 'react-router-dom'
import { errMsg, type ApiError } from '../../core/model'
import { useGateway } from '../../core/gateway'
import { fmt, fullDate, METHOD_NAME } from '../../lib/format'
import { Avatar, Icon, MethodIcon, Spinner, Stripes } from '../../components/ui'

type Session = {
  id: string; status: string; amount: number; description: string; livemode: boolean
  line_items: { name: string; quantity: number; unit_amount: number }[]; payment_method_types: string[]
  success_url: string; cancel_url: string; return_host: string; client_reference_id: string; customer_phone: string; customer_name: string; email_on_file: boolean; coming_soon?: string[]
  expires_at: string; merchant: { name: string; initials: string; avatar_color: string; descriptor: string }
  payment: null | { id: string; reference: string; status: string; method: string; phone_prefix: string; phone_last4: string; card_brand: string; card_last4: string; failure_code: string; failure_message: string; created_at: string; succeeded_at: string | null }
}

const WALLET_TIMEOUT = 120

function fmtPhone(v: string) {
  const d = v.replace(/\D/g, '').slice(0, 9)
  return [d.slice(0, 2), d.slice(2, 5), d.slice(5)].filter(Boolean).join(' ')
}
function fmtCard(v: string) { return v.replace(/\D/g, '').slice(0, 19).replace(/(.{4})/g, '$1 ').trim() }
function fmtExp(v: string) { const d = v.replace(/\D/g, '').slice(0, 4); return d.length > 2 ? d.slice(0, 2) + ' / ' + d.slice(2) : d }

export default function Checkout() {
  const g = useGateway()
  const { id } = useParams()
  const [s, setS] = useState<Session | null>(null)
  const [loadErr, setLoadErr] = useState<ApiError | null>(null)
  const [method, setMethod] = useState('mpesa')
  const [phone, setPhone] = useState('')
  const [card, setCard] = useState({ number: '', exp: '', cvc: '', name: '' })
  const [email, setEmail] = useState('')
  const [err, setErr] = useState<ApiError | null>(null)
  // The USSD prompt cuts the phone's data, so losing the connection here is normal.
  const [reconnecting, setReconnecting] = useState(false)
  const [busy, setBusy] = useState(false)
  const [view, setView] = useState<'pay' | 'confirm' | 'done'>('pay')
  const [tick, setTick] = useState(0)
  const [redirectIn, setRedirectIn] = useState(8)
  const startRef = useRef(0)
  // Set when the answer to "pay" was lost: the payment before it, to tell them apart.
  const lostPay = useRef<{ before: string | null } | null>(null)

  const refresh = async () => {
    try {
      const d = await g.checkout.session(id!)
      setS(d)
      setReconnecting(false)
      return d
    } catch { setReconnecting(true); return null }
  }

  // A failed check says nothing about the payment, so only the first load can fail
  // the page: phones drop requests while the customer is away entering their PIN.
  const open = async () => {
    setLoadErr(null)
    for (let attempt = 0; ; attempt++) {
      try {
        const d = await g.checkout.session(id!)
        setS(d)
        return d
      } catch (e) {
        const err = e as ApiError
        if (err.code !== 'network_error' || attempt >= 2) { setLoadErr(err); return null }
        await new Promise(r => setTimeout(r, 1500))
      }
    }
  }

  const start = () => open().then(d => {
    if (!d) return
    if (d.payment_method_types.length && !d.payment_method_types.includes('mpesa')) setMethod(d.payment_method_types[0])
    if (d.customer_phone) setPhone(fmtPhone(d.customer_phone))
    if (d.status === 'complete') setView('done')
    else if (d.payment?.status === 'processing') { setView('confirm'); startRef.current = Date.now() }
  })

  useEffect(() => { start() }, [id]) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (view !== 'confirm') return
    let checking = false
    const check = async () => {
      if (checking || document.hidden) return
      checking = true
      const d = await refresh()
      checking = false
      if (!d) return
      const lost = lostPay.current
      if (lost && (d.payment?.id ?? null) === lost.before) {
        // Connected again and no new payment, so the request never arrived.
        lostPay.current = null
        setView('pay')
        setErr({ code: 'network_error', message: 'O pedido não chegou ao M-Pesa. Nada foi cobrado. Tente de novo.', status: 0 })
        return
      }
      lostPay.current = null
      if (!d.payment) return
      if (d.status === 'complete' || d.payment.status === 'succeeded') setView('done')
      else if (d.payment.status === 'failed') { setView('pay'); setErr({ code: d.payment.failure_code, message: d.payment.failure_message || 'O pagamento falhou.', status: 402 }) }
    }
    const t = setInterval(() => { setTick(x => x + 1); check() }, 1500)
    // Back from entering the PIN: answer at once instead of at the next tick.
    document.addEventListener('visibilitychange', check)
    window.addEventListener('online', check)
    return () => { clearInterval(t); document.removeEventListener('visibilitychange', check); window.removeEventListener('online', check) }
  }, [view]) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (view !== 'done' || !s?.success_url) return
    if (redirectIn <= 0) { window.location.href = s.success_url; return }
    const t = setTimeout(() => setRedirectIn(x => x - 1), 1000)
    return () => clearTimeout(t)
  }, [view, redirectIn, s?.success_url])

  const pay = async () => {
    if (!s) return
    setBusy(true); setErr(null)
    try {
      const body = method === 'card'
        ? { method, card_number: card.number, card_expiry: card.exp, card_cvc: card.cvc, name: card.name, email }
        : { method, phone: phone.replace(/\D/g, ''), email }
      const d = await g.checkout.pay(s.id, body)
      setS(d)
      if (d.status === 'complete') setView('done')
      else if (d.payment?.status === 'processing') { startRef.current = Date.now(); setTick(0); setView('confirm') }
      else if (d.payment?.status === 'failed') setErr({ code: d.payment.failure_code, message: d.payment.failure_message, status: 402 })
    } catch (e) {
      const failure = e as ApiError
      if (failure.code === 'network_error') {
        // The answer was lost, and the prompt on the phone may be why: wait for the
        // connection and let the server say whether the payment started.
        lostPay.current = { before: s.payment?.id ?? null }
        startRef.current = Date.now(); setTick(0); setReconnecting(true); setView('confirm')
      } else setErr(failure)
    } finally { setBusy(false) }
  }

  const cancel = async () => {
    if (!s) return
    if (s.cancel_url) window.location.href = s.cancel_url
    else window.history.back()
  }

  if (loadErr?.code === 'network_error') return <Center><div className="display" style={{ fontSize: 26 }}>Sem ligação</div><p className="muted">Não foi possível abrir o pagamento. Verifique a internet e tente de novo.</p><button className="btn btn-green" onClick={start}>Tentar de novo</button></Center>
  if (loadErr) return <Center><div className="display" style={{ fontSize: 26 }}>Pagamento não encontrado</div><p className="muted">{errMsg(loadErr)}</p></Center>
  if (!s) return <Center><Spinner dark /></Center>

  const amount = fmt(s.amount)
  const wallet = method !== 'card'
  const walletName = METHOD_NAME[method]
  const walletPrefix = method === 'emola' ? '86' : '84'
  const ref = s.client_reference_id ? (/^\d+$/.test(s.client_reference_id) ? `Encomenda #${s.client_reference_id}` : s.client_reference_id) : s.description
  const expired = s.status === 'expired'
  const elapsed = startRef.current ? Math.floor((Date.now() - startRef.current) / 1000) : 0
  const left = Math.max(0, WALLET_TIMEOUT - elapsed)
  const waitingPhone = s.payment?.status === 'processing' ? `${s.payment.phone_prefix} ••• ${s.payment.phone_last4}` : phone
  void tick

  const items = s.line_items.length ? s.line_items : [{ name: s.description || 'Pagamento', quantity: 1, unit_amount: s.amount }]

  const summary = (
    <>
      {s.return_host && <a href={s.cancel_url || '#'} onClick={e => { e.preventDefault(); cancel() }} className="co-back"><Icon name="arrowLeft" size={16} stroke={2.2} />Voltar a {s.return_host}</a>}
      <div className="co-merchant">
        <Avatar text={s.merchant.initials} color={s.merchant.avatar_color} size={44} radius={13} font={15} />
        <div><div style={{ fontWeight: 600, fontSize: 16 }}>{s.merchant.name}</div><div className="co-sub">{ref}</div></div>
      </div>
      <div className="co-total-label">Total a pagar</div>
      <div className="co-total">{amount} <span>MT</span></div>
      <div className="co-items">
        {items.map((li, i) => <div key={i} className="between"><span>{li.name}{li.quantity > 1 ? ` × ${li.quantity}` : ''}</span><span className="tnum">{fmt(li.unit_amount * li.quantity)} MT</span></div>)}
        <div className="co-line" />
        <div className="between" style={{ fontWeight: 600 }}><span>Total</span><span className="tnum">{amount} MT</span></div>
      </div>
    </>
  )

  return (
    <div className="co">
      <style>{CSS}</style>
      {!s.livemode && <div className="testbar" style={{ position: 'fixed', top: 0, left: 0, right: 0, zIndex: 5 }}>Modo de teste · use 84 xxx 0000 para simular PIN errado, ou o cartão 4242 4242 4242 4242</div>}
      <aside className="co-side">
        <div className="stripe g" style={{ bottom: -80, left: 80, width: 70, height: 520 }} />
        <div className="stripe r" style={{ bottom: -80, left: 170, width: 70, height: 520 }} />
        <div style={{ position: 'absolute', inset: 0, background: 'linear-gradient(0deg,rgba(11,18,14,.3),#0B120E 60%)' }} />
        <div className="co-side-in">
          {summary}
          <div style={{ flex: 1 }} />
          <Secure dark />
        </div>
      </aside>

      <main className="co-main">
        <div className="co-main-in">
          <div className="co-url only-mobile"><Icon name="lock" size={12} stroke={2.4} />rihagateway.com</div>

          {view === 'pay' && (
            <>
              <div className="only-mobile co-mcard">
                <div className="row">
                  <Avatar text={s.merchant.initials} color={s.merchant.avatar_color} size={44} radius={14} font={15} />
                  <div style={{ flex: 1 }}><div style={{ fontWeight: 600, fontSize: 16 }}>{s.merchant.name}</div><div style={{ fontSize: 12.5, color: 'var(--muted)', marginTop: 1 }}>{ref}</div></div>
                </div>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-end', marginTop: 16, paddingTop: 14, borderTop: '1px dashed #E1DFD7' }}>
                  <div><div style={{ fontSize: 13, color: 'var(--muted)' }}>{items[0].name}{items.length > 1 ? ` + ${items.length - 1}` : ''}</div><div style={{ fontSize: 13, color: 'var(--muted)', marginTop: 2 }}>Total a pagar</div></div>
                  <div className="display" style={{ fontSize: 30, letterSpacing: '-0.035em', lineHeight: 1 }}>{amount} <span style={{ fontSize: 15, color: 'var(--muted)' }}>MT</span></div>
                </div>
              </div>

              {expired ? (
                <div style={{ marginTop: 22 }}>
                  <div className="display" style={{ fontSize: 26 }}>Este pagamento expirou</div>
                  <p className="muted" style={{ lineHeight: 1.5 }}>Volte à loja para criar um novo pedido.</p>
                  {s.cancel_url && <button className="btn btn-ink btn-lg btn-block" onClick={cancel}>Voltar a {s.return_host}</button>}
                </div>
              ) : (
                <>
                  <div className="co-h">Como quer pagar?</div>
                  <div className="co-methods">
                    {s.payment_method_types.map(m => (
                      <button key={m} type="button" onClick={() => { setMethod(m); setErr(null) }} className={'co-m' + (method === m ? ' on' : '')} aria-pressed={method === m}>
                        <MethodIcon method={m} size={38} />
                        <span className="co-m-txt"><span className="co-m-name">{METHOD_NAME[m]}</span><span className="co-m-sub">{m === 'mpesa' ? 'Vodacom · 84, 85' : m === 'emola' ? 'Movitel · 86, 87' : 'Visa, Mastercard'}</span></span>
                        <span className="co-radio"><span /></span>
                      </button>
                    ))}
                    {(s.coming_soon || []).map(m => (
                      <div key={m} className="co-m" aria-disabled="true" style={{ opacity: 0.55, cursor: 'not-allowed', borderStyle: 'dashed' }}>
                        <MethodIcon method={m} size={38} />
                        <span className="co-m-txt"><span className="co-m-name">{METHOD_NAME[m]}</span><span className="co-m-sub">Em breve</span></span>
                      </div>
                    ))}
                  </div>
                  {wallet ? (
                    <div style={{ marginTop: 20 }}>
                      <span className="label" style={{ marginBottom: 7 }}>Número {walletName}</span>
                      <div className="phone-input" style={{ border: '2px solid var(--ink)', height: 56 }}><span className="cc">+258</span><input inputMode="tel" autoComplete="tel-national" value={phone} onChange={e => setPhone(fmtPhone(e.target.value))} placeholder={`${walletPrefix} 412 4231`} style={{ fontSize: 17 }} /></div>
                      <div style={{ fontSize: 13, color: 'var(--muted)', marginTop: 8, lineHeight: 1.45 }}>Vai receber um pedido no telemóvel para confirmar com o seu PIN.</div>
                    </div>
                  ) : (
                    <div style={{ marginTop: 20, display: 'flex', flexDirection: 'column', gap: 10 }}>
                      <span className="label" style={{ marginBottom: 0 }}>Dados do cartão</span>
                      <input className="input lg" style={{ height: 56, fontSize: 17, letterSpacing: '.04em', border: '2px solid var(--ink)' }} inputMode="numeric" autoComplete="cc-number" placeholder="4410 2398 0015 4410" value={card.number} onChange={e => setCard({ ...card, number: fmtCard(e.target.value) })} />
                      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 10 }}>
                        <input className="input lg" style={{ height: 56 }} inputMode="numeric" autoComplete="cc-exp" placeholder="MM / AA" value={card.exp} onChange={e => setCard({ ...card, exp: fmtExp(e.target.value) })} />
                        <input className="input lg" style={{ height: 56 }} inputMode="numeric" autoComplete="cc-csc" placeholder="CVV" maxLength={4} value={card.cvc} onChange={e => setCard({ ...card, cvc: e.target.value.replace(/\D/g, '') })} />
                      </div>
                      <input className="input lg" style={{ height: 56 }} autoComplete="cc-name" placeholder="Nome no cartão" value={card.name} onChange={e => setCard({ ...card, name: e.target.value })} />
                    </div>
                  )}
                  {!s.email_on_file && (
                    <label style={{ display: 'block', marginTop: 14 }}>
                      <span className="label" style={{ marginBottom: 7 }}>Email para o recibo (opcional)</span>
                      <input className="input lg" style={{ height: 52 }} type="email" inputMode="email" autoComplete="email" value={email} onChange={e => setEmail(e.target.value)} />
                    </label>
                  )}
                  <div className="co-pay">
                    {/* Inside the bar that stays on screen, or the Pay button covers it on a phone. */}
                    {err && <div className="form-err" style={{ marginBottom: 10 }} role="alert">{err.message}</div>}
                    <button className="btn btn-green btn-lg btn-block" style={{ height: 58, fontSize: 17, boxShadow: '0 14px 30px -14px rgba(10,130,5,.9)' }} onClick={pay} disabled={busy}>
                      {busy ? <Spinner /> : <><Icon name="lock" size={17} stroke={2.2} />Pagar {amount} MT</>}
                    </button>
                    {s.return_host && <div className="only-desktop" style={{ fontSize: 12.5, lineHeight: 1.5, color: 'var(--muted)', marginTop: 14 }}>Depois de pagar, volta automaticamente para {s.return_host}.</div>}
                    <div className="only-mobile" style={{ marginTop: 12 }}><Secure /></div>
                  </div>
                </>
              )}
            </>
          )}

          {view === 'confirm' && (
            <div className="co-center">
              <div style={{ position: 'relative', width: 250, height: 250, display: 'grid', placeItems: 'center' }}>
                <div style={{ position: 'absolute', inset: 0, borderRadius: '50%', background: 'radial-gradient(circle,rgba(10,130,5,.14),rgba(10,130,5,0) 70%)' }} />
                <div style={{ position: 'absolute', inset: 28, borderRadius: '50%', border: '1.5px dashed #C9D9C4', animation: 'spin 18s linear infinite' }} />
                <div style={{ position: 'relative', width: 200, padding: 16, borderRadius: 16, background: '#2B2F2D', color: '#fff', textAlign: 'left', boxShadow: '0 24px 40px -18px rgba(0,0,0,.6)' }}>
                  <div style={{ fontSize: 12, lineHeight: 1.5, color: '#E6E9E7' }}>{METHOD_NAME[s.payment?.method ?? method]}: Confirma o pagamento de <b>{amount} MT</b> a {s.merchant.descriptor || s.merchant.name}? Introduza o PIN:</div>
                  <div style={{ height: 30, marginTop: 10, borderBottom: '1.5px solid var(--lime)', fontSize: 16, letterSpacing: '.4em', display: 'flex', alignItems: 'flex-end', paddingBottom: 4 }}>••••</div>
                  <div className="between" style={{ marginTop: 12, fontSize: 12.5, fontWeight: 600, color: 'var(--lime)' }}><span>Cancelar</span><span>Enviar</span></div>
                </div>
              </div>
              <h1 className="display" style={{ margin: '14px 0 0', fontSize: 28 }}>Confirme no seu telemóvel</h1>
              <p style={{ margin: '8px 0 0', fontSize: 15, lineHeight: 1.5, color: 'var(--muted)', maxWidth: 320 }}>Enviámos um pedido para <b style={{ color: 'var(--ink)' }}>{waitingPhone}</b>. Introduza o PIN {METHOD_NAME[s.payment?.method ?? method]} para concluir.</p>
              {reconnecting && <div role="status" style={{ width: '100%', marginTop: 18, padding: '11px 14px', borderRadius: 12, background: 'var(--sun-soft)', color: '#5B4600', fontSize: 13.5, lineHeight: 1.45 }}>O M-Pesa pode cortar a internet enquanto o pedido está aberto. Não precisa de fazer nada: o pagamento continua e atualizamos assim que a ligação voltar.</div>}
              <div className="card" style={{ width: '100%', marginTop: 26, borderRadius: 18, padding: 16, textAlign: 'left' }}>
                <div className="between" style={{ fontSize: 13.5 }}><span style={{ display: 'flex', alignItems: 'center', gap: 8, fontWeight: 600 }}><span style={{ width: 8, height: 8, borderRadius: '50%', background: 'var(--sun)', boxShadow: '0 0 0 4px var(--sun-bg)' }} />A aguardar confirmação</span><span className="mono muted">{Math.floor(left / 60)}:{String(left % 60).padStart(2, '0')}</span></div>
                <div style={{ height: 6, borderRadius: 3, background: '#EEECE5', marginTop: 12, overflow: 'hidden' }}><div style={{ width: `${Math.min(100, (elapsed / WALLET_TIMEOUT) * 100)}%`, height: '100%', borderRadius: 3, background: 'var(--green)', transition: 'width 1s linear' }} /></div>
              </div>
              <div style={{ width: '100%', display: 'flex', flexDirection: 'column', gap: 8, marginTop: 24 }}>
                <button className="btn btn-outline btn-block" style={{ height: 52, borderRadius: 15, fontSize: 15 }} onClick={() => refresh()}>Não recebeu? Verificar estado</button>
                <button className="btn btn-ghost" onClick={() => setView('pay')}>Mudar método de pagamento</button>
              </div>
            </div>
          )}

          {view === 'done' && s.payment && (
            <div className="co-center" style={{ paddingTop: 34 }}>
              <div style={{ width: 96, height: 96, borderRadius: '50%', background: 'var(--green)', display: 'grid', placeItems: 'center', boxShadow: '0 0 0 12px #E2F3DE,0 0 0 26px #EEF6EB' }}><Icon name="check" size={44} stroke={2.6} color="#fff" /></div>
              <h1 className="display" style={{ margin: '38px 0 0', fontSize: 30 }}>Pagamento concluído</h1>
              <p style={{ margin: '6px 0 0', fontSize: 15.5, color: 'var(--muted)' }}>Pagou <b style={{ color: 'var(--ink)' }}>{amount} MT</b> a {s.merchant.name}</p>
              <div className="card" style={{ width: '100%', marginTop: 24, borderRadius: 22, padding: '6px 18px', textAlign: 'left' }}>
                <div className="between" style={{ padding: '12px 0', borderBottom: '1px solid #EEECE5', fontSize: 14 }}><span className="muted">Referência</span><span className="mono" style={{ fontSize: 13 }}>{s.payment.reference}</span></div>
                <div className="between" style={{ padding: '12px 0', borderBottom: '1px solid #EEECE5', fontSize: 14 }}><span className="muted">Método</span><span style={{ fontWeight: 500 }}>{s.payment.method === 'card' ? `${s.payment.card_brand} · •• ${s.payment.card_last4}` : `${METHOD_NAME[s.payment.method]} · ${s.payment.phone_prefix} ••• ${s.payment.phone_last4}`}</span></div>
                <div className="between" style={{ padding: '12px 0', fontSize: 14 }}><span className="muted">Data</span><span style={{ fontWeight: 500 }}>{fullDate(s.payment.succeeded_at || s.payment.created_at)}</span></div>
              </div>
              {s.success_url && (
                <div style={{ width: '100%', marginTop: 12, padding: '14px 16px', borderRadius: 18, background: 'var(--ink)', color: '#fff', textAlign: 'left' }}>
                  <div className="between" style={{ fontSize: 13.5 }}><span>A voltar para <b>{s.return_host}</b></span><span className="mono" style={{ color: '#A9B5AE' }}>{redirectIn} s</span></div>
                  <div style={{ height: 4, borderRadius: 2, background: 'rgba(255,255,255,.14)', marginTop: 10, overflow: 'hidden' }}><div style={{ width: `${((8 - redirectIn) / 8) * 100}%`, height: '100%', background: 'var(--lime)', borderRadius: 2, transition: 'width 1s linear' }} /></div>
                </div>
              )}
              <div style={{ width: '100%', display: 'flex', flexDirection: 'column', gap: 8, marginTop: 24 }}>
                {s.success_url && <button className="btn btn-ink btn-lg btn-block" onClick={() => (window.location.href = s.success_url)}>Voltar para {s.merchant.name}</button>}
                <button className="btn btn-ghost" onClick={() => window.print()}>Descarregar recibo</button>
              </div>
            </div>
          )}
        </div>
      </main>
    </div>
  )
}

function Secure({ dark }: { dark?: boolean }) {
  return (
    <>
    <div style={{ display: 'flex', alignItems: 'center', justifyContent: dark ? 'flex-start' : 'center', gap: 8, fontSize: 12.5, color: dark ? '#93A09A' : 'var(--muted)' }}>
      Pagamento seguro por<img src={dark ? '/assets/riha-icon-light.png' : '/assets/riha-icon-dark.png'} alt="" style={{ width: 18, height: 18, objectFit: 'contain' }} />
      <span style={{ fontFamily: 'var(--display)', fontWeight: 800, letterSpacing: '.18em', color: dark ? '#fff' : 'var(--ink)' }}>RIHA</span>
    </div>
    <div style={{ marginTop: 6, textAlign: dark ? 'left' : 'center', fontSize: 12.5, color: dark ? '#93A09A' : 'var(--muted)' }}>Precisa de ajuda? <a href={SUPPORT_TEL} style={{ color: 'inherit', fontWeight: 600 }}>{SUPPORT_PHONE}</a></div>
    </>
  )
}

function Center({ children }: { children: React.ReactNode }) {
  return <div style={{ minHeight: '100%', display: 'grid', placeItems: 'center', padding: 24, textAlign: 'center' }}><div>{children}</div></div>
}

void Stripes

const CSS = `
.co{min-height:100%;display:grid;grid-template-columns:1fr 1fr}
.co-side{background:var(--ink);color:#fff;position:relative;overflow:hidden;display:flex;justify-content:flex-end}
.co-side-in{position:relative;width:420px;max-width:100%;padding:56px 72px 40px 40px;display:flex;flex-direction:column;min-height:100vh}
.co-back{display:inline-flex;align-items:center;gap:8px;font-size:14px;color:#B9C3BD}
.co-back:hover{color:#fff}
.co-merchant{display:flex;align-items:center;gap:12px;margin-top:40px}
.co-sub{font-size:13px;color:#93A09A}
.co-total-label{font-size:14px;color:#A9B5AE;margin-top:36px}
.co-total{font-family:var(--display);font-weight:700;font-size:56px;letter-spacing:-0.045em;line-height:1.05}
.co-total span{font-size:22px;color:#A9B5AE}
.co-items{margin-top:32px;display:flex;flex-direction:column;gap:12px;font-size:14.5px}
.co-line{height:1px;background:rgba(255,255,255,.12)}
.co-main{background:var(--paper);display:flex}
.co-main-in{width:460px;max-width:100%;padding:96px 40px 40px 72px;display:flex;flex-direction:column}
.co-h{font-family:var(--display);font-weight:700;font-size:28px;letter-spacing:-0.03em}
.co-methods{display:grid;grid-template-columns:repeat(3,1fr);gap:10px;margin-top:20px}
.co-m{display:flex;flex-direction:column;align-items:flex-start;gap:14px;padding:14px;border-radius:16px;border:2px solid #E4E2DA;background:#fff;text-align:left}
.co-m.on{border-color:var(--green);background:var(--green-tint)}
.co-m-txt{display:block}.co-m-name{display:block;font-weight:600;font-size:14.5px}.co-m-sub{display:none;font-size:12.5px;color:var(--muted)}
.co-radio{display:none}
.co-pay{margin-top:22px}
.co-center{display:flex;flex-direction:column;align-items:center;text-align:center}
.co-url{display:flex;align-items:center;justify-content:center;gap:6px;height:30px;padding:0 14px;border-radius:10px;background:var(--paper-2);font-size:13px;color:var(--text-2);align-self:center}
.co-mcard{margin-top:14px;background:#fff;border:1px solid var(--line);border-radius:24px;padding:18px}
@media (max-width:860px){
  .co{grid-template-columns:1fr}
  .co-side{display:none}
  .co-main-in{width:100%;padding:max(10px,env(safe-area-inset-top)) 20px 16px;min-height:100vh}
  .co-h{font-size:20px;letter-spacing:-0.02em;margin:24px 2px 10px}
  .co-methods{grid-template-columns:1fr;gap:8px;margin-top:0}
  .co-m{flex-direction:row;align-items:center;gap:12px;height:66px;padding:0 16px;border-radius:18px}
  .co-m-txt{flex:1}.co-m-name{font-size:15.5px}.co-m-sub{display:block}
  .co-radio{display:grid;width:22px;height:22px;border-radius:50%;border:2px solid #E4E2DA;place-items:center}
  .co-m.on .co-radio{border-color:var(--green)}
  .co-radio span{width:12px;height:12px;border-radius:50%}
  .co-m.on .co-radio span{background:var(--green)}
  .co-pay{position:sticky;bottom:0;margin-top:auto;padding:14px 0 10px;background:linear-gradient(180deg,rgba(244,243,238,0),#F4F3EE 30%)}
  .co-center{padding-top:26px}
}
@media print{.co-side,.co-url,.testbar,button{display:none!important}.co{display:block}}
`
