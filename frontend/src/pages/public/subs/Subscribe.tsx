import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { Icon, Spinner } from '../../../components/ui'
import { useGateway } from '../../../core/gateway'
import { errMsg, type ApiError } from '../../../core/model'
import { money } from '../../app/subs/common'
import { BrandFrame, CatalogView, Hero, Highlights, PaysWith, PlanFacts, per, Price, type CustomerPlan } from './CatalogView'
import { Center } from './PayFrame'

// On a catalog's own subdomain, the server names the catalog in the page.
export const catalogHost = () => document.querySelector('meta[name="riha-catalog"]')?.getAttribute('content') || ''

function Unavailable({ what, err }: { what: string; err: string }) {
  return <Center><div className="display" style={{ fontSize: 24 }}>{what} indisponível</div><p className="muted">{err}</p></Center>
}

export function PlanPage({ catalog }: { catalog?: string }) {
  const params = useParams()
  const merchant = catalog || params.merchant!
  const slug = params.slug!
  const g = useGateway()
  const [d, setD] = useState<any>(null)
  const [err, setErr] = useState('')
  useEffect(() => { g.publicSubs.plan(merchant, slug).then(setD).catch(e => setErr(errMsg(e))) }, [merchant, slug])
  if (err) return <Unavailable what="Plano" err={err} />
  if (!d) return <Center><Spinner dark /></Center>
  return <SubscribeForm merchantSlug={merchant} merchant={d.merchant} page={d.page} plans={[d.plan]} test={d.test_mode} />
}

export function CatalogPage({ catalog }: { catalog?: string }) {
  const params = useParams()
  const merchant = catalog || params.merchant!
  const g = useGateway()
  const [d, setD] = useState<any>(null)
  const [err, setErr] = useState('')
  const [sel, setSel] = useState<string[]>([])
  const [go, setGo] = useState(false)
  useEffect(() => { g.publicSubs.catalog(merchant).then(r => { setD(r); if (r.plans.length === 1) setSel([r.plans[0].id]) }).catch(e => setErr(errMsg(e))) }, [merchant])
  useEffect(() => { window.scrollTo(0, 0) }, [go])
  if (err) return <Unavailable what="Catálogo" err={err} />
  if (!d) return <Center><Spinner dark /></Center>
  const chosen = d.plans.filter((p: any) => sel.includes(p.id))
  if (go && chosen.length) return <SubscribeForm merchantSlug={merchant} merchant={d.merchant} page={d.page} plans={chosen} test={d.test_mode} onBack={() => setGo(false)} />
  const toggle = (id: string) => setSel(d.multi ? (sel.includes(id) ? sel.filter(x => x !== id) : [...sel, id]) : sel.includes(id) ? [] : [id])
  return (
    <BrandFrame page={d.page} test={d.test_mode}>
      <CatalogView merchant={d.merchant} page={d.page} plans={d.plans} multi={d.multi} sel={sel} onToggle={toggle} onContinue={() => setGo(true)} />
    </BrandFrame>
  )
}

