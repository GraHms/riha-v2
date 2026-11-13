import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { Check, Drawer, Icon, Modal, Skeleton, Spinner } from '../../../components/ui'
import { useGateway } from '../../../core/gateway'
import { errMsg } from '../../../core/model'
import { useQuery } from '../../../core/useQuery'
import { digitsMT, dl, dly, effort, mt, parseMT, planRows, prest, quote, rateLabel, unit, type Product } from '../../../lib/credit'
import { useSession } from '../../../lib/session'
import { Back, MT, Plan, Radio, REQ_TONE, shortWhen, Step, Tone, useMicro } from './common'

const COLS = 'minmax(180px,1.6fr) minmax(110px,1fr) 80px 90px 110px 70px 110px 110px'

export default function Requests() {
  const nav = useNavigate()
  const mc = useMicro()
  const [filter, setFilter] = useState('decidir')
  const [product, setProduct] = useState('')
  const [q, setQ] = useState('')
  const { data } = useQuery(g => g.microcredit.requests(filter, product, q), [filter, product, q, mc.version])
  const rows: any[] = data?.data || []
  const c = data?.counts || {}
  const g = useGateway()
  return (
    <div>
      <div className="between" style={{ flexWrap: 'wrap', marginBottom: 14 }}>
        <div className="mc-chips">
          {[['decidir', `Por decidir ${c.decidir ?? ''}`], ['enviado', `Acordo enviado ${c.enviado ?? ''}`], ['todos', `Todos ${c.todos ?? ''}`]].map(([k, l]) => <button key={k} className={'mc-chip' + (filter === k ? ' on' : '')} onClick={() => setFilter(k)}>{l}</button>)}
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
            <div className="mc-th" style={{ gridTemplateColumns: COLS }}><span>Cliente</span><span>Produto</span><span style={{ textAlign: 'right' }}>Pede</span><span>Prazo</span><span style={{ textAlign: 'right' }}>Prestação</span><span style={{ textAlign: 'right' }}>Esforço</span><span>Recebido</span><span>Estado</span></div>
            {rows.length === 0 && <div className="empty">{filter === 'decidir' ? 'Nenhum pedido por decidir. Partilhe o link de pedido para receber pedidos.' : 'Nenhum pedido.'}</div>}
            {rows.map(r => (
              <div key={r.id} className="mc-row click" style={{ gridTemplateColumns: COLS }} onClick={() => nav('/painel/microcredito/pedidos/' + r.id)}>
                <span style={{ minWidth: 0 }}><b>{r.name}</b><div className="mc-sub">{r.phone}{r.activity && ` · ${r.activity}`}</div></span>
                <span>{r.product}</span>
                <b className="tnum" style={{ textAlign: 'right' }}>{mt(r.amount)}</b>
                <span>{r.term_label}</span>
                <span className="tnum" style={{ textAlign: 'right' }}>{mt(r.instalment)} / {r.per}</span>
                <span className="tnum" style={{ textAlign: 'right', fontWeight: r.effort > 30 ? 700 : 400, color: r.effort > r.effort_limit ? 'var(--red-dark)' : r.effort > 30 ? 'var(--sun-text)' : undefined }}>{r.income ? r.effort + '%' : '—'}</span>
                <span style={{ fontSize: 13.5 }}>{shortWhen(r.created_at)}</span>
                <span><Tone tone={REQ_TONE[r.status]}>{r.status_label}</Tone></span>
              </div>
            ))}
            <div className="between" style={{ padding: '12px 20px', fontSize: 13, color: 'var(--muted)', borderTop: '1px solid var(--line-3)' }}>
              <span>Esforço: prestação mensal sobre o rendimento declarado.</span>
              <a className="link-btn" style={{ color: 'var(--muted)' }} href={g.microcredit.exportUrl('requests')}>Exportar CSV</a>
            </div>
          </div>
          <div className="mc-card-list">
            {rows.length === 0 && <div className="card empty">Nenhum pedido.</div>}
            {rows.map(r => (
              <button key={r.id} className="card" style={{ padding: 16, textAlign: 'left', border: '1px solid var(--line)' }} onClick={() => nav('/painel/microcredito/pedidos/' + r.id)}>
                <div className="between" style={{ alignItems: 'flex-start' }}><span><b>{r.name}</b><div className="mc-sub">{r.phone}{r.activity && ` · ${r.activity}`}</div></span><Tone tone={REQ_TONE[r.status]}>{r.status_label}</Tone></div>
                <div className="grid-3" style={{ gap: 8, marginTop: 12, fontSize: 13 }}>
                  <span><span className="muted">Pede</span><b style={{ display: 'block' }}>{mt(r.amount)} MT</b></span>
                  <span><span className="muted">Prazo</span><b style={{ display: 'block' }}>{r.term_label}</b></span>
                  <span><span className="muted">Esforço</span><b style={{ display: 'block', color: r.effort > r.effort_limit ? 'var(--red-dark)' : undefined }}>{r.income ? r.effort + '%' : '—'}</b></span>
                </div>
                <div className="mc-sub" style={{ marginTop: 10 }}>{r.product} · {mt(r.instalment)} / {r.per} · {shortWhen(r.created_at)}</div>
              </button>
            ))}
          </div>
        </>
      )}
    </div>
  )
}

