import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { Icon, Spinner } from '../../../components/ui'
import { useGateway } from '../../../core/gateway'
import { errMsg, type ApiError } from '../../../core/model'
import { adjPlural, digitsMT, dl, dly, effort, mt, parseMT, prest, quote, rateLabel, unit, type Product } from '../../../lib/credit'
import { Center, PayFrame } from '../subs/PayFrame'
import { CodeSheet, Cta, Head, Row, Stepper } from './parts'

const PPM_WORD: Record<string, string> = { mensal: 'por mês', semanal: 'por semana', quinzenal: 'por quinzena' }

export default function RequestPage() {
  const { merchant } = useParams()
  const g = useGateway()
  const nav = useNavigate()
  const [d, setD] = useState<any>(null)
  const [loadErr, setLoadErr] = useState('')
  const [pi, setPi] = useState(0)
  const [step, setStep] = useState<'sim' | 'data'>('sim')
  const [amt, setAmt] = useState(0)
  const [n, setN] = useState(0)
  const [day, setDay] = useState(new Date().getDate() > 28 ? 28 : new Date().getDate())
  const [f, setF] = useState({ name: '', phone: '', id: '', activity: '', purpose: '', income: '' })
  const [extra, setExtra] = useState<Record<string, string>>({})
  const [front, setFront] = useState<File | null>(null)
  const [back, setBack] = useState<File | null>(null)
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState<ApiError | null>(null)
  const [sheet, setSheet] = useState<null | { token: string; phone: string; test?: string }>(null)
  const [codeErr, setCodeErr] = useState('')
  useEffect(() => { g.publicCredit.products(merchant!).then(setD).catch(e => setLoadErr(errMsg(e))) }, [merchant]) // eslint-disable-line react-hooks/exhaustive-deps
  const p: Product | undefined = d?.products[pi]
  useEffect(() => {
    if (!p) return
    const mid = Math.round((p.min + p.max) / 2 / 100000) * 100000
    setAmt(Math.min(Math.max(mid, p.min), p.max)); setN(Math.min(Math.max(Math.round((p.term_min + p.term_max) / 3), p.term_min), p.term_max))
  }, [p?.id]) // eslint-disable-line react-hooks/exhaustive-deps
  if (loadErr) return <Center><div className="display" style={{ fontSize: 24 }}>Página indisponível</div><p className="muted">{loadErr}</p></Center>
  if (!d || !p || !amt) return <Center><Spinner dark /></Center>
  const q = quote(p, amt, n, day)
  const step$ = p.max - p.min >= 2000000 ? 100000 : 50000
  const income = parseMT(f.income)
  const eff = effort(q.inst, p.freq, income)
  const asksIncome = p.fields.includes('income'), asksID = p.fields.includes('id_doc')
  const effOk = !asksIncome || (income > 0 && eff <= p.effort_pct)
  const custom = p.fields.filter(x => !['id_doc', 'activity', 'income', 'purpose'].includes(x))
  const fe = (k: string) => (err?.param === k ? ' err' : '')
  const digits = f.phone.replace(/\D/g, '').replace(/^258/, '')
  const wallet = digits.length >= 2 ? (/^8[67]/.test(digits) ? 'e-Mola' : 'M-Pesa') : ''

  const send = async () => {
    if (asksIncome && income > 0 && !effOk) { setStep('sim'); return }
    if (asksID && (!front || !back)) { setErr({ code: 'parameter_invalid', param: 'photo', message: 'Junte a foto do BI, frente e verso.', status: 400 }); return }
    setBusy(true); setErr(null)
    try {
      const r = await g.publicCredit.request(merchant!, { product_id: p.id, amount: amt, term: n, day, name: f.name, phone: digits, id_number: f.id, activity: f.activity, purpose: f.purpose, income, extra })
      if (front) await g.publicCredit.document(r.token, 'id_front', front)
      if (back) await g.publicCredit.document(r.token, 'id_back', back)
      setSheet({ token: r.token, phone: r.phone, test: r.test_code })
    } catch (e) { setErr(e as ApiError) } finally { setBusy(false) }
  }
  const confirm = async (code: string) => {
    if (!sheet) return
    setBusy(true); setCodeErr('')
    try { await g.publicCredit.confirm(sheet.token, code); nav('/cr/' + sheet.token, { replace: true }) } catch (e) { setCodeErr(errMsg(e)) } finally { setBusy(false) }
  }
  const resend = async () => { if (!sheet) return; try { const r = await g.publicCredit.requestCode(sheet.token); setSheet({ ...sheet, test: r.test_code }); setCodeErr('') } catch (e) { setCodeErr(errMsg(e)) } }

  if (step === 'data') {
    return (
      <PayFrame test={d.test_mode}>
        <button className="icon-btn" onClick={() => setStep('sim')} aria-label="Voltar" style={{ marginTop: 14 }}><Icon name="arrowLeft" size={18} /></button>
        <Head m={d.merchant} sub={p.name} />
        <h1 className="display" style={{ fontSize: 26, margin: '20px 0 14px' }}>Sobre si</h1>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
          <label><span className="label">Nome completo</span><input className={'input lg' + fe('name')} value={f.name} autoComplete="name" onChange={e => setF({ ...f, name: e.target.value })} /></label>
          <label><span className="label">Número para receber e pagar</span><div className={'phone-input' + fe('phone')}><span className="cc">+258</span><input inputMode="tel" autoComplete="tel-national" placeholder="84 123 4567" value={f.phone} onChange={e => setF({ ...f, phone: e.target.value })} />{wallet && <span className="pill" style={{ marginRight: 10, background: wallet === 'M-Pesa' ? '#FDE7E6' : '#FFF1C2', color: wallet === 'M-Pesa' ? '#C2080D' : '#6B5200' }}>{wallet}</span>}</div></label>
          {asksID && (
            <div className="grid-2" style={{ gap: 10 }}>
              <label><span className="label">Número do BI</span><input className={'input lg mono' + fe('id_number')} value={f.id} onChange={e => setF({ ...f, id: e.target.value.toUpperCase() })} placeholder="110100456789B" /></label>
              <div><span className="label">Foto do BI</span>
                <div style={{ display: 'flex', gap: 6 }}>
                  {[['Frente', front, setFront], ['Verso', back, setBack]].map(([l, file, set]: any) => (
                    <label key={l} style={{ flex: 1, height: 54, borderRadius: 14, display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 6, fontSize: 13.5, fontWeight: 600, cursor: 'pointer', background: file ? 'var(--green-bg)' : '#fff', color: file ? 'var(--green-dark)' : 'var(--ink)', border: err?.param === 'photo' && !file ? '1px solid var(--red-dark)' : file ? '0' : '1px dashed var(--line-2)' }}>
                      {file ? <Icon name="check" size={15} stroke={2.6} /> : <Icon name="upload" size={15} />}{l}
                      <input type="file" accept="image/jpeg,image/png,application/pdf" capture="environment" style={{ display: 'none' }} onChange={e => set(e.target.files?.[0] || null)} />
                    </label>
                  ))}
                </div>
              </div>
            </div>
          )}
          {(p.fields.includes('activity') || p.fields.includes('purpose')) && (
            <div className="grid-2" style={{ gap: 10 }}>
              {p.fields.includes('activity') && <label><span className="label">O que faz</span><input className={'input lg' + fe('activity')} value={f.activity} onChange={e => setF({ ...f, activity: e.target.value })} placeholder="Vendo capulanas" /></label>}
              {p.fields.includes('purpose') && <label><span className="label">Para que é</span><input className={'input lg' + fe('purpose')} value={f.purpose} onChange={e => setF({ ...f, purpose: e.target.value })} placeholder="Comprar stock" /></label>}
            </div>
          )}
          {custom.map(c => <label key={c}><span className="label">{c}</span><input className={'input lg' + fe('extra.' + c)} value={extra[c] || ''} onChange={e => setExtra({ ...extra, [c]: e.target.value })} /></label>)}
          {asksIncome && (
            <>
              <label><span className="label">Quanto ganha por mês</span><div className={'phone-input' + fe('income')} style={{ border: '2px solid var(--ink)' }}><input inputMode="numeric" value={f.income} onChange={e => setF({ ...f, income: digitsMT(e.target.value) })} /><span style={{ padding: '0 14px', color: 'var(--muted)' }}>MT</span></div></label>
              <div style={{ display: 'flex', gap: 10, padding: '12px 14px', borderRadius: 14, background: !income ? '#EEEDE8' : effOk ? '#E2F3DE' : '#FDE7E6', color: !income ? '#3E4742' : effOk ? '#0A4F06' : '#8A0A0E', fontSize: 13.5, lineHeight: 1.45 }}>
                <span style={{ width: 8, height: 8, borderRadius: '50%', marginTop: 6, flex: 'none', background: !income ? '#8A938E' : effOk ? 'var(--green)' : 'var(--red-dark)' }} />
                <span>{!income ? `Diga quanto ganha para a ${d.merchant.name} saber se a prestação cabe no seu mês.` : effOk ? `A prestação é ${eff}% do que ganha por mês. Cabe no limite de ${p.effort_pct}%.` : `A prestação é ${eff}% do que ganha por mês. O limite é ${p.effort_pct}%. Peça menos dinheiro ou escolha mais ${unit(p.freq, 2).split(' ')[1]}.`}</span>
              </div>
            </>
          )}
        </div>
        {err && <div className="form-err" style={{ marginTop: 14 }} role="alert">{err.message}</div>}
        <div style={{ flex: 1, minHeight: 20 }} />
        <p className="muted" style={{ fontSize: 12.5, textAlign: 'center', margin: '18px 0 8px' }}>Só a {d.merchant.name} vê os seus dados.</p>
        <Cta busy={busy} tone={effOk || !income ? 'green' : 'ink'} disabled={!f.name.trim() || digits.length !== 9 || (asksIncome && !income)} onClick={send}>{effOk ? `Enviar pedido · ${mt(amt)} MT` : 'Mudar montante ou prazo'}</Cta>
        {sheet && <CodeSheet phone={sheet.phone} testCode={sheet.test} busy={busy} err={codeErr} onSubmit={confirm} onResend={resend} onClose={() => setSheet(null)}
          note={<>O código confirma que este número é seu. O pedido só chega à {d.merchant.name} depois de confirmar.</>} />}
      </PayFrame>
    )
  }

  return (
    <PayFrame test={d.test_mode}>
      <Head m={d.merchant} sub={p.name} />
      {d.products.length > 1 && <div style={{ display: 'flex', gap: 8, marginTop: 14, overflowX: 'auto', paddingBottom: 2 }}>{d.products.map((x: Product, i: number) => <button key={x.id} className={'btn btn-sm ' + (i === pi ? 'btn-ink' : 'btn-outline')} style={{ height: 36, borderRadius: 999, flex: 'none' }} onClick={() => setPi(i)}>{x.name}</button>)}</div>}
      <h1 className="display" style={{ fontSize: 24, margin: '20px 0 0' }}>Quanto precisa?</h1>
      <div className="display tnum" style={{ fontSize: 48, letterSpacing: '-0.04em', marginTop: 4 }}>{mt(amt)} <span style={{ fontSize: 20, color: 'var(--muted)' }}>MT</span></div>
      <input type="range" min={p.min} max={p.max} step={step$} value={amt} onChange={e => setAmt(Number(e.target.value))} style={{ width: '100%', accentColor: 'var(--green)', marginTop: 8 }} aria-label="Montante" />
      <div className="between mono muted" style={{ fontSize: 11.5 }}><span>{mt(p.min)}</span><span>{mt(p.max)}</span></div>
      <div className="between" style={{ fontSize: 13.5, margin: '16px 0 6px' }}><span>Em quanto tempo paga</span><span className="muted">de {p.term_min} a {unit(p.freq, p.term_max)}</span></div>
      <Stepper label={unit(p.freq, n)} canDec={n > p.term_min} canInc={n < p.term_max} onDec={() => setN(n - 1)} onInc={() => setN(n + 1)} />
      {p.client_day && <><div className="between" style={{ fontSize: 13.5, margin: '14px 0 6px' }}><span>Dia de pagamento</span><span className="muted">Por exemplo, o dia do salário</span></div><Stepper label={`Dia ${day} de cada mês`} canDec={day > 1} canInc={day < 28} onDec={() => setDay(day - 1)} onInc={() => setDay(day + 1)} /></>}
      <div className="card" style={{ marginTop: 14, padding: 18 }}>
        <div style={{ fontSize: 13, color: 'var(--muted)' }}>Paga {PPM_WORD[p.freq]}</div>
        <div><span className="display tnum" style={{ fontSize: 30 }}>{mt(q.inst)}</span> <span className="muted" style={{ fontSize: 13 }}>MT × {n}</span></div>
        <div style={{ borderTop: '1px dashed #E1DFD7', margin: '12px 0 6px' }} />
        <Row k="Recebe" v={`${mt(amt)} MT`} />
        <Row k={`Juros · ${rateLabel(p.rate_bp, p.freq)}`} v={`${mt(q.interest)} MT`} />
        <Row k="Total a pagar" v={`${mt(q.total)} MT`} strong />
        <Row k="Datas" v={n === 1 ? dly(q.dates[0]) : `${dl(q.dates[0])} a ${dly(q.dates[n - 1])}`} />
      </div>
      <div style={{ flex: 1, minHeight: 20 }} />
      <div style={{ marginTop: 18 }}><Cta onClick={() => setStep('data')}>Pedir {mt(amt)} MT</Cta></div>
      <p className="muted" style={{ fontSize: 12.5, textAlign: 'center', margin: '10px 0 0' }}>Sem outras taxas. {prest(n)} {adjPlural(p.freq, n)} de {mt(q.inst)} MT. Resposta por SMS.</p>
    </PayFrame>
  )
}
