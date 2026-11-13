import { useState } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { Drawer, Icon, Modal, Skeleton, Spinner, Toggle } from '../../../components/ui'
import { useGateway } from '../../../core/gateway'
import { errMsg } from '../../../core/model'
import { useQuery } from '../../../core/useQuery'
import { asDate, digitsMT, dl, dly, evenSplit, mt, nextDates, parseMT, planRows, prest, quote, rateLabel, unit, type Product } from '../../../lib/credit'
import { useSession } from '../../../lib/session'
import { Back, INST_STATUS, KV, MT, Pips, Plan, Radio, shortWhen, Step, Tone, useMicro } from './common'
import { DeliveryChoice } from './Requests'

const COLS = 'minmax(170px,1.5fr) minmax(110px,1fr) 90px 90px 150px 70px minmax(130px,1fr) minmax(120px,1fr)'
const TABS: [string, string][] = [['ativos', 'Ativos'], ['atraso', 'Em atraso'], ['acordo', 'Novo acordo'], ['aceitar', 'Por aceitar'], ['liquidados', 'Liquidados'], ['perdas', 'Perdas']]

export default function Loans() {
  const nav = useNavigate()
  const g = useGateway()
  const mc = useMicro()
  const [sp, setSp] = useSearchParams()
  const tab = sp.get('tab') || 'ativos'
  const [product, setProduct] = useState('')
  const [q, setQ] = useState('')
  const { data } = useQuery(g => g.microcredit.loans(tab, product, q), [tab, product, q, mc.version])
  const rows: any[] = data?.data || []
  const c = data?.counts || {}
  return (
    <div>
      <div className="between" style={{ flexWrap: 'wrap', marginBottom: 14 }}>
        <div className="mc-chips">
          {TABS.filter(([k]) => k !== 'perdas' || c.perdas).map(([k, l]) => <button key={k} className={'mc-chip' + (tab === k ? ' on' : '')} onClick={() => setSp({ tab: k })}>{l} {c[k] ?? 0}</button>)}
          <select className="mc-chip" style={{ borderStyle: 'dashed' }} value={product} onChange={e => setProduct(e.target.value)} aria-label="Produto">
            <option value="">Produto ▾</option>
            {mc.products.map((p: any) => <option key={p.id} value={p.id}>{p.name}</option>)}
          </select>
        </div>
        <input className="input" style={{ width: 240, height: 38 }} placeholder="Nome ou número" value={q} onChange={e => setQ(e.target.value)} />
      </div>
      {!data ? <Skeleton h={300} r={22} /> : (
        <>
          <div className="table mc-table">
            <div className="mc-th" style={{ gridTemplateColumns: COLS }}><span>Cliente</span><span>Produto</span><span style={{ textAlign: 'right' }}>Emprestado</span><span style={{ textAlign: 'right' }}>Em dívida</span><span>Prestações</span><span>Próxima</span><span>Estado</span><span>Último pagamento</span></div>
            {rows.length === 0 && <div className="empty">Nenhum empréstimo aqui.</div>}
            {rows.map(r => (
              <div key={r.id} className="mc-row click" style={{ gridTemplateColumns: COLS, background: r.tone === 'late' ? '#FFFDF4' : r.tone === 'bad' ? '#FFF8F7' : undefined }} onClick={() => nav('/painel/microcredito/emprestimos/' + r.id)}>
                <span style={{ minWidth: 0 }}><b>{r.name}</b><div className="mc-sub">{r.phone}</div></span>
                <span>{r.product}</span>
                <span className="tnum" style={{ textAlign: 'right' }}>{mt(r.amount)}</span>
                <b className="tnum" style={{ textAlign: 'right' }}>{r.status === 'offered' || r.status === 'approving' ? '—' : mt(r.debt)}</b>
                <span style={{ display: 'flex', alignItems: 'center', gap: 8 }}><span className="mono muted" style={{ fontSize: 12 }}>{r.paid}/{r.n}</span><Pips pips={r.pips} /></span>
                <span style={{ fontSize: 13.5 }}>{r.next ? dl(asDate(r.next)) : '—'}</span>
                <span><Tone tone={r.tone}>{r.state}</Tone></span>
                <span style={{ fontSize: 13.5, color: 'var(--text-2)' }}>{r.last || '—'}</span>
              </div>
            ))}
            <div className="between" style={{ padding: '12px 20px', fontSize: 13, color: 'var(--muted)', borderTop: '1px solid var(--line-3)' }}>
              <span>{rows.length} {rows.length === 1 ? 'empréstimo' : 'empréstimos'}</span>
              <a className="link-btn" style={{ color: 'var(--muted)' }} href={g.microcredit.exportUrl('loans')}>Exportar CSV</a>
            </div>
          </div>
          <div className="mc-card-list">
            {rows.length === 0 && <div className="card empty">Nenhum empréstimo aqui.</div>}
            {rows.map(r => (
              <button key={r.id} className="card" style={{ padding: 16, textAlign: 'left', border: '1px solid var(--line)', background: r.tone === 'late' ? '#FFFDF4' : undefined }} onClick={() => nav('/painel/microcredito/emprestimos/' + r.id)}>
                <div className="between" style={{ alignItems: 'flex-start' }}><span><b>{r.name}</b><div className="mc-sub">{r.product} · {r.phone}</div></span><Tone tone={r.tone}>{r.state}</Tone></div>
                <div className="between" style={{ marginTop: 12 }}><span><span className="muted" style={{ fontSize: 12.5 }}>Em dívida</span><div className="display tnum" style={{ fontSize: 20 }}>{mt(r.debt)} MT</div></span><span style={{ textAlign: 'right' }}><Pips pips={r.pips} /><div className="mc-sub">{r.paid}/{r.n} · próxima {r.next ? dl(asDate(r.next)) : '—'}</div></span></div>
              </button>
            ))}
          </div>
        </>
      )}
    </div>
  )
}

