import { useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { Drawer, Icon, Modal, Skeleton, Spinner } from '../../../components/ui'
import { useGateway } from '../../../core/gateway'
import { errMsg } from '../../../core/model'
import { useQuery } from '../../../core/useQuery'
import { useSession } from '../../../lib/session'
import { abbrevName, CHANNEL, CHARGE_STATUS, money, MonthStrip, OUTSIDE_METHODS, StatusPill, SUB_STATUS, VIA } from './common'
import { useSubs } from './SubsShell'

const KIND: Record<string, string> = { invite: 'Convite', reminder: 'Lembrete', link: 'Link de pagamento', resend: 'Reenvio', last_warning: 'Último aviso', final: 'Ação final', receipt: 'Recibo', price_notice: 'Aviso de preço', pause: 'Pausa', skip: 'Mês saltado', cancel: 'Cancelamento', plan_change: 'Mudança de plano' }
const MAPUTO = new Intl.DateTimeFormat('pt-PT', { timeZone: 'Africa/Maputo', day: 'numeric', month: 'numeric', hour: '2-digit', minute: '2-digit', hour12: false })
const hm = (iso: string) => { const p = Object.fromEntries(MAPUTO.formatToParts(new Date(iso)).map(x => [x.type, x.value])); return `${Number(p.day)} ${['jan', 'fev', 'mar', 'abr', 'mai', 'jun', 'jul', 'ago', 'set', 'out', 'nov', 'dez'][Number(p.month) - 1]} · ${p.hour}:${p.minute}` }

function chargeDetail(c: any, msgs: any[]) {
  switch (c.status) {
    case 'scheduled': return `link a ${c.due}`
    case 'open': return c.last_failure || (c.opened_at ? 'link aberto' : `link enviado${c.link_sent_at ? ' · ' + hm(c.link_sent_at).split(' · ')[0] : ''}`)
    case 'past_due': { const n = msgs.filter(m => m.charge_id === c.id && m.kind === 'resend').length + c.resends; return [n ? `${n} ${n === 1 ? 'reenvio' : 'reenvios'}` : '', c.last_failure].filter(Boolean).join(' · ') || 'por pagar' }
    case 'paid': return `${VIA[c.method] || c.method} · ${c.paid_at}`
    case 'paid_outside': return `${c.outside_label}${c.marked_by ? ' · marcado por ' + abbrevName(c.marked_by) : ''}`
    case 'skipped': return 'sem cobrança neste período'
    case 'forfeited': return 'não pago · período perdido'
  }
  return ''
}

export default function Member() {
  const { id } = useParams()
  const g = useGateway()
  const { toast } = useSession()
  const sx = useSubs()
  const { data, reload } = useQuery(g => g.subscriptions.member(id!), [id, sx.version])
  const [subIdx, setSubIdx] = useState(0)
  const [year, setYear] = useState<number | null>(null)
  const [mark, setMark] = useState<string[] | null>(null)
  const [open, setOpen] = useState<string | null>(null)
  const [dialog, setDialog] = useState<null | 'pause' | 'plan' | 'cancel'>(null)
  const [busy, setBusy] = useState('')
  const years = useMemo(() => Array.from(new Set((data?.charges || []).map((c: any) => c.year))).sort((a: any, b: any) => b - a) as number[], [data])
  if (!data) return <Skeleton h={420} r={22} />
  const m = data.member
  const subs = data.subscriptions
  const sub = subs[Math.min(subIdx, subs.length - 1)]
  const y = year ?? years[0] ?? new Date().getFullYear()
  const charges = data.charges.filter((c: any) => c.year === y && (subs.length === 1 || c.subscription_id === sub?.id))
  const unpaid = data.charges.filter((c: any) => c.status === 'open' || c.status === 'past_due')
  const debt = data.charges.filter((c: any) => c.status === 'past_due').reduce((a: number, c: any) => a + c.amount, 0)
  const paidMap: Record<number, string> = {}
  charges.forEach((c: any) => { paidMap[c.month - 1] = c.status })
  const run = async (key: string, f: () => Promise<any>, ok: string) => { setBusy(key); try { const r = await f(); toast(typeof ok === 'string' ? ok.replace('{label}', r?.skipped || '') : ok); reload(); sx.bump() } catch (e) { toast(errMsg(e), true) } finally { setBusy('') } }
  const live = sub && ['active', 'past_due', 'paused', 'suspended'].includes(sub.status)
  const nextCharge = sub && data.charges.filter((c: any) => c.subscription_id === sub.id && (c.status === 'scheduled' || c.status === 'open')).sort((a: any, b: any) => a.due_iso.localeCompare(b.due_iso))[0]
  const nextDue = nextCharge?.due || sub?.next_due
  return (
    <div>
      <Link to="/painel/assinaturas/membros" style={{ fontSize: 13, color: 'var(--text-2)' }}>‹ Membros</Link>
      <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginTop: 6, flexWrap: 'wrap' }}><h2 className="display" style={{ margin: 0, fontSize: 24 }}>{m.name}</h2><StatusPill map={SUB_STATUS} s={m.status} /></div>
      {subs.length > 1 && <div className="seg" style={{ display: 'inline-flex', marginTop: 12 }}>{subs.map((s: any, i: number) => <button key={s.id} className={i === subIdx ? 'on' : ''} onClick={() => setSubIdx(i)}>{s.plan.name}</button>)}</div>}
      <div style={{ display: 'grid', gridTemplateColumns: 'minmax(0,1.7fr) minmax(260px,1fr)', gap: 14, marginTop: 16, alignItems: 'start' }} className="mb-grid">
        <style>{`@media (max-width:1000px){.mb-grid{grid-template-columns:1fr!important}} .ch-row{display:grid;grid-template-columns:110px 90px minmax(0,1fr) auto;gap:12px;align-items:center;padding:12px 20px;border-bottom:1px solid var(--line-3);font-size:14px} @media (max-width:600px){.ch-row{grid-template-columns:1fr auto;}.ch-row .ch-d{grid-column:1/-1}}`}</style>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 14, minWidth: 0 }}>
          <div className="card" style={{ overflow: 'hidden' }}>
            <div className="between" style={{ padding: '16px 20px', borderBottom: '1px solid var(--line-3)' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}><b style={{ fontSize: 15.5 }}>Cobranças</b>{years.length > 1 ? <select className="input" style={{ height: 30, width: 'auto', fontSize: 13 }} value={y} onChange={e => setYear(Number(e.target.value))}>{years.map(x => <option key={x}>{x}</option>)}</select> : <b style={{ fontSize: 15.5 }}>{y}</b>}</div>
              <div style={{ fontSize: 13, color: debt ? 'var(--sun-text)' : 'var(--muted)', fontWeight: debt ? 600 : 400 }}>{debt ? `Em dívida ${money(debt)} MT` : 'Sem dívida'}</div>
            </div>
            <div style={{ padding: '12px 20px 4px' }}><MonthStrip mask={sub?.plan.months ?? 0xfff} paid={paidMap} size={24} /></div>
            {charges.length === 0 && <div style={{ padding: 20, color: 'var(--muted)', fontSize: 14 }}>Sem cobranças neste ano.</div>}
            {charges.map((c: any) => (
              <div key={c.id}>
                <div className="ch-row" style={{ background: c.status === 'past_due' ? '#FFFCF0' : undefined, cursor: 'pointer' }} onClick={() => setOpen(open === c.id ? null : c.id)}>
                  <b>{c.label.split(' ')[0]}{subs.length > 1 && <span style={{ display: 'block', fontWeight: 400, fontSize: 12, color: 'var(--muted)' }}>{c.plan_name}</span>}</b>
                  <span className="tnum">{money(c.amount)},00</span>
                  <span className="ch-d" style={{ display: 'flex', gap: 8, alignItems: 'center', minWidth: 0 }}><StatusPill map={CHARGE_STATUS} s={c.status} /><span style={{ fontSize: 13, color: 'var(--muted)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{chargeDetail(c, data.messages)}</span></span>
                  <span style={{ textAlign: 'right' }}>
                    {['open', 'past_due', 'scheduled'].includes(c.status) && <button className="link-btn" style={{ fontSize: 13 }} onClick={e => { e.stopPropagation(); setMark([c.id]) }}>Marcar como pago</button>}
                    {c.has_proof && <a href={g.subscriptions.proofUrl(c.id)} onClick={e => e.stopPropagation()} style={{ fontSize: 13 }}>Comprovativo</a>}
                  </span>
                </div>
                {open === c.id && <Timeline c={c} msgs={data.messages} tolerance={data.tolerance_days} finalAction={data.final_action} />}
              </div>
            ))}
          </div>
          <div className="card" style={{ overflow: 'hidden' }}>
            <div style={{ padding: '16px 20px', borderBottom: '1px solid var(--line-3)', fontWeight: 700, fontSize: 15.5 }}>Mensagens <span style={{ fontWeight: 400, fontSize: 12.5, color: 'var(--muted)' }}>· SMS e WhatsApp simulados nesta versão</span></div>
            {data.messages.length === 0 && <div style={{ padding: 20, color: 'var(--muted)', fontSize: 14 }}>Ainda sem mensagens.</div>}
            {data.messages.slice(0, 12).map((x: any) => (
              <div key={x.id} style={{ padding: '11px 20px', borderBottom: '1px solid var(--line-3)', display: 'flex', gap: 12 }}>
                <span style={{ width: 8, height: 8, borderRadius: '50%', background: x.status === 'failed' ? 'var(--red)' : x.status === 'queued' ? '#C9C7BE' : 'var(--green)', marginTop: 6, flex: 'none' }} />
                <div style={{ minWidth: 0, flex: 1 }}>
                  <div style={{ fontSize: 13, display: 'flex', gap: 8, flexWrap: 'wrap' }}><b>{KIND[x.kind] || x.kind}</b><span style={{ color: 'var(--muted)' }}>{CHANNEL[x.channel]} · {x.to} · {hm(x.created_at)}</span>{x.status === 'failed' && <span style={{ color: 'var(--red-dark)', fontWeight: 600 }}>Falhou: {x.error}</span>}</div>
                  <div style={{ fontSize: 13, color: 'var(--text-2)', marginTop: 3, lineHeight: 1.45 }}>{x.body}</div>
                </div>
              </div>
            ))}
          </div>
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
          {sub && <div className="card" style={{ padding: '6px 18px' }}>
            {[['Plano', `${sub.plan.name} · v${sub.plan_version}`], ['Estado', <StatusPill map={SUB_STATUS} s={sub.status} />], ['Quem paga', m.payer_name || '—'], ['Número', `${m.phone} · ${CHANNEL[m.channel] || 'SMS'}`],
              ...Object.entries(m.fields || {}).filter(([k]) => k !== sub.plan.fields?.[0]).map(([k, v]) => [k, v]),
              ['Cobra', sub.plan.schedule_label], ['Desde', sub.since_label], ...(nextDue && sub.status !== 'canceled' ? [['Próxima', nextDue]] : []), ...(sub.paused_until ? [['Pausada até', sub.paused_until]] : [])].map(([k, v]: any, i, arr) => (
              <div key={k} className="between" style={{ padding: '10px 0', borderBottom: i < arr.length - 1 ? '1px solid var(--line-3)' : 0, fontSize: 14, gap: 12 }}><span style={{ color: 'var(--muted)' }}>{k}</span><span style={{ fontWeight: 600, textAlign: 'right' }}>{v}</span></div>
            ))}
            {sub.pending && <div style={{ margin: '4px 0 10px', padding: 10, borderRadius: 10, background: 'var(--sun-soft)', fontSize: 12.5, color: '#5B4600' }}>Passa a {money(sub.pending.amount)} MT (v{sub.pending.version}) a partir de {sub.pending.from}.</div>}
          </div>}
          {sub && <div className="card" style={{ padding: 6 }}>
            {[
              unpaid.length > 0 && ['Reenviar link', 'green', () => run('resend', () => g.subscriptions.resend(m.id), 'Link reenviado')],
              live && sub.status !== 'paused' && ['Saltar próxima cobrança', '', () => run('skip', () => g.subscriptions.action(sub.id, 'skip'), '{label} não será cobrado')],
              live && sub.status !== 'paused' && ['Pausar até…', '', () => setDialog('pause')],
              (sub.status === 'paused' || sub.status === 'suspended') && ['Retomar', 'green', () => run('resume', () => g.subscriptions.action(sub.id, 'resume'), 'Assinatura retomada')],
              live && ['Mudar de plano', '', () => setDialog('plan')],
              data.test_mode && live && ['Enviar próximo link agora (teste)', '', () => run('send', () => g.subscriptions.action(sub.id, 'send-now'), 'Link enviado')],
              sub.status !== 'canceled' && ['Cancelar assinatura', 'red', () => setDialog('cancel')],
            ].filter(Boolean).map(([l, tone, f]: any) => (
              <button key={l} onClick={f} disabled={!!busy} style={{ display: 'flex', width: '100%', alignItems: 'center', justifyContent: 'space-between', padding: '12px 12px', border: 0, background: 'none', borderRadius: 10, fontWeight: 600, fontSize: 14, color: tone === 'green' ? 'var(--green-text)' : tone === 'red' ? 'var(--red-dark)' : 'var(--ink)', textAlign: 'left' }}>{l}{busy && l.startsWith('Reenviar') && busy === 'resend' ? <Spinner dark /> : <Icon name="chevronRight" size={14} color="#B9BEB8" />}</button>
            ))}
          </div>}
          {sub?.consent_text && <div style={{ fontSize: 12, color: 'var(--muted)', lineHeight: 1.5, padding: '0 4px' }}><b>Acordo aceite</b> {sub.consent_at && new Date(sub.consent_at).toLocaleString('pt-PT')}: “{sub.consent_text}”</div>}
        </div>
      </div>
      {mark && <MarkPaid multi={subs.length > 1} member={m} charges={data.charges.filter((c: any) => ['open', 'past_due', 'scheduled'].includes(c.status))} initial={mark} requireProof={data.require_proof} receipt={data.receipt_sms} onClose={() => setMark(null)} onDone={() => { setMark(null); reload(); sx.bump() }} />}
      {dialog === 'pause' && <PauseDialog onClose={() => setDialog(null)} onSave={until => run('pause', () => g.subscriptions.action(sub.id, 'pause', { until }), 'Assinatura pausada').then(() => setDialog(null))} />}
      {dialog === 'plan' && <PlanDialog plans={sx.plans.filter((p: any) => p.status === 'active' && p.id !== sub.plan.id)} onClose={() => setDialog(null)} onSave={pid => run('plan', () => g.subscriptions.action(sub.id, 'change-plan', { plan_id: pid }), 'Plano alterado').then(() => setDialog(null))} />}
      {dialog === 'cancel' && (
        <Modal onClose={() => setDialog(null)} width={440}>
          <div className="display" style={{ fontSize: 22 }}>Cancelar assinatura?</div>
          <div style={{ fontSize: 14, color: 'var(--muted)', marginTop: 8, lineHeight: 1.5 }}>{m.name} deixa de receber links de {sub.plan.name}. As cobranças agendadas são canceladas{debt ? '; a dívida fica registada no histórico' : ''}. Avisamos por {CHANNEL[m.channel] || 'SMS'}.</div>
          <div style={{ display: 'flex', gap: 10, justifyContent: 'flex-end', marginTop: 18 }}><button className="btn btn-outline" onClick={() => setDialog(null)}>Voltar</button><button className="btn btn-danger" onClick={() => run('cancel', () => g.subscriptions.action(sub.id, 'cancel'), 'Assinatura cancelada').then(() => setDialog(null))}>Cancelar assinatura</button></div>
        </Modal>
      )}
    </div>
  )
}

function Timeline({ c, msgs, tolerance, finalAction }: { c: any; msgs: any[]; tolerance: number; finalAction: string }) {
  const mine = msgs.filter(m => m.charge_id === c.id).slice().reverse()
  const items: [string, string, string][] = mine.map(m => [m.status === 'failed' ? 'red' : 'green', `${KIND[m.kind] || m.kind}${m.status === 'failed' ? ' não entregue' : ' entregue'}`, `${hm(m.created_at)} · ${CHANNEL[m.channel]}`])
  if (c.opened_at) items.push(['green', 'Link aberto', hm(c.opened_at)])
  if (c.last_failure) items.push(['red', c.last_failure, 'última tentativa'])
  if (c.status === 'past_due' || c.status === 'open') {
    const due = new Date(c.due_iso); const end = new Date(due); end.setDate(end.getDate() + tolerance)
    const act = { suspend: 'Suspende', cancel: 'Cancela', keep: 'Continua ativa; fica em dívida' }[finalAction] || 'Ação final'
    items.push(['grey', finalAction === 'keep' ? act : `${act} a ${end.getDate()} ${['jan', 'fev', 'mar', 'abr', 'mai', 'jun', 'jul', 'ago', 'set', 'out', 'nov', 'dez'][end.getMonth()]}`, `tolerância de ${tolerance} dias`])
  }
  return (
    <div style={{ padding: '6px 20px 16px 40px', background: '#FBFBF8', borderBottom: '1px solid var(--line-3)' }}>
      {items.length === 0 && <div style={{ fontSize: 13, color: 'var(--muted)', paddingTop: 8 }}>Sem eventos.</div>}
      {items.map(([tone, t, s], i) => (
        <div key={i} style={{ display: 'flex', gap: 12, padding: '8px 0', position: 'relative' }}>
          <span style={{ width: 10, height: 10, borderRadius: '50%', marginTop: 4, flex: 'none', background: tone === 'red' ? 'var(--red)' : tone === 'grey' ? '#C9C7BE' : 'var(--green)' }} />
          <div><div style={{ fontSize: 13.5, fontWeight: 600 }}>{t}</div><div style={{ fontSize: 12.5, color: 'var(--muted)' }}>{s}</div></div>
        </div>
      ))}
    </div>
  )
}

function MarkPaid({ multi, member, charges, initial, requireProof, receipt, onClose, onDone }: { multi: boolean; member: any; charges: any[]; initial: string[]; requireProof: boolean; receipt: boolean; onClose: () => void; onDone: () => void }) {
  const g = useGateway()
  const { toast } = useSession()
  const [sel, setSel] = useState<Record<string, boolean>>(Object.fromEntries(initial.map(i => [i, true])))
  const [method, setMethod] = useState('cash')
  const today = new Date(); const iso = `${today.getFullYear()}-${String(today.getMonth() + 1).padStart(2, '0')}-${String(today.getDate()).padStart(2, '0')}`
  const [date, setDate] = useState(iso)
  const [ref, setRef] = useState('')
  const [proof, setProof] = useState<File | null>(null)
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')
  const sorted = charges.slice().sort((a, b) => a.due_iso.localeCompare(b.due_iso))
  const total = sorted.filter(c => sel[c.id]).reduce((a, c) => a + c.amount, 0)
  const go = async () => {
    setBusy(true); setErr('')
    try { await g.subscriptions.markPaidOutside(member.id, { charge_ids: Object.keys(sel).filter(k => sel[k]), method, date, reference: ref, proof }); toast('Marcado como pago fora da RIHA'); onDone() }
    catch (e) { setErr(errMsg(e)) } finally { setBusy(false) }
  }
  return (
    <Drawer title="Marcar como pago" onClose={onClose} footer={<><button className="btn btn-outline" onClick={onClose}>Cancelar</button><button className="btn btn-ink" style={{ flex: 1 }} disabled={busy || total === 0 || (requireProof && !proof)} onClick={go}>{busy ? <Spinner /> : `Marcar ${money(total)} MT como pago`}</button></>}>
      <div style={{ padding: '4px 26px 20px' }}>
        <div style={{ fontSize: 13, color: 'var(--muted)' }}>{member.name}</div>
        <span className="label" style={{ marginTop: 16, display: 'block' }}>Cobranças</span>
        <div className="card" style={{ overflow: 'hidden' }}>
          {sorted.map((c, i) => (
            <label key={c.id} style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '12px 14px', borderBottom: i < sorted.length - 1 ? '1px solid var(--line-3)' : 0, cursor: 'pointer' }}>
              <input type="checkbox" checked={!!sel[c.id]} onChange={e => setSel({ ...sel, [c.id]: e.target.checked })} style={{ width: 18, height: 18, accentColor: 'var(--green)' }} />
              <span style={{ flex: 1, fontSize: 14 }}><b>{c.label.split(' ')[0]}</b> <span style={{ color: 'var(--muted)' }}>· {c.status === 'past_due' ? 'em dívida' : `vence ${c.due}`}{multi && c.plan_name ? ` · ${c.plan_name}` : ''}</span></span>
              <span className="tnum">{money(c.amount)},00</span>
            </label>
          ))}
        </div>
        <span className="label" style={{ marginTop: 16, display: 'block' }}>Como pagou</span>
        <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>{OUTSIDE_METHODS.map(([k, l]) => <button key={k} className={'btn btn-sm ' + (method === k ? 'btn-ink' : 'btn-outline')} onClick={() => setMethod(k)} style={{ borderRadius: 999 }}>{l}</button>)}</div>
        <div className="grid-2" style={{ gap: 10, marginTop: 14 }}>
          <label><span className="label">Data</span><input className="input" type="date" value={date} max={iso} onChange={e => setDate(e.target.value)} /></label>
          <label><span className="label">Referência</span><input className="input" value={ref} onChange={e => setRef(e.target.value)} placeholder="Opcional" /></label>
        </div>
        <label style={{ display: 'block', marginTop: 14, padding: 16, borderRadius: 14, border: '1.5px dashed #D6D4CB', textAlign: 'center', fontSize: 13.5, color: 'var(--text-2)', cursor: 'pointer', background: proof ? '#fff' : '#FAFAF7' }}>
          {proof ? <><b>{proof.name}</b> · {Math.round(proof.size / 1000)} KB · <span className="link-btn">trocar</span></> : <>Arraste o comprovativo ou <span className="link-btn">escolha um ficheiro</span>{requireProof ? ' (obrigatório)' : ' (opcional)'}</>}
          <input type="file" accept=".pdf,.jpg,.jpeg,.png" hidden onChange={e => setProof(e.target.files?.[0] || null)} />
        </label>
        <div className="card" style={{ marginTop: 14, padding: 14, fontSize: 13, lineHeight: 1.7 }}>
          <div>Os lembretes param e {member.name.split(' ')[0]} volta a Ativa.</div>
          {receipt && <div>{member.payer_name ? 'O encarregado' : 'O cliente'} recebe um recibo por {CHANNEL[member.channel] || 'SMS'}.</div>}
          <div style={{ fontWeight: 700 }}>Não entra no saldo RIHA nem nos levantamentos, e não paga taxa.</div>
        </div>
        {err && <div className="form-err" style={{ marginTop: 12 }}>{err}</div>}
      </div>
    </Drawer>
  )
}