export function RequestDetail() {
  const { id } = useParams()
  const nav = useNavigate()
  const g = useGateway()
  const mc = useMicro()
  const { toast } = useSession()
  const { data: r, setData, error } = useQuery(g => g.microcredit.request(id!), [id, mc.version])
  const [approve, setApprove] = useState(false)
  const [decline, setDecline] = useState(false)
  const [note, setNote] = useState('')
  const [busy, setBusy] = useState(false)
  useEffect(() => { if (r?.status === 'new') g.microcredit.reviewRequest(id!).then(setData).catch(() => {}) }, [r?.status]) // eslint-disable-line react-hooks/exhaustive-deps
  if (error) return <div className="card empty">{error}</div>
  if (!r) return <Skeleton h={400} r={22} />
  const p: Product = r.product_full
  const open = r.status === 'new' || r.status === 'review'
  const doDecline = async () => {
    setBusy(true)
    try { setData(await g.microcredit.declineRequest(id!, note)); setDecline(false); toast('Pedido recusado. O cliente recebe um SMS.'); mc.bump() } catch (e) { toast(errMsg(e), true) } finally { setBusy(false) }
  }
  const docs: any[] = r.documents || []
  return (
    <div>
      <Back to="/painel/microcredito/pedidos" label="Pedidos" nav={nav} />
      <div className="between" style={{ flexWrap: 'wrap', marginTop: 6, alignItems: 'flex-end' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12, flexWrap: 'wrap' }}>
          <h2 className="display" style={{ fontSize: 26, margin: 0 }}>{r.name}</h2>
          <Tone tone={REQ_TONE[r.status]}>{r.status_label}</Tone>
          <span className="muted" style={{ fontSize: 13.5 }}>Recebido {shortWhen(r.created_at)}</span>
        </div>
        {open && <div style={{ display: 'flex', gap: 8 }}>
          <button className="btn btn-danger" onClick={() => setDecline(true)}>Recusar</button>
          <button className="btn btn-green" onClick={() => mc.approved ? setApprove(true) : toast('A sua conta ainda não foi aprovada.', true)}>Aprovar e enviar acordo</button>
        </div>}
        {r.loan && <button className="btn btn-outline" onClick={() => nav('/painel/microcredito/emprestimos/' + r.loan.id)}>Ver empréstimo</button>}
      </div>
      {r.status === 'offered' && r.loan && <div style={{ marginTop: 14, padding: '12px 16px', borderRadius: 14, background: 'var(--green-bg)', color: 'var(--green-dark)', fontSize: 14 }}>Acordo enviado {shortWhen(r.loan.created_at)}. {mt(r.loan.amount)} MT em {prest(r.loan.term)}, {r.loan.delivery === 'riha' ? 'entregues pelo saldo RIHA' : 'entregues fora da RIHA'}. O dinheiro só sai quando {r.name.split(' ')[0]} aceitar com o código SMS.</div>}
      {r.status === 'declined' && <div className="form-err" style={{ marginTop: 14 }}>Recusado{r.decline_note && `: ${r.decline_note}`}</div>}
      <div className="mc-cols" style={{ marginTop: 16 }}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 14, minWidth: 0 }}>
          <div className="card" style={{ overflow: 'hidden' }}>
            <div className="grid-4" style={{ padding: 20, gap: 12 }}>
              <div><div className="muted" style={{ fontSize: 13 }}>Pede</div><MT v={r.amount} size={24} /></div>
              <div><div className="muted" style={{ fontSize: 13 }}>Prazo</div><span className="display" style={{ fontSize: 24 }}>{r.term_label}</span></div>
              <div><div className="muted" style={{ fontSize: 13 }}>Prestação</div><MT v={r.instalment} size={24} /></div>
              <div><div className="muted" style={{ fontSize: 13 }}>Total a pagar</div><MT v={r.quote.total} size={24} /></div>
            </div>
            <div style={{ borderTop: '1px solid var(--line-3)', padding: '4px 20px' }}>
              <Line k="Produto" v={<b>{p.name} · {rateLabel(p.rate_bp, p.freq)} · {p.term_label}</b>} />
              {r.purpose && <Line k="Para quê" v={r.purpose} />}
              <Line k="Recebe e paga em" v={`${r.wallet} · ${r.phone}`} />
              {p.client_day && <Line k="Dia de pagamento" v={`Dia ${r.day}`} />}
            </div>
          </div>
          <div className="card" style={{ overflow: 'hidden' }}>
            <div style={{ padding: '14px 20px', fontWeight: 700, borderBottom: '1px solid var(--line-3)' }}>Sobre {r.name.split(' ')[0]}</div>
            <div style={{ padding: '4px 20px' }}>
              {(r.id_number || r.nuit) && <Line k="Documento" v={<span className="mono">{[r.id_number && `BI ${r.id_number}`, r.nuit && `NUIT ${r.nuit}`].filter(Boolean).join(' · ')}</span>} />}
              {r.address && <Line k="Morada" v={r.address} />}
              {r.activity && <Line k="Atividade" v={r.activity} />}
              {r.income > 0 && <Line k="Rendimento declarado" v={<b>{mt(r.income)} MT por mês</b>} />}
              {Object.entries(r.extra || {}).map(([k, v]) => <Line key={k} k={k} v={v as string} />)}
            </div>
          </div>
          <div className="card card-pad">
            <b>Documentos enviados</b>
            {docs.length === 0 ? <div className="muted" style={{ fontSize: 13.5, marginTop: 8 }}>Nenhum documento.</div> : (
              <div className="grid-3" style={{ marginTop: 12, gap: 10 }}>
                {docs.map(d => (
                  <a key={d.id} href={g.microcredit.documentUrl(d.id)} target="_blank" rel="noreferrer" style={{ display: 'block', height: 100, borderRadius: 12, border: '1px solid var(--line)', overflow: 'hidden', position: 'relative', background: 'repeating-linear-gradient(135deg,#F0EEE8 0 6px,#F7F6F2 6px 12px)' }}>
                    {d.content_type.startsWith('image/') && <img src={g.microcredit.documentUrl(d.id)} alt={d.label} style={{ width: '100%', height: '100%', objectFit: 'cover' }} />}
                    <span className="mono" style={{ position: 'absolute', left: 8, bottom: 6, fontSize: 11, background: 'rgba(255,255,255,.85)', padding: '1px 6px', borderRadius: 6 }}>{d.label}</span>
                  </a>
                ))}
              </div>
            )}
          </div>
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 14, minWidth: 0 }}>
          <div className="card" style={{ overflow: 'hidden' }}>
            <div style={{ padding: '14px 20px', fontWeight: 700, borderBottom: '1px solid var(--line-3)' }}>Verificações</div>
            {r.checks.map((c: any, i: number) => (
              <div key={i} style={{ display: 'flex', gap: 12, padding: '12px 20px', borderBottom: i < r.checks.length - 1 ? '1px solid var(--line-3)' : 0 }}>
                <span style={{ width: 22, height: 22, borderRadius: '50%', background: c.ok ? 'var(--green-bg)' : 'var(--sun-bg)', display: 'grid', placeItems: 'center', flex: 'none' }}>{c.ok ? <Check /> : <Icon name="alert" size={13} color="var(--sun-text)" />}</span>
                <div><div style={{ fontWeight: 600, fontSize: 14 }}>{c.title}</div><div className="mc-sub">{c.note}</div></div>
              </div>
            ))}
          </div>
          <div className="card" style={{ overflow: 'hidden' }}>
            <div style={{ padding: '14px 20px', fontWeight: 700, borderBottom: '1px solid var(--line-3)' }}>Histórico com {mc.merchant.name}</div>
            {r.history.length === 0 && <div style={{ padding: '14px 20px', fontSize: 13.5, color: 'var(--muted)' }}>Primeiro pedido deste número.</div>}
            {r.history.map((h: any) => (
              <div key={h.id} className="between" style={{ padding: '12px 20px', borderBottom: '1px solid var(--line-3)', fontSize: 14 }}>
                <span>{mt(h.amount)} MT · {dl(new Date(h.created_at))}{h.closed_at && ` a ${dly(new Date(h.closed_at))}`}</span>
                <Tone tone={h.status === 'settled' ? 'ok' : h.status === 'written_off' ? 'bad' : 'neu'}>{h.status === 'settled' ? 'Liquidado' : h.status === 'written_off' ? 'Perda' : 'A correr'}</Tone>
              </div>
            ))}
          </div>
        </div>
      </div>
      {approve && <Approve r={r} p={p} onClose={() => setApprove(false)} onDone={d => { setData(d); setApprove(false); mc.bump(); toast('Acordo enviado por SMS') }} />}
      {decline && (
        <Modal onClose={() => setDecline(false)}>
          <div style={{ padding: 24 }}>
            <div className="display" style={{ fontSize: 22 }}>Recusar o pedido de {r.name.split(' ')[0]}?</div>
            <p className="muted" style={{ fontSize: 14, lineHeight: 1.5 }}>O cliente recebe um SMS a dizer que o pedido não foi aprovado.</p>
            <label><span className="label">Nota interna (opcional)</span><input className="input" value={note} maxLength={300} onChange={e => setNote(e.target.value)} /></label>
            <div style={{ display: 'flex', gap: 8, marginTop: 18 }}><button className="btn btn-outline" style={{ flex: 1 }} onClick={() => setDecline(false)}>Cancelar</button><button className="btn btn-ink" style={{ flex: 1 }} disabled={busy} onClick={doDecline}>{busy ? <Spinner /> : 'Recusar'}</button></div>
          </div>
        </Modal>
      )}
    </div>
  )
}

