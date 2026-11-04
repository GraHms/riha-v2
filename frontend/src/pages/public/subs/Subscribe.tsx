import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { Icon, Spinner } from '../../../components/ui'
import { useGateway } from '../../../core/gateway'
import { errMsg, type ApiError } from '../../../core/model'
import { money, MonthStrip } from '../../app/subs/common'
import { Center, MerchantHead, PayFrame } from './PayFrame'

const per = (i: string) => (i === 'week' ? 'semana' : i === 'quarter' ? 'trimestre' : i === 'year' ? 'ano' : 'mês')

function whenText(p: any) {
  const months = p.months_label === 'todos os meses' ? '' : `, ${p.months_label.replace(' a ', ' a ')}`
  const day = p.anchor === 'signup' ? 'No dia em que subscreve' : p.interval === 'week' ? p.when_label : `Dia ${p.billing_day}`
  return `${day}${months}.${p.first_period ? ` Paga ${p.first_period} hoje.` : ''}`
}

export function PlanPage() {
  const { merchant, slug } = useParams()
  const g = useGateway()
  const [d, setD] = useState<any>(null)
  const [err, setErr] = useState('')
  useEffect(() => { g.publicSubs.plan(merchant!, slug!).then(setD).catch(e => setErr(errMsg(e))) }, [merchant, slug])
  if (err) return <Center><div className="display" style={{ fontSize: 24 }}>Plano indisponível</div><p className="muted">{err}</p></Center>
  if (!d) return <Center><Spinner dark /></Center>
  return <SubscribeForm merchantSlug={merchant!} merchant={d.merchant} plans={[d.plan]} test={d.test_mode} />
}

export function CatalogPage() {
  const { merchant } = useParams()
  const g = useGateway()
  const [d, setD] = useState<any>(null)
  const [err, setErr] = useState('')
  const [sel, setSel] = useState<string[]>([])
  const [go, setGo] = useState(false)
  useEffect(() => { g.publicSubs.catalog(merchant!).then(setD).catch(e => setErr(errMsg(e))) }, [merchant])
  if (err) return <Center><div className="display" style={{ fontSize: 24 }}>Catálogo indisponível</div><p className="muted">{err}</p></Center>
  if (!d) return <Center><Spinner dark /></Center>
  const chosen = d.plans.filter((p: any) => sel.includes(p.id))
  if (go && chosen.length) return <SubscribeForm merchantSlug={merchant!} merchant={d.merchant} plans={chosen} test={d.test_mode} onBack={() => setGo(false)} />
  const total = chosen.reduce((a: number, p: any) => a + p.amount, 0)
  const toggle = (id: string) => setSel(d.multi ? (sel.includes(id) ? sel.filter(x => x !== id) : [...sel, id]) : [id])
  return (
    <PayFrame test={d.test_mode}>
      <MerchantHead m={d.merchant} sub="Planos disponíveis" />
      <h1 className="display" style={{ fontSize: 26, margin: '22px 0 12px' }}>O que quer pagar?</h1>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
        {d.plans.map((p: any) => {
          const on = sel.includes(p.id)
          return (
            <button key={p.id} onClick={() => toggle(p.id)} className="card" style={{ textAlign: 'left', padding: '14px 16px', borderRadius: 16, display: 'flex', gap: 12, alignItems: 'center', border: on ? '2px solid var(--ink)' : '1px solid var(--line)', margin: on ? 0 : 1 }}>
              <span style={{ flex: 1 }}><b style={{ display: 'block', fontSize: 15 }}>{p.name}</b><span style={{ fontSize: 12.5, color: 'var(--muted)' }}>{p.when_label} · {p.months_label}</span></span>
              <span className="tnum" style={{ fontWeight: 700 }}>{money(p.amount)} MT <span style={{ fontWeight: 400, color: 'var(--muted)', fontSize: 12 }}>/ {per(p.interval)}</span></span>
              <span style={{ width: 22, height: 22, borderRadius: d.multi ? 7 : '50%', background: on ? 'var(--ink)' : '#fff', border: on ? 0 : '1.5px solid #CFCDC4', display: 'grid', placeItems: 'center', flex: 'none' }}>{on && <Icon name="check" size={13} stroke={3} color="#fff" />}</span>
            </button>
          )
        })}
      </div>
      <div style={{ flex: 1, minHeight: 24 }} />
      {chosen.length > 0 && <div className="between" style={{ margin: '18px 4px 10px', fontSize: 14 }}><span>{chosen.length} {chosen.length === 1 ? 'plano' : 'planos'}</span><b className="tnum">{money(total)} MT</b></div>}
      <button className="btn btn-green btn-lg btn-block" disabled={!chosen.length} onClick={() => setGo(true)}>Continuar</button>
    </PayFrame>
  )
}