export function NewLoan({ products, onClose, onDone }: { products: Product[]; onClose: () => void; onDone: (id: string) => void }) {
  const g = useGateway()
  const [pi, setPi] = useState(0)
  const p = products[pi]
  const [phone, setPhone] = useState('')
  const [name, setName] = useState('')
  const [amt, setAmt] = useState('')
  const [n, setN] = useState(p ? Math.min(Math.max(3, p.term_min), p.term_max) : 3)
  const [day, setDay] = useState(Math.min(28, new Date().getDate()))
  const [delivery, setDelivery] = useState('outside')
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')
  if (!p) return <Modal onClose={onClose}><div style={{ padding: 24 }}><div className="display" style={{ fontSize: 22 }}>Novo empréstimo</div><p className="muted">Crie um produto ativo primeiro.</p><button className="btn btn-ink" onClick={onClose}>Fechar</button></div></Modal>
  const a = parseMT(amt)
  const nn = Math.min(Math.max(n, p.term_min), p.term_max)
  const ok = a >= p.min && a <= p.max
  const q = quote(p, a, nn, day)
  const digits = phone.replace(/\D/g, '').replace(/^258/, '')
  const send = async () => {
    setBusy(true); setErr('')
    try { const r = await g.microcredit.createLoan({ product_id: p.id, phone: digits, name, amount: a, term: nn, day, delivery }); onDone(r.id) } catch (e) { setErr(errMsg(e)) } finally { setBusy(false) }
  }
  return (
    <Modal onClose={onClose} width={660}>
      <div style={{ padding: 24, maxHeight: '90vh', overflow: 'auto' }}>
        <div className="between"><div className="display" style={{ fontSize: 24 }}>Novo empréstimo</div><button className="icon-btn" onClick={onClose} aria-label="Fechar"><Icon name="close" size={16} /></button></div>
        <p className="muted" style={{ fontSize: 14, margin: '4px 0 16px' }}>Para um cliente que já falou consigo. Recebe o acordo por SMS.</p>
        <span className="label">Produto</span>
        <div className="mc-chips">{products.map((x, i) => <button key={x.id} className={'mc-chip' + (i === pi ? ' on' : '')} onClick={() => setPi(i)}>{x.name}</button>)}</div>
        <div className="grid-2" style={{ gap: 12, marginTop: 14 }}>
          <label><span className="label">Número do cliente</span><div className="phone-input" style={{ height: 48 }}><span className="cc">+258</span><input inputMode="tel" value={phone} onChange={e => setPhone(e.target.value)} placeholder="85 118 4402" /></div></label>
          <label><span className="label">Nome</span><input className="input" style={{ height: 48 }} value={name} onChange={e => setName(e.target.value)} placeholder="Nome completo" /></label>
          <label><span className="label">Montante</span><div className="phone-input" style={{ height: 48 }}><input inputMode="numeric" value={amt} onChange={e => setAmt(digitsMT(e.target.value))} placeholder={mt(p.min)} /><span style={{ padding: '0 12px', color: 'var(--muted)' }}>MT</span></div></label>
          <div><span className="label">Prazo</span><Step label={unit(p.freq, nn)} canDec={nn > p.term_min} canInc={nn < p.term_max} onDec={() => setN(nn - 1)} onInc={() => setN(nn + 1)} /></div>
          {p.freq === 'mensal' ? <div><span className="label">Dia da prestação</span><Step label={`Dia ${day}`} canDec={day > 1} canInc={day < 28} onDec={() => setDay(day - 1)} onInc={() => setDay(day + 1)} /></div>
            : <div><span className="label">Prestações</span><div style={{ height: 48, display: 'flex', alignItems: 'center', fontSize: 14 }} className="muted">{p.freq === 'semanal' ? 'A cada 7 dias' : 'A cada 14 dias'}</div></div>}
          <div><span className="label">1ª prestação</span><div style={{ height: 48, borderRadius: 13, background: 'var(--paper-2)', display: 'flex', alignItems: 'center', padding: '0 14px' }}>{q.dates[0] && dly(q.dates[0])}</div></div>
        </div>
        <div style={{ fontSize: 12.5, color: ok || !a ? 'var(--muted)' : 'var(--red-dark)', marginTop: 10 }}>{ok || !a ? `${p.name}: de ${mt(p.min)} a ${mt(p.max)} MT, de ${p.term_min} a ${unit(p.freq, p.term_max)}, ${rateLabel(p.rate_bp, p.freq)}.` : `Fora dos limites de ${p.name}: de ${mt(p.min)} a ${mt(p.max)} MT.`}</div>
        {ok && <div className="card between" style={{ padding: '14px 16px', marginTop: 12, borderRadius: 14 }}><b>{prest(nn)} de {mt(q.inst)} MT</b><span className="muted" style={{ fontSize: 13 }}>Total {mt(q.total)} MT · juros {mt(q.interest)} MT</span></div>}
        <div style={{ marginTop: 14 }}><span className="label">Como entrega o dinheiro</span><DeliveryChoice value={delivery} onChange={setDelivery} phone={phone} wallet={/^8[67]/.test(digits) ? 'e-Mola' : 'M-Pesa'} amount={a} /></div>
        {err && <div className="form-err" style={{ marginTop: 12 }}>{err}</div>}
        <div style={{ display: 'flex', gap: 12, alignItems: 'center', marginTop: 18, flexWrap: 'wrap' }}>
          <span className="muted" style={{ fontSize: 12.5, flex: 1, minWidth: 200 }}>O dinheiro só sai depois de o cliente aceitar o acordo com o código SMS.</span>
          <button className="btn btn-outline" onClick={onClose}>Cancelar</button>
          <button className="btn btn-green" disabled={!ok || busy || digits.length !== 9 || name.trim().length < 3} onClick={send}>{busy ? <Spinner /> : 'Enviar acordo'}</button>
        </div>
      </div>
    </Modal>
  )
}