function Line({ k, v }: { k: string; v: React.ReactNode }) {
  return <div style={{ display: 'grid', gridTemplateColumns: 'minmax(120px,190px) 1fr', gap: 12, padding: '11px 0', borderBottom: '1px solid var(--line-3)', fontSize: 14 }}><span className="muted">{k}</span><span style={{ minWidth: 0, overflowWrap: 'anywhere' }}>{v}</span></div>
}

export function DeliveryChoice({ value, onChange, phone, wallet, amount }: { value: string; onChange: (v: string) => void; phone: string; wallet: string; amount: number }) {
  const { data: s } = useQuery(g => g.microcredit.settings())
  const { data: o } = useQuery(g => g.microcredit.overview())
  if (!s || !o) return <Skeleton h={120} r={16} />
  const set = s.settings
  const rihaOk = set.via_riha && s.can_disburse && amount <= set.delivery_limit
  const why = !set.via_riha ? 'Desligado nas definições.' : !s.can_disburse ? 'Ainda não disponível nesta conta. Entregue fora da RIHA.' : amount > set.delivery_limit ? `Acima do limite por entrega de ${mt(set.delivery_limit)} MT.` : ''
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
      <div style={{ opacity: rihaOk ? 1 : 0.5, pointerEvents: rihaOk ? 'auto' : 'none' }}>
        <Radio on={value === 'riha'} onClick={() => onChange('riha')} title="Pelo saldo RIHA" note={why || `Para o ${wallet} ${phone}. Sai do saldo quando aceitar. Disponível: ${mt(o.balance)} MT.`} />
      </div>
      {set.outside && <Radio on={value === 'outside'} onClick={() => onChange('outside')} title="Fora da RIHA" note="Dinheiro ou transferência. Marca como entregue depois de aceitar." />}
    </div>
  )
}

