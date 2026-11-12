import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { useNavigate } from 'react-router-dom'
import { Icon, MethodIcon, Skeleton } from '../../components/ui'
import { Badge, Empty, KV, List, Page, PageHead, Panel, Section, Stat, Tip } from '../../components/ops'
import { useQuery } from '../../core/useQuery'
import { METHOD_NAME } from '../../lib/format'

// How payments are going, and what is wrong now. Colours: the two series use
// slots 1 and 2 of the validated default palette (light mode; the admin panel has
// no dark mode). Status colours only ever mark alerts, with an icon and a word.
const PAID = '#2a78d6'
const FAILED = '#eb6834'

const FAILURES: Record<string, string> = {
  payment_rejected: 'Recusado pela carteira', payment_not_completed: 'Não concluído no telemóvel', customer_timeout: 'Sem resposta do cliente',
  pin_incorrect: 'PIN errado', insufficient_funds: 'Saldo insuficiente', operator_unavailable: 'Operador indisponível', canceled: 'Cancelado', desconhecido: 'Sem código',
}

const pct = (a: number, b: number) => (b ? Math.round((a / b) * 100) + '%' : '—')
const secs = (s: number | null | undefined) => (s == null ? '—' : s < 90 ? `${Math.round(s)} s` : `${Math.floor(s / 60)} min ${Math.round(s % 60)} s`)
const mt2 = (v: number) => v.toLocaleString('pt-PT', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
const ms = (v: number) => (!v ? '—' : v >= 1000 ? `${(v / 1000).toFixed(1)} s` : `${Math.round(v)} ms`)
const wallet = (m: string) => METHOD_NAME[m] || m

const CSS = `
.mx-alerts{display:flex;flex-direction:column;gap:8px;margin-bottom:14px}
.mx-alert{display:flex;gap:12px;align-items:flex-start;padding:12px 16px;border-radius:var(--o-r-sm);border:1px solid transparent;font-size:13px;line-height:1.45}
.mx-alert.warn{background:var(--o-warn-soft);border-color:#F5DFAE}.mx-alert.warn svg{color:var(--o-warn)}
.mx-alert.bad{background:var(--o-bad-soft);border-color:#F7C9C5}.mx-alert.bad svg{color:var(--o-bad)}
.mx-alert b{display:block;font-size:13.5px}
.mx-alert span{color:var(--o-ink-2)}
.mx-calm{display:inline-flex;align-items:center;gap:8px;font-size:13px;font-weight:600;color:var(--o-brand-ink);margin-bottom:14px}
.mx-calm i{display:grid;place-items:center;width:22px;height:22px;border-radius:50%;background:var(--o-brand-soft)}
.mx-legend{display:flex;gap:14px;align-items:center;flex-wrap:wrap;font-size:12px;color:var(--o-ink-2)}
.mx-legend span{display:inline-flex;align-items:center;gap:6px}
.mx-chart{position:relative;margin-top:2px}
.mx-chart svg{display:block;overflow:visible}
.mx-chart .mx-col{cursor:default}
.mx-tip{position:absolute;top:0;pointer-events:none;background:var(--o-ink);color:#fff;border-radius:8px;padding:8px 10px;font-size:12px;line-height:1.5;white-space:nowrap;box-shadow:var(--o-shadow-lift);z-index:2}
.mx-tip b{display:block;font-weight:700}
.mx-tip span{color:#C9D2CD}
.mx-funnel{display:flex;flex-direction:column;gap:14px}
.mx-funnel .between{font-size:13px}
.mx-funnel .o-bar{margin-top:7px}
.mx-funnel b{font-variant-numeric:tabular-nums}
.mx-funnel small{color:var(--o-muted);font-weight:500;margin-left:6px;font-size:12px}
.mx-rate{display:inline-flex;align-items:center;gap:8px;justify-content:flex-end;font-variant-numeric:tabular-nums;font-weight:600}
.mx-rate .o-bar{width:56px}
.mx-two{display:grid;grid-template-columns:repeat(auto-fit,minmax(min(100%,460px),1fr));gap:22px 14px;margin-top:22px}
.mx-two .o-section{margin:0 0 10px}
.mx-two .o-section h2{display:flex;align-items:center;gap:6px}
.mx-num{font-variant-numeric:tabular-nums}
.mx-who{display:flex;align-items:center;gap:8px;min-width:0}
.mx-flat > .o-list{border:0;box-shadow:none;border-radius:0}
.ops .mx-seg button{white-space:nowrap}
@media(max-width:860px){
  .ops .mx-seg button{flex:1}
  .mx-flat > .o-list{padding:0 12px 12px}
  .mx-two{gap:18px}
}`

// RateBar is a thin bar for a share, in one hue; the number beside it carries the value.
function RateBar({ n, of }: { n: number; of: number }) {
  return (
    <span className="mx-rate" title={`${n} de ${of}`}>
      <span className="o-bar"><i style={{ width: (of ? (n / of) * 100 : 0) + '%', background: PAID }} /></span>
      {pct(n, of)}
    </span>
  )
}

function Legend() {
  return (
    <span className="mx-legend" aria-label="Legenda">
      <span><i className="o-dot" style={{ background: PAID }} />Pagos</span>
      <span><i className="o-dot" style={{ background: FAILED }} />Falhados</span>
    </span>
  )
}

// useWidth keeps the chart drawn at its real width, so its text stays legible on phones.
function useWidth() {
  const [el, setEl] = useState<HTMLDivElement | null>(null)
  const [w, setW] = useState(0)
  useEffect(() => {
    if (!el) return
    const ro = new ResizeObserver(([e]) => setW(Math.round(e.contentRect.width)))
    ro.observe(el)
    setW(Math.round(el.getBoundingClientRect().width))
    return () => ro.disconnect()
  }, [el])
  return [setEl, w] as const
}

// A bar segment with its top corners rounded when it's the top of the stack.
function seg(x: number, top: number, w: number, h: number, round: boolean) {
  const r = round ? Math.min(4, h, w / 2) : 0
  return `M${x},${top + h} V${top + r} ${r ? `Q${x},${top} ${x + r},${top}` : ''} H${x + w - r} ${r ? `Q${x + w},${top} ${x + w},${top + r}` : ''} V${top + h} Z`
}

const dayLabel = (d: string) => new Date(d + 'T12:00:00').toLocaleDateString('pt-PT', { day: 'numeric', month: 'short' })

function DailyChart({ days, table }: { days: { day: string; paid: number; failed: number }[]; table: boolean }) {
  const [hover, setHover] = useState<number | null>(null)
  const [ref, width] = useWidth()
  if (!days.length) return <Empty icon="chart" title="Sem pagamentos neste período" />
  if (table) {
    return (
      <div className="mx-flat">
        <List rows={days} rowKey={d => d.day}
          cols={[
            { head: 'Dia', w: 'minmax(0,1fr)', cell: d => dayLabel(d.day) },
            { head: 'Pagos', w: '80px', right: true, cell: d => <span className="mx-num">{d.paid}</span> },
            { head: 'Falhados', w: '80px', right: true, cell: d => <span className="mx-num">{d.failed}</span> },
            { head: 'Taxa', w: '80px', right: true, cell: d => <b className="mx-num">{pct(d.paid, d.paid + d.failed)}</b> },
          ]}
          card={d => ({ title: dayLabel(d.day), sub: `${d.paid} pagos · ${d.failed} falhados`, amount: pct(d.paid, d.paid + d.failed) })} />
      </div>
    )
  }
  const W = Math.max(260, width), H = 190, pad = 30, top = 10
  const max = Math.max(1, ...days.map(d => d.paid + d.failed))
  const step = (W - pad) / days.length
  const bw = Math.max(3, Math.min(22, step * 0.56))
  const y = (v: number) => (v / max) * (H - top)
  const every = Math.max(1, Math.ceil(days.length / Math.max(2, Math.floor((W - pad) / 64))))
  const h = hover === null ? null : days[hover]
  return (
    <div className="mx-chart" ref={ref}>
      {width > 0 && (
        <svg width={W} height={H + 22} role="img" aria-label="Pagamentos por dia, pagos e falhados" onMouseLeave={() => setHover(null)}>
          {[0, 0.5, 1].map(f => (
            <g key={f}>
              <line x1={pad} x2={W} y1={H - f * (H - top)} y2={H - f * (H - top)} stroke={f ? 'var(--o-line-2)' : 'var(--o-line)'} strokeDasharray={f ? '3 4' : undefined} />
              <text x={pad - 8} y={H - f * (H - top) + 4} fontSize="11" textAnchor="end" fill="var(--o-muted)">{Math.round(f * max)}</text>
            </g>
          ))}
          {days.map((d, i) => {
            const x = pad + i * step + (step - bw) / 2
            const hp = y(d.paid), hf = y(d.failed)
            const gap = hp > 0 && hf > 0 ? 2 : 0
            const dim = hover !== null && hover !== i ? 0.4 : 1
            return (
              <g key={d.day} className="mx-col" onMouseEnter={() => setHover(i)} onFocus={() => setHover(i)} onBlur={() => setHover(null)} tabIndex={0}
                aria-label={`${dayLabel(d.day)}: ${d.paid} pagos, ${d.failed} falhados`}>
                <title>{`${dayLabel(d.day)} · ${d.paid} pagos · ${d.failed} falhados · taxa ${pct(d.paid, d.paid + d.failed)}`}</title>
                <rect x={pad + i * step} y={0} width={step} height={H} fill={hover === i ? 'var(--o-surface-2)' : 'transparent'} />
                {hp > 0 && <path d={seg(x, H - hp, bw, hp, hf === 0)} fill={PAID} opacity={dim} />}
                {hf > 0 && <path d={seg(x, H - hp - gap - hf, bw, hf, true)} fill={FAILED} opacity={dim} />}
                {!hp && !hf && <rect x={x} y={H - 2} width={bw} height={2} rx={1} fill="var(--o-line)" />}
                {i % every === 0 && <text x={x + bw / 2} y={H + 16} fontSize="11" textAnchor="middle" fill="var(--o-muted)">{dayLabel(d.day)}</text>}
              </g>
            )
          })}
        </svg>
      )}
      {h && hover !== null && (
        <div role="tooltip" className="mx-tip" style={{ left: Math.max(0, Math.min(pad + hover * step + step / 2 - 70, W - 150)) }}>
          <b>{dayLabel(h.day)}</b>
          <span>Pagos</span> {h.paid} · <span>Falhados</span> {h.failed}<br /><span>Taxa</span> {pct(h.paid, h.paid + h.failed)}
        </div>
      )}
    </div>
  )
}

// Block is a titled list in the grid of breakdowns.
function Block({ title, tip, children }: { title: string; tip?: string; children: ReactNode }) {
  return <div style={{ minWidth: 0 }}><Section title={<>{title}{tip && <Tip text={tip} />}</>} />{children}</div>
}

const none = <Empty icon="chart" title="Sem dados neste período" />

export default function AdminMetrics() {
  const nav = useNavigate()
  const [days, setDays] = useState(7)
  const [live, setLive] = useState(true)
  const [table, setTable] = useState(false)
  const { data } = useQuery(g => g.admin.metrics(days, live), [days, live], 30000)
  const m = data?.metrics
  const daily = useMemo(() => {
    const by: Record<string, { day: string; paid: number; failed: number; amount: number }> = {}
    for (const r of m?.daily || []) {
      const d = (by[r.day] ||= { day: r.day, paid: 0, failed: 0, amount: 0 })
      d.paid += r.paid; d.failed += r.failed; d.amount += r.amount
    }
    return Object.values(by).sort((a, b) => a.day.localeCompare(b.day))
  }, [m])
  const total = daily.reduce((a, d) => ({ paid: a.paid + d.paid, failed: a.failed + d.failed, amount: a.amount + d.amount }), { paid: 0, failed: 0, amount: 0 })
  const busiest = (m?.methods || []).slice().sort((a: any, b: any) => b.paid - a.paid)[0]
  return (
    <Page>
      <style>{CSS}</style>
      <PageHead title="Métricas" sub="Como estão a correr os pagamentos, e o que está mal agora"
        actions={<>
          <div className="seg mx-seg">{[[1, '24 h'], [7, '7 dias'], [30, '30 dias'], [90, '90 dias']].map(([k, l]) => <button key={k} className={days === k ? 'on' : ''} onClick={() => setDays(k as number)}>{l}</button>)}</div>
          <div className="seg mx-seg">{[[true, 'Produção'], [false, 'Teste']].map(([k, l]) => <button key={String(k)} className={live === k ? 'on' : ''} onClick={() => setLive(k as boolean)}>{l}</button>)}</div>
        </>} />
      {!m ? <><div className="o-stats">{[0, 1, 2, 3].map(i => <Skeleton key={i} h={96} r={16} />)}</div><Skeleton h={260} r={16} style={{ marginTop: 14 }} /></> : (
        <>
          {data.metrics.alerts?.length ? (
            <div className="mx-alerts">
              {data.metrics.alerts.map((a: any) => (
                <div key={a.key} role="alert" className={'mx-alert ' + (a.level === 'error' ? 'bad' : 'warn')}>
                  <Icon name="alert" size={18} stroke={2.2} style={{ marginTop: 1, flex: 'none' }} />
                  <div style={{ minWidth: 0 }}><b>{a.level === 'error' ? 'Erro' : 'Aviso'}: {a.title}</b><span>{a.detail}</span></div>
                </div>
              ))}
            </div>
          ) : <div role="status" className="mx-calm"><i><Icon name="check" size={13} stroke={2.6} /></i>Sem alertas agora</div>}

          <div className="o-stats">
            <Stat label="Pagamentos" value={String(total.paid + total.failed)} hint={`${total.paid} pagos · ${total.failed} falhados`} />
            <Stat label="Taxa de sucesso" value={pct(total.paid, total.paid + total.failed)} tip="Dos pagamentos que terminaram" />
            <Stat label="Recebido" value={total.amount} tone={m.stuck ? 'warn' : undefined}
              hint={m.stuck ? <Badge tone="warn" dot>{m.stuck} à espera há mais de 15 min</Badge> : 'Nenhum pagamento preso'} />
            <Stat label="Até pagar, mediana" value={busiest?.paid ? secs(busiest.p50_seconds) : '—'} hint={busiest ? `${wallet(busiest.method)} · 90% em ${secs(busiest.p90_seconds)}` : undefined} />
          </div>

          <div className="o-grid main-side" style={{ marginTop: 14 }}>
            <Panel title="Pagamentos por dia" pad={!table}
              action={<div className="row" style={{ gap: 10, flexWrap: 'wrap' }}>{!table && <Legend />}<button className="btn btn-ghost btn-sm" onClick={() => setTable(!table)}>{table ? 'Ver gráfico' : 'Ver tabela'}</button></div>}>
              <DailyChart days={daily} table={table} />
            </Panel>
            <Panel title="Do checkout ao pagamento" sub="Checkouts criados neste período">
              <div className="mx-funnel">
                {([['Abertos', m.funnel.opened], ['Tentaram pagar', m.funnel.tried], ['Pagos', m.funnel.paid]] as [string, number][]).map(([l, n]) => (
                  <div key={l} title={`${l}: ${n} de ${m.funnel.opened}`}>
                    <div className="between"><span>{l}</span><span><b>{n}</b><small>{pct(n, m.funnel.opened)}</small></span></div>
                    <div className="o-bar"><i style={{ width: (m.funnel.opened ? (n / m.funnel.opened) * 100 : 0) + '%', background: PAID }} /></div>
                  </div>
                ))}
              </div>
            </Panel>
          </div>

          <div className="mx-two">
            <Block title="Por carteira" tip="Tempo do pedido até pago.">
              <List rows={m.methods || []} rowKey={(r: any) => r.method} empty={none}
                cols={[
                  { head: 'Carteira', w: 'minmax(0,1fr)', cell: (r: any) => <span className="mx-who"><MethodIcon method={r.method} size={22} radius={7} /><span className="o-t1">{wallet(r.method)}</span></span> },
                  { head: 'Pagos', w: '56px', right: true, cell: (r: any) => <span className="mx-num">{r.paid}</span> },
                  { head: 'Taxa', w: '110px', right: true, cell: (r: any) => <RateBar n={r.paid} of={r.paid + r.failed} /> },
                  { head: 'Mediana', w: '72px', right: true, cell: (r: any) => <span className="mx-num">{r.paid ? secs(r.p50_seconds) : '—'}</span> },
                  { head: '90%', w: '72px', right: true, cell: (r: any) => <span className="mx-num">{r.paid ? secs(r.p90_seconds) : '—'}</span> },
                ]}
                card={(r: any) => ({ lead: <MethodIcon method={r.method} size={32} radius={9} />, title: wallet(r.method), sub: `${r.paid} pagos · mediana ${r.paid ? secs(r.p50_seconds) : '—'} · 90% ${r.paid ? secs(r.p90_seconds) : '—'}`, meta: <RateBar n={r.paid} of={r.paid + r.failed} /> })} />
            </Block>
            <Block title="Rede do cliente" tip="A rede de onde o cliente pediu para pagar, pelo endereço IP.">
              <List rows={m.networks || []} rowKey={(r: any) => r.network} empty={none}
                cols={[
                  { head: 'Rede', w: 'minmax(0,1fr)', cell: (r: any) => <span className="o-t1">{r.network}</span> },
                  { head: 'Pedidos', w: '70px', right: true, cell: (r: any) => <span className="mx-num">{r.total}</span> },
                  { head: 'Pagos', w: '60px', right: true, cell: (r: any) => <span className="mx-num">{r.paid}</span> },
                  { head: 'Taxa', w: '110px', right: true, cell: (r: any) => <RateBar n={r.paid} of={r.paid + r.failed} /> },
                ]}
                card={(r: any) => ({ title: r.network, sub: `${r.total} pedidos · ${r.paid} pagos`, meta: <RateBar n={r.paid} of={r.paid + r.failed} /> })} />
            </Block>
            <Block title="Porque falharam">
              <List rows={m.failures || []} rowKey={(r: any) => r.code + '/' + r.method} empty={none}
                cols={[
                  { head: 'Motivo', w: 'minmax(0,1fr)', cell: (r: any) => <><div className="o-t1">{FAILURES[r.code] || r.code}</div><div className="o-t2 o-mono">{r.code}</div></> },
                  { head: 'Carteira', w: '96px', cell: (r: any) => <span className="o-t2">{wallet(r.method)}</span> },
                  { head: 'Vezes', w: '60px', right: true, cell: (r: any) => <b className="mx-num">{r.n}</b> },
                ]}
                card={(r: any) => ({ title: FAILURES[r.code] || r.code, sub: <><span className="o-mono">{r.code}</span> · {wallet(r.method)}</>, amount: <>{r.n}<small>×</small></> })} />
            </Block>
            <Block title="Resposta do bim, via mtoca" tip="Do pedido à resposta do bim, que inclui o tempo do cliente a confirmar.">
              <List rows={m.operator || []} rowKey={(r: any) => r.method} empty={none}
                cols={[
                  { head: 'Carteira', w: 'minmax(0,1fr)', cell: (r: any) => <span className="o-t1">{wallet(r.method)}</span> },
                  { head: 'Pedidos', w: '70px', right: true, cell: (r: any) => <span className="mx-num">{r.n}</span> },
                  { head: 'Mediana', w: '72px', right: true, cell: (r: any) => <span className="mx-num">{ms(r.p50_ms)}</span> },
                  { head: '90%', w: '72px', right: true, cell: (r: any) => <span className="mx-num">{ms(r.p90_ms)}</span> },
                  { head: 'Falhas', w: '60px', right: true, cell: (r: any) => r.errors ? <Badge tone="bad">{r.errors}</Badge> : <span className="mx-num">0</span> },
                ]}
                card={(r: any) => ({ title: wallet(r.method), sub: `${r.n} pedidos · mediana ${ms(r.p50_ms)} · 90% ${ms(r.p90_ms)}`, meta: r.errors ? <Badge tone="bad">{r.errors} falhas</Badge> : <Badge>0 falhas</Badge> })} />
            </Block>
            <Block title="Comissão do bim" tip="O que o bim disse cobrar em cada aviso de pagamento.">
              <List rows={m.commission || []} rowKey={(r: any) => r.wallet} empty={none}
                cols={[
                  { head: 'Carteira', w: 'minmax(0,1fr)', cell: (r: any) => <span className="o-t1">{r.wallet}</span> },
                  { head: 'Pagamentos', w: '84px', right: true, cell: (r: any) => <span className="mx-num">{r.payments}</span> },
                  { head: 'Valor', w: 'minmax(96px,auto)', right: true, cell: (r: any) => <span className="o-amt">{mt2(r.amount)}<small>MT</small></span> },
                  { head: 'Comissão', w: 'minmax(84px,auto)', right: true, cell: (r: any) => <span className="o-amt">{mt2(r.commission)}<small>MT</small></span> },
                  { head: 'Taxa', w: '56px', right: true, cell: (r: any) => <span className="mx-num">{r.amount ? ((r.commission / r.amount) * 100).toFixed(2).replace('.', ',') + '%' : '—'}</span> },
                ]}
                card={(r: any) => ({ title: r.wallet, sub: `${r.payments} pagamentos · ${mt2(r.amount)} MT`, amount: <>{mt2(r.commission)}<small>MT</small></>, meta: <span className="o-t2">{r.amount ? ((r.commission / r.amount) * 100).toFixed(2).replace('.', ',') + '%' : '—'}</span> })} />
            </Block>
            <Block title="Webhooks e API">
              <List rows={[
                { k: 'Webhooks', total: m.webhooks.total, bad: `${m.webhooks.failed} falharam · ${m.webhooks.pending} a tentar`, hot: m.webhooks.failed > 0, p90: ms(m.webhooks.p90_ms) },
                { k: 'Pedidos à API', total: m.api.total, bad: `${m.api.client_errors} 4xx · ${m.api.server_errors} 5xx`, hot: m.api.server_errors > 0, p90: ms(m.api.p90_ms) },
              ]} rowKey={r => r.k}
                cols={[
                  { head: '', w: 'minmax(0,1fr)', cell: r => <span className="o-t1">{r.k}</span> },
                  { head: 'Total', w: '64px', right: true, cell: r => <span className="mx-num">{r.total}</span> },
                  { head: 'Com problema', w: '170px', right: true, cell: r => <span className="o-t2" style={{ color: r.hot ? 'var(--o-bad)' : undefined }}>{r.bad}</span> },
                  { head: '90%', w: '64px', right: true, cell: r => <span className="mx-num">{r.p90}</span> },
                ]}
                card={r => ({ title: r.k, sub: <span style={{ color: r.hot ? 'var(--o-bad)' : undefined }}>{r.bad}</span>, amount: <span className="mx-num">{r.total}</span>, meta: <span className="o-t2">90% em {r.p90}</span> })} />
            </Block>
            <Block title="Registo e cópias de segurança">
              <Panel foot={<button className="btn btn-ghost btn-sm" onClick={() => nav('/admin/rasto')}>Ver eventos no Rasto<Icon name="arrowRight" size={14} /></button>}>
                <KV k="Eventos escritos desde o arranque" v={m.trail_written} />
                <KV k="Eventos perdidos" v={m.trail_dropped ? <Badge tone="bad">{m.trail_dropped}</Badge> : m.trail_dropped} />
                <KV k="Última cópia de segurança" v={m.last_backup ? new Date(m.last_backup).toLocaleString('pt-PT') : 'Sem registo'} />
              </Panel>
            </Block>
          </div>
        </>
      )}
    </Page>
  )
}
