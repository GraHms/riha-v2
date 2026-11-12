import { useEffect, useRef, useState } from 'react'
import { submitWords } from './LinkCard'
import { startTelemetry, track } from '../../lib/telemetry'
import { SUPPORT_EMAIL, SUPPORT_MAILTO } from '../../lib/support'
import { useParams } from 'react-router-dom'
import { errMsg, type ApiError } from '../../core/model'
import { useGateway } from '../../core/gateway'
import { fmt, fullDate, METHOD_NAME, METHOD_SUB } from '../../lib/format'
import { walletPrompt } from '../../lib/walletPrompt'
import { Avatar, Icon, MethodIcon, Spinner, Stripes } from '../../components/ui'

type Session = {
  id: string; status: string; amount: number; description: string; livemode: boolean
  line_items: { name: string; quantity: number; unit_amount: number }[]; payment_method_types: string[]
  success_url: string; cancel_url: string; return_host: string; client_reference_id: string; customer_phone: string; customer_name: string; email_on_file: boolean; coming_soon?: string[]; abroad_methods?: string[]; hosted_methods?: string[]; abroad_countries?: { code: string; name: string; dial: string; currency: string }[]; link?: { details: string; image_url: string }; success_message?: string; submit_type?: string
  expires_at: string; merchant: { name: string; initials: string; avatar_color: string; descriptor: string }
  payment: null | { id: string; reference: string; status: string; method: string; country?: string; redirect_url?: string; phone_prefix: string; phone_last4: string; card_brand: string; card_last4: string; failure_code: string; failure_message: string; created_at: string; succeeded_at: string | null; stage?: string }
}

// bim holds a wallet payment for up to 5 minutes while it waits for the customer.
const WALLET_TIMEOUT = 300
// The server closes an unanswered payment at 10 minutes, so past this something is wrong.
const NO_ANSWER_AFTER = 12 * 60
const MAX_CHECK_GAP = 5_000

type Conn = 'ok' | 'trying' | 'down'

const ATTEMPT_LETTERS = 'ABCDEFGHJKLMNPQRSTUVWXYZ23456789'

// Made before asking to pay, so the customer has a code even if the answer never comes,
// and the same request can't charge twice.
function newAttemptId() {
  const bytes = crypto.getRandomValues(new Uint8Array(12))
  const code = Array.from(bytes, b => ATTEMPT_LETTERS[b % ATTEMPT_LETTERS.length]).join('')
  return `${code.slice(0, 4)}-${code.slice(4, 8)}-${code.slice(8)}`
}

function savedAttempt(key: string): string | null {
  try { return sessionStorage.getItem(key) } catch { return null }
}
function saveAttempt(key: string, id: string | null) {
  try { if (id) sessionStorage.setItem(key, id); else sessionStorage.removeItem(key) } catch { /* it still works without it */ }
}

