import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { Avatar, Skeleton, Spinner } from '../../../components/ui'
import { useGateway } from '../../../core/gateway'
import { errMsg } from '../../../core/model'
import { initials } from '../../../lib/format'
import { consent, digitsMT, FIELD_LABEL, FREQ_DEFAULTS, FREQ_LABEL, mt, parseMT, perLabel, prest, quote, rateLabel, TERM_MAX, unit, type Freq, type Product } from '../../../lib/credit'
import { useSession } from '../../../lib/session'
import { RequestLink } from './MicroShell'
import { Seg, Step, Tone, useMicro } from './common'

const COLS = 'minmax(150px,1.3fr) 150px 120px 120px minmax(120px,1fr) 70px 110px 90px'

export default function Products() {
  const nav = useNavigate()
  const mc = useMicro()
  const ps: Product[] = mc.products
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
      <div className="card card-pad" style={{ display: 'flex', gap: 18, alignItems: 'center', flexWrap: 'wrap' }}>
        <div style={{ flex: 2, minWidth: 280 }}>{mc.requestUrl && <RequestLink url={mc.requestUrl} name={mc.merchant.name} compact />}</div>
        <p className="muted" style={{ flex: 1, minWidth: 220, fontSize: 13.5, lineHeight: 1.55, margin: 0 }}>O cliente escolhe o produto, simula a prestação e envia o pedido. O QR pode ficar impresso no balcão.</p>
      </div>
      <div className="table mc-table">
        <div className="mc-th" style={{ gridTemplateColumns: COLS }}><span>Produto</span><span>Montante (MT)</span><span>Prazo</span><span>Taxa</span><span>Prestações</span><span style={{ textAlign: 'right' }}>Ativos</span><span style={{ textAlign: 'right' }}>Em carteira</span><span>Estado</span></div>
        {ps.length === 0 && <div className="empty">Nenhum produto. <button className="link-btn" onClick={() => nav('/painel/microcredito/produtos/novo')}>Criar o primeiro</button></div>}
        {ps.map(p => (
          <div key={p.id} className="mc-row click" style={{ gridTemplateColumns: COLS }} onClick={() => nav('/painel/microcredito/produtos/' + p.id)}>
            <b>{p.name}</b>
            <span className="tnum">{mt(p.min)} – {mt(p.max)}</span>
            <span>{p.term_label}</span>
            <span>{rateLabel(p.rate_bp, p.freq)}</span>
            <span>{FREQ_LABEL[p.freq]}{p.client_day ? ' · dia à escolha' : ''}</span>
            <span className="tnum" style={{ textAlign: 'right' }}>{p.loans}</span>
            <span className="tnum" style={{ textAlign: 'right' }}>{p.book ? mt(p.book) : '—'}</span>
            <span><Tone tone={p.status === 'active' ? 'ok' : 'wait'}>{p.status === 'active' ? 'Ativo' : 'Rascunho'}</Tone></span>
          </div>
        ))}
        {ps.length > 0 && <div style={{ padding: '12px 20px', fontSize: 13, color: 'var(--muted)', borderTop: '1px solid var(--line-3)' }}>Clique num produto para editar. Os empréstimos já aceites mantêm as condições do seu acordo.</div>}
      </div>
      <div className="mc-card-list">
        {ps.map(p => (
          <button key={p.id} className="card" style={{ padding: 16, textAlign: 'left', border: '1px solid var(--line)' }} onClick={() => nav('/painel/microcredito/produtos/' + p.id)}>
            <div className="between"><b>{p.name}</b><Tone tone={p.status === 'active' ? 'ok' : 'wait'}>{p.status === 'active' ? 'Ativo' : 'Rascunho'}</Tone></div>
            <div className="mc-sub" style={{ marginTop: 6 }}>{mt(p.min)} – {mt(p.max)} MT · {p.term_label} · {rateLabel(p.rate_bp, p.freq)}</div>
            <div className="mc-sub">{p.loans} ativos · {p.book ? mt(p.book) + ' MT em carteira' : 'sem carteira'}</div>
          </button>
        ))}
        <button className="btn btn-green btn-block" onClick={() => nav('/painel/microcredito/produtos/novo')}>Novo produto</button>
      </div>
    </div>
  )
}

type Draft = { name: string; min: string; max: string; freq: Freq; tMin: number; tMax: number; rate: string; dayMode: string; mora: boolean; moraP: string; grace: number; moraMax: number; effort: string; fields: string[]; status: string }

const blank: Draft = { name: '', min: '2 000', max: '30 000', freq: 'mensal', tMin: 1, tMax: 10, rate: '5', dayMode: 'cliente', mora: true, moraP: '1', grace: 2, moraMax: 4, effort: '35', fields: ['id_doc', 'activity', 'income', 'purpose'], status: 'active' }