function SubscribeForm({ merchantSlug, merchant, plans, test, onBack }: { merchantSlug: string; merchant: any; plans: any[]; test: boolean; onBack?: () => void }) {
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
  const p0 = plans[0]
  const firstMonth = p0.first_period ? ['janeiro', 'fevereiro', 'março', 'abril', 'maio', 'junho', 'julho', 'agosto', 'setembro', 'outubro', 'novembro', 'dezembro'].indexOf(p0.first_period.split(' ')[0]) : -1
  const submit = async () => {
    setBusy(true); setErr(null)
    try {
      const r = await g.publicSubs.subscribe(merchantSlug, { plan_ids: plans.map(p => p.id), phone: digits, fields: vals, name: fields.length ? '' : name, payer_name: fields.length ? name : '', channel, accept })
      nav('/i/' + r.token)
    } catch (e) { setErr(e as ApiError) } finally { setBusy(false) }
  }
  const fe = (p: string) => (err?.param === p ? ' err' : '')
  return (
    <PayFrame test={test}>
      {onBack && <button className="icon-btn" onClick={onBack} aria-label="Voltar" style={{ marginTop: 12 }}><Icon name="arrowLeft" size={18} /></button>}
      <MerchantHead m={merchant} sub={plans.map(p => p.name).join(' + ')}>
        <div style={{ borderTop: '1px dashed #E1DFD7', marginTop: 14, paddingTop: 14 }}>
          <div className="display tnum" style={{ fontSize: 32 }}>{money(total)} <span style={{ fontSize: 15, color: 'var(--muted)', fontFamily: 'var(--sans)' }}>MT por {per(p0.interval)}</span></div>
          <div style={{ marginTop: 10 }}><MonthStrip mask={p0.months} size={22} highlight={firstMonth >= 0 ? firstMonth : undefined} /></div>
          <div style={{ fontSize: 13, color: 'var(--muted)', marginTop: 10 }}>{whenText(p0)}</div>
        </div>
      </MerchantHead>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 14, marginTop: 18 }}>
        {fields.map(f => <label key={f}><span className="label">{f}</span><input className={'input lg' + fe('fields.' + f)} value={vals[f] || ''} onChange={e => setVals({ ...vals, [f]: e.target.value })} /></label>)}
        <label><span className="label">{fields.length ? 'O seu nome (opcional)' : 'O seu nome'}</span><input className={'input lg' + fe('name')} value={name} onChange={e => setName(e.target.value)} autoComplete="name" /></label>
        <label><span className="label">O seu número</span><div className={'phone-input' + fe('phone')}><span className="cc">+258</span><input inputMode="tel" value={phone} onChange={e => setPhone(e.target.value)} placeholder="84 123 4567" autoComplete="tel-national" /></div></label>
        <div style={{ display: 'flex', alignItems: 'center', gap: 10, fontSize: 14 }}>Receber os links por<div className="seg"><button type="button" className={channel === 'sms' ? 'on' : ''} onClick={() => setChannel('sms')}>SMS</button><button type="button" className={channel === 'whatsapp' ? 'on' : ''} onClick={() => setChannel('whatsapp')}>WhatsApp</button></div></div>
        <label style={{ display: 'flex', gap: 12, alignItems: 'flex-start', padding: 14, borderRadius: 16, background: '#fff', border: '1px solid var(--line)', cursor: 'pointer' }}>
          <input type="checkbox" checked={accept} onChange={e => setAccept(e.target.checked)} style={{ width: 20, height: 20, marginTop: 2, accentColor: 'var(--green)', flex: 'none' }} />
          <span style={{ fontSize: 13.5, lineHeight: 1.5, color: 'var(--text-2)' }}>{texts.length ? texts.map((t, i) => <span key={i} style={{ display: 'block', marginTop: i ? 6 : 0 }}>{t}</span>) : 'A carregar o acordo…'}</span>
        </label>
      </div>
      {err && <div className="form-err" style={{ marginTop: 14 }}>{err.message}</div>}
      <div style={{ flex: 1, minHeight: 20 }} />
      <button className="btn btn-green btn-lg btn-block" style={{ marginTop: 18 }} disabled={busy || !accept} onClick={submit}>{busy ? <Spinner /> : `Subscrever e pagar ${money(total)} MT`}</button>
      <div style={{ textAlign: 'center', fontSize: 12.5, color: 'var(--muted)', marginTop: 10 }}>Nada é cobrado sem a sua confirmação.</div>
    </PayFrame>
  )
}