function noteFor(x: any) {
  if (x.status === 'paid') return `${x.method} · ${dl(asDate(x.paid_at))}${x.mora_paid ? ` · mora ${mt(x.mora_paid)}` : ''}`
  if (x.status === 'paid_outside') return `${x.method || 'Fora da RIHA'}${x.marked_by ? ` · marcado por ${x.marked_by}` : ''}${x.note ? ` · ${x.note}` : ''}`
  if (x.status === 'late') return `${x.days_late} ${x.days_late === 1 ? 'dia' : 'dias'}${x.link_sent ? ' · link enviado' : ''}${x.opened ? ' · aberto' : ''}`
  if (x.status === 'scheduled') return `link a ${dl(asDate(x.due))}`
  return ''
}

export function LoanDetail() {
  const { id } = useParams()
  const nav = useNavigate()
  const g = useGateway()
  const mc = useMicro()
  const { toast } = useSession()
  const { data: l, setData, error } = useQuery(g => g.microcredit.loan(id!), [id, mc.version])
  const [modal, setModal] = useState<null | 'agreement' | 'settle' | 'writeoff' | 'delivered' | 'cancel' | { mark: any }>(null)
  const [busy, setBusy] = useState('')
  if (error) return <div className="card empty">{error}</div>
  if (!l) return <Skeleton h={400} r={22} />
  const act = async (action: string, body?: any, msg?: string) => {
    setBusy(action)
    try { setData(await g.microcredit.loanAction(id!, action, body)); setModal(null); if (msg) toast(msg); mc.bump() } catch (e) { toast(errMsg(e), true) } finally { setBusy('') }
  }
  const offered = l.status === 'offered' || l.status === 'approving'
  const plan: any[] = l.plan.length ? l.plan : (l.proposal ? l.proposal.dates.map((d: string, i: number) => ({ id: 'p' + i, seq: i + 1, due: d, amount: l.proposal.amounts[i], status: 'scheduled', mora: 0 })) : [])
  const total = plan.reduce((a: number, x: any) => a + x.amount, 0)
  const sameAmt = plan.every((x: any) => x.amount === plan[0]?.amount)
  const accepted = (l.agreements as any[]).filter(x => x.status === 'accepted').pop()
  return (
    <div>
      <Back to="/painel/microcredito/emprestimos" label="Empréstimos" nav={nav} />
      <div className="between" style={{ flexWrap: 'wrap', marginTop: 6, alignItems: 'flex-end' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12, flexWrap: 'wrap' }}>
          <h2 className="display" style={{ fontSize: 26, margin: 0 }}>{l.name}</h2>
          <Tone tone={l.tone}>{l.state}</Tone>
          <span className="muted" style={{ fontSize: 13.5 }}>{l.product} · {l.accepted_at ? `desde ${dly(asDate(l.accepted_at))}` : `criado ${shortWhen(l.created_at)}`} · <span className="mono">{l.reference}</span></span>
        </div>
        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
          {(l.status === 'active' || l.status === 'offered') && <button className="btn btn-outline" disabled={!!busy} onClick={() => act('resend', undefined, 'SMS reenviado')}>{busy === 'resend' ? <Spinner dark /> : l.status === 'offered' ? 'Reenviar acordo' : 'Reenviar link'}</button>}
          {l.status === 'approving' && <button className="btn btn-green" disabled={!!busy} onClick={() => act('second', undefined, 'Aprovado. O acordo foi enviado.')}>Aprovar (segunda pessoa)</button>}
          {offered && <button className="btn btn-danger" onClick={() => setModal('cancel')}>Cancelar acordo</button>}
          {l.status === 'active' && <button className="btn btn-ink" onClick={() => setModal('agreement')}>Novo acordo de pagamento</button>}
        </div>
      </div>
      {l.delivery_status === 'failed' && (
        <div className="form-err" style={{ marginTop: 14, display: 'flex', gap: 12, alignItems: 'center', flexWrap: 'wrap' }}>
          <span style={{ flex: 1 }}><b>A entrega falhou.</b> {l.delivery_error}</span>
          <button className="btn btn-sm btn-ink" disabled={!!busy} onClick={() => act('retry', undefined, 'Entrega enviada de novo')}>Tentar de novo</button>
          <button className="btn btn-sm btn-outline" onClick={() => setModal('delivered')}>Entreguei fora da RIHA</button>
        </div>
      )}
      {l.delivery_status === 'to_deliver' && l.status === 'active' && (
        <div style={{ marginTop: 14, padding: '12px 16px', borderRadius: 14, background: 'var(--sun-soft)', color: '#5B4600', display: 'flex', gap: 12, alignItems: 'center', flexWrap: 'wrap' }}>
          <span style={{ flex: 1, fontSize: 14 }}><b>Aceitou o acordo.</b> Entregue {mt(l.amount)} MT fora da RIHA e marque como entregue.</span>
          <button className="btn btn-sm btn-ink" onClick={() => setModal('delivered')}>Marcar como entregue</button>
        </div>
      )}
      {l.proposed > 0 && l.status === 'active' && l.proposal && <div style={{ marginTop: 14, padding: '12px 16px', borderRadius: 14, background: '#E6EEFB', color: '#2F4E86', fontSize: 14 }}><b>Novo acordo enviado {shortWhen(l.proposal.sent_at)}.</b> {prest(l.proposal.amounts.length)} de {mt(l.proposal.amounts[0])} MT, no dia {asDate(l.proposal.dates[0]).getDate()}. O plano abaixo vale até {l.name.split(' ')[0]} aceitar.</div>}
      {l.asked_at && !l.proposed && l.status === 'active' && <div style={{ marginTop: 14, padding: '12px 16px', borderRadius: 14, background: 'var(--sun-soft)', color: '#5B4600', fontSize: 14 }}><b>{l.name.split(' ')[0]} pediu um novo acordo</b> a {dl(asDate(l.asked_at))}. Proponha um com o botão acima.</div>}
      {l.manual && l.status === 'active' && <div className="form-err" style={{ marginTop: 14 }}><b>Cobrança manual.</b> Passaram os dias do último passo; os SMS automáticos pararam.</div>}
      <div className="mc-cols" style={{ marginTop: 16, gridTemplateColumns: 'minmax(0,1.7fr) minmax(0,1fr)' }}>
        <div className="card" style={{ overflow: 'hidden', minWidth: 0 }}>
          <div className="between" style={{ padding: '16px 20px', borderBottom: '1px solid var(--line-3)', flexWrap: 'wrap' }}><b style={{ fontSize: 15.5 }}>{offered ? 'Plano proposto' : 'Plano de pagamento'}</b><span className="muted" style={{ fontSize: 13 }}>{plan.length && sameAmt ? `${prest(plan.length)} de ${mt(plan[0].amount)} MT` : `${prest(plan.length)}, ${mt(total)} MT`}{l.day && l.freq === 'mensal' ? ` · dia ${plan[0] ? asDate(plan[0].due).getDate() : l.day}` : ''}</span></div>
          {plan.map((x: any) => {
            const [label, tone] = INST_STATUS[x.status] || [x.status, 'wait']
            return (
              <div key={x.id} style={{ display: 'grid', gridTemplateColumns: '28px 70px 80px minmax(0,1fr) auto', gap: 10, alignItems: 'center', padding: '12px 20px', borderBottom: '1px solid var(--line-3)', fontSize: 14, background: x.status === 'late' ? '#FFFBEA' : undefined }}>
                <span className="muted">{x.seq}</span>
                <b>{dl(asDate(x.due))}</b>
                <span className="tnum">{mt(x.amount + (x.mora || 0))}</span>
                <span style={{ display: 'flex', gap: 8, alignItems: 'center', minWidth: 0, flexWrap: 'wrap' }}><Tone tone={tone}>{label}</Tone><span className="muted" style={{ fontSize: 13 }}>{noteFor(x)}</span></span>
                {(x.status === 'late' || x.status === 'scheduled') && l.status === 'active' ? <button className="link-btn" style={{ fontSize: 13 }} onClick={() => setModal({ mark: x })}>Marcar como pago</button> : <span />}
              </div>
            )
          })}
          {l.mora_bp > 0 && <div style={{ padding: '12px 20px', fontSize: 13, color: 'var(--muted)' }}>Mora de {(l.mora_bp / 100).toString().replace('.', ',')}% por semana sobre a prestação em atraso, depois de {l.grace} {l.grace === 1 ? 'dia' : 'dias'} de tolerância, até {unit('semanal', l.mora_max)}.</div>}
          {l.old.length > 0 && <details style={{ padding: '10px 20px', fontSize: 13, borderTop: '1px solid var(--line-3)' }}><summary className="muted" style={{ cursor: 'pointer' }}>Planos anteriores ({l.old.length} prestações)</summary>
            {l.old.map((x: any) => <div key={x.id} className="between" style={{ padding: '6px 0' }}><span>v{x.version} · {x.seq} · {dl(asDate(x.due))}</span><span className="tnum">{mt(x.amount)} · {(INST_STATUS[x.status] || [x.status])[0]}</span></div>)}
          </details>}
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 14, minWidth: 0 }}>
          <div className="card card-pad">
            <div className="between"><span className="muted" style={{ fontSize: 13.5 }}>{offered ? 'A pagar, se aceitar' : 'Em dívida'}</span><MT v={l.debt} size={26} /></div>
            {plan.length > 0 && <div style={{ display: 'flex', height: 8, gap: 3, margin: '12px 0 6px' }}>{plan.map((x: any) => <span key={x.id} style={{ flex: 1, borderRadius: 2, background: x.status === 'paid' ? 'var(--green)' : x.status === 'paid_outside' ? 'repeating-linear-gradient(135deg,#8A938E 0 2px,#D5D3CB 2px 4px)' : x.status === 'late' ? 'var(--sun)' : '#E6E4DC' }} />)}</div>}
            <KV k="Emprestado" v={`${mt(l.amount)} MT`} />
            <KV k={`Juros · ${l.rate_label}`} v={`${mt(l.interest)} MT`} />
            <KV k="Pago pela RIHA" v={`${mt(l.paid_riha)} MT`} color="var(--green-text)" strong />
            <KV k="Pago fora da RIHA" v={`${mt(l.paid_outside)} MT`} />
            <KV k="Mora" v={`${mt(l.mora + l.mora_paid)} MT`} />
            {l.settle_offer > 0 && <KV k="Liquidação proposta" v={`${mt(l.settle_offer)} MT`} strong />}
          </div>
          {accepted && (
            <div className="card card-pad">
              <b>Acordo de pagamento · v{accepted.version}</b>
              <p style={{ fontSize: 13.5, color: 'var(--muted)', margin: '6px 0 10px', lineHeight: 1.5 }}>Aceite a {dly(asDate(accepted.accepted_at))}, {asDate(accepted.accepted_at).toLocaleTimeString('pt-PT', { hour: '2-digit', minute: '2-digit' })}, com código SMS no {accepted.phone}. {l.delivery === 'riha' ? `Entregue pelo saldo RIHA no ${l.wallet}.` : 'Entregue fora da RIHA.'}</p>
              <div style={{ display: 'flex', gap: 14, flexWrap: 'wrap' }}>
                <a className="link-btn" href={g.microcredit.agreementUrl(l.id, accepted.version)} target="_blank" rel="noreferrer">Ver acordo (PDF)</a>
                {l.agreements.length > 1 && <details><summary className="link-btn" style={{ listStyle: 'none' }}>Histórico</summary>{l.agreements.map((x: any) => <div key={x.id} style={{ fontSize: 13, marginTop: 6 }}><a className="link-btn" href={g.microcredit.agreementUrl(l.id, x.version)} target="_blank" rel="noreferrer">v{x.version}</a> · {x.status === 'accepted' ? 'em vigor' : x.status === 'superseded' ? 'substituído' : x.status === 'proposed' ? 'por aceitar' : 'não aceite'}</div>)}</details>}
              </div>
            </div>
          )}
          {l.status === 'active' && (
            <div className="card" style={{ padding: '6px 0' }}>
              {plan.some((x: any) => x.status === 'late' || x.status === 'scheduled') && <button className="mc-menu" onClick={() => setModal({ mark: plan.find((x: any) => x.status === 'late' || x.status === 'scheduled') })}>Marcar prestação como paga</button>}
              <button className="mc-menu" onClick={() => setModal('settle')}>Liquidação antecipada</button>
              <button className="mc-menu" style={{ color: 'var(--red-dark)' }} onClick={() => setModal('writeoff')}>Passar a perda</button>
            </div>
          )}
          {l.close_note && <div className="card card-pad" style={{ fontSize: 13.5 }}><b>Nota</b><div className="muted" style={{ marginTop: 4 }}>{l.close_note}</div></div>}
        </div>
      </div>
      <style>{`.mc-menu{display:block;width:100%;text-align:left;padding:12px 20px;border:0;background:none;font-weight:600;font-size:14px}.mc-menu:hover{background:#FBFBF8}`}</style>
      {modal === 'agreement' && <AgreementDrawer l={l} onClose={() => setModal(null)} onSend={body => act('agreement', body, 'Novo acordo enviado por SMS')} busy={busy === 'agreement'} />}
      {modal === 'settle' && <SettleModal l={l} onClose={() => setModal(null)} onSend={body => act('settle', body, body.outside ? 'Empréstimo liquidado' : 'Proposta de liquidação enviada por SMS')} busy={busy === 'settle'} />}
      {modal === 'writeoff' && <NoteModal title={`Passar ${l.name.split(' ')[0]} a perda?`} text={`Os ${mt(l.debt)} MT em dívida deixam de ser cobrados e os SMS param. Fica no histórico.`} label="Porquê" cta="Passar a perda" danger onClose={() => setModal(null)} onSend={note => act('write-off', { note }, 'Passado a perda')} busy={busy === 'write-off'} required />}
      {modal === 'delivered' && <NoteModal title="Marcar como entregue" text={`Confirme que entregou ${mt(l.amount)} MT a ${l.name} fora da RIHA.`} label="Como entregou (opcional)" cta="Marcar como entregue" onClose={() => setModal(null)} onSend={note => act('delivered', { note }, 'Marcado como entregue')} busy={busy === 'delivered'} />}
      {modal === 'cancel' && <NoteModal title="Cancelar o acordo?" text="O cliente deixa de poder aceitar. Nenhum dinheiro saiu." cta="Cancelar acordo" danger noInput onClose={() => setModal(null)} onSend={() => act('cancel', undefined, 'Acordo cancelado')} busy={busy === 'cancel'} />}
      {modal && typeof modal === 'object' && 'mark' in modal && <MarkPaid l={l} x={modal.mark} onClose={() => setModal(null)} onDone={d => { setData(d); setModal(null); toast('Prestação marcada como paga'); mc.bump() }} />}
    </div>
  )
}

