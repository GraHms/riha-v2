import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { Icon, Modal, Pill, QR, Skeleton, Spinner } from '../../../components/ui'
import { useGateway } from '../../../core/gateway'
import { errMsg, type ApiError } from '../../../core/model'
import { useQuery } from '../../../core/useQuery'
import { copy, initials, parseMT } from '../../../lib/format'
import { useSession } from '../../../lib/session'
import { BrandFrame, Hero, Highlights, PlanFacts, per, Price } from '../../public/subs/CatalogView'
import { useShell } from '../AppShell'
import { ALL_MONTHS, customerView, downloadSvg, KV, merchantWhen, money, monthsLabel, MonthStrip, nextOnDue, PLAN_STATUS, Radio, Section, StatusPill } from './common'
import { useSubs } from './SubsShell'

export async function sharePlan(name: string, url: string, toast: (m: string, err?: boolean) => void) {
  if (navigator.share && window.matchMedia('(max-width: 860px)').matches) {
    try { await navigator.share({ title: name, url }); return } catch { /* closed */ }
  }
  await copy(url); toast('Link copiado')
}

export function PlansList() {
  const nav = useNavigate()
  const sx = useSubs()
  const { toast } = useSession()
  if (!sx.plans) return <Skeleton h={240} r={22} />
  const plans = sx.plans as any[]
  const drafts = plans.filter(p => p.status === 'draft').length
  return (
    <div>
      <style>{`
        .plan-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(300px,1fr));gap:14px}
        .plan-tile{padding:18px 18px 14px;display:flex;flex-direction:column;gap:8px;cursor:pointer;text-align:left;transition:box-shadow .15s,border-color .15s}
        .plan-tile:hover{box-shadow:0 10px 28px -18px rgba(11,18,14,.45)}
        .plan-tile:focus-visible{outline:3px solid var(--ink);outline-offset:2px}
        .plan-new{border:1.5px dashed #CFCDC4;border-radius:22px;background:transparent;min-height:150px;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:8px;font:inherit;font-weight:600;color:var(--text-2);cursor:pointer}
        .plan-new:hover{border-color:var(--ink);color:var(--ink)}
        @media (max-width:640px){.plan-grid{grid-template-columns:1fr}}
      `}</style>
      {plans.length === 0 ? (
        <div className="card" style={{ padding: '36px 24px', textAlign: 'center' }}>
          <div className="display" style={{ fontSize: 22 }}>Crie o primeiro plano</div>
          <p style={{ color: 'var(--muted)', fontSize: 14.5, maxWidth: 420, margin: '8px auto 18px', lineHeight: 1.5 }}>Um plano é o que os seus clientes pagam com regularidade: uma propina, uma mensalidade do ginásio, um curso de 3 meses.</p>
          <button className="btn btn-green" onClick={sx.newPlan}><Icon name="plus" size={17} stroke={2.4} />Novo plano</button>
        </div>
      ) : (
        <>
          <div style={{ fontSize: 13.5, color: 'var(--muted)', margin: '0 2px 12px' }}>{plans.length} {plans.length === 1 ? 'plano' : 'planos'}{drafts ? ` · ${drafts} em rascunho` : ''} · toque num plano para ver os membros e o link</div>
          <div className="plan-grid">
            {plans.map(p => {
              const open = () => nav('/painel/assinaturas/planos/' + p.id)
              return (
                <div key={p.id} className="card plan-tile" role="link" tabIndex={0} onClick={open} onKeyDown={e => e.key === 'Enter' && open()}>
                  <div style={{ display: 'flex', gap: 10, alignItems: 'flex-start' }}>
                    <b style={{ flex: 1, fontSize: 15.5, lineHeight: 1.3 }}>{p.name}</b>
                    {p.status !== 'active' ? <StatusPill map={PLAN_STATUS} s={p.status} /> : !p.in_catalog && <Pill t="Fora do catálogo" bg="#EDECE6" fg="#3E4742" />}
                  </div>
                  <Price amount={p.amount} interval={p.interval} size={24} />
                  <div style={{ fontSize: 13.5, color: 'var(--text-2)' }}>{merchantWhen(p)}</div>
                  {(p.duration_label || p.highlights?.length > 0) && (
                    <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
                      {p.duration_label && <span className="chip" style={{ fontFamily: 'var(--sans)', background: 'var(--sun-bg)', color: 'var(--sun-text)', fontWeight: 600 }}>{p.duration_label}</span>}
                      {p.highlights?.slice(0, 2).map((h: string) => <span key={h} className="chip" style={{ fontFamily: 'var(--sans)', background: 'var(--paper-2)', maxWidth: '100%', overflow: 'hidden', textOverflow: 'ellipsis' }}>{h}</span>)}
                      {p.highlights?.length > 2 && <span className="chip" style={{ fontFamily: 'var(--sans)', background: 'var(--paper-2)' }}>+{p.highlights.length - 2}</span>}
                    </div>
                  )}
                  <div className="between" style={{ marginTop: 'auto', paddingTop: 10, borderTop: '1px solid var(--line-3)', fontSize: 13 }}>
                    <span style={{ color: 'var(--muted)' }}><b style={{ color: 'var(--ink)' }}>{p.stats?.members ?? 0}</b> {(p.stats?.members ?? 0) === 1 ? 'membro' : 'membros'}{p.stats?.monthly ? ` · ${money(p.stats.monthly)} MT/mês` : ''}</span>
                    <span style={{ display: 'flex', gap: 4 }} onClick={e => e.stopPropagation()}>
                      <button className="btn btn-ghost btn-sm" onClick={() => nav(`/painel/assinaturas/planos/${p.id}/editar`)}>Editar</button>
                      {p.status === 'active' && <button className="btn btn-ghost btn-sm" onClick={() => sharePlan(p.name, p.url, toast)} aria-label={'Partilhar ' + p.name}><Icon name="share" size={15} />Partilhar</button>}
                    </span>
                  </div>
                </div>
              )
            })}
            <button className="plan-new" onClick={sx.newPlan}><Icon name="plus" size={22} />Novo plano</button>
          </div>
        </>
      )}
    </div>
  )
}