function PauseDialog({ onClose, onSave }: { onClose: () => void; onSave: (until: string) => void }) {
  const d = new Date(); d.setMonth(d.getMonth() + 2, 1)
  const [v, setV] = useState(`${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-01`)
  return (
    <Modal onClose={onClose} width={420}>
      <div className="display" style={{ fontSize: 22 }}>Pausar até</div>
      <div style={{ fontSize: 14, color: 'var(--muted)', marginTop: 6, lineHeight: 1.5 }}>Não há cobranças até essa data. A assinatura retoma sozinha.</div>
      <input className="input" type="date" value={v} onChange={e => setV(e.target.value)} style={{ marginTop: 14 }} />
      <div style={{ display: 'flex', gap: 10, justifyContent: 'flex-end', marginTop: 18 }}><button className="btn btn-outline" onClick={onClose}>Cancelar</button><button className="btn btn-ink" onClick={() => onSave(v)}>Pausar</button></div>
    </Modal>
  )
}

function PlanDialog({ plans, onClose, onSave }: { plans: any[]; onClose: () => void; onSave: (id: string) => void }) {
  const [v, setV] = useState(plans[0]?.id || '')
  return (
    <Modal onClose={onClose} width={460}>
      <div className="display" style={{ fontSize: 22 }}>Mudar de plano</div>
      <div style={{ fontSize: 14, color: 'var(--muted)', marginTop: 6, lineHeight: 1.5 }}>As próximas cobranças passam a seguir o novo plano. O que já foi cobrado não muda.</div>
      {plans.length === 0 ? <div style={{ marginTop: 14 }}>Não há outros planos ativos.</div> : <select className="input" value={v} onChange={e => setV(e.target.value)} style={{ marginTop: 14 }}>{plans.map(p => <option key={p.id} value={p.id}>{p.name} · {money(p.amount)} MT</option>)}</select>}
      <div style={{ display: 'flex', gap: 10, justifyContent: 'flex-end', marginTop: 18 }}><button className="btn btn-outline" onClick={onClose}>Cancelar</button><button className="btn btn-ink" disabled={!v} onClick={() => onSave(v)}>Mudar</button></div>
    </Modal>
  )
}