function NoteModal({ title, text, label, cta, danger, required, noInput, busy, onClose, onSend }: { title: string; text: string; label?: string; cta: string; danger?: boolean; required?: boolean; noInput?: boolean; busy: boolean; onClose: () => void; onSend: (note: string) => void }) {
  const [note, setNote] = useState('')
  return (
    <Modal onClose={onClose}>
      <div style={{ padding: 24 }}>
        <div className="display" style={{ fontSize: 22 }}>{title}</div>
        <p className="muted" style={{ fontSize: 14, lineHeight: 1.5 }}>{text}</p>
        {!noInput && <label><span className="label">{label}</span><input className="input" value={note} maxLength={300} onChange={e => setNote(e.target.value)} /></label>}
        <div style={{ display: 'flex', gap: 8, marginTop: 18 }}><button className="btn btn-outline" style={{ flex: 1 }} onClick={onClose}>Voltar</button><button className={'btn ' + (danger ? 'btn-ink' : 'btn-green')} style={{ flex: 1 }} disabled={busy || (required && !note.trim())} onClick={() => onSend(note)}>{busy ? <Spinner /> : cta}</button></div>
      </div>
    </Modal>
  )
}

function MarkPaid({ l, x, onClose, onDone }: { l: any; x: any; onClose: () => void; onDone: (d: any) => void }) {
  const g = useGateway()
  const [method, setMethod] = useState('cash')
  const [note, setNote] = useState('')
  const [withMora, setWithMora] = useState(x.mora > 0)
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')
  const send = async () => {
    setBusy(true); setErr('')
    try { onDone(await g.microcredit.markPaid(l.id, x.id, { method, note, with_mora: withMora })) } catch (e) { setErr(errMsg(e)) } finally { setBusy(false) }
  }
  return (
    <Modal onClose={onClose}>
      <div style={{ padding: 24 }}>
        <div className="display" style={{ fontSize: 22 }}>Prestação {x.seq} paga fora da RIHA</div>
        <p className="muted" style={{ fontSize: 14 }}>{mt(x.amount)} MT, vencia a {dly(asDate(x.due))}. Não entra no saldo RIHA.</p>
        <span className="label">Como foi paga</span>
        <div className="mc-chips">{[['cash', 'Dinheiro'], ['transfer', 'Transferência'], ['wallet', 'Carteira'], ['other', 'Outro']].map(([k, t]) => <button key={k} className={'mc-chip' + (method === k ? ' on' : '')} onClick={() => setMethod(k)}>{t}</button>)}</div>
        {x.mora > 0 && <div className="between card" style={{ padding: '12px 14px', marginTop: 14, borderRadius: 14 }}><span style={{ fontSize: 14 }}>Recebeu também a mora de {mt(x.mora)} MT</span><Toggle on={withMora} onChange={setWithMora} /></div>}
        <label style={{ display: 'block', marginTop: 14 }}><span className="label">Nota (opcional)</span><input className="input" value={note} maxLength={200} onChange={e => setNote(e.target.value)} /></label>
        {err && <div className="form-err" style={{ marginTop: 12 }}>{err}</div>}
        <div style={{ display: 'flex', gap: 8, marginTop: 18 }}><button className="btn btn-outline" style={{ flex: 1 }} onClick={onClose}>Cancelar</button><button className="btn btn-green" style={{ flex: 1 }} disabled={busy} onClick={send}>{busy ? <Spinner /> : 'Marcar como paga'}</button></div>
      </div>
    </Modal>
  )
}