type Duration = 'none' | 'cycles' | 'date'
type Form = { name: string; amount: string; interval: string; anchor: string; billing_day: number; months: number; missed: string; fields: string[]; highlights: string[]; duration: Duration; cycles: string; ends_at: string }
const EMPTY: Form = { name: '', amount: '', interval: 'month', anchor: 'fixed', billing_day: 5, months: ALL_MONTHS, missed: 'debt', fields: ['Nome do aluno'], highlights: [], duration: 'none', cycles: '3', ends_at: '' }
const WEEKDAYS = ['segunda', 'terça', 'quarta', 'quinta', 'sexta', 'sábado', 'domingo']
const WEEKDAY_OF = ['domingo', 'segunda', 'terça', 'quarta', 'quinta', 'sexta', 'sábado']

function Block({ title, hint, children, id }: { title: string; hint?: ReactNode; children: ReactNode; id?: string }) {
  return (
    <section className="card pf-block" id={id}>
      <h3 style={{ margin: 0, fontSize: 16, fontWeight: 700 }}>{title}</h3>
      {hint && <div style={{ fontSize: 13, color: 'var(--muted)', marginTop: 3, lineHeight: 1.45 }}>{hint}</div>}
      <div style={{ marginTop: 14 }}>{children}</div>
    </section>
  )
}

