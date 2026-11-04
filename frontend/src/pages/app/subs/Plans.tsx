import { useEffect, useMemo, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { Avatar, Icon, Modal, QR, Skeleton, Spinner } from '../../../components/ui'
import { useGateway } from '../../../core/gateway'
import { errMsg, type ApiError } from '../../../core/model'
import { useQuery } from '../../../core/useQuery'
import { copy, initials, parseMT } from '../../../lib/format'
import { useSession } from '../../../lib/session'
import { useShell } from '../AppShell'
import { agreementPreview, ALL_MONTHS, downloadSvg, KV, money, MonthStrip, nextOnDue, PLAN_STATUS, Radio, Section, StatusPill } from './common'
import { useSubs } from './SubsShell'

const MISSED: Record<string, string> = { debt: 'Fica em dívida', forfeit: 'Perde esse mês' }

export function PlansList() {
  const nav = useNavigate()
  const sx = useSubs()
  if (!sx.plans) return <Skeleton h={240} r={22} />
  return (
    <div className="table" style={{ overflowX: 'auto' }}>
      <div style={{ minWidth: 860 }}>
        <div className="thead" style={{ gridTemplateColumns: '2fr 100px 1.3fr 1fr 80px 100px 100px' }}><span>Plano</span><span style={{ textAlign: 'right' }}>Valor</span><span>Cobra</span><span>Se não pagar</span><span style={{ textAlign: 'right' }}>Membros</span><span style={{ textAlign: 'right' }}>Por mês</span><span>Estado</span></div>
        {sx.plans.length === 0 && <div style={{ padding: 28, textAlign: 'center', color: 'var(--muted)', fontSize: 14 }}>Ainda não há planos. <Link to="/painel/assinaturas/planos/novo" style={{ fontWeight: 600 }}>Criar o primeiro</Link></div>}
        {sx.plans.map((p: any) => (
          <div key={p.id} className="trow click" style={{ gridTemplateColumns: '2fr 100px 1.3fr 1fr 80px 100px 100px' }} onClick={() => nav('/painel/assinaturas/planos/' + p.id)}>
            <span style={{ fontWeight: 600, fontSize: 14 }}>{p.name}</span>
            <span className="tnum" style={{ textAlign: 'right', fontSize: 14 }}>{money(p.amount)} MT</span>
            <span style={{ fontSize: 13.5, color: 'var(--text-2)' }}>{p.schedule_label}</span>
            <span style={{ fontSize: 13.5, color: 'var(--text-2)' }}>{MISSED[p.missed]}</span>
            <span className="tnum" style={{ textAlign: 'right', fontSize: 14 }}>{p.stats?.members ?? 0}</span>
            <span className="tnum" style={{ textAlign: 'right', fontSize: 14 }}>{p.stats?.monthly ? money(p.stats.monthly) : '—'}</span>
            <span><StatusPill map={PLAN_STATUS} s={p.status} /></span>
          </div>
        ))}
      </div>
    </div>
  )
}

type Form = { name: string; amount: string; interval: string; anchor: string; billing_day: number; months: number; missed: string; fields: string[]; ends_at: string }
const EMPTY: Form = { name: '', amount: '', interval: 'month', anchor: 'fixed', billing_day: 5, months: ALL_MONTHS, missed: 'debt', fields: ['Nome do aluno'], ends_at: '' }
const WEEKDAYS = ['segunda', 'terça', 'quarta', 'quinta', 'sexta', 'sábado', 'domingo']

export function PlanForm() {
  const { id } = useParams()
  const g = useGateway()
  const nav = useNavigate()
  const { toast } = useSession()
  const shell = useShell()
  const sx = useSubs()
  const [f, setF] = useState<Form>(EMPTY)
  const [loaded, setLoaded] = useState(!id)
  const [field, setField] = useState('')
  const [adding, setAdding] = useState(false)
  const [err, setErr] = useState<ApiError | null>(null)
  const [busy, setBusy] = useState(false)
  const [plan, setPlan] = useState<any>(null)
  useEffect(() => {
    if (!id) return
    g.subscriptions.plan(id).then(p => {
      setPlan(p)
      setF({ name: p.name, amount: String(p.amount / 100).replace('.', ','), interval: p.interval, anchor: p.anchor, billing_day: p.billing_day || 5, months: p.months, missed: p.missed, fields: p.fields, ends_at: p.ends_at ? p.ends_at.slice(0, 10) : '' })
      setLoaded(true)
    }).catch(e => toast(errMsg(e), true))
  }, [id])
  const set = (patch: Partial<Form>) => setF({ ...f, ...patch })
  const amount = parseMT(f.amount)
  const hasMembers = (plan?.stats?.members || 0) > 0
  const submit = async (draft: boolean) => {
    setBusy(true); setErr(null)
    const body = { name: f.name, amount: id ? plan.amount : amount, interval: f.interval, anchor: f.anchor, billing_day: f.anchor === 'fixed' ? Number(f.billing_day) : 0, months: f.months, missed: f.missed, fields: f.fields, ends_at: f.ends_at ? new Date(f.ends_at + 'T23:59:59+02:00').toISOString() : null, draft }
    try {
      const p = id ? await g.subscriptions.updatePlan(id, body) : await g.subscriptions.createPlan(body)
      sx.reloadPlans(); sx.bump()
      toast(id ? 'Plano guardado' : draft ? 'Rascunho guardado' : 'Plano criado')
      nav('/painel/assinaturas/planos/' + p.id)
    } catch (e) { setErr(e as ApiError) } finally { setBusy(false) }
  }
  const per = f.interval === 'week' ? 'semana' : f.interval === 'quarter' ? 'trimestre' : f.interval === 'year' ? 'ano' : 'mês'
  const dayLabel = f.interval === 'week' ? `Todos pagam à ${WEEKDAYS[(f.billing_day + 6) % 7]}.` : `Todos pagam no dia ${f.billing_day}. Quem entra a meio paga esse ${per} ao subscrever.`
  const preview = { anchor: f.anchor, billing_day: f.billing_day, interval: f.interval, months: f.months, amount: id ? plan?.amount || 0 : amount }
  if (!loaded) return <Skeleton h={500} r={22} />
  return (
    <div style={{ display: 'grid', gridTemplateColumns: 'minmax(0,1fr) minmax(280px,380px)', gap: 20, alignItems: 'start' }} className="plan-form">
      <style>{`@media (max-width:1100px){.plan-form{grid-template-columns:1fr!important}} @media (max-width:700px){.pf-row{grid-template-columns:1fr!important}.plan-form .grid-2{grid-template-columns:1fr!important}}`}</style>
      <div className="card" style={{ padding: 24 }}>
        <div className="between"><div className="display" style={{ fontSize: 22 }}>{id ? 'Editar plano' : 'Novo plano'}</div>{id && <Link to={'/painel/assinaturas/planos/' + id} style={{ fontSize: 13 }}>Voltar</Link>}</div>
        <div style={{ display: 'grid', gridTemplateColumns: '1.4fr 1fr', gap: 12, marginTop: 18 }} className="pf-row">
          <label><span className="label">Nome</span><input className={'input' + (err?.param === 'name' ? ' err' : '')} value={f.name} onChange={e => set({ name: e.target.value })} placeholder="Propina · 1ª a 4ª classe" /></label>
          <label><span className="label">Valor</span><div className="phone-input"><input inputMode="decimal" value={id ? money(plan.amount) : f.amount} disabled={!!id} onChange={e => set({ amount: e.target.value })} placeholder="2 500,00" style={{ textAlign: 'right' }} /><span className="cc" style={{ borderLeft: '1px solid #E1DFD7', borderRight: 0 }}>MT</span></div>{id && <div style={{ fontSize: 12, color: 'var(--muted)', marginTop: 5 }}>Use Mudar valor no plano.</div>}</label>
        </div>
        <span className="label" style={{ marginTop: 16, display: 'block' }}>Frequência</span>
        <div className="seg" style={{ display: 'inline-flex' }}>
          {[['week', 'Semanal'], ['month', 'Mensal'], ['quarter', 'Trimestral'], ['year', 'Anual']].map(([k, l]) => <button key={k} type="button" className={f.interval === k ? 'on' : ''} disabled={hasMembers} onClick={() => set({ interval: k, billing_day: k === 'week' ? 1 : 5 })}>{l}</button>)}
        </div>
        <span className="label" style={{ marginTop: 18, display: 'block' }}>Quando cobrar</span>
        <div className="grid-2" style={{ gap: 10 }}>
          <Radio on={f.anchor === 'fixed'} title="Dia fixo para todos" sub={dayLabel} onClick={() => !hasMembers && set({ anchor: 'fixed' })} />
          <Radio on={f.anchor === 'signup'} title="No dia em que subscreve" sub={`Quem subscreve a 18 paga sempre a 18.`} onClick={() => !hasMembers && set({ anchor: 'signup' })} />
        </div>
        {f.anchor === 'fixed' && (
          <label style={{ display: 'flex', alignItems: 'center', gap: 10, marginTop: 10, fontSize: 14 }}>
            {f.interval === 'week' ? 'Dia da semana' : 'Dia de cobrança'}
            <select className="input" style={{ width: 160, height: 40 }} value={f.billing_day} disabled={hasMembers} onChange={e => set({ billing_day: Number(e.target.value) })}>
              {f.interval === 'week' ? WEEKDAYS.map((w, i) => <option key={w} value={i + 1}>{w}</option>) : Array.from({ length: 28 }, (_, i) => <option key={i} value={i + 1}>Dia {i + 1}</option>)}
            </select>
          </label>
        )}
        <div className="between" style={{ marginTop: 18 }}><span className="label" style={{ margin: 0 }}>Meses com cobrança</span><span style={{ display: 'flex', gap: 12, fontSize: 13 }}><button type="button" className="link-btn" onClick={() => set({ months: ALL_MONTHS })}>Todos</button><button type="button" className="link-btn" onClick={() => set({ months: 0 })}>Limpar</button></span></div>
        <div style={{ marginTop: 8 }}><MonthStrip mask={f.months} size={40} onToggle={i => set({ months: f.months ^ (1 << i) })} /></div>
        <div style={{ fontSize: 12.5, color: 'var(--muted)', marginTop: 6 }}>Toque num mês para não cobrar nesse mês, por exemplo numa pausa sazonal.{f.interval !== 'month' && ' Num plano semanal, trimestral ou anual, uma cobrança que caia num mês desligado é saltada.'}</div>
        <span className="label" style={{ marginTop: 18, display: 'block' }}>Se um mês não for pago</span>
        <div className="grid-2" style={{ gap: 10 }}>
          <Radio on={f.missed === 'debt'} title="Fica em dívida" sub="O mês em falta junta-se ao seguinte. Continua ativo durante a tolerância." onClick={() => set({ missed: 'debt' })} />
          <Radio on={f.missed === 'forfeit'} title="Perde esse mês" sub="A cobrança fecha sem dívida e o serviço fica suspenso nesse mês." onClick={() => set({ missed: 'forfeit' })} />
        </div>
        <div className="grid-2" style={{ gap: 14, marginTop: 18 }}>
          <div>
            <span className="label">Perguntar ao cliente</span>
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
              {f.fields.map((x, i) => <span key={x} className="chip" style={{ background: 'var(--paper-2)', fontFamily: 'var(--sans)', fontWeight: 600 }}>{x}{i === 0 && <span style={{ fontWeight: 400, color: 'var(--muted)' }}>· nome</span>}<button type="button" onClick={() => set({ fields: f.fields.filter(y => y !== x) })} aria-label={'Remover ' + x} style={{ border: 0, background: 'none', padding: 0, marginLeft: 2 }}>×</button></span>)}
              {adding ? <form onSubmit={e => { e.preventDefault(); if (field.trim() && !f.fields.includes(field.trim()) && f.fields.length < 6) set({ fields: [...f.fields, field.trim()] }); setField(''); setAdding(false) }} style={{ display: 'flex' }}><input className="input" autoFocus value={field} onChange={e => setField(e.target.value)} onBlur={() => setAdding(false)} placeholder="Turma" style={{ height: 30, width: 130, fontSize: 13 }} /></form>
                : <button type="button" className="chip" onClick={() => setAdding(true)} style={{ border: '1px dashed #C9C7BE', background: '#fff', fontFamily: 'var(--sans)' }}>+ Campo</button>}
            </div>
            <div style={{ fontSize: 12, color: 'var(--muted)', marginTop: 6 }}>O primeiro campo é o nome do membro, por exemplo o aluno.</div>
          </div>
          <div>
            <span className="label">Lembretes e atrasos</span>
            <div className="input between" style={{ display: 'flex', alignItems: 'center' }}><span>Padrão da conta</span><Link to="/painel/assinaturas/definicoes" style={{ fontSize: 13 }}>Ver</Link></div>
            <label style={{ display: 'block', marginTop: 10 }}><span className="label">Termina (opcional)</span><input className="input" type="date" value={f.ends_at} onChange={e => set({ ends_at: e.target.value })} /></label>
          </div>
        </div>
        {err && <div className="form-err" style={{ marginTop: 16 }}>{err.message}</div>}
        <div style={{ display: 'flex', gap: 10, justifyContent: 'flex-end', borderTop: '1px solid var(--line-3)', marginTop: 22, paddingTop: 18, flexWrap: 'wrap' }}>
          <button className="btn btn-outline" onClick={() => nav(id ? '/painel/assinaturas/planos/' + id : '/painel/assinaturas/planos')}>Cancelar</button>
          {!id && <button className="btn btn-outline" disabled={busy} onClick={() => submit(true)}>Guardar rascunho</button>}
          <button className="btn btn-green" disabled={busy} onClick={() => submit(false)}>{busy ? <Spinner /> : id ? (plan?.status === 'draft' ? 'Guardar e ativar' : 'Guardar') : 'Criar plano'}</button>
        </div>
      </div>
      <div>
        <div style={{ fontSize: 12, fontWeight: 700, color: 'var(--muted)', textTransform: 'uppercase', letterSpacing: '.06em', margin: '4px 0 10px' }}>O que o cliente vê</div>
        <div className="card" style={{ padding: 18 }}>
          <div style={{ display: 'flex', gap: 12, alignItems: 'center', paddingBottom: 14, borderBottom: '1px dashed #E1DFD7' }}>
            <Avatar text={initials(shell.merchant.name)} color={shell.merchant.avatar_color} size={40} radius={12} />
            <div><div style={{ fontWeight: 700 }}>{shell.merchant.name}</div><div style={{ fontSize: 12.5, color: 'var(--muted)' }}>{f.name || 'Nome do plano'}</div></div>
          </div>
          <div className="display tnum" style={{ fontSize: 30, marginTop: 14 }}>{money(preview.amount)} <span style={{ fontSize: 14, color: 'var(--muted)', fontFamily: 'var(--sans)' }}>MT por {per}</span></div>
          <div style={{ marginTop: 10 }}><MonthStrip mask={f.months} size={20} /></div>
          <div style={{ marginTop: 14, padding: 12, borderRadius: 12, background: 'var(--paper)', fontSize: 13, lineHeight: 1.5, color: 'var(--text-2)' }}>{agreementPreview(shell.merchant.name, preview)}</div>
          {f.fields.map(x => <div key={x} style={{ marginTop: 10 }}><div style={{ fontSize: 12, color: 'var(--muted)' }}>{x}</div><div style={{ height: 34, borderRadius: 10, border: '1px solid #E7E5DD', marginTop: 4 }} /></div>)}
        </div>
        <div style={{ fontSize: 12.5, color: 'var(--muted)', marginTop: 10, lineHeight: 1.5 }}>{f.missed === 'debt' ? 'Se falhar um mês, o valor fica em dívida e soma-se ao link seguinte.' : 'Se falhar um mês, esse mês fecha sem dívida e o serviço fica suspenso nesse período.'}</div>
      </div>
    </div>
  )
}

export function PlanDetail() {
  const { id } = useParams()
  const g = useGateway()
  const nav = useNavigate()
  const { toast } = useSession()
  const sx = useSubs()
  const [price, setPrice] = useState(false)
  const { data: p, reload } = useQuery(g => g.subscriptions.plan(id!), [id, sx.version])
  if (!p) return <Skeleton h={400} r={22} />
  const by = p.stats?.by_status || {}
  const activeCount = (by.active || 0) + (by.past_due || 0) + (by.paused || 0) + (by.suspended || 0)
  const archive = async () => { if (!confirm('Arquivar este plano? Deixa de aparecer no catálogo.')) return; try { await g.subscriptions.archivePlan(p.id); sx.reloadPlans(); nav('/painel/assinaturas/planos') } catch (e) { toast(errMsg(e), true) } }
  return (
    <div>
      <Link to="/painel/assinaturas/planos" style={{ fontSize: 13, color: 'var(--text-2)' }}>‹ Planos</Link>
      <div className="between" style={{ marginTop: 6, gap: 12, flexWrap: 'wrap' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}><h2 className="display" style={{ margin: 0, fontSize: 24 }}>{p.name}</h2><StatusPill map={PLAN_STATUS} s={p.status} /></div>
        <div style={{ display: 'flex', gap: 8 }}>
          {p.status !== 'archived' && <button className="btn btn-ink" onClick={() => setPrice(true)}>Mudar valor</button>}
          <button className="btn btn-outline" onClick={() => nav(`/painel/assinaturas/planos/${p.id}/editar`)}>Editar</button>
        </div>
      </div>
      <div style={{ display: 'grid', gridTemplateColumns: 'minmax(0,1.6fr) minmax(260px,1fr)', gap: 14, marginTop: 16, alignItems: 'start' }} className="pd-grid">
        <style>{`@media (max-width:1000px){.pd-grid{grid-template-columns:1fr!important}}`}</style>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 14, minWidth: 0 }}>
          <div className="card" style={{ padding: '4px 20px' }}>
            <KV k="Valor" v={<b>{money(p.amount)} MT por {p.interval_label}</b>} />
            <KV k="Quando cobra" v={p.anchor === 'signup' ? 'No dia em que cada membro subscreveu' : p.interval === 'week' ? p.when_label : `Dia fixo · dia ${p.billing_day}`} />
            <KV k="Meses" v={<MonthStrip mask={p.months} />} />
            <KV k="Se não pagar" v={p.missed === 'debt' ? 'Fica em dívida · junta-se ao mês seguinte' : 'Perde esse mês · o serviço fica suspenso nesse período'} />
            <KV k="Lembretes" v={<span>Padrão da conta · <Link to="/painel/assinaturas/definicoes">ver</Link></span>} />
            <KV k="Pergunta ao cliente" v={p.fields.join(', ') || '—'} last={!p.ends_at} />
            {p.ends_at && <KV k="Termina" v={new Date(p.ends_at).toLocaleDateString('pt-PT')} last />}
          </div>
          <Section title="Versões de preço" pad={false}>
            {p.versions.map((v: any) => (
              <div key={v.version} style={{ display: 'grid', gridTemplateColumns: '50px 1fr auto auto', gap: 16, padding: '13px 20px', background: v.current ? 'var(--green-tint)' : undefined, borderBottom: '1px solid var(--line-3)', fontSize: 14 }}>
                <span className="mono" style={{ color: 'var(--green-text)' }}>v{v.version}</span><b>{money(v.amount)} MT / {p.interval_label}</b><span style={{ color: 'var(--muted)' }}>desde {v.since}</span><span>{v.members} membros</span>
              </div>
            ))}
            <div style={{ padding: '12px 20px', fontSize: 13, color: 'var(--muted)' }}>Mudar o valor cria uma versão nova. Quem já está mantém a sua, a não ser que escolha mudar.{p.pending_changes > 0 && ` ${p.pending_changes} membros já têm uma mudança de valor agendada.`}</div>
          </Section>
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
          {p.status !== 'draft' && <div className="card" style={{ padding: 16, display: 'flex', gap: 14, alignItems: 'center' }}>
            <div id="plan-qr" style={{ padding: 6, borderRadius: 12, boxShadow: 'inset 0 0 0 1px #EEECE5', flex: 'none' }}><QR value={p.url} size={96} logo={false} /></div>
            <div style={{ minWidth: 0 }}>
              <div className="mono" style={{ fontSize: 12.5, wordBreak: 'break-all' }}>{p.url.replace(/^https?:\/\//, '')}</div>
              <div style={{ display: 'flex', gap: 12, marginTop: 8, fontSize: 13 }}>
                <button className="link-btn" onClick={async () => { await copy(p.url); toast('Link copiado') }}>Copiar</button>
                <button className="link-btn" onClick={() => downloadSvg('#plan-qr', `qr-${p.slug}.svg`)}>Descarregar QR</button>
                <a href={`https://wa.me/?text=${encodeURIComponent(`${p.name}: subscreva aqui ${p.url}`)}`} target="_blank" rel="noreferrer" className="link-btn">WhatsApp</a>
              </div>
            </div>
          </div>}
          <div className="card card-pad">
            <div style={{ fontSize: 13, color: 'var(--muted)' }}>Membros</div>
            <div className="display tnum" style={{ fontSize: 30, marginTop: 6 }}>{p.stats?.members || 0}</div>
            <div style={{ fontSize: 13, color: 'var(--muted)', marginTop: 4 }}>{by.active || 0} ativos · {by.past_due || 0} em dívida · {by.paused || 0} pausados · {by.invited || 0} convites</div>
            <div style={{ display: 'flex', gap: 8, marginTop: 14, flexWrap: 'wrap' }}>
              <button className="btn btn-outline btn-sm" onClick={() => nav('/painel/assinaturas/membros?plan=' + p.id)}>Ver membros</button>
              {p.status === 'active' && <button className="btn btn-green btn-sm" onClick={() => sx.openAdd('one', p.id)}>Adicionar membros</button>}
            </div>
          </div>
          {p.status !== 'archived' && activeCount === 0 && (by.invited || 0) === 0 && <button className="btn btn-ghost" style={{ color: 'var(--red-dark)' }} onClick={archive}>Arquivar plano</button>}
        </div>
      </div>
      {price && <ChangePrice plan={p} members={activeCount + (by.invited || 0)} onClose={() => setPrice(false)} onDone={() => { setPrice(false); reload(); sx.reloadPlans() }} />}
    </div>
  )
}

function ChangePrice({ plan, members, onClose, onDone }: { plan: any; members: number; onClose: () => void; onDone: () => void }) {
  const g = useGateway()
  const { toast } = useSession()
  const [v, setV] = useState('')
  const [scope, setScope] = useState('new')
  const [picked, setPicked] = useState<Record<string, boolean>>({})
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')
  const subs = useQuery(scope === 'selected' ? g => g.subscriptions.members({ plan: plan.id, limit: 200 }) : null, [scope])
  const first = useMemo(() => { const d = new Date(); d.setDate(d.getDate() + 30); return nextOnDue(plan, d) }, [plan])
  const firstLabel = first ? `${first.getDate()} ${['jan', 'fev', 'mar', 'abr', 'mai', 'jun', 'jul', 'ago', 'set', 'out', 'nov', 'dez'][first.getMonth()]} ${first.getFullYear()}` : 'a primeira cobrança depois do aviso'
  const amount = parseMT(v)
  const save = async () => {
    setBusy(true); setErr('')
    try {
      const ids: string[] = []
      if (scope === 'selected') {
        const chosen = Object.keys(picked).filter(k => picked[k])
        for (const mid of chosen) { const d = await g.subscriptions.member(mid); for (const s of d.subscriptions) if (s.plan.id === plan.id && s.status !== 'canceled') ids.push(s.id) }
        if (!ids.length) { setErr('Escolha pelo menos um membro.'); setBusy(false); return }
      }
      const r = await g.subscriptions.changePrice(plan.id, amount, scope, ids)
      toast(scope === 'new' ? `Versão v${r.plan.version} criada` : `${r.affected} membros avisados · novo valor a partir de ${r.effective}`)
      onDone()
    } catch (e) { setErr(errMsg(e)) } finally { setBusy(false) }
  }
  return (
    <Modal onClose={onClose} width={560}>
      <div className="display" style={{ fontSize: 22 }}>Mudar valor</div>
      <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginTop: 16 }}>
        <div className="input" style={{ flex: 1, textDecoration: 'line-through', color: 'var(--muted)', display: 'flex', alignItems: 'center' }}>{money(plan.amount)},00 MT</div>
        <Icon name="arrowRight" size={16} color="var(--muted)" />
        <div className="phone-input" style={{ flex: 1, border: '2px solid var(--ink)' }}><input autoFocus inputMode="decimal" value={v} onChange={e => setV(e.target.value)} placeholder="2 800,00" /><span className="cc" style={{ borderLeft: '1px solid #E1DFD7', borderRight: 0 }}>MT</span></div>
      </div>
      <div style={{ fontSize: 13.5, fontWeight: 600, marginTop: 18 }}>Quem passa a pagar o novo valor?</div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 8, marginTop: 8 }}>
        <Radio on={scope === 'new'} title="Só quem subscrever a partir de hoje" sub={`Os ${members} membros atuais continuam com ${money(plan.amount)} MT.`} onClick={() => setScope('new')} />
        <Radio on={scope === 'all'} title="Todos" sub={`Avisamos por SMS com 30 dias. Primeira cobrança com o novo valor: ${firstLabel}.`} onClick={() => setScope('all')} />
        <Radio on={scope === 'selected'} title="Escolher membros" sub="Um a um ou por campo, como a turma. Mesmo aviso." onClick={() => setScope('selected')} />
      </div>
      {scope === 'selected' && (
        <div style={{ maxHeight: 200, overflow: 'auto', border: '1px solid var(--line)', borderRadius: 12, marginTop: 10 }}>
          {!subs.data ? <div style={{ padding: 14 }}><Spinner dark /></div> : subs.data.data.map((m: any) => (
            <label key={m.id} style={{ display: 'flex', gap: 10, alignItems: 'center', padding: '9px 12px', borderBottom: '1px solid var(--line-3)', fontSize: 14 }}>
              <input type="checkbox" checked={!!picked[m.id]} onChange={e => setPicked({ ...picked, [m.id]: e.target.checked })} />
              <span style={{ flex: 1 }}>{m.name}</span><span style={{ fontSize: 12.5, color: 'var(--muted)' }}>{Object.entries(m.fields || {}).filter(([k]) => k !== Object.keys(m.fields || {})[0]).map(([, x]) => x).join(' · ')}</span>
            </label>
          ))}
        </div>
      )}
      {err && <div className="form-err" style={{ marginTop: 12 }}>{err}</div>}
      <div style={{ display: 'flex', gap: 10, justifyContent: 'flex-end', marginTop: 18 }}>
        <button className="btn btn-outline" onClick={onClose}>Cancelar</button>
        <button className="btn btn-ink" disabled={busy || amount <= 0} onClick={save}>{busy ? <Spinner /> : `Guardar como v${plan.version + 1}`}</button>
      </div>
    </Modal>
  )
}