function SettleModal({ l, busy, onClose, onSend }: { l: any; busy: boolean; onClose: () => void; onSend: (b: any) => void }) {
  const [amt, setAmt] = useState(digitsMT(String(Math.round(l.debt / 100))))
  const [outside, setOutside] = useState(false)
  const [method, setMethod] = useState('cash')
  const a = parseMT(amt)
  const ok = a > 0 && a <= l.debt
  return (
    <Modal onClose={onClose}>
      <div style={{ padding: 24 }}>
        <div className="display" style={{ fontSize: 22 }}>Liquidação antecipada</div>
        <p className="muted" style={{ fontSize: 14, lineHeight: 1.5 }}>Falta pagar {mt(l.debt)} MT. Pode baixar o valor, por exemplo para perdoar juros futuros.</p>
        <label><span className="label">Valor para liquidar</span><div className="phone-input" style={{ height: 48 }}><input inputMode="numeric" value={amt} onChange={e => setAmt(digitsMT(e.target.value))} /><span style={{ padding: '0 12px', color: 'var(--muted)' }}>MT</span></div></label>
        {a > 0 && a < l.debt && <div style={{ fontSize: 13, color: 'var(--green-text)', marginTop: 6 }}>Desconto de {mt(l.debt - a)} MT.</div>}
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8, marginTop: 14 }}>
          <Radio on={!outside} onClick={() => setOutside(false)} title="Enviar o link por SMS" note="O cliente paga com o PIN e o crédito fica liquidado." />
          <Radio on={outside} onClick={() => setOutside(true)} title="Já recebi fora da RIHA" note="Fecha o empréstimo agora. Não entra no saldo RIHA." />
        </div>
        {outside && <div className="mc-chips" style={{ marginTop: 12 }}>{[['cash', 'Dinheiro'], ['transfer', 'Transferência'], ['wallet', 'Carteira'], ['other', 'Outro']].map(([k, t]) => <button key={k} className={'mc-chip' + (method === k ? ' on' : '')} onClick={() => setMethod(k)}>{t}</button>)}</div>}
        <div style={{ display: 'flex', gap: 8, marginTop: 18 }}><button className="btn btn-outline" style={{ flex: 1 }} onClick={onClose}>Cancelar</button><button className="btn btn-green" style={{ flex: 1 }} disabled={busy || !ok} onClick={() => onSend({ amount: a, outside, method })}>{busy ? <Spinner /> : outside ? 'Liquidar' : 'Enviar link'}</button></div>
      </div>
    </Modal>
  )
}