function Approve({ r, p, onClose, onDone }: { r: any; p: Product; onClose: () => void; onDone: (d: any) => void }) {
  const g = useGateway()
  const [amt, setAmt] = useState(digitsMT(String(r.amount / 100)))
  const [n, setN] = useState(r.term)
  const [day, setDay] = useState(r.day || new Date().getDate())
  const [delivery, setDelivery] = useState('outside')
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')
  const a = parseMT(amt)
  const ok = a >= p.min && a <= p.max
  const q = quote(p, a, n, day)
  const eff = r.income ? effort(q.inst, p.freq, r.income) : 0
  const first = r.name.split(' ')[0]
  const send = async () => {
    setBusy(true); setErr('')
    try { onDone(await g.microcredit.approveRequest(r.id, { amount: a, term: n, day, delivery })) } catch (e) { setErr(errMsg(e)) } finally { setBusy(false) }
  }
  const rows = planRows(q.dates.map((d, i) => ({ i: String(i + 1), d: dly(d), v: mt(q.inst) + ' MT' })), h => ({ i: '⋯', d: `mais ${h} prestações`, v: mt(q.inst) + ' MT cada' }))
  return (
    <Drawer title="Aprovar e enviar acordo" onClose={onClose} footer={<><button className="btn btn-outline" onClick={onClose}>Cancelar</button><button className="btn btn-green" style={{ flex: 1 }} disabled={!ok || busy} onClick={send}>{busy ? <Spinner /> : ok ? `Enviar acordo · ${mt(a)} MT` : 'Montante fora dos limites'}</button></>}>
      <div style={{ padding: '18px 26px', display: 'flex', flexDirection: 'column', gap: 14 }}>
        <div className="muted" style={{ fontSize: 13, marginTop: -6 }}>{r.name} · {p.name}</div>
        <div className="grid-2" style={{ gap: 10 }}>
          <label><span className="label">Montante <span className="muted">· pediu {mt(r.amount)}</span></span><div className="phone-input" style={{ height: 48 }}><input inputMode="numeric" value={amt} onChange={e => setAmt(digitsMT(e.target.value))} /><span style={{ padding: '0 12px', color: 'var(--muted)' }}>MT</span></div></label>
          <div><span className="label">Prazo <span className="muted">· pediu {r.term}</span></span><Step label={unit(p.freq, n)} canDec={n > p.term_min} canInc={n < p.term_max} onDec={() => setN(n - 1)} onInc={() => setN(n + 1)} /></div>
          {p.freq === 'mensal' && <div><span className="label">Dia da prestação</span><Step label={`Dia ${day}`} canDec={day > 1} canInc={day < 28} onDec={() => setDay(day - 1)} onInc={() => setDay(day + 1)} /></div>}
          <div><span className="label">1ª prestação</span><div style={{ height: 48, borderRadius: 13, background: 'var(--paper-2)', display: 'flex', alignItems: 'center', padding: '0 14px', fontSize: 15 }}>{q.dates[0] && dly(q.dates[0])}</div></div>
        </div>
        <div style={{ fontSize: 12.5, color: ok ? 'var(--muted)' : 'var(--red-dark)' }}>{ok ? `${p.name}: de ${mt(p.min)} a ${mt(p.max)} MT, de ${p.term_min} a ${unit(p.freq, p.term_max)}.` : `Fora dos limites de ${p.name}: de ${mt(p.min)} a ${mt(p.max)} MT.`}</div>
        <Plan rows={rows} head={<div className="between" style={{ alignItems: 'flex-start' }}><b style={{ fontSize: 14 }}>{prest(n)} de {mt(q.inst)} MT</b><span className="muted" style={{ fontSize: 12.5, textAlign: 'right' }}>Total {mt(q.total)} MT · juros {mt(q.interest)} MT</span></div>}
          foot={r.income ? <b style={{ color: eff > p.effort_pct ? 'var(--red-dark)' : 'var(--green-text)' }}>{eff > p.effort_pct ? `Esforço de ${eff}%. Acima do limite de ${p.effort_pct}%.` : `Esforço de ${eff}% do rendimento declarado`}</b> : undefined} />
        <div><span className="label">Como entrega o dinheiro</span><DeliveryChoice value={delivery} onChange={setDelivery} phone={r.phone} wallet={r.wallet} amount={a} /></div>
        <div className="card" style={{ padding: '12px 16px', fontSize: 13, lineHeight: 1.6, borderRadius: 14 }}>{first} recebe um SMS com o acordo e aceita com um código.<br /><b>O dinheiro só sai depois de aceitar.</b></div>
        {err && <div className="form-err">{err}</div>}
      </div>
    </Drawer>
  )
}