function RefCode({ label, code, dark }: { label: string; code: string; dark?: boolean }) {
  const [copied, setCopied] = useState(false)
  const copy = () => { navigator.clipboard.writeText(code).then(() => { setCopied(true); setTimeout(() => setCopied(false), 1800) }).catch(() => {}) }
  return (
    <div className="between" style={{ width: '100%', marginTop: 14, gap: 10, fontSize: 13.5, textAlign: 'left' }}>
      <span><span className={dark ? undefined : 'muted'} style={dark ? { color: '#93A09A' } : undefined}>{label}: </span><b className="mono" style={{ fontSize: 13 }}>{code}</b></span>
      {navigator.clipboard && <button type="button" className="btn btn-ghost" style={{ padding: '4px 8px', fontSize: 13, flex: 'none', color: dark ? '#C9D3CD' : undefined }} onClick={copy}>{copied ? 'Copiado' : 'Copiar'}</button>}
    </div>
  )
}

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
  const [country, setCountry] = useState('')
  const [card, setCard] = useState({ number: '', exp: '', cvc: '', name: '' })
  const [email, setEmail] = useState('')
  const [err, setErr] = useState<ApiError | null>(null)
  // The USSD prompt cuts the phone's data, so losing the connection here is normal.
  const [conn, setConn] = useState<Conn>('ok')
  const [checkedAt, setCheckedAt] = useState<Date | null>(null)
  const [manualCheck, setManualCheck] = useState(false)
  const checkNow = useRef<() => Promise<unknown>>(async () => {})
  const attemptKey = 'riha_attempt_' + id
  const attemptRef = useRef<string | null>(savedAttempt(attemptKey))
  const setAttempt = (v: string | null) => { attemptRef.current = v; saveAttempt(attemptKey, v) }
  const [busy, setBusy] = useState(false)
  const [view, setView] = useState<'pay' | 'confirm' | 'done'>('pay')
  const [tick, setTick] = useState(0)
  const [redirectIn, setRedirectIn] = useState(8)
  const startRef = useRef(0)
  // Set when the answer to "pay" was lost: the payment before it, to tell them apart.
  const lostPay = useRef<{ before: string | null } | null>(null)

  const refresh = async (signal?: AbortSignal) => {
    try {
      const d = await g.checkout.session(id!, signal)
      setS(d)
      setConn('ok')
      setCheckedAt(new Date())
      if (d.payment?.attempt_id && d.payment.attempt_id === attemptRef.current) setAttempt(null)
      return d
    } catch { return null }
  }

  // A failed check says nothing about the payment, so only the first load can fail
  // the page: phones drop requests while the customer is away entering their PIN.
  const open = async () => {
    setLoadErr(null)
    for (let attempt = 0; ; attempt++) {
      try {
        const d = await g.checkout.session(id!)
        setS(d)
        if (d.payment?.attempt_id && d.payment.attempt_id === attemptRef.current) setAttempt(null)
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
    // An unconfirmed payment shows its own notice on the pay screen, with its wallet picked.
    else if (d.payment?.status === 'unconfirmed' && d.payment_method_types.includes(d.payment.method)) setMethod(d.payment.method)
    // Coming back to a failed attempt: say so, rather than a form with no explanation.
    else if (d.payment?.status === 'failed') setErr({ code: d.payment.failure_code, message: d.payment.failure_message || 'O pagamento falhou.', status: 402 })
  })

  useEffect(() => { start() }, [id]) // eslint-disable-line react-hooks/exhaustive-deps
  // What the customer's side looks like goes to the payment's trail. It never blocks paying.
  useEffect(() => { if (id) startTelemetry(id) }, [id])
  useEffect(() => { if (s) track('method', { method }) }, [method]) // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => {
    if (view === 'done') track('result', { status: 'pago', payment: s?.payment?.id })
    if (view === 'confirm') track('waiting', { payment: s?.payment?.id })
  }, [view]) // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => { if (s?.payment?.status === 'failed') track('result', { status: 'falhou: ' + (s.payment.failure_code || ''), payment: s.payment.id }) }, [s?.payment?.status, s?.payment?.id])
  useEffect(() => { if (conn === 'down') track('no_answer') }, [conn])

  // The page never opened for lack of a connection: open it when the connection is back.
  const noConnection = loadErr?.code === 'network_error'
  useEffect(() => {
    if (!noConnection) return
    const retry = () => { start() }
    const t = setInterval(retry, MAX_CHECK_GAP)
    window.addEventListener('online', retry)
    return () => { clearInterval(t); window.removeEventListener('online', retry) }
  }, [noConnection]) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (view !== 'confirm') return
    let inFlight: Promise<void> | null = null
    let current: AbortController | null = null
    let freshAt = 0
    let failures = 0
    let nextCheck = 0
    const run = async (signal: AbortSignal) => {
      const d = await refresh(signal)
      // A check replaced by a newer one says nothing about the connection.
      if (signal.aborted) return
      if (!d) {
        // Keep trying, but less often, for as long as the page is open.
        failures += 1
        nextCheck = Date.now() + Math.min(1500 * 2 ** failures, MAX_CHECK_GAP)
        setConn(failures >= 2 || !navigator.onLine ? 'down' : 'trying')
        return
      }
      failures = 0
      nextCheck = 0
      const lost = lostPay.current
      if (lost && (d.payment?.id ?? null) === lost.before) {
        // Connected again and no new payment, so the request never arrived.
        lostPay.current = null
        setView('pay')
        setErr({ code: 'network_error', message: 'O pedido não chegou ao operador. Nada foi cobrado. Tente de novo.', status: 0 })
        return
      }
      lostPay.current = null
      if (!d.payment) return
      if (d.status === 'complete' || d.payment.status === 'succeeded') setView('done')
      else if (d.payment.status === 'failed') { setView('pay'); setErr({ code: d.payment.failure_code, message: d.payment.failure_message || 'O pagamento falhou.', status: 402 }) }
      else if (d.payment.status === 'unconfirmed') { setView('pay'); setErr(null) }
    }
    const start = () => {
      current = new AbortController()
      const mine: Promise<void> = run(current.signal).finally(() => { if (inFlight === mine) inFlight = null })
      inFlight = mine
      return mine
    }
    const check = () => {
      if (inFlight || document.hidden || Date.now() < nextCheck) return inFlight ?? Promise.resolve()
      return start()
    }
    // A check open when the phone froze the page, or when its address changed, can stay
    // stuck on a dead connection. So coming back replaces it instead of waiting for it,
    // even if the timer fired one a moment ago. Only the wake-up signals, which come
    // together, count as repeats.
    const fresh = () => {
      if (inFlight && Date.now() - freshAt < 500) return inFlight
      freshAt = Date.now()
      current?.abort()
      nextCheck = 0
      return start()
    }
    checkNow.current = fresh
    const t = setInterval(() => { setTick(x => x + 1); check() }, 1500)
    // Back from entering the PIN, or back online: answer at once instead of at the next tick.
    // The browser's own "online" can be wrong, as on a phone showing LTE with no data,
    // so these only bring the next check forward: the check itself is the real test.
    const now = () => {
      if (document.hidden) return
      setManualCheck(true)
      fresh().finally(() => setManualCheck(false))
    }
    const offline = () => setConn('down')
    document.addEventListener('visibilitychange', now)
    window.addEventListener('online', now)
    window.addEventListener('focus', now)
    window.addEventListener('pageshow', now)
    window.addEventListener('offline', offline)
    return () => {
      clearInterval(t)
      current?.abort()
      document.removeEventListener('visibilitychange', now)
      window.removeEventListener('online', now)
      window.removeEventListener('focus', now)
      window.removeEventListener('pageshow', now)
      window.removeEventListener('offline', offline)
    }
  }, [view]) // eslint-disable-line react-hooks/exhaustive-deps

  // A payment left unconfirmed can still be confirmed while the page is open.
  const unconfirmedId = view === 'pay' && s?.payment?.status === 'unconfirmed' ? s.payment.id : null
  useEffect(() => {
    if (!unconfirmedId) return
    const t = setInterval(() => { if (!document.hidden) refresh().then(d => { if (d?.status === 'complete') setView('done') }) }, 10000)
    return () => clearInterval(t)
  }, [unconfirmedId]) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (view !== 'done' || !s?.success_url) return
    if (redirectIn <= 0) { window.location.href = s.success_url; return }
    const t = setTimeout(() => setRedirectIn(x => x - 1), 1000)
    return () => clearTimeout(t)
  }, [view, redirectIn, s?.success_url])

  const pay = async () => {
    track('submit', { method })
    if (!s) return
    setBusy(true); setErr(null)
    // Kept when the answer is lost, so trying again can't start a second payment.
    const attemptId = attemptRef.current ?? newAttemptId()
    setAttempt(attemptId)
    // The notice about the unconfirmed payment is on screen, so paying now is a choice.
    const payAgain = s.payment?.status === 'unconfirmed'
    try {
      const abroad = (s.abroad_methods || []).includes(method)
      const where = abroad ? ((s.abroad_countries || []).find(c => c.code === country) || (s.abroad_countries || [])[0])?.code : undefined
      const hosted = (s.hosted_methods || []).includes(method)
      const body = hosted
        ? { method, email, attempt_id: attemptId, pay_again: payAgain }
        : abroad
        ? { method, country: where, phone: phone.replace(/\D/g, ''), email, attempt_id: attemptId, pay_again: payAgain }
        : method === 'card'
          ? { method, card_number: card.number, card_expiry: card.exp, card_cvc: card.cvc, name: card.name, email, attempt_id: attemptId, pay_again: payAgain }
          : { method, phone: phone.replace(/\D/g, ''), email, attempt_id: attemptId, pay_again: payAgain }
      const d = await g.checkout.pay(s.id, body)
      setAttempt(null)
      setS(d)
      // Paid on the operator's own page: go there, and come back here after.
      if (d.payment?.status === 'processing' && d.payment.redirect_url) { setView('confirm'); window.location.assign(d.payment.redirect_url); return }
      if (d.status === 'complete') setView('done')
      else if (d.payment?.status === 'processing') { startRef.current = Date.now(); setTick(0); setView('confirm') }
      else if (d.payment?.status === 'failed') setErr({ code: d.payment.failure_code, message: d.payment.failure_message, status: 402 })
    } catch (e) {
      const failure = e as ApiError
      if (failure.code === 'network_error') {
        // The answer was lost, and the prompt on the phone may be why: wait for the
        // connection and let the server say whether the payment started.
        lostPay.current = { before: s.payment?.id ?? null }
        startRef.current = Date.now(); setTick(0); setConn('down'); setView('confirm')
      } else if (failure.code === 'payment_unconfirmed') {
        // The page didn't know yet: show the notice, and the next tap pays again.
        setAttempt(null); await refresh()
      } else { setAttempt(null); setErr(failure) }
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
  // Paid on the operator's page: the customer picks their country first.
  const abroadMethod = (s.abroad_methods || []).includes(method)
  // Paid wholly on the operator's page: nothing to fill in here.
  const hostedMethod = (s.hosted_methods || []).includes(method)
  const countries = s.abroad_countries || []
  const pickedCountry = countries.find(c => c.code === country) || countries[0]
  const wallet = method !== 'card' && !abroadMethod
  const walletName = METHOD_NAME[method]
  const walletPrefix = method === 'emola' ? '86' : method === 'bim' ? '82' : '84'
  const ref = s.client_reference_id ? (/^\d+$/.test(s.client_reference_id) ? `Encomenda #${s.client_reference_id}` : s.client_reference_id) : s.description
  const expired = s.status === 'expired'
  const elapsed = startRef.current ? Math.floor((Date.now() - startRef.current) / 1000) : 0
  const left = Math.max(0, WALLET_TIMEOUT - elapsed)
  const waitingPhone = s.payment?.status === 'processing' ? `${s.payment.phone_prefix} ••• ${s.payment.phone_last4}` : phone
  const prompt = walletPrompt(s.payment?.method ?? method)
  const waitingRef = lostPay.current || s.payment?.status !== 'processing' ? null : s.payment.reference
  const failedRef = s.payment?.status === 'failed' ? s.payment.reference : null
  const noAnswer = elapsed >= NO_ANSWER_AFTER
  const checkResult = async () => { track('check', { payment: s?.payment?.id }); setManualCheck(true); try { await checkNow.current() } finally { setManualCheck(false) } }
  void tick

  const items = s.line_items.length ? s.line_items : [{ name: s.description || 'Pagamento', quantity: 1, unit_amount: s.amount }]

  const summary = (
    <>
      {s.cancel_url && s.return_host && <a href={s.cancel_url} onClick={e => { e.preventDefault(); cancel() }} className="co-back"><Icon name="arrowLeft" size={16} stroke={2.2} />Voltar a {s.return_host}</a>}
      <div className="co-merchant">
        <Avatar text={s.merchant.initials} color={s.merchant.avatar_color} size={44} radius={13} font={15} />
        <div><div style={{ fontWeight: 600, fontSize: 16 }}>{s.merchant.name}</div><div className="co-sub">{ref}</div></div>
      </div>
      {s.link?.image_url && <img src={s.link.image_url} alt="" style={{ display: 'block', width: '100%', maxWidth: 360, aspectRatio: '16 / 9', objectFit: 'cover', borderRadius: 16, marginTop: 18 }} />}
      {s.link?.details && <p style={{ margin: '12px 0 0', fontSize: 14, lineHeight: 1.55, opacity: 0.78, whiteSpace: 'pre-line', maxWidth: 380 }}>{s.link.details}</p>}
      <div className="co-total-label">{submitWords(s.submit_type).total}</div>
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
      {!s.livemode && <div className="testbar" style={{ position: 'fixed', top: 0, left: 0, right: 0, zIndex: 5 }}>{method === 'mpesa' ? 'Modo de teste · use 84 xxx 0000 para simular PIN errado' : 'Modo de teste'}</div>}
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
                {s.link?.image_url && <img src={s.link.image_url} alt="" style={{ display: 'block', width: 'calc(100% + 36px)', margin: '-18px -18px 16px', aspectRatio: '16 / 9', objectFit: 'cover' }} />}
                <div className="row">
                  <Avatar text={s.merchant.initials} color={s.merchant.avatar_color} size={44} radius={14} font={15} />
                  <div style={{ flex: 1 }}><div style={{ fontWeight: 600, fontSize: 16 }}>{s.merchant.name}</div><div style={{ fontSize: 12.5, color: 'var(--muted)', marginTop: 1 }}>{ref}</div></div>
                </div>
                {s.link?.details && <p style={{ margin: '12px 0 0', fontSize: 14, lineHeight: 1.5, color: 'var(--text-2)', whiteSpace: 'pre-line' }}>{s.link.details}</p>}
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-end', marginTop: 16, paddingTop: 14, borderTop: '1px dashed #E1DFD7' }}>
                  <div><div style={{ fontSize: 13, color: 'var(--muted)' }}>{items[0].name}{items.length > 1 ? ` + ${items.length - 1}` : ''}</div><div style={{ fontSize: 13, color: 'var(--muted)', marginTop: 2 }}>{submitWords(s.submit_type).total}</div></div>
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
                  {s.payment?.status === 'unconfirmed' && <Unconfirmed p={s.payment} amount={amount} />}
                  <div className="co-h" style={s.payment?.status === 'unconfirmed' ? { marginTop: 28 } : undefined}>{s.payment?.status === 'unconfirmed' ? 'Ou pague de novo' : 'Como quer pagar?'}</div>
                  <div className="co-methods">
                    {s.payment_method_types.map(m => (
                      <button key={m} type="button" onClick={() => { setMethod(m); setErr(null) }} className={'co-m' + (method === m ? ' on' : '')} aria-pressed={method === m}>
                        <MethodIcon method={m} size={38} />
                        <span className="co-m-txt"><span className="co-m-name">{METHOD_NAME[m]}</span><span className="co-m-sub">{METHOD_SUB[m]}</span></span>
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
                  {hostedMethod ? (
                    <div style={{ marginTop: 20, padding: '14px 16px', borderRadius: 14, background: '#fff', border: '1px solid var(--line)', display: 'flex', gap: 12, alignItems: 'flex-start' }}>
                      <Icon name="lock" size={18} stroke={2.2} />
                      <div style={{ fontSize: 14, lineHeight: 1.5 }}>Vai para a página segura da U2opia para pagar com Visa ou Mastercard. Os dados do cartão são escritos lá, nunca aqui. No fim volta a esta página.</div>
                    </div>
                  ) : abroadMethod && pickedCountry ? (
                    <div style={{ marginTop: 20, display: 'flex', flexDirection: 'column', gap: 12 }}>
                      <label>
                        <span className="label" style={{ marginBottom: 7 }}>País {method === 'card' ? 'do cartão' : 'da carteira'}</span>
                        <select className="input lg" style={{ height: 56, fontSize: 16, border: '2px solid var(--ink)' }} value={pickedCountry.code} onChange={e => setCountry(e.target.value)}>
                          {countries.map(c => <option key={c.code} value={c.code}>{c.name}</option>)}
                        </select>
                      </label>
                      <div>
                        <span className="label" style={{ marginBottom: 7 }}>Número de telemóvel</span>
                        <div className="phone-input" style={{ border: '2px solid var(--ink)', height: 56 }}><span className="cc">+{pickedCountry.dial}</span><input inputMode="tel" autoComplete="tel-national" value={phone} onChange={e => setPhone(e.target.value.replace(/[^\d ]/g, ''))} placeholder="Número sem o indicativo" style={{ fontSize: 17 }} /></div>
                      </div>
                      <div style={{ fontSize: 13, color: 'var(--muted)', lineHeight: 1.45 }}>Vai para a página segura da U2opia para pagar em {pickedCountry.currency}. No fim volta aqui.</div>
                    </div>
                  ) : wallet ? (
                    <div style={{ marginTop: 20 }}>
                      <span className="label" style={{ marginBottom: 7 }}>Número {walletName}</span>
                      <div className="phone-input" style={{ border: '2px solid var(--ink)', height: 56 }}><span className="cc">+258</span><input inputMode="tel" autoComplete="tel-national" value={phone} onChange={e => setPhone(fmtPhone(e.target.value))} placeholder={`${walletPrefix} 412 4231`} style={{ fontSize: 17 }} /></div>
                      <div style={{ fontSize: 13, color: 'var(--muted)', marginTop: 8, lineHeight: 1.45 }}>{walletPrompt(method).before}</div>
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
                    {err && failedRef && <RefCode label="Referência" code={failedRef} />}
                    {err && !failedRef && attemptRef.current && <RefCode label="Código do pedido" code={attemptRef.current} />}
                    <button className="btn btn-green btn-lg btn-block" style={{ height: 58, fontSize: 17, boxShadow: '0 14px 30px -14px rgba(10,130,5,.9)' }} onClick={pay} disabled={busy}>
                      {busy ? <Spinner /> : <><Icon name="lock" size={17} stroke={2.2} />{s.payment?.status === 'unconfirmed' ? 'Pagar de novo' : submitWords(s.submit_type).button} {amount} MT</>}
                    </button>
                    {s.return_host && <div className="only-desktop" style={{ fontSize: 12.5, lineHeight: 1.5, color: 'var(--muted)', marginTop: 14 }}>Depois de pagar, volta automaticamente para {s.return_host}.</div>}
                    <div className="only-mobile" style={{ marginTop: 12 }}><Secure /></div>
                  </div>
                </>
              )}
            </>
          )}

          {view === 'confirm' && s.payment?.status === 'processing' && s.payment.redirect_url && (
            <div className="co-abroad" role="status" style={{ marginTop: 22 }}>
              <div className="display" style={{ fontSize: 26 }}>A confirmar o pagamento</div>
              <p className="muted" style={{ lineHeight: 1.55, marginTop: 8 }}>Assim que a U2opia confirmar o pagamento de {amount} MT, esta página avança sozinha. Se ainda não pagou, continue na página da U2opia.</p>
              <a className="btn btn-ink btn-lg btn-block" style={{ marginTop: 14 }} href={s.payment.redirect_url}><Icon name="globe" size={17} />Voltar à página de pagamento</a>
              <div style={{ marginTop: 12 }}><RefCode label="Referência" code={s.payment.reference} /></div>
            </div>
          )}
          {view === 'confirm' && !(s.payment?.status === 'processing' && s.payment.redirect_url) && (
            <WaitScreen
              s={s} amount={amount} prompt={prompt} walletName={METHOD_NAME[s.payment?.method ?? method] ?? 'carteira'} phone={waitingPhone}
              stage={lostPay.current ? 'sent' : s.payment?.stage || 'sent'} conn={conn} noAnswer={noAnswer} pastLimit={left === 0}
              refCode={waitingRef ? { label: 'Referência', code: waitingRef } : attemptRef.current ? { label: 'Código do pedido', code: attemptRef.current } : null}
              onCheck={checkResult} checking={manualCheck} checkedAt={checkedAt}
              onChangeMethod={s.payment?.status !== 'processing' ? () => setView('pay') : null}
            />
          )}

          {view === 'done' && s.payment && (
            <div className="co-center" style={{ paddingTop: 34 }}>
              <div style={{ width: 96, height: 96, borderRadius: '50%', background: 'var(--green)', display: 'grid', placeItems: 'center', boxShadow: '0 0 0 12px #E2F3DE,0 0 0 26px #EEF6EB' }}><Icon name="check" size={44} stroke={2.6} color="#fff" /></div>
              <h1 className="display" style={{ margin: '38px 0 0', fontSize: 30 }}>{submitWords(s.submit_type).done}</h1>
              <p style={{ margin: '6px 0 0', fontSize: 15.5, color: 'var(--muted)' }}>{submitWords(s.submit_type).paid} <b style={{ color: 'var(--ink)' }}>{amount} MT</b> a {s.merchant.name}</p>
              {s.success_message && <div className="card" style={{ width: '100%', marginTop: 20, borderRadius: 20, padding: '14px 18px', textAlign: 'left', fontSize: 15, lineHeight: 1.55, whiteSpace: 'pre-line' }}><div style={{ fontSize: 12.5, color: 'var(--muted)', marginBottom: 4 }}>Mensagem de {s.merchant.name}</div>{s.success_message}</div>}
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

// The wait for the wallet: everything RIHA does takes under a second, so what the
// customer waits for is the phone and the wallet. The steps move on real events: the
// request is sent, then bim's call comes back once the customer has answered, then the
// wallet's answer. Nothing here counts down, and no answer is never shown as a failure.
type WaitProps = {
  s: Session; amount: string; prompt: ReturnType<typeof walletPrompt>; walletName: string; phone: string
  stage: string; conn: Conn; noAnswer: boolean; pastLimit: boolean; refCode: { label: string; code: string } | null
  onCheck: () => void; checking: boolean; checkedAt: Date | null; onChangeMethod: (() => void) | null
}

function WaitScreen(p: WaitProps) {
  const off = p.conn === 'down'
  const answered = p.stage === 'answered'
  let title: string
  let lines: string[]
  if (off) {
    title = 'Sem ligação à internet'
    lines = [`O pagamento continua na ${p.walletName}. Quando a ligação voltar, mostramos o resultado.`]
  } else if (p.noAnswer) {
    title = `Ainda sem resposta da ${p.walletName}`
    lines = []
  } else if (p.pastLimit) {
    title = `Ainda sem resposta da ${p.walletName}`
    lines = [p.prompt.stillWaiting]
  } else if (answered) {
    title = `A ${p.walletName} está a confirmar`
    lines = ['Não precisa de fazer mais nada', 'Normalmente demora menos de 1 minuto', `O dinheiro só sai quando a ${p.walletName} confirmar`]
  } else {
    title = p.prompt.title
    lines = p.prompt.cues(p.phone)
  }
  const [i, setI] = useState(0)
  const key = title + lines.length
  useEffect(() => {
    setI(0)
    if (lines.length < 2) return
    const t = setInterval(() => setI(x => x + 1), 4500)
    return () => clearInterval(t)
  }, [key]) // eslint-disable-line react-hooks/exhaustive-deps
  const line = lines.length ? lines[i % lines.length] : ''
  const step = answered ? 2 : 1

  return (
    <div className="wait" role="status" aria-live="polite">
      {!p.s.livemode && <div className="wait-test">Modo de teste</div>}
      <div className="wait-top">
        <Avatar text={p.s.merchant.initials} color={p.s.merchant.avatar_color} size={32} radius={10} font={12} />
        <span className="wait-name">{p.s.merchant.name}</span>
        <span className="wait-amt">{p.amount} MT</span>
      </div>

      <div className="wait-mid">
        <div className="wait-logo" aria-hidden="true">
          <div className="wait-ring" />
          <div className="wait-arc" />
          <div className="wait-core"><img src="/assets/riha-icon-dark.png" alt="" /></div>
        </div>
        {(off || p.conn === 'trying') && (
          <div className="wait-pill">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.4" strokeLinecap="round"><path d="M2 8.5a15 15 0 0 1 20 0M5.5 12a10 10 0 0 1 13 0M9 15.5a5 5 0 0 1 6 0M3 3l18 18" /></svg>
            {off ? 'Sem ligação · a tentar ligar' : 'A tentar contactar o servidor…'}
          </div>
        )}
        <h1 className="wait-title">{title}</h1>
        {line && <p key={line} className={'wait-line' + (lines.length > 1 ? ' turn' : '')}>{line}</p>}
        {p.noAnswer && (
          <div role="alert" className="wait-alert">Se o dinheiro saiu da sua conta, não pague outra vez: escreva para <a href={SUPPORT_MAILTO}>{SUPPORT_EMAIL}</a>{p.refCode ? <> com o código abaixo</> : null}.</div>
        )}
        <div className="wait-steps" aria-hidden="true">
          {[0, 1, 2].map(k => <span key={k} className={k < step ? 'done' : k === step ? 'now' : ''} />)}
        </div>
      </div>

      <div className="wait-foot">
        {p.refCode && <RefCode dark label={p.refCode.label} code={p.refCode.code} />}
        <button className="wait-check" onClick={p.onCheck}>{p.checking ? 'A verificar…' : 'Verificar o resultado agora'}</button>
        {p.checkedAt && <div className="wait-small">Última resposta do servidor às {p.checkedAt.toLocaleTimeString('pt-PT')}</div>}
        {p.onChangeMethod && <button className="wait-check" onClick={p.onChangeMethod}>Mudar método de pagamento</button>}
        <div className="wait-note">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="#8EE07F" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round"><rect x="5" y="11" width="14" height="10" rx="2" /><path d="M8 11V8a4 4 0 0 1 8 0v3" /></svg>
          <span>Fique nesta página até a {p.walletName} responder, para não pagar duas vezes. Se a fechar, o pagamento continua: volte a este link para ver o resultado.</span>
        </div>
      </div>
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
    <div style={{ marginTop: 6, textAlign: dark ? 'left' : 'center', fontSize: 12.5, color: dark ? '#93A09A' : 'var(--muted)' }}>Precisa de ajuda? <a href={SUPPORT_MAILTO} style={{ color: 'inherit', fontWeight: 600 }}>{SUPPORT_EMAIL}</a></div>
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
.co-mcard{margin-top:14px;background:#fff;border:1px solid var(--line);border-radius:24px;padding:18px;overflow:hidden}
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
.wait{position:fixed;inset:0;z-index:20;background:#0B120E;color:#fff;display:flex;flex-direction:column;align-items:center;overflow-y:auto;padding:max(18px,env(safe-area-inset-top)) 22px max(22px,env(safe-area-inset-bottom))}
.wait>*{width:100%;max-width:440px}
.wait-test{font-size:12px;font-weight:700;color:#FFD21F;text-align:center;margin-bottom:8px}
.wait-top{display:flex;align-items:center;gap:10px;font-size:13.5px;color:#A9B5AE;padding-top:6px}
.wait-name{flex:1;min-width:0;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.wait-amt{font-weight:700;color:#fff;white-space:nowrap}
.wait-mid{flex:1;display:flex;flex-direction:column;align-items:center;justify-content:center;text-align:center;padding:28px 0}
.wait-logo{position:relative;width:150px;height:150px;display:grid;place-items:center;flex:none}
.wait-ring{position:absolute;inset:0;border-radius:50%;border:2px dashed #2E3A33;animation:waitSpin 14s linear infinite}
.wait-arc{position:absolute;inset:14px;border-radius:50%;border:3px solid transparent;border-top-color:#8EE07F;animation:waitSpin 1.8s linear infinite}
.wait-core{width:84px;height:84px;border-radius:50%;background:#fff;display:grid;place-items:center;animation:waitBreath 2.4s ease-in-out infinite}
.wait-core img{width:46px;height:46px;object-fit:contain}
.wait-pill{margin-top:22px;height:30px;padding:0 12px;border-radius:999px;background:#3D3000;color:#FFD21F;font-size:12.5px;font-weight:700;display:flex;align-items:center;gap:6px;white-space:nowrap}
.wait-title{font-family:var(--display);font-weight:800;font-size:28px;line-height:1.12;letter-spacing:-0.03em;margin:30px 0 0;text-wrap:balance}
.wait-line{min-height:48px;margin:12px 0 0;font-size:15.5px;line-height:1.5;color:#C9D3CD;max-width:300px;text-wrap:balance}
.wait-line.turn{animation:waitLine 4.5s ease both}
.wait-alert{margin-top:16px;padding:12px 14px;border-radius:14px;background:#3D3000;color:#FFE9A3;font-size:13.5px;line-height:1.5;text-align:left}
.wait-alert a{color:#fff;font-weight:700}
.wait-steps{display:flex;gap:6px;margin-top:22px}
.wait-steps span{width:14px;height:6px;border-radius:3px;background:#2E3A33;transition:width .4s,background .4s}
.wait-steps span.done{background:#8EE07F}
.wait-steps span.now{width:34px;background:#8EE07F}
.wait-foot{display:flex;flex-direction:column;align-items:center;gap:8px}
.wait-check{width:100%;height:48px;border-radius:14px;border:1.5px solid #2E3A33;background:transparent;color:#fff;font-weight:700;font-size:14.5px;cursor:pointer}
.wait-check:hover{border-color:#5B6A62}
.wait-small{font-size:12px;color:#6F7D75}
.wait-note{margin-top:8px;display:flex;gap:10px;align-items:flex-start;padding:14px;border-radius:16px;background:#1C2620;font-size:13px;line-height:1.5;color:#C9D3CD;text-align:left}
.wait-note svg{flex:none;margin-top:1px}
@keyframes waitSpin{to{transform:rotate(360deg)}}
@keyframes waitBreath{0%,100%{transform:scale(1)}50%{transform:scale(1.06)}}
@keyframes waitLine{0%{opacity:0;transform:translateY(6px)}8%{opacity:1;transform:none}92%{opacity:1;transform:none}100%{opacity:0;transform:translateY(-4px)}}
@media (prefers-reduced-motion:reduce){.wait-ring,.wait-arc,.wait-core,.wait-line.turn{animation:none}}
@media print{.wait{display:none!important}.co-side,.co-url,.testbar,button{display:none!important}.co{display:block}}
`

// Unconfirmed tells a customer back on the page that their last payment is
// still being confirmed, so they don't pay twice by accident.
function Unconfirmed({ p, amount }: { p: { method: string; phone_prefix: string; phone_last4: string; reference: string }; amount: string }) {
  const wallet = METHOD_NAME[p.method] ?? 'a carteira'
  return (
    <div role="status" style={{ marginTop: 22, padding: '16px 18px', borderRadius: 18, background: '#FFF6D6', border: '1px solid #F1DC8C', color: '#3B3410' }}>
      <div className="row" style={{ gap: 8, fontWeight: 700, fontSize: 15.5 }}><Icon name="clock" size={18} stroke={2.2} />O pagamento anterior está a ser confirmado</div>
      <p style={{ margin: '8px 0 0', fontSize: 14, lineHeight: 1.55 }}>
        Pediu para pagar {amount} MT com {wallet}{p.phone_last4 ? ` (${p.phone_prefix} ••• ${p.phone_last4})` : ''} e o operador ainda não nos deu a resposta final.
      </p>
      <p style={{ margin: '8px 0 0', fontSize: 14, lineHeight: 1.55 }}>
        <b>Se aprovou no telemóvel, não pague de novo.</b> Confirmamos sozinhos e avisamos por SMS. Se não aprovou ou viu um erro, pode pagar de novo abaixo.
      </p>
      <div style={{ marginTop: 10, fontSize: 12.5, opacity: 0.8 }}>Referência <span className="mono">{p.reference}</span></div>
    </div>
  )
}