const toBP = (v: string) => Math.round(parseFloat(v.replace(',', '.') || '0') * 100)
const fromBP = (bp: number) => (bp / 100).toString().replace('.', ',')

export function ProductForm() {
  const { id } = useParams()
  const nav = useNavigate()
  const g = useGateway()
  const mc = useMicro()
  const { toast } = useSession()
  const editing = !!id && id !== 'novo'
  const [d, setD] = useState<Draft | null>(editing ? null : blank)
  const [pvAmt, setPvAmt] = useState(15000)
  const [pvN, setPvN] = useState(7)
  const [pvDay, setPvDay] = useState(10)
  const [custom, setCustom] = useState('')
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')
  useEffect(() => {
    if (!editing) return
    g.microcredit.product(id!).then((p: Product) => {
      setD({ name: p.name, min: digitsMT(String(p.min / 100)), max: digitsMT(String(p.max / 100)), freq: p.freq, tMin: p.term_min, tMax: p.term_max, rate: fromBP(p.rate_bp),
        dayMode: p.day_mode, mora: p.mora, moraP: p.mora ? fromBP(p.mora_bp) : '1', grace: p.grace, moraMax: p.mora_max_weeks || 4, effort: String(p.effort_pct), fields: p.fields, status: p.status || 'active' })
      setPvAmt(Math.round((p.min + p.max) / 200000) * 1000); setPvN(Math.round((p.term_min + p.term_max) / 2))
    }).catch(e => setErr(errMsg(e)))
  }, [id]) // eslint-disable-line react-hooks/exhaustive-deps
  if (!d) return err ? <div className="card empty">{err}</div> : <Skeleton h={500} r={22} />
  const set = (o: Partial<Draft>) => setD({ ...d, ...o })
  const a1 = parseMT(d.min), a2 = parseMT(d.max)
  const lo = Math.min(a1, a2), hi = Math.max(a1, a2)
  const product: Product = { id: '', name: d.name, min: lo, max: hi, freq: d.freq, term_min: d.tMin, term_max: d.tMax, rate_bp: toBP(d.rate), day_mode: d.dayMode,
    client_day: d.freq === 'mensal' && d.dayMode === 'cliente', mora: d.mora, mora_bp: toBP(d.moraP), grace: d.grace, mora_max_weeks: d.moraMax, effort_pct: Number(d.effort) || 35, fields: d.fields }
  const step = hi - lo >= 2000000 ? 1000 : hi - lo >= 500000 ? 500 : 100
  const pa = Math.min(Math.max(pvAmt * 100, lo), hi)
  const pn = Math.min(Math.max(pvN, d.tMin), d.tMax)
  const pday = product.client_day ? pvDay : new Date().getDate()
  const q = quote(product, pa, pn, pday)
  const save = async () => {
    setBusy(true); setErr('')
    const body = { name: d.name, min: lo, max: hi, freq: d.freq, term_min: d.tMin, term_max: d.tMax, rate_bp: product.rate_bp, day_mode: d.dayMode, mora: d.mora,
      mora_bp: product.mora_bp, grace: d.grace, mora_max_weeks: d.moraMax, effort_pct: product.effort_pct, fields: d.fields, status: d.status }
    try {
      if (editing) await g.microcredit.updateProduct(id!, body); else await g.microcredit.createProduct(body)
      toast(editing ? 'Produto guardado' : 'Produto criado'); mc.reloadProducts(); mc.bump(); nav('/painel/microcredito/produtos')
    } catch (e) { setErr(errMsg(e)) } finally { setBusy(false) }
  }
  const lbl = (t: string) => <span className="label">{t}</span>
  const money = (v: string, on: (s: string) => void) => <div className="phone-input" style={{ height: 48 }}><input inputMode="numeric" value={v} onChange={e => on(digitsMT(e.target.value))} /><span style={{ padding: '0 12px', color: 'var(--muted)' }}>MT</span></div>
  const dec = (v: string, on: (s: string) => void, suffix: string) => <div className="phone-input" style={{ height: 48 }}><input inputMode="decimal" value={v} onChange={e => on(e.target.value.replace(/[^\d,.]/g, '').slice(0, 5))} /><span style={{ padding: '0 12px', color: 'var(--muted)', whiteSpace: 'nowrap', fontSize: 13 }}>{suffix}</span></div>
  return (
    <div style={{ display: 'grid', gridTemplateColumns: 'minmax(0,1.45fr) minmax(0,1fr)', gap: 18, alignItems: 'start' }} className="pf-grid">
      <style>{`@media (max-width:1250px){.pf-grid{grid-template-columns:1fr!important}}.pf-3>*,.pf-4>*{min-width:0}.pf-3 .phone-input input,.pf-4 .phone-input input{min-width:0;width:100%}`}</style>
      <div className="card card-pad">
        <div className="between"><h3 className="display" style={{ fontSize: 22, margin: 0 }}>{editing ? 'Editar produto' : 'Novo produto'}</h3>{editing && <Seg value={d.status} options={[['active', 'Ativo'], ['draft', 'Rascunho']]} onChange={v => set({ status: v })} style={{ width: 200 }} />}</div>
        {editing && <p className="muted" style={{ fontSize: 13, margin: '6px 0 0' }}>Os empréstimos já aceites mantêm o seu acordo.</p>}
        <div style={{ display: 'grid', gridTemplateColumns: 'minmax(0,1.3fr) minmax(0,1fr) minmax(0,1fr)', gap: 12, marginTop: 16 }} className="pf-3">
          <label>{lbl('Nome')}<input className="input" style={{ height: 48 }} value={d.name} maxLength={60} onChange={e => set({ name: e.target.value })} placeholder="Crédito Escolar" /></label>
          <label>{lbl('Montante mínimo')}{money(d.min, v => set({ min: v }))}</label>
          <label>{lbl('Montante máximo')}{money(d.max, v => set({ max: v }))}</label>
          <div>{lbl('Prestações')}<Seg value={d.freq} options={[['semanal', 'Semanal'], ['quinzenal', 'Quinzenal'], ['mensal', 'Mensal']]} onChange={f => set({ freq: f, tMin: FREQ_DEFAULTS[f][0], tMax: FREQ_DEFAULTS[f][1] })} /></div>
          <div>{lbl('Prazo mínimo')}<Step label={unit(d.freq, d.tMin)} canDec={d.tMin > 1} canInc={d.tMin < d.tMax} onDec={() => set({ tMin: d.tMin - 1 })} onInc={() => set({ tMin: d.tMin + 1 })} /></div>
          <div>{lbl('Prazo máximo')}<Step label={unit(d.freq, d.tMax)} canDec={d.tMax > d.tMin} canInc={d.tMax < TERM_MAX[d.freq]} onDec={() => set({ tMax: d.tMax - 1 })} onInc={() => set({ tMax: d.tMax + 1 })} /></div>
          <label>{lbl('Taxa de juro')}{dec(d.rate, v => set({ rate: v }), '% ' + perLabel(d.freq))}</label>
          <div>{lbl('Dia da prestação')}{d.freq === 'mensal' ? <Seg value={d.dayMode} options={[['entrega', 'Dia da entrega'], ['cliente', 'O cliente escolhe']]} onChange={v => set({ dayMode: v })} /> : <div className="muted" style={{ height: 48, display: 'flex', alignItems: 'center', fontSize: 13.5 }}>{d.freq === 'semanal' ? 'A cada 7 dias, desde a entrega' : 'A cada 14 dias, desde a entrega'}</div>}</div>
          <label>{lbl('Limite de esforço')}<div className="phone-input" style={{ height: 48 }}><input inputMode="numeric" value={d.effort} onChange={e => set({ effort: e.target.value.replace(/\D/g, '').slice(0, 3) })} /><span style={{ padding: '0 12px', color: 'var(--muted)', fontSize: 13, whiteSpace: 'nowrap' }}>% do rendimento</span></div></label>
        </div>
        <div style={{ display: 'grid', gridTemplateColumns: 'minmax(0,1.2fr) minmax(0,1fr) minmax(0,1fr) minmax(0,1fr)', gap: 12, marginTop: 12 }} className="pf-4">
          <div>{lbl('Se atrasar')}<Seg value={d.mora} options={[[true, 'Com mora'], [false, 'Sem mora']]} onChange={v => set({ mora: v })} /></div>
          <label style={{ opacity: d.mora ? 1 : 0.4, pointerEvents: d.mora ? 'auto' : 'none' }}>{lbl('Mora')}{dec(d.moraP, v => set({ moraP: v }), '% / semana')}</label>
          <div style={{ opacity: d.mora ? 1 : 0.4, pointerEvents: d.mora ? 'auto' : 'none' }}>{lbl('Tolerância')}<Step label={`${d.grace} ${d.grace === 1 ? 'dia' : 'dias'}`} canDec={d.grace > 0} canInc={d.grace < 15} onDec={() => set({ grace: d.grace - 1 })} onInc={() => set({ grace: d.grace + 1 })} /></div>
          <div style={{ opacity: d.mora ? 1 : 0.4, pointerEvents: d.mora ? 'auto' : 'none' }}>{lbl('Mora até')}<Step label={unit('semanal', d.moraMax)} canDec={d.moraMax > 1} canInc={d.moraMax < 12} onDec={() => set({ moraMax: d.moraMax - 1 })} onInc={() => set({ moraMax: d.moraMax + 1 })} /></div>
        </div>
        <div style={{ marginTop: 14 }}>
          {lbl('Pedir ao cliente')}
          <div className="mc-chips">
            {d.fields.map(f => <button key={f} className="mc-chip" style={{ background: 'var(--paper-2)', border: 0 }} onClick={() => set({ fields: d.fields.filter(x => x !== f) })}>{FIELD_LABEL[f] || f} ×</button>)}
            {Object.keys(FIELD_LABEL).filter(k => !d.fields.includes(k)).map(k => <button key={k} className="mc-chip" style={{ borderStyle: 'dashed' }} onClick={() => set({ fields: [...d.fields, k] })}>+ {FIELD_LABEL[k]}</button>)}
            <span style={{ display: 'inline-flex', gap: 6 }}><input className="input" style={{ height: 34, width: 140, borderRadius: 999 }} placeholder="Outro campo" value={custom} maxLength={40} onChange={e => setCustom(e.target.value)} onKeyDown={e => { if (e.key === 'Enter' && custom.trim()) { set({ fields: [...d.fields, custom.trim()] }); setCustom('') } }} />{custom.trim() && <button className="mc-chip" onClick={() => { set({ fields: [...d.fields, custom.trim()] }); setCustom('') }}>+ Campo</button>}</span>
          </div>
        </div>
        {err && <div className="form-err" style={{ marginTop: 14 }}>{err}</div>}
        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8, marginTop: 18, paddingTop: 16, borderTop: '1px solid var(--line-3)' }}>
          <button className="btn btn-outline" onClick={() => nav('/painel/microcredito/produtos')}>Cancelar</button>
          <button className="btn btn-green" disabled={busy || !d.name.trim()} onClick={save}>{busy ? <Spinner /> : editing ? 'Guardar alterações' : 'Criar produto'}</button>
        </div>
        <style>{`@media (max-width:860px){.pf-3,.pf-4{grid-template-columns:1fr!important}}`}</style>
      </div>
      <div>
        <div style={{ fontSize: 12, fontWeight: 700, letterSpacing: '.08em', color: 'var(--muted)', margin: '4px 0 10px' }}>O QUE O CLIENTE VÊ</div>
        <div className="card card-pad">
          <div style={{ display: 'flex', gap: 12, alignItems: 'center', paddingBottom: 14, borderBottom: '1px dashed var(--line)' }}><Avatar text={initials(mc.merchant.name)} color={mc.merchant.avatar_color} size={40} radius={12} /><span><b>{mc.merchant.name}</b><div className="mc-sub">{d.name || 'Sem nome'}</div></span></div>
          <div style={{ fontSize: 13, color: 'var(--muted)', marginTop: 14 }}>Quanto precisa?</div>
          <div className="display tnum" style={{ fontSize: 30 }}>{mt(pa)} <span style={{ fontSize: 14, color: 'var(--muted)' }}>MT</span></div>
          <input type="range" min={lo / 100} max={hi / 100} step={step} value={pa / 100} onChange={e => setPvAmt(Number(e.target.value))} style={{ width: '100%', accentColor: 'var(--green)' }} />
          <div className="between mono muted" style={{ fontSize: 11 }}><span>{mt(lo)}</span><span>{mt(hi)}</span></div>
          <div className="between" style={{ fontSize: 13, marginTop: 10 }}><span>Em quanto tempo paga</span><span className="muted">de {d.tMin} a {unit(d.freq, d.tMax)}</span></div>
          <Step style={{ marginTop: 6 }} label={unit(d.freq, pn)} canDec={pn > d.tMin} canInc={pn < d.tMax} onDec={() => setPvN(pn - 1)} onInc={() => setPvN(pn + 1)} />
          {product.client_day && <><div className="between" style={{ fontSize: 13, marginTop: 10 }}><span>Dia de pagamento</span><span className="muted">de 1 a 28</span></div><Step style={{ marginTop: 6 }} label={`Dia ${pday}`} canDec={pday > 1} canInc={pday < 28} onDec={() => setPvDay(pday - 1)} onInc={() => setPvDay(pday + 1)} /></>}
          <div className="between" style={{ marginTop: 12, alignItems: 'flex-start' }}><b>{prest(pn)} de {mt(q.inst)} MT</b><span className="muted" style={{ fontSize: 12.5, textAlign: 'right' }}>Total {mt(q.total)} MT · juros {mt(q.interest)}</span></div>
          <div style={{ marginTop: 10, padding: 12, borderRadius: 12, background: 'var(--paper-2)', fontSize: 12.5, lineHeight: 1.55, color: 'var(--text-2)' }}>{consent(mc.merchant.name, product, pa, pn, pday)}</div>
        </div>
        <p className="muted" style={{ fontSize: 12.5, lineHeight: 1.5 }}>O cliente escolhe qualquer montante e qualquer prazo dentro destes limites. Vê a prestação, os juros e o total antes de pedir.</p>
      </div>
    </div>
  )
}