function joinList(xs: string[]) { return xs.length <= 1 ? xs.join('') : xs.slice(0, -1).join(', ') + ' e ' + xs[xs.length - 1] }

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
  const [pickMonths, setPickMonths] = useState(false)
  const [preview, setPreview] = useState(false)
  const [err, setErr] = useState<ApiError | null>(null)
  const [busy, setBusy] = useState(false)
  const [plan, setPlan] = useState<any>(null)
  const hlRefs = useRef<(HTMLInputElement | null)[]>([])
  const [focusHl, setFocusHl] = useState(-1)
  useEffect(() => {
    if (!id) return
    g.subscriptions.plan(id).then(p => {
      setPlan(p)
      setF({ name: p.name, amount: String(p.amount / 100).replace('.', ','), interval: p.interval, anchor: p.anchor, billing_day: p.billing_day || 5, months: p.months, missed: p.missed, fields: p.fields,
        highlights: p.highlights || [], duration: p.cycles ? 'cycles' : p.ends_at ? 'date' : 'none', cycles: String(p.cycles || 3), ends_at: p.ends_at ? p.ends_at.slice(0, 10) : '' })
      setPickMonths((p.months & ALL_MONTHS) !== ALL_MONTHS)
      setLoaded(true)
    }).catch(e => toast(errMsg(e), true))
  }, [id])
  useEffect(() => { if (focusHl >= 0) { hlRefs.current[focusHl]?.focus(); setFocusHl(-1) } }, [focusHl])
  useEffect(() => { if (err?.param) document.querySelector('.pf-err')?.scrollIntoView({ behavior: 'smooth', block: 'center' }) }, [err])
  const set = (patch: Partial<Form>) => setF({ ...f, ...patch })
  const amount = id ? plan?.amount || 0 : parseMT(f.amount)
  const hasMembers = (plan?.stats?.members || 0) > 0
  const cycles = f.duration === 'cycles' ? Math.max(0, Math.round(Number(f.cycles) || 0)) : 0
  const endsAt = f.duration === 'date' && f.ends_at ? new Date(f.ends_at + 'T23:59:59+02:00').toISOString() : null
  const shape = { interval: f.interval, anchor: f.anchor, billing_day: f.anchor === 'fixed' ? Number(f.billing_day) : 0, months: f.months, cycles, ends_at: endsAt, amount }
  const view = useMemo(() => customerView(shape), [JSON.stringify(shape)])
  const today = new Date()
  const highlights = f.highlights.map(h => h.trim()).filter(Boolean)
  const submit = async (draft: boolean) => {
    setBusy(true); setErr(null)
    const body = { name: f.name, amount: id ? plan.amount : amount, interval: f.interval, anchor: f.anchor, billing_day: f.anchor === 'fixed' ? Number(f.billing_day) : 0, months: f.months, missed: f.missed,
      fields: f.fields, highlights, cycles, ends_at: endsAt, draft }
    try {
      const p = id ? await g.subscriptions.updatePlan(id, body) : await g.subscriptions.createPlan(body)
      sx.reloadPlans(); sx.bump()
      toast(id ? 'Plano guardado' : draft ? 'Rascunho guardado' : 'Plano criado')
      nav('/painel/assinaturas/planos/' + p.id)
    } catch (e) { setErr(e as ApiError) } finally { setBusy(false) }
  }
  const setHl = (i: number, v: string) => set({ highlights: f.highlights.map((h, j) => (j === i ? v : h)) })
  const addHl = (at = f.highlights.length) => { if (f.highlights.length >= 8) return; const n = f.highlights.slice(); n.splice(at, 0, ''); set({ highlights: n }); setFocusHl(at) }
  const dayWord = f.interval === 'week' ? `à ${WEEKDAY_OF[today.getDay()]}` : `a ${today.getDate()}`
  const errAt = (...ps: string[]) => (err && ps.includes(err.param || '') ? <div className="form-err pf-err" style={{ marginTop: 10 }}>{err.message}</div> : null)
  const durationSentence = (() => {
    if (!view.dates.length) return f.duration === 'date' && !f.ends_at ? 'Escolha a data de fim.' : 'Com estas escolhas, não há pagamentos. Reveja os meses ou a data de fim.'
    const labels = view.dates.map(d => (d.label === 'Hoje' ? 'hoje' : d.label))
    if (f.duration === 'none') return `Quem entrar hoje paga ${labels[0] === 'hoje' ? 'hoje' : 'a ' + labels[0]} e depois ${view.when_text.charAt(0).toLowerCase() + view.when_text.slice(1)}, até cancelar.`
    if (f.duration === 'cycles' && cycles > 0 && cycles <= 6) return `Quem entrar hoje paga ${cycles === 1 ? 'uma vez' : cycles + ' vezes'}: ${joinList(labels)}. Depois, a assinatura termina sozinha.`
    return `Quem entrar hoje paga ${labels[0] === 'hoje' ? 'hoje' : 'a ' + labels[0]}, depois ${view.when_text.charAt(0).toLowerCase() + view.when_text.slice(1)}${f.duration === 'date' ? `, até ${view.duration_label.replace('Até ', '')}` : `, ${cycles} vezes no total`}. Depois, a assinatura termina sozinha.`
  })()
  const customer = { id: 'preview', name: f.name || 'Nome do plano', amount, interval: f.interval, highlights, ...view }
  if (!loaded) return <Skeleton h={500} r={22} />
  const Preview = (
    <BrandFrame page={sx.page} host={false}>
      <Hero merchant={{ name: shell.merchant.name, initials: initials(shell.merchant.name), color: shell.merchant.avatar_color }} page={{ ...sx.page, description: '' }} />
      <div style={{ padding: '12px 16px 0' }}>
        <div className="bf-card">
          <div className="bf-plan-name">{customer.name}</div>
          <div style={{ marginTop: 4 }}><Price amount={customer.amount} interval={customer.interval} size={28} /></div>
          <Highlights items={highlights} />
          <PlanFacts plan={customer} />
        </div>
      </div>
    </BrandFrame>
  )
  return (
    <div className="plan-form">
      <style>{`
        .plan-form{display:grid;grid-template-columns:minmax(0,1fr) 380px;gap:20px;align-items:start}
        .pf-col{display:flex;flex-direction:column;gap:14px;min-width:0}
        .pf-block{padding:20px 22px}
        .pf-row{display:grid;grid-template-columns:1.4fr 1fr;gap:12px}
        .pf-preview{position:sticky;top:16px}
        .pf-phone{border-radius:30px;overflow:hidden;border:8px solid #0B120E;max-height:calc(100vh - 120px);overflow-y:auto;background:#0B120E}
        .pf-actions{position:sticky;bottom:0;z-index:2;display:flex;gap:10px;justify-content:flex-end;flex-wrap:wrap;padding:14px 0 calc(14px + env(safe-area-inset-bottom));background:linear-gradient(to top,var(--paper) 75%,transparent)}
        .pf-hl{display:flex;gap:8px;align-items:center}
        .pf-hl+.pf-hl{margin-top:8px}
        .pf-dur{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:10px}
        .pf-sentence{margin-top:12px;padding:12px 14px;border-radius:12px;background:var(--green-tint);border:1px solid #D7ECD2;font-size:13.5px;line-height:1.5;color:var(--green-dark)}
        @media (max-width:1100px){.plan-form{grid-template-columns:1fr}.pf-preview{display:none}}
        @media (max-width:700px){.pf-row,.pf-dur,.plan-form .grid-2{grid-template-columns:1fr!important}.pf-block{padding:18px 16px}.pf-actions{flex-wrap:nowrap}.pf-actions .btn{flex:1;padding:0 12px}.pf-actions .pf-eye{flex:none;width:48px;padding:0}}
      `}</style>
      <div className="pf-col">
        <div className="between">
          <h2 className="display" style={{ margin: 0, fontSize: 24 }}>{id ? 'Editar plano' : 'Novo plano'}</h2>
          <Link to={id ? '/painel/assinaturas/planos/' + id : '/painel/assinaturas/planos'} style={{ fontSize: 13.5 }}>Cancelar</Link>
        </div>

        <Block title="Nome e valor">
          <div className="pf-row">
            <label><span className="label">Nome do plano</span><input className={'input' + (err?.param === 'name' ? ' err' : '')} value={f.name} onChange={e => set({ name: e.target.value })} placeholder="Propina · 1ª a 4ª classe" maxLength={120} /></label>
            <label><span className="label">Valor</span><div className={'phone-input' + (err?.param === 'amount' ? ' err' : '')}><input inputMode="decimal" value={id ? money(plan.amount) : f.amount} disabled={!!id} onChange={e => set({ amount: e.target.value })} placeholder="2 500,00" style={{ textAlign: 'right' }} /><span className="cc" style={{ borderLeft: '1px solid #E1DFD7', borderRight: 0 }}>MT</span></div>
              {id && <div style={{ fontSize: 12, color: 'var(--muted)', marginTop: 5 }}>Para mudar o valor use <b>Mudar valor</b> no plano.</div>}</label>
          </div>
          <span className="label" style={{ marginTop: 16, display: 'block' }}>Frequência</span>
          <div className="seg" style={{ display: 'flex' }}>
            {[['week', 'Semanal'], ['month', 'Mensal'], ['quarter', 'Trimestral'], ['year', 'Anual']].map(([k, l]) => <button key={k} type="button" style={{ flex: 1, whiteSpace: 'nowrap' }} className={f.interval === k ? 'on' : ''} disabled={hasMembers} onClick={() => set({ interval: k, billing_day: k === 'week' ? 1 : 5 })}>{l}</button>)}
          </div>
          {hasMembers && <div style={{ fontSize: 12.5, color: 'var(--muted)', marginTop: 6 }}>Este plano já tem membros: a frequência, o dia e a duração ficam como estão. Para mudar, crie um plano novo.</div>}
          {errAt('name', 'amount', 'interval')}
        </Block>

        <Block title="O que inclui" hint="Opcional. Frases curtas que o cliente vê no catálogo, uma por linha. Por exemplo: Material escolar incluído.">
          {f.highlights.map((h, i) => (
            <div key={i} className="pf-hl">
              <Icon name="check" size={16} stroke={2.4} color="var(--green)" />
              <input ref={el => { hlRefs.current[i] = el }} className="input" value={h} maxLength={90} placeholder="Escreva um ponto" onChange={e => setHl(i, e.target.value)}
                onKeyDown={e => {
                  if (e.key === 'Enter') { e.preventDefault(); addHl(i + 1) }
                  if (e.key === 'Backspace' && !h) { e.preventDefault(); set({ highlights: f.highlights.filter((_, j) => j !== i) }); setFocusHl(Math.max(0, i - 1)) }
                }} />
              <button type="button" className="icon-btn" style={{ border: 0, flex: 'none' }} onClick={() => set({ highlights: f.highlights.filter((_, j) => j !== i) })} aria-label="Remover ponto"><Icon name="close" size={16} /></button>
            </div>
          ))}
          {f.highlights.length < 8
            ? <button type="button" className="btn btn-outline btn-sm" style={{ marginTop: f.highlights.length ? 12 : 0 }} onClick={() => addHl()}><Icon name="plus" size={15} />{f.highlights.length ? 'Adicionar outro ponto' : 'Adicionar um ponto'}</button>
            : <div style={{ fontSize: 12.5, color: 'var(--muted)', marginTop: 10 }}>Chegou ao máximo de 8 pontos.</div>}
          {errAt('highlights')}
        </Block>

        <Block title="Quando paga">
          <div className="grid-2" style={{ gap: 10 }}>
            <Radio on={f.anchor === 'fixed'} title="No mesmo dia para todos" sub={f.interval === 'week' ? `Todos pagam à ${WEEKDAYS[(f.billing_day + 6) % 7]}.` : `Todos pagam no dia ${f.billing_day}. Quem entra a meio paga o primeiro ${per(f.interval)} ao entrar.`} onClick={() => !hasMembers && set({ anchor: 'fixed' })} />
            <Radio on={f.anchor === 'signup'} title="No dia em que cada um entra" sub={`Quem entrar hoje paga sempre ${dayWord}. O cliente vê a data dele, não esta regra.`} onClick={() => !hasMembers && set({ anchor: 'signup' })} />
          </div>
          {f.anchor === 'fixed' && (
            <label style={{ display: 'flex', alignItems: 'center', gap: 10, marginTop: 12, fontSize: 14 }}>
              {f.interval === 'week' ? 'Dia da semana' : 'Dia do pagamento'}
              <select className="input" style={{ width: 160, height: 40 }} value={f.billing_day} disabled={hasMembers} onChange={e => set({ billing_day: Number(e.target.value) })}>
                {f.interval === 'week' ? WEEKDAYS.map((w, i) => <option key={w} value={i + 1}>{w}</option>) : Array.from({ length: 28 }, (_, i) => <option key={i} value={i + 1}>Dia {i + 1}</option>)}
              </select>
            </label>
          )}
          <div style={{ marginTop: 16, paddingTop: 14, borderTop: '1px solid var(--line-3)' }}>
            {!pickMonths ? (
              <div className="between" style={{ fontSize: 14 }}><span>Paga todos os meses</span><button type="button" className="link-btn" onClick={() => setPickMonths(true)}>Tirar meses, por exemplo nas férias</button></div>
            ) : (
              <>
                <div className="between" style={{ fontSize: 14 }}><span>{(f.months & ALL_MONTHS) === ALL_MONTHS ? 'Paga todos os meses' : `Sem pagamento em ${monthsLabel(ALL_MONTHS & ~f.months)}`}</span><button type="button" className="link-btn" onClick={() => { set({ months: ALL_MONTHS }); setPickMonths(false) }}>Todos os meses</button></div>
                <div style={{ marginTop: 10 }}><MonthStrip mask={f.months} size={40} onToggle={i => set({ months: f.months ^ (1 << i) })} /></div>
                <div style={{ fontSize: 12.5, color: 'var(--muted)', marginTop: 6 }}>Toque num mês para o tirar ou voltar a pôr.{f.interval !== 'month' && ' Um pagamento que caia num mês sem pagamento é saltado.'}</div>
              </>
            )}
          </div>
          {errAt('anchor', 'billing_day', 'months')}
        </Block>

        <Block title="Durante quanto tempo">
          <div className="pf-dur">
            <Radio on={f.duration === 'none'} title="Sem fim" sub="Paga até cancelar." onClick={() => !hasMembers && set({ duration: 'none' })} />
            <Radio on={f.duration === 'cycles'} title="Número de pagamentos" sub="Um curso de 3 meses, por exemplo." onClick={() => !hasMembers && set({ duration: 'cycles' })} />
            <Radio on={f.duration === 'date'} title="Até uma data" sub="O fim do ano letivo, por exemplo." onClick={() => set({ duration: 'date' })} />
          </div>
          {f.duration === 'cycles' && (
            <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginTop: 12, flexWrap: 'wrap' }}>
              <input className={'input' + (err?.param === 'cycles' ? ' err' : '')} inputMode="numeric" value={f.cycles} disabled={hasMembers} onChange={e => set({ cycles: e.target.value.replace(/\D/g, '').slice(0, 3) })} style={{ width: 80, height: 40, textAlign: 'center' }} aria-label="Número de pagamentos" />
              <span style={{ fontSize: 14 }}>pagamentos</span>
              <span style={{ display: 'flex', gap: 6 }}>{[3, 6, 10, 12].map(n => <button key={n} type="button" disabled={hasMembers} className={'btn btn-sm ' + (cycles === n ? 'btn-ink' : 'btn-outline')} style={{ borderRadius: 999, minWidth: 40 }} onClick={() => set({ cycles: String(n) })}>{n}</button>)}</span>
            </div>
          )}
          {f.duration === 'date' && (
            <label style={{ display: 'flex', alignItems: 'center', gap: 10, marginTop: 12, fontSize: 14 }}>Último dia<input className={'input' + (err?.param === 'ends_at' ? ' err' : '')} type="date" value={f.ends_at} onChange={e => set({ ends_at: e.target.value })} style={{ width: 190, height: 40 }} /></label>
          )}
          <div className="pf-sentence">{durationSentence}</div>
          {errAt('cycles', 'ends_at')}
        </Block>

        <Block title="Se um pagamento falhar">
          <div className="grid-2" style={{ gap: 10 }}>
            <Radio on={f.missed === 'debt'} title="Fica em dívida" sub="O valor em falta junta-se ao pagamento seguinte. Continua ativo durante a tolerância." onClick={() => set({ missed: 'debt' })} />
            <Radio on={f.missed === 'forfeit'} title="Perde esse período" sub="Fecha sem dívida e o serviço fica suspenso nesse período." onClick={() => set({ missed: 'forfeit' })} />
          </div>
          <div style={{ fontSize: 13, color: 'var(--muted)', marginTop: 10 }}>Lembretes e tolerância seguem as <Link to="/painel/assinaturas/definicoes">definições da conta</Link>.</div>
          {errAt('missed')}
        </Block>

        <Block title="O que perguntar ao cliente" hint="O primeiro campo é o nome do membro, por exemplo o aluno. Quem paga escreve o nome dele à parte.">
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
            {f.fields.map((x, i) => <span key={x} className="chip" style={{ background: 'var(--paper-2)', fontFamily: 'var(--sans)', fontWeight: 600 }}>{x}{i === 0 && <span style={{ fontWeight: 400, color: 'var(--muted)' }}>· nome</span>}<button type="button" onClick={() => set({ fields: f.fields.filter(y => y !== x) })} aria-label={'Remover ' + x} style={{ border: 0, background: 'none', padding: 0, marginLeft: 2, cursor: 'pointer', display: 'inline-flex' }}><Icon name="close" size={13} /></button></span>)}
            {adding ? <form onSubmit={e => { e.preventDefault(); if (field.trim() && !f.fields.includes(field.trim()) && f.fields.length < 6) set({ fields: [...f.fields, field.trim()] }); setField(''); setAdding(false) }} style={{ display: 'flex' }}><input className="input" autoFocus value={field} onChange={e => setField(e.target.value)} onBlur={() => setAdding(false)} placeholder="Turma" style={{ height: 32, width: 140 }} maxLength={40} /></form>
              : f.fields.length < 6 && <button type="button" className="chip" onClick={() => setAdding(true)} style={{ border: '1px dashed #C9C7BE', background: '#fff', fontFamily: 'var(--sans)', cursor: 'pointer' }}>+ Campo</button>}
          </div>
          {errAt('fields')}
        </Block>

        {err && !document.querySelector('.pf-err') && <div className="form-err">{err.message}</div>}
        <div className="pf-actions">
          <button className="btn btn-outline only-mobile pf-eye" type="button" onClick={() => setPreview(true)} aria-label="Ver como o cliente vê" title="Ver como o cliente vê"><Icon name="eye" size={18} /></button>
          {!id && <button className="btn btn-outline" disabled={busy} onClick={() => submit(true)}>Guardar rascunho</button>}
          <button className="btn btn-green" disabled={busy} onClick={() => submit(false)}>{busy ? <Spinner /> : id ? (plan?.status === 'draft' ? 'Guardar e ativar' : 'Guardar alterações') : 'Criar plano'}</button>
        </div>
      </div>
      <aside className="pf-preview">
        <div style={{ fontSize: 12, fontWeight: 700, color: 'var(--muted)', textTransform: 'uppercase', letterSpacing: '.06em', margin: '6px 0 10px' }}>O que o cliente vê</div>
        <div className="pf-phone">{Preview}</div>
      </aside>
      {preview && <Modal onClose={() => setPreview(false)} width={Math.min(420, window.innerWidth - 24)}>
        <div className="between" style={{ marginBottom: 12 }}><b>O que o cliente vê</b><button className="icon-btn" onClick={() => setPreview(false)} aria-label="Fechar"><Icon name="close" size={16} /></button></div>
        <div style={{ borderRadius: 22, overflow: 'hidden', margin: '0 -6px' }}>{Preview}</div>
      </Modal>}
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
            <KV k="Quando paga" v={merchantWhen(p)} />
            <KV k="Duração" v={p.duration_label || 'Sem fim, até cancelar'} />
            {(p.months & ALL_MONTHS) !== ALL_MONTHS && <KV k="Meses" v={<MonthStrip mask={p.months} />} />}
            {p.highlights?.length > 0 && <KV k="O que inclui" v={<ul style={{ margin: 0, paddingLeft: 18, lineHeight: 1.6 }}>{p.highlights.map((h: string) => <li key={h}>{h}</li>)}</ul>} />}
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
                <button className="link-btn" onClick={() => sharePlan(p.name, p.url, toast)}>Partilhar</button>
                <a className="link-btn" href={p.url} target="_blank" rel="noreferrer">Abrir</a>
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
