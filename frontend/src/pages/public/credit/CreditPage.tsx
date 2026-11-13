import { useEffect, useRef, useState } from 'react'
import { useParams } from 'react-router-dom'
import { Icon, MethodIcon, Spinner, Stripes } from '../../../components/ui'
import { useGateway } from '../../../core/gateway'
import { errMsg } from '../../../core/model'
import { adjPlural, asDate, dl, dly, mt, pct, planRows, prest, unit, whenText, type Freq } from '../../../lib/credit'
import { walletPrompt } from '../../../lib/walletPrompt'
import { Center, PayFrame } from '../subs/PayFrame'
import { CodeSheet, Cta, Done, Head, Row, Talk } from './parts'

const WALLET_KEY: Record<string, string> = { 'M-Pesa': 'mpesa', 'e-Mola': 'emola' }
const cents = (c: number) => mt(c) + ',' + String(Math.abs(c) % 100).padStart(2, '0')

export default function CreditPage() {
  const { token } = useParams()
  const g = useGateway()
  const [d, setD] = useState<any>(null)
  const [loadErr, setLoadErr] = useState('')
  const [view, setView] = useState<'' | 'pay' | 'confirm' | 'paid' | 'sent'>('')
  const [payAll, setPayAll] = useState(false)
  const [ok, setOk] = useState(false)
  const [sheet, setSheet] = useState<null | { test?: string }>(null)
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')
  const [again, setAgain] = useState(false)
  const paidMsg = useRef('')
  const load = async (opened = false) => {
    try { const x = await g.publicCredit.page(token!, opened); setD(x); return x } catch (e) { setLoadErr(errMsg(e)); return null }
  }
  useEffect(() => { load(true).then(x => { if (x?.payment?.status === 'processing') setView('confirm') }) }, [token]) // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => {
    const pending = view === 'confirm' || (d?.kind === 'request' && ['new', 'review'].includes(d?.request?.status)) || (d?.loan?.status === 'offered') || d?.loan?.status === 'disbursing'
    if (!pending) return
    const t = setInterval(async () => {
      const x = await load()
      if (!x || view !== 'confirm' || !x.payment) return
      if (x.payment.status === 'succeeded') setView('paid')
      else if (x.payment.status === 'failed') { setView('pay'); setErr(x.payment.failure_message || 'O pagamento falhou.') }
    }, view === 'confirm' ? 1500 : 6000)
    return () => clearInterval(t)
  }, [view, d?.kind, d?.request?.status, d?.loan?.status]) // eslint-disable-line react-hooks/exhaustive-deps

  if (loadErr) return <Center><div className="display" style={{ fontSize: 24 }}>Link indisponível</div><p className="muted">{loadErr}</p></Center>
  if (!d) return <Center><Spinner dark /></Center>
  const m = d.merchant, l = d.loan, r = d.request, p = d.product
  const who = (l?.name || r?.name || '').trim()
  const freq: Freq = l?.freq || p?.freq || 'mensal'

  const askCode = async () => {
    setBusy(true); setErr('')
    try { const x = await g.publicCredit.agreementCode(token!, true); setSheet({ test: x.test_code }) } catch (e) { setErr(errMsg(e)) } finally { setBusy(false) }
  }
  const accept = async (code: string) => {
    setBusy(true); setErr('')
    try { const x = await g.publicCredit.accept(token!, code); setD(x); setSheet(null); setOk(false); if (x.loan.version === 1) setView('sent') } catch (e) { setErr(errMsg(e)) } finally { setBusy(false) }
  }
  const pay = async () => {
    setBusy(true); setErr('')
    try {
      const before = l.next
      const x = await g.publicCredit.pay(token!, { all: payAll, pay_again: again })
      setD(x)
      paidMsg.current = payAll ? 'all' : before ? `Prestação ${before.seq} de ${l.plan.length}` : ''
      if (x.payment?.status === 'processing') setView('confirm')
      else if (x.payment?.status === 'succeeded') setView('paid')
      else if (x.payment?.status === 'failed') setErr(x.payment.failure_message || 'O pagamento falhou.')
    } catch (e: any) {
      if (e?.code === 'payment_unconfirmed') setAgain(true)
      setErr(errMsg(e))
    } finally { setBusy(false) }
  }
  const ask = async () => { setBusy(true); try { setD(await g.publicCredit.ask(token!)) } catch (e) { setErr(errMsg(e)) } finally { setBusy(false) } }

  // Request, before a decision.
  if (d.kind === 'request') {
    const st = r.status
    if (st === 'declined' || st === 'canceled') {
      return <PayFrame test={d.test_mode}><Head m={m} sub={`Pedido de ${who}`} /><h1 className="display" style={{ fontSize: 26, margin: '22px 0 8px' }}>Pedido não aprovado</h1><p style={{ color: 'var(--text-2)', lineHeight: 1.5 }}>A {m.name} analisou o seu pedido de {mt(r.amount)} MT e não o pode aprovar agora.</p><div style={{ marginTop: 16 }}><Talk contact={d.contact} name={m.name} /></div></PayFrame>
    }
    return (
      <PayFrame test={d.test_mode}>
        <Head m={m} sub={`Pedido de ${who}`} />
        <h1 className="display" style={{ fontSize: 28, margin: '22px 0 6px' }}>Pedido enviado</h1>
        <p style={{ color: 'var(--text-2)', lineHeight: 1.5, margin: 0 }}>A {m.name} está a analisar. Normalmente responde em 1 dia útil.</p>
        <Timeline steps={[
          ['done', 'Pedido enviado', `${dl(asDate(r.created_at))}, ${asDate(r.created_at).toLocaleTimeString('pt-PT', { hour: '2-digit', minute: '2-digit' })} · ${mt(r.amount)} MT em ${prest(r.term)} ${adjPlural(freq, r.term)}`],
          ['now', `Em análise pela ${m.name.split(' ')[0]}`, 'Podem ligar para confirmar os dados'],
          ['todo', 'Acordo de pagamento', 'Lê e aceita com um código SMS'],
          ['todo', `Dinheiro no ${r.wallet}`, r.phone],
        ]} />
        <div style={{ flex: 1, minHeight: 20 }} />
        <div style={{ display: 'flex', gap: 10, padding: '12px 14px', borderRadius: 14, background: 'var(--green-bg)', color: 'var(--green-dark)', fontSize: 13.5, lineHeight: 1.45, marginTop: 20 }}><Icon name="sms" size={16} />Pode fechar esta página. Avisamos por SMS no {r.phone} quando houver resposta.</div>
        {d.contact && <a className="btn btn-outline btn-lg btn-block" style={{ marginTop: 10 }} href={`https://wa.me/258${d.contact}`} target="_blank" rel="noreferrer">Falar com a {m.name.split(' ')[0]} no WhatsApp</a>}
      </PayFrame>
    )
  }

  // Waiting for the second approver.
  if (l.status === 'approving') {
    return <PayFrame test={d.test_mode}><Head m={m} sub={`Pedido de ${who}`} /><h1 className="display" style={{ fontSize: 26, margin: '22px 0 8px' }}>Quase aprovado</h1><p style={{ color: 'var(--text-2)', lineHeight: 1.5 }}>Falta uma segunda aprovação na {m.name}. Recebe o acordo por SMS logo a seguir.</p></PayFrame>
  }

  // The agreement, first or a new one.
  const g1 = l.proposal
  if (g1 && (l.status === 'offered' || (l.status === 'active' && g1.version > 1))) {
    const dates: Date[] = g1.dates.map(asDate)
    const amounts: number[] = g1.amounts
    const same = amounts.every(a => a === amounts[0])
    const rows = planRows(dates.map((dt, i) => ({ i: String(i + 1), d: g1.version === 1 ? dly(dt) : dl(dt), v: mt(amounts[i]) })), h => ({ i: '⋯', d: `mais ${h} prestações`, v: mt(amounts[3]) + ' cada' }))
    const sheetEl = sheet && <CodeSheet phone={l.phone} testCode={sheet.test} busy={busy} err={err} onSubmit={accept} onResend={askCode} onClose={() => setSheet(null)}
      note={g1.version === 1 ? `Ao introduzir o código, aceita o acordo: ${prest(amounts.length)} de ${mt(amounts[0])} MT, total ${mt(g1.total)} MT.` : `Ao introduzir o código, aceita o novo acordo: ${prest(amounts.length)} de ${mt(amounts[0])} MT.`} />
    if (g1.version > 1) {
      const open = l.plan.filter((x: any) => x.status === 'late' || x.status === 'scheduled')
      const late = open.filter((x: any) => x.status === 'late')
      const cur = `${late.length ? `${mt(late.reduce((a: number, x: any) => a + x.amount + x.mora, 0))} MT em atraso, mais ` : ''}${prest(open.length - late.length)} de ${mt(open[open.length - 1]?.amount || 0)} MT${l.freq === 'mensal' ? ` no dia ${asDate(open[0]?.due || new Date().toISOString()).getDate()}` : ''}`
      return (
        <PayFrame test={d.test_mode}>
          <Head m={m} sub={`Crédito de ${who}`} />
          <h1 className="display" style={{ fontSize: 26, margin: '20px 0 4px' }}>Novo acordo de pagamento</h1>
          <p className="muted" style={{ margin: 0, fontSize: 14 }}>A {m.name} propõe mudar o seu plano.</p>
          <div style={{ marginTop: 16, padding: 16, borderRadius: 18, border: '1px dashed #D6D4CB' }}><div className="muted" style={{ fontSize: 13 }}>Plano atual</div><div style={{ fontSize: 13.5, color: 'var(--muted)', textDecoration: 'line-through', marginTop: 4 }}>{cur}</div></div>
          <div className="card" style={{ marginTop: 10, padding: 18, border: '2px solid var(--ink)' }}>
            <div className="muted" style={{ fontSize: 13 }}>Novo plano</div>
            <b style={{ fontSize: 17 }}>{same ? `${prest(amounts.length)} de ${mt(amounts[0])} MT` : `${prest(amounts.length)}, ${mt(g1.total)} MT`}</b>
            <div style={{ marginTop: 8 }}>{rows.map((x, i) => <div key={i} style={{ display: 'grid', gridTemplateColumns: '20px 1fr auto', fontSize: 14, padding: '4px 0' }}><span className="muted">{x.i}</span><span>{x.d}</span><span className="tnum">{x.v}</span></div>)}</div>
            <div style={{ borderTop: '1px solid var(--line-3)', marginTop: 10, paddingTop: 10 }}><Row k="Total" v={`${mt(g1.total)} MT`} strong />{g1.mora_waived > 0 && <div style={{ color: 'var(--green-text)', fontSize: 13, fontWeight: 600 }}>Mora de {mt(g1.mora_waived)} MT perdoada.</div>}</div>
          </div>
          <div style={{ flex: 1, minHeight: 20 }} />
          {err && !sheet && <div className="form-err" style={{ marginBottom: 10 }}>{err}</div>}
          <Cta busy={busy} onClick={askCode}>Aceitar com código SMS</Cta>
          <p className="muted" style={{ fontSize: 12.5, textAlign: 'center' }}>Se não aceitar, o plano atual continua. <Talk contact={d.contact} name={m.name.split(' ')[0]} /></p>
          {sheetEl}
        </PayFrame>
      )
    }
    return (
      <PayFrame test={d.test_mode}>
        <Head m={m} sub={`Pedido de ${who}`} />
        <h1 className="display" style={{ fontSize: 26, margin: '20px 0 14px', lineHeight: 1.15 }}>Pedido aprovado. Este é o seu acordo.</h1>
        <div className="card" style={{ padding: 18 }}>
          <div className="muted" style={{ fontSize: 13 }}>Recebe</div>
          <div className="display tnum" style={{ fontSize: 32 }}>{mt(l.amount)} <span style={{ fontSize: 14, color: 'var(--muted)' }}>MT</span></div>
          <div className="muted" style={{ fontSize: 13 }}>{l.delivery === 'riha' ? `No ${l.wallet} ${l.phone}, logo depois de aceitar` : `A ${m.name} entrega-lhe depois de aceitar`}</div>
          <div style={{ borderTop: '1px dashed #E1DFD7', margin: '14px 0 10px' }} />
          <div className="muted" style={{ fontSize: 13 }}>Paga</div>
          <b style={{ fontSize: 16 }}>{prest(amounts.length)} {adjPlural(freq, amounts.length)} de {mt(amounts[0])} MT</b>
          <div style={{ marginTop: 8 }}>{rows.map((x, i) => <div key={i} style={{ display: 'grid', gridTemplateColumns: '20px 1fr auto', fontSize: 14, padding: '4px 0' }}><span className="muted">{x.i}</span><span>{x.d}</span><span className="tnum">{x.v}</span></div>)}</div>
          <div style={{ borderTop: '1px solid var(--line-3)', marginTop: 10, paddingTop: 10 }}><Row k="Total a pagar" v={<><b>{mt(g1.total)} MT</b> <span className="muted" style={{ fontSize: 12.5 }}>· juros {mt(g1.total - l.amount)}</span></>} strong /></div>
        </div>
        <div style={{ fontSize: 13.5, color: 'var(--text-2)', lineHeight: 1.5, marginTop: 12 }}>
          <p style={{ margin: '0 0 8px' }}><b>Como paga.</b> Em cada data recebe um SMS com o link e confirma com o seu PIN. Nada sai da conta sem confirmar.</p>
          {l.mora_bp > 0 && <p style={{ margin: '0 0 8px' }}><b>Se atrasar.</b> {l.grace ? `Depois de ${l.grace} ${l.grace === 1 ? 'dia' : 'dias'}, m` : 'M'}ora de {pct(l.mora_bp)}% por semana sobre a prestação em atraso ({mt(Math.round(amounts[0] * l.mora_bp / 10000 / 100) * 100)} MT), até {unit('semanal', l.mora_max)}.</p>}
          <a className="link-btn" href={g.publicCredit.agreementUrl(token!)} target="_blank" rel="noreferrer">Ler o acordo completo · PDF</a>
        </div>
        <div style={{ flex: 1, minHeight: 16 }} />
        <label style={{ display: 'flex', gap: 12, alignItems: 'flex-start', padding: '14px 0', borderTop: '1px solid var(--line-3)', marginTop: 14, cursor: 'pointer' }}>
          <input type="checkbox" checked={ok} onChange={e => setOk(e.target.checked)} style={{ width: 22, height: 22, accentColor: 'var(--green)', flex: 'none' }} />
          <span style={{ fontSize: 13.5, lineHeight: 1.45 }}>Li e aceito o acordo de pagamento com a {m.name}.</span>
        </label>
        {err && !sheet && <div className="form-err" style={{ marginBottom: 10 }}>{err}</div>}
        <Cta busy={busy} disabled={!ok} onClick={askCode}>Aceitar com código SMS</Cta>
        {sheetEl}
      </PayFrame>
    )
  }

  if (l.status === 'canceled') return <PayFrame test={d.test_mode}><Head m={m} sub={`Pedido de ${who}`} /><h1 className="display" style={{ fontSize: 26, margin: '22px 0 8px' }}>Acordo cancelado</h1><p style={{ color: 'var(--text-2)' }}>Este acordo já não pode ser aceite. Fale com a {m.name}.</p><Talk contact={d.contact} name={m.name} /></PayFrame>
  if (l.status === 'written_off') return <PayFrame test={d.test_mode}><Head m={m} sub={`Crédito de ${who}`} /><h1 className="display" style={{ fontSize: 26, margin: '22px 0 8px' }}>Crédito fechado</h1><p style={{ color: 'var(--text-2)' }}>Fale com a {m.name} sobre este crédito.</p><Talk contact={d.contact} name={m.name} /></PayFrame>

  const plan: any[] = l.plan
  const paidN = plan.filter(x => x.status === 'paid' || x.status === 'paid_outside').length
  const next = l.next
  const firstDue = plan[0] ? asDate(plan[0].due) : null

  // Money just sent, or still on its way.
  if (view === 'sent' || l.status === 'disbursing') {
    const sending = l.status === 'disbursing'
    return (
      <PayFrame test={d.test_mode}>
        <Head m={m} sub={`Pedido de ${who}`} />
        {sending ? (
          <div style={{ marginTop: 26 }}><Spinner dark /><h1 className="display" style={{ fontSize: 26, margin: '18px 0 6px' }}>A enviar {mt(l.amount)} MT</h1><p style={{ color: 'var(--text-2)', lineHeight: 1.5 }}>{l.delivery_status === 'failed' ? `A entrega para o ${l.wallet} ainda não foi possível. A ${m.name} já foi avisada e vai tentar de novo.` : `Para o ${l.wallet} ${l.phone}. Recebe um SMS do ${l.wallet} quando chegar.`}</p></div>
        ) : (
          <Done title={l.delivery === 'riha' ? `${mt(l.amount)} MT enviados` : 'Acordo aceite'} text={l.delivery === 'riha' ? `Para o ${l.wallet} ${l.phone}. O ${l.wallet} envia um SMS a confirmar a entrada.` : `A ${m.name} entrega-lhe os ${mt(l.amount)} MT.`} />
        )}
        <div className="card" style={{ marginTop: 22, padding: '6px 18px' }}>
          <Row k="Acordo aceite" v={l.agreement?.accepted_at ? `${dly(asDate(l.agreement.accepted_at))}, ${asDate(l.agreement.accepted_at).toLocaleTimeString('pt-PT', { hour: '2-digit', minute: '2-digit' })}` : '—'} />
          <Row k="Referência" v={<span className="mono">{l.reference}</span>} />
          {firstDue && <Row k="Primeira prestação" v={`${dl(firstDue)} · ${mt(plan[0].amount)} MT`} />}
        </div>
        {firstDue && <div className="card" style={{ marginTop: 10, padding: 16, fontSize: 13.5, color: 'var(--text-2)', lineHeight: 1.5 }}>Antes de cada data enviamos um lembrete. A {dl(firstDue)} enviamos o link para pagar com o PIN.</div>}
        <div style={{ flex: 1, minHeight: 20 }} />
        <Cta tone="ink" onClick={() => setView('')}>Ver o meu crédito</Cta>
        <a className="btn btn-ghost btn-block" style={{ textDecoration: 'underline' }} href={g.publicCredit.agreementUrl(token!)} target="_blank" rel="noreferrer">Guardar o acordo em PDF</a>
      </PayFrame>
    )
  }

  if (l.status === 'settled') {
    return (
      <PayFrame test={d.test_mode}>
        <Head m={m} sub={`Crédito de ${who}`} />
        <Done title="Crédito liquidado" text={`Pagou tudo à ${m.name}. Obrigado.`} />
        <div className="card" style={{ marginTop: 22, padding: '6px 18px' }}><Row k="Referência" v={<span className="mono">{l.reference}</span>} /><Row k="Prestações" v={`${paidN} de ${plan.length}`} /></div>
        <div style={{ flex: 1, minHeight: 20 }} />
        <a className="btn btn-outline btn-lg btn-block" href={g.publicCredit.agreementUrl(token!)} target="_blank" rel="noreferrer">Acordo em PDF</a>
      </PayFrame>
    )
  }

  if (view === 'confirm' && d.payment) {
    const wp = walletPrompt(WALLET_KEY[d.payment.method] || 'mpesa', m.name)
    return (
      <PayFrame test={d.test_mode}>
        <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', textAlign: 'center', paddingTop: 40 }}>
          <div style={{ width: 220, padding: 16, borderRadius: 16, background: '#2B2F2D', color: '#fff', textAlign: 'left' }}><div style={{ fontSize: 12, lineHeight: 1.5 }}>{wp.screen(cents(d.payment.amount))}</div></div>
          <h1 className="display" style={{ margin: '26px 0 0', fontSize: 26 }}>{wp.title}</h1>
          <p style={{ margin: '8px 0 0', fontSize: 15, color: 'var(--muted)', lineHeight: 1.5 }}>{wp.steps(<b>{l.phone}</b>)}</p>
          <p className="muted" style={{ fontSize: 13, marginTop: 16 }}>O dinheiro só sai quando o {d.payment.method} confirmar. Enviamos o recibo por SMS.</p>
          <button className="btn btn-ghost" style={{ marginTop: 10 }} onClick={() => setView('pay')}>Voltar</button>
        </div>
      </PayFrame>
    )
  }

  if (view === 'paid') {
    const left = plan.filter(x => x.status === 'scheduled' || x.status === 'late')
    return (
      <PayFrame test={d.test_mode}>
        <Head m={m} sub={`Crédito de ${who}`} />
        <Done title="Pago" text={<>{paidMsg.current === 'all' ? 'O crédito está liquidado.' : `${paidMsg.current} paga. ${left.length === 0 ? 'O crédito está liquidado.' : left.length === 1 ? `Falta 1 prestação de ${mt(left[0].amount)} MT.` : `Faltam ${left.length} prestações, ${mt(l.debt)} MT.`}`} O recibo foi enviado por SMS.</>} />
        {d.payment && <div className="card" style={{ marginTop: 22, padding: '6px 18px' }}><Row k="Valor" v={`${cents(d.payment.amount)} MT`} /><Row k="Carteira" v={`${d.payment.method} · ${l.phone}`} /><Row k="Referência" v={<span className="mono">{d.payment.reference}</span>} /></div>}
        <div style={{ flex: 1, minHeight: 20 }} />
        <Cta tone="ink" onClick={() => setView('')}>Ver o meu crédito</Cta>
      </PayFrame>
    )
  }

  if (view === 'pay' && next) {
    const amount = payAll ? (l.settle_offer || l.debt) : next.amount + next.mora
    return (
      <PayFrame test={d.test_mode}>
        <button className="icon-btn" onClick={() => setView('')} aria-label="Voltar" style={{ marginTop: 14 }}><Icon name="arrowLeft" size={18} /></button>
        <Head m={m} sub={`Crédito de ${who}`} />
        <div className="card" style={{ marginTop: 16, padding: 18 }}>
          <div className="between"><span>{payAll ? 'Tudo o que falta' : `Prestação ${next.seq} de ${plan.length}`}</span>{!payAll && <span className="pill" style={{ background: 'var(--paper-2)' }}>vence {dl(asDate(next.due))}</span>}</div>
          <div style={{ borderTop: '1px dashed #E1DFD7', margin: '12px 0' }} />
          <div className="between"><span className="muted">Total</span><span className="display tnum" style={{ fontSize: 32 }}>{cents(amount)} <span style={{ fontSize: 14, color: 'var(--muted)' }}>MT</span></span></div>
          {!payAll && next.mora > 0 && <div className="muted" style={{ fontSize: 13, textAlign: 'right' }}>{mt(next.amount)} MT da prestação e {mt(next.mora)} MT de mora</div>}
          {payAll && l.settle_offer > 0 && <div style={{ fontSize: 13, textAlign: 'right', color: 'var(--green-text)' }}>Proposta da {m.name}: menos {mt(l.debt - l.settle_offer)} MT</div>}
        </div>
        <div style={{ fontWeight: 700, fontSize: 16, marginTop: 22 }}>Pagar com</div>
        <div className="card" style={{ marginTop: 10, padding: '14px 16px', display: 'flex', gap: 12, alignItems: 'center', border: '2px solid var(--ink)' }}>
          <MethodIcon method={WALLET_KEY[l.wallet] || 'mpesa'} /><span style={{ flex: 1 }}><b style={{ display: 'block' }}>{l.wallet}</b><span style={{ fontSize: 12.5, color: 'var(--muted)' }}>{l.phone} · o número do acordo</span></span><span style={{ width: 20, height: 20, borderRadius: '50%', border: '6px solid var(--ink)' }} />
        </div>
        {err && <div className="form-err" style={{ marginTop: 14 }} role="alert">{err}</div>}
        <div style={{ flex: 1, minHeight: 20 }} />
        <Cta busy={busy} onClick={pay}>{again ? 'Pagar de novo' : 'Pagar'} {cents(amount)} MT</Cta>
        {!payAll && plan.filter(x => x.status === 'scheduled' || x.status === 'late').length > 1 && <button className="btn btn-ghost btn-block" style={{ textDecoration: 'underline', fontSize: 13.5 }} onClick={() => setPayAll(true)}>Pagar tudo o que falta · {cents(l.settle_offer || l.debt)} MT</button>}
        {payAll && <button className="btn btn-ghost btn-block" style={{ fontSize: 13.5 }} onClick={() => setPayAll(false)}>Pagar só a próxima prestação</button>}
      </PayFrame>
    )
  }

  const schedule = (
    <div className="card" style={{ marginTop: 10, overflow: 'hidden' }}>
      {plan.map((x: any) => (
        <div key={x.id} className="between" style={{ padding: '11px 16px', borderBottom: '1px solid var(--line-3)', fontSize: 14, background: x.status === 'late' ? '#FFFBEA' : undefined }}>
          <span><span className="muted" style={{ display: 'inline-block', width: 22 }}>{x.seq}</span><b style={{ fontWeight: x.id === next?.id ? 700 : 500 }}>{dl(asDate(x.due))}</b><span style={{ fontSize: 12.5, color: x.status === 'paid' || x.status === 'paid_outside' ? 'var(--green-text)' : x.status === 'late' ? 'var(--sun-text)' : 'var(--muted)' }}> · {x.status === 'paid' ? `paga, ${x.method}` : x.status === 'paid_outside' ? 'paga' : x.status === 'late' ? 'em atraso' : x.id === next?.id ? 'próxima' : 'agendada'}</span></span>
          <span className="tnum" style={{ color: x.status === 'paid' || x.status === 'paid_outside' ? 'var(--muted)' : undefined }}>{mt(x.amount + (x.mora || 0))}</span>
        </div>
      ))}
      <div className="between" style={{ padding: '12px 16px', fontSize: 13.5 }}><a className="link-btn" href={g.publicCredit.agreementUrl(token!)} target="_blank" rel="noreferrer">Acordo em PDF</a><Talk contact={d.contact} name={m.name.split(' ')[0]} /></div>
    </div>
  )

  // Late: pay with mora, or ask for a new agreement.
  if (next && next.status === 'late') {
    return (
      <PayFrame test={d.test_mode}>
        <Head m={m} sub={`Crédito de ${who}`} />
        <div style={{ marginTop: 16, padding: 18, borderRadius: 22, background: 'var(--sun-soft)', color: '#3F3200' }}>
          <div style={{ fontSize: 13.5, fontWeight: 600 }}>Prestação {next.seq} em atraso há {next.days_late} {next.days_late === 1 ? 'dia' : 'dias'}</div>
          <div className="display tnum" style={{ fontSize: 34, marginTop: 4 }}>{mt(next.amount + next.mora)} <span style={{ fontSize: 14 }}>MT</span></div>
          <div style={{ fontSize: 13 }}>{mt(next.amount)} MT da prestação{next.mora > 0 ? ` e ${mt(next.mora)} MT de mora` : ''}</div>
          <button className="btn btn-green btn-lg btn-block" style={{ marginTop: 14 }} onClick={() => { setPayAll(false); setView('pay') }}>Pagar {mt(next.amount + next.mora)} MT</button>
          {l.asked ? <div style={{ marginTop: 10, fontSize: 13.5, lineHeight: 1.45 }}><b>Pedido de novo acordo enviado.</b> A {m.name} responde por SMS. A mora continua a contar até lá.</div>
            : <button className="btn btn-block" style={{ marginTop: 8, background: 'transparent', border: '1px solid #C9B25A', color: '#3F3200', height: 48 }} disabled={busy} onClick={ask}>Não consigo pagar agora</button>}
        </div>
        <div className="between" style={{ marginTop: 18 }}><b>Falta pagar {mt(l.debt)} MT</b><span className="muted" style={{ fontSize: 13 }}>{paidN} de {plan.length} pagas</span></div>
        {schedule}
      </PayFrame>
    )
  }

  return (
    <PayFrame test={d.test_mode}>
      <Head m={m} sub={`Crédito de ${who}`} />
      {l.delivery === 'outside' && l.delivery_status === 'to_deliver' && <div style={{ marginTop: 14, padding: '12px 14px', borderRadius: 14, background: 'var(--green-bg)', color: 'var(--green-dark)', fontSize: 13.5 }}>Acordo aceite. A {m.name} entrega-lhe os {mt(l.amount)} MT.</div>}
      <div style={{ position: 'relative', overflow: 'hidden', marginTop: 16, padding: '18px 20px', borderRadius: 22, background: 'var(--ink)', color: '#fff' }}>
        <Stripes top={-40} right={-10} w={30} h={260} gap={9} />
        <div style={{ position: 'relative' }}>
          <div style={{ fontSize: 13, color: '#B9C3BD' }}>Falta pagar</div>
          <div className="display tnum" style={{ fontSize: 36 }}>{mt(l.debt)} <span style={{ fontSize: 15, color: '#93A09A' }}>MT</span></div>
          <div style={{ display: 'flex', gap: 4, marginTop: 12, maxWidth: 240 }}>{plan.map((x: any) => <span key={x.id} style={{ flex: 1, height: 5, borderRadius: 3, background: x.status === 'paid' || x.status === 'paid_outside' ? '#8EE07F' : '#3A453F' }} />)}</div>
          <div style={{ fontSize: 13, color: '#B9C3BD', marginTop: 8 }}>{paidN} de {prest(plan.length)} pagas</div>
        </div>
      </div>
      {next && (
        <div className="card" style={{ marginTop: 10, padding: 18 }}>
          <div className="between"><span className="muted" style={{ fontSize: 13 }}>Próxima · {next.seq} de {plan.length}</span><b style={{ fontSize: 13.5 }}>vence {dl(asDate(next.due))}</b></div>
          <div className="between" style={{ marginTop: 6 }}><span className="display tnum" style={{ fontSize: 28 }}>{mt(next.amount)} <span style={{ fontSize: 13, color: 'var(--muted)' }}>MT</span></span><button className="btn btn-green" onClick={() => { setPayAll(false); setView('pay') }}>Pagar agora</button></div>
          <div className="muted" style={{ fontSize: 13, marginTop: 8 }}>{l.settle_offer > 0 ? <>A {m.name} propõe liquidar com {mt(l.settle_offer)} MT. <button className="link-btn" onClick={() => { setPayAll(true); setView('pay') }}>Pagar tudo</button></> : 'Enviamos o link por SMS nesse dia. Pode pagar antes.'}</div>
        </div>
      )}
      {schedule}
      <p className="muted" style={{ fontSize: 12.5, marginTop: 12, textAlign: 'center' }}>{prest(plan.length)} {whenText(freq, firstDue?.getDate() || l.day)} · Ref. <span className="mono">{l.reference}</span></p>
    </PayFrame>
  )
}