function AgreementDrawer({ l, busy, onClose, onSend }: { l: any; busy: boolean; onClose: () => void; onSend: (b: any) => void }) {
  const left = l.plan.filter((x: any) => x.status === 'scheduled').length
  const [opt, setOpt] = useState<'join' | 'extend'>('extend')
  const [ext, setExt] = useState(2)
  const [day, setDay] = useState(Math.min(28, l.day || new Date().getDate()))
  const [waive, setWaive] = useState(true)
  const total = l.overdue + l.to_come + (waive ? 0 : l.mora)
  const n = Math.max(1, opt === 'join' ? left : left + ext)
  const dates = nextDates(day, n)
  const amounts = evenSplit(total, n)
  const nJoin = Math.max(1, left), nExt = left + ext
  const first = l.name.split(' ')[0]
  const rows = planRows(dates.map((d, i) => ({ i: String(i + 1), d: dly(d), v: mt(amounts[i]) + ' MT' })), h => ({ i: '⋯', d: `mais ${h} prestações`, v: mt(amounts[3]) + ' MT cada' }))
  return (
    <Drawer title="Novo acordo de pagamento" onClose={onClose} footer={<><button className="btn btn-outline" onClick={onClose}>Cancelar</button><button className="btn btn-ink" style={{ flex: 1 }} disabled={busy || total <= 0} onClick={() => onSend({ option: opt, extra_months: ext, day, waive })}>{busy ? <Spinner /> : 'Enviar novo acordo'}</button></>}>
      <div style={{ padding: '18px 26px', display: 'flex', flexDirection: 'column', gap: 14 }}>
        <div className="muted" style={{ fontSize: 13, marginTop: -6 }}>{l.name}</div>
        <div className="grid-3" style={{ gap: 8 }}>
          <div className="card" style={{ padding: '10px 12px', borderRadius: 14, background: l.overdue ? 'var(--sun-soft)' : '#fff' }}><div style={{ fontSize: 12.5 }}>Em atraso</div><b>{mt(l.overdue)} MT</b></div>
          <div className="card" style={{ padding: '10px 12px', borderRadius: 14 }}><div style={{ fontSize: 12.5 }} className="muted">Mora</div><b>{mt(l.mora)} MT</b></div>
          <div className="card" style={{ padding: '10px 12px', borderRadius: 14 }}><div style={{ fontSize: 12.5 }} className="muted">Por vencer</div><b>{mt(l.to_come)} MT</b></div>
        </div>
        <Radio on={opt === 'join'} onClick={() => setOpt('join')} title={left ? `Juntar o atraso às ${prest(left)} que faltam` : 'Pagar o atraso numa prestação'} note={`${prest(nJoin)} de ${mt(Math.ceil(total / nJoin / 100) * 100)} MT · termina a ${dly(nextDates(day, nJoin)[nJoin - 1])}`} />
        <Radio on={opt === 'extend'} onClick={() => setOpt('extend')} title="Alargar o prazo" note={`${prest(nExt)} de ${mt(Math.ceil(total / nExt / 100) * 100)} MT · termina a ${dly(nextDates(day, nExt)[nExt - 1])}`} />
        <div className="grid-2" style={{ gap: 10 }}>
          <div style={{ opacity: opt === 'extend' ? 1 : 0.4, pointerEvents: opt === 'extend' ? 'auto' : 'none' }}><span className="label">Meses a mais</span><Step label={`+ ${unit('mensal', ext)}`} canDec={ext > 1} canInc={ext < 12} onDec={() => setExt(ext - 1)} onInc={() => setExt(ext + 1)} /></div>
          <div><span className="label">Dia de pagamento</span><Step label={`Dia ${day}`} canDec={day > 1} canInc={day < 28} onDec={() => setDay(day - 1)} onInc={() => setDay(day + 1)} /></div>
        </div>
        {l.mora > 0 && <div className="card between" style={{ padding: '12px 16px', borderRadius: 14 }}><b style={{ fontSize: 14 }}>Perdoar a mora de {mt(l.mora)} MT</b><Toggle on={waive} onChange={setWaive} /></div>}
        <Plan rows={rows} head={<div className="between"><b style={{ fontSize: 14 }}>Novo plano</b><span className="muted" style={{ fontSize: 12.5 }}>Total {mt(total)} MT</span></div>} />
        <div className="card" style={{ padding: '12px 16px', fontSize: 13, lineHeight: 1.7, borderRadius: 14 }}>{first} recebe um SMS e aceita com um código.<br /><b>O plano atual vale até aceitar.</b><br />O acordo antigo fica guardado no histórico.</div>
      </div>
    </Drawer>
  )
}