function SubscribeForm({ merchantSlug, merchant, page, plans, test, onBack }: { merchantSlug: string; merchant: any; page?: any; plans: CustomerPlan[] & any[]; test: boolean; onBack?: () => void }) {
  const g = useGateway()
  const nav = useNavigate()
  const fields = useMemo(() => { const s: string[] = []; plans.forEach(p => p.fields.forEach((f: string) => !s.includes(f) && s.push(f))); return s }, [plans])
  const [vals, setVals] = useState<Record<string, string>>({})
  const [name, setName] = useState('')
  const [phone, setPhone] = useState('')
  const [channel, setChannel] = useState('sms')
  const [accept, setAccept] = useState(false)
  const [texts, setTexts] = useState<string[]>([])
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState<ApiError | null>(null)
  const digits = phone.replace(/\D/g, '')
  useEffect(() => {
    const t = setTimeout(() => { g.publicSubs.agreement(merchantSlug, plans.map(p => p.id), digits.length === 9 ? digits : '', channel).then(r => setTexts(r.texts)).catch(() => {}) }, 250)
    return () => clearTimeout(t)
  }, [digits, channel, plans.map(p => p.id).join()])
  const total = plans.reduce((a, p) => a + p.amount, 0)
  const one = plans.length === 1 ? plans[0] : null
  const submit = async () => {
    setBusy(true); setErr(null)
    try {
      const r = await g.publicSubs.subscribe(merchantSlug, { plan_ids: plans.map(p => p.id), phone: digits, fields: vals, name: fields.length ? '' : name, payer_name: fields.length ? name : '', channel, accept })
      nav('/i/' + r.token)
    } catch (e) { setErr(e as ApiError) } finally { setBusy(false) }
  }
  const fe = (p: string) => (err?.param === p ? ' err' : '')
  return (
    <BrandFrame page={page} test={test}>
      {onBack && <button className="icon-btn" onClick={onBack} aria-label="Voltar aos planos" style={{ margin: '12px 16px 0' }}><Icon name="arrowLeft" size={18} /></button>}
      <Hero merchant={merchant} page={onBack ? { ...page, description: '', cover_url: page?.cover_url } : page} />
      <div style={{ padding: '0 16px' }}>
        <div className="bf-card" style={{ marginTop: 12 }}>
          {one ? (
            <>
              <div className="bf-plan-name">{one.name}</div>
              <div style={{ marginTop: 4 }}><Price amount={one.amount} interval={one.interval} size={30} /></div>
              <Highlights items={one.highlights} />
              <PlanFacts plan={one} />
            </>
          ) : (
            <>
              {plans.map(p => (
                <div key={p.id} className="between" style={{ padding: '10px 0', borderBottom: '1px solid #F0EEE7', fontSize: 14.5, alignItems: 'flex-start' }}>
                  <span><b>{p.name}</b><span style={{ display: 'block', fontSize: 13, color: 'var(--muted)', marginTop: 2 }}>{p.when_text}{p.duration_label ? ` · ${p.duration_label}` : ''}</span></span>
                  <span className="tnum" style={{ fontWeight: 700, whiteSpace: 'nowrap' }}>{money(p.amount)} MT</span>
                </div>
              ))}
              <div className="between" style={{ paddingTop: 12, fontSize: 15 }}><span>Hoje paga</span><b className="tnum">{money(total)} MT</b></div>
            </>
          )}
        </div>
        <div className="bf-label">Os seus dados</div>
        <div className="bf-card" style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
          {fields.map(f => <label key={f}><span className="label">{f}</span><input className={'input lg' + fe('fields.' + f)} value={vals[f] || ''} onChange={e => setVals({ ...vals, [f]: e.target.value })} /></label>)}
          <label><span className="label">{fields.length ? 'O seu nome (opcional)' : 'O seu nome'}</span><input className={'input lg' + fe('name')} value={name} onChange={e => setName(e.target.value)} autoComplete="name" /></label>
          <label><span className="label">O seu número</span><div className={'phone-input' + fe('phone')}><span className="cc">+258</span><input inputMode="tel" value={phone} onChange={e => setPhone(e.target.value)} placeholder="84 123 4567" autoComplete="tel-national" /></div></label>
          <div>
            <span className="label">Receber o link de pagamento antes de cada data por</span>
            <div className="seg" style={{ display: 'flex' }}><button type="button" style={{ flex: 1 }} className={channel === 'sms' ? 'on' : ''} onClick={() => setChannel('sms')}>SMS</button><button type="button" style={{ flex: 1 }} className={channel === 'whatsapp' ? 'on' : ''} onClick={() => setChannel('whatsapp')}>WhatsApp</button></div>
          </div>
        </div>
        <label className="bf-card" style={{ display: 'flex', gap: 12, alignItems: 'flex-start', marginTop: 12, cursor: 'pointer' }}>
          <input type="checkbox" checked={accept} onChange={e => setAccept(e.target.checked)} style={{ width: 22, height: 22, marginTop: 1, accentColor: 'var(--brand)', flex: 'none' }} />
          <span style={{ fontSize: 13.5, lineHeight: 1.5, color: 'var(--text-2)' }}>{texts.length ? texts.map((t, i) => <span key={i} style={{ display: 'block', marginTop: i ? 6 : 0 }}>{t}</span>) : 'A carregar o acordo…'}</span>
        </label>
        {err && <div className="form-err" style={{ marginTop: 14 }}>{err.message}</div>}
        <PaysWith />
      </div>
      <div className="bf-bar">
        <button className="bf-cta" disabled={busy || !accept} onClick={submit}>{busy ? <Spinner /> : accept ? `Subscrever e pagar ${money(total)} MT` : 'Aceite o acordo para continuar'}</button>
      </div>
    </BrandFrame>
  )
}

export { per }