function Timeline({ steps }: { steps: [string, string, string][] }) {
  return (
    <div className="card" style={{ marginTop: 20, padding: '18px 18px 6px' }}>
      {steps.map(([st, t, n], i) => (
        <div key={i} style={{ display: 'flex', gap: 12, position: 'relative', paddingBottom: 16 }}>
          {i < steps.length - 1 && <span style={{ position: 'absolute', left: 10, top: 24, bottom: 0, width: 2, background: st === 'done' ? 'var(--green)' : '#E4E2DA' }} />}
          <span style={{ width: 22, height: 22, borderRadius: '50%', flex: 'none', display: 'grid', placeItems: 'center', background: st === 'done' ? 'var(--green)' : '#fff', border: st === 'done' ? 0 : st === 'now' ? '2px solid var(--ink)' : '1.5px solid #C9CDC6' }}>{st === 'done' ? <Icon name="check" size={12} stroke={3} color="#fff" /> : st === 'now' ? <span style={{ width: 10, height: 10, borderRadius: '50%', background: 'var(--ink)' }} /> : null}</span>
          <div style={{ opacity: st === 'todo' ? 0.6 : 1 }}><b style={{ fontSize: 14.5 }}>{t}</b><div className="muted" style={{ fontSize: 13 }}>{n}</div></div>
        </div>
      ))}
    </div>
  )
}
