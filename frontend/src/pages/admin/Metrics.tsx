import { useMemo, useState, type ReactNode } from 'react'
import { useNavigate } from 'react-router-dom'
import { Icon, MethodIcon, Skeleton } from '../../components/ui'
import { useQuery } from '../../core/useQuery'
import { fmt, METHOD_NAME } from '../../lib/format'

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

function Tile({ label, value, sub }: { label: string; value: ReactNode; sub?: ReactNode }) {
  return (
    <div className="card" style={{ padding: 18, minWidth: 0 }}>
      <div style={{ fontSize: 13, color: 'var(--muted)' }}>{label}</div>
      <div className="display tnum" style={{ fontSize: 28, letterSpacing: '-0.03em', marginTop: 6 }}>{value}</div>
      {sub && <div style={{ fontSize: 12.5, color: 'var(--muted)', marginTop: 4 }}>{sub}</div>}
    </div>
  )
}

function Panel({ title, note, children }: { title: string; note?: string; children: ReactNode }) {
  return (
    <section className="card" style={{ padding: 0, overflow: 'hidden', minWidth: 0 }}>
      <div style={{ padding: '16px 20px 10px' }}><h2 style={{ margin: 0, fontSize: 15.5 }}>{title}</h2>{note && <div style={{ fontSize: 12.5, color: 'var(--muted)', marginTop: 3, lineHeight: 1.45 }}>{note}</div>}</div>
      {children}
    </section>
  )
}

function Rows({ head, rows, cols }: { head: string[]; rows: ReactNode[][]; cols: string }) {
  if (!rows.length) return <div style={{ padding: '6px 20px 18px', fontSize: 13.5, color: 'var(--muted)' }}>Sem dados neste período.</div>
  return (
    <div style={{ overflowX: 'auto' }}>
      <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13.5, minWidth: 420 }}>
        <thead><tr>{head.map((h, i) => <th key={h} style={{ textAlign: i ? 'right' : 'left', padding: '8px 20px', fontSize: 11.5, fontWeight: 700, color: 'var(--muted)', textTransform: 'uppercase', letterSpacing: '.05em', borderTop: '1px solid var(--line-3)', width: i ? cols : undefined }}>{h}</th>)}</tr></thead>
        <tbody>{rows.map((r, i) => <tr key={i}>{r.map((c, j) => <td key={j} className={j ? 'tnum' : ''} style={{ textAlign: j ? 'right' : 'left', padding: '10px 20px', borderTop: '1px solid var(--line-3)', whiteSpace: j ? 'nowrap' : undefined }}>{c}</td>)}</tr>)}</tbody>
      </table>
    </div>
  )
}

// RateBar is a thin bar for a share, in one hue; the number beside it carries the value.
function RateBar({ n, of }: { n: number; of: number }) {
  return (
    <span style={{ display: 'inline-flex', alignItems: 'center', gap: 8, justifyContent: 'flex-end' }}>
      <span style={{ width: 70, height: 6, borderRadius: 3, background: '#EEECE5', overflow: 'hidden', display: 'inline-block' }}><span style={{ display: 'block', height: '100%', width: (of ? (n / of) * 100 : 0) + '%', background: PAID, borderRadius: 3 }} /></span>
      {pct(n, of)}
    </span>
  )
}

function DailyChart({ days }: { days: { day: string; paid: number; failed: number }[] }) {
  const [hover, setHover] = useState<number | null>(null)
  const [table, setTable] = useState(false)
  const W = 720, H = 200, pad = 28
  const max = Math.max(1, ...days.map(d => d.paid + d.failed))
  const bw = Math.max(6, Math.min(36, (W - pad) / Math.max(1, days.length) - 6))
  const step = (W - pad) / Math.max(1, days.length)
  const y = (v: number) => (v / max) * (H - 24)
  const label = (d: string) => new Date(d + 'T12:00:00').toLocaleDateString('pt-PT', { day: 'numeric', month: 'short' })
  if (!days.length) return <div style={{ padding: '6px 20px 18px', fontSize: 13.5, color: 'var(--muted)' }}>Sem pagamentos neste período.</div>
  return (
    <div style={{ padding: '0 20px 16px' }}>
      <div className="between" style={{ fontSize: 12.5, color: 'var(--text-2)', flexWrap: 'wrap', gap: 8 }}>
        <span style={{ display: 'flex', gap: 14 }} aria-label="Legenda">
          <span className="row" style={{ gap: 6 }}><i style={{ width: 10, height: 10, borderRadius: 3, background: PAID, display: 'inline-block' }} />Pagos</span>
          <span className="row" style={{ gap: 6 }}><i style={{ width: 10, height: 10, borderRadius: 3, background: FAILED, display: 'inline-block' }} />Falhados</span>
        </span>
        <button className="link-btn" style={{ fontSize: 12.5 }} onClick={() => setTable(!table)}>{table ? 'Ver gráfico' : 'Ver como tabela'}</button>
      </div>
      {table ? (
        <Rows head={['Dia', 'Pagos', 'Falhados', 'Taxa']} cols="90px" rows={days.map(d => [label(d.day), d.paid, d.failed, pct(d.paid, d.paid + d.failed)])} />
      ) : (
        <div style={{ position: 'relative', marginTop: 8 }}>
          <svg viewBox={`0 0 ${W} ${H + 22}`} width="100%" role="img" aria-label="Pagamentos por dia, pagos e falhados" style={{ display: 'block', overflow: 'visible' }}>
            {[0, 0.5, 1].map(f => <g key={f}><line x1={pad} x2={W} y1={H - f * (H - 24)} y2={H - f * (H - 24)} stroke="#EEECE5" /><text x={pad - 6} y={H - f * (H - 24) + 4} fontSize="11" textAnchor="end" fill="#8A938E">{Math.round(f * max)}</text></g>)}
            {days.map((d, i) => {
              const x = pad + i * step + (step - bw) / 2
              const hp = y(d.paid), hf = y(d.failed)
              return (
                <g key={d.day} onMouseEnter={() => setHover(i)} onMouseLeave={() => setHover(null)}>
                  <rect x={pad + i * step} y={0} width={step} height={H} fill="transparent" />
                  {hp > 0 && <path d={`M${x},${H} v${-hp + (hf > 0 ? 0 : 4)} ${hf > 0 ? '' : `q0,-4 4,-4 h${bw - 8} q4,0 4,4`} ${hf > 0 ? `h${bw}` : ''} v${hp - (hf > 0 ? 0 : 4)} z`} fill={PAID} opacity={hover === null || hover === i ? 1 : 0.55} />}
                  {hf > 0 && <path d={`M${x},${H - hp - (hp > 0 ? 2 : 0)} v${-hf + 4} q0,-4 4,-4 h${bw - 8} q4,0 4,4 v${hf - 4} z`} fill={FAILED} opacity={hover === null || hover === i ? 1 : 0.55} />}
                  {(days.length <= 10 || i % Math.ceil(days.length / 8) === 0) && <text x={x + bw / 2} y={H + 16} fontSize="11" textAnchor="middle" fill="#8A938E">{label(d.day)}</text>}
                </g>
              )
            })}
          </svg>
          {hover !== null && (
            <div role="tooltip" style={{ position: 'absolute', top: 0, left: `min(${((pad + hover * step + step / 2) / W) * 100}%, calc(100% - 160px))`, pointerEvents: 'none', background: '#0B120E', color: '#fff', borderRadius: 10, padding: '8px 10px', fontSize: 12.5, lineHeight: 1.5, whiteSpace: 'nowrap' }}>
              <b>{label(days[hover].day)}</b><br />Pagos {days[hover].paid} · Falhados {days[hover].failed}<br />Taxa {pct(days[hover].paid, days[hover].paid + days[hover].failed)}
            </div>
          )}
        </div>
      )}
    </div>
  )
}

export default function AdminMetrics() {
  const nav = useNavigate()
  const [days, setDays] = useState(7)
  const [live, setLive] = useState(true)
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
    <div className="page">
      <div className="between" style={{ flexWrap: 'wrap', gap: 12, alignItems: 'flex-end' }}>
        <div><h1 className="h1">Métricas</h1><div style={{ fontSize: 14.5, color: 'var(--muted)', marginTop: 4 }}>Como estão a correr os pagamentos, e o que está mal agora.</div></div>
        <div className="row" style={{ gap: 8, flexWrap: 'wrap' }}>
          <div className="seg">{[[1, '24 h'], [7, '7 dias'], [30, '30 dias'], [90, '90 dias']].map(([k, l]) => <button key={k} className={days === k ? 'on' : ''} onClick={() => setDays(k as number)}>{l}</button>)}</div>
          <div className="seg">{[[true, 'Produção'], [false, 'Teste']].map(([k, l]) => <button key={String(k)} className={live === k ? 'on' : ''} onClick={() => setLive(k as boolean)}>{l}</button>)}</div>
        </div>
      </div>
      {!m ? <Skeleton h={400} r={22} style={{ marginTop: 20 }} /> : (
        <>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 8, marginTop: 18 }}>
            {data.metrics.alerts?.length ? data.metrics.alerts.map((a: any) => (
              <div key={a.key} role="alert" style={{ display: 'flex', gap: 12, padding: '12px 16px', borderRadius: 14, background: a.level === 'error' ? '#FFE3E1' : '#FFF3C4', color: a.level === 'error' ? '#8E0609' : '#5B4600' }}>
                <Icon name="alert" size={18} stroke={2.2} style={{ marginTop: 1 }} />
                <div><b>{a.level === 'error' ? 'Erro' : 'Aviso'}: {a.title}</b><div style={{ fontSize: 13, marginTop: 2 }}>{a.detail}</div></div>
              </div>
            )) : <div role="status" className="row" style={{ gap: 10, padding: '12px 16px', borderRadius: 14, background: '#E2F3DE', color: '#0A6B05', fontWeight: 600 }}><Icon name="check" size={18} stroke={2.4} />Sem alertas agora.</div>}
          </div>

          <div className="grid-4" style={{ marginTop: 14 }}>
            <Tile label="Pagamentos" value={total.paid + total.failed} sub={`${total.paid} pagos · ${total.failed} falhados`} />
            <Tile label="Taxa de sucesso" value={pct(total.paid, total.paid + total.failed)} sub="Dos pagamentos que terminaram" />
            <Tile label="Recebido" value={<>{fmt(total.amount, false)} <span style={{ fontSize: 14, color: 'var(--muted)' }}>MT</span></>} sub={m.stuck ? `${m.stuck} à espera há mais de 15 min` : 'Nenhum pagamento preso'} />
            <Tile label="Até pagar, mediana" value={busiest?.paid ? secs(busiest.p50_seconds) : '—'} sub={busiest ? `${METHOD_NAME[busiest.method] || busiest.method} · 90% em ${secs(busiest.p90_seconds)}` : undefined} />
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: 'minmax(0,2fr) minmax(0,1fr)', gap: 14, marginTop: 14 }} className="mx-grid">
            <style>{`@media(max-width:1000px){.mx-grid{grid-template-columns:minmax(0,1fr)!important}}`}</style>
            <Panel title="Pagamentos por dia"><DailyChart days={daily} /></Panel>
            <Panel title="Do checkout ao pagamento" note="Checkouts criados neste período.">
              <div style={{ padding: '4px 20px 18px', display: 'flex', flexDirection: 'column', gap: 12 }}>
                {[['Abertos', m.funnel.opened], ['Tentaram pagar', m.funnel.tried], ['Pagos', m.funnel.paid]].map(([l, n]) => (
                  <div key={l as string}>
                    <div className="between" style={{ fontSize: 13.5 }}><span>{l}</span><b className="tnum">{n as number} <span style={{ color: 'var(--muted)', fontWeight: 400 }}>{pct(n as number, m.funnel.opened)}</span></b></div>
                    <div style={{ height: 8, borderRadius: 4, background: '#EEECE5', marginTop: 6, overflow: 'hidden' }}><div style={{ height: '100%', width: (m.funnel.opened ? ((n as number) / m.funnel.opened) * 100 : 0) + '%', background: PAID, borderRadius: 4 }} /></div>
                  </div>
                ))}
              </div>
            </Panel>
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit,minmax(min(100%,460px),1fr))', gap: 14, marginTop: 14 }}>
            <Panel title="Por carteira" note="Tempo do pedido até pago.">
              <Rows head={['Carteira', 'Pagos', 'Taxa', 'Mediana', '90%']} cols="76px" rows={(m.methods || []).map((r: any) => [
                <span className="row" style={{ gap: 8, whiteSpace: 'nowrap' }}><MethodIcon method={r.method} size={20} radius={6} />{METHOD_NAME[r.method] || r.method}</span>, r.paid, <RateBar n={r.paid} of={r.paid + r.failed} />, r.paid ? secs(r.p50_seconds) : '—', r.paid ? secs(r.p90_seconds) : '—'])} />
            </Panel>
            <Panel title="Rede do cliente" note="A rede de onde o cliente pediu para pagar, pelo endereço IP.">
              <Rows head={['Rede', 'Pedidos', 'Pagos', 'Taxa']} cols="90px" rows={(m.networks || []).map((r: any) => [r.network, r.total, r.paid, <RateBar n={r.paid} of={r.paid + r.failed} />])} />
            </Panel>
            <Panel title="Porque falharam">
              <Rows head={['Motivo', 'Carteira', 'Vezes']} cols="110px" rows={(m.failures || []).map((r: any) => [<span>{FAILURES[r.code] || r.code} <span className="mono" style={{ fontSize: 11.5, color: 'var(--muted)' }}>{r.code}</span></span>, METHOD_NAME[r.method] || r.method, r.n])} />
            </Panel>
            <Panel title="Resposta do bim, via mtoca" note="Do pedido à resposta do bim, que inclui o tempo do cliente a confirmar.">
              <Rows head={['Carteira', 'Pedidos', 'Mediana', '90%', 'Falhas']} cols="80px" rows={(m.operator || []).map((r: any) => [METHOD_NAME[r.method] || r.method, r.n, ms(r.p50_ms), ms(r.p90_ms), r.errors])} />
            </Panel>
            <Panel title="Comissão do bim" note="O que o bim disse cobrar em cada aviso de pagamento.">
              <Rows head={['Carteira', 'Pagamentos', 'Valor', 'Comissão', 'Taxa']} cols="90px" rows={(m.commission || []).map((r: any) => [r.wallet, r.payments, mt2(r.amount) + ' MT', mt2(r.commission) + ' MT', r.amount ? ((r.commission / r.amount) * 100).toFixed(2).replace('.', ',') + '%' : '—'])} />
            </Panel>
            <Panel title="Webhooks e API">
              <Rows head={['', 'Total', 'Com problema', '90%']} cols="110px" rows={[
                ['Webhooks', m.webhooks.total, `${m.webhooks.failed} falharam · ${m.webhooks.pending} a tentar`, ms(m.webhooks.p90_ms)],
                ['Pedidos à API', m.api.total, `${m.api.client_errors} 4xx · ${m.api.server_errors} 5xx`, ms(m.api.p90_ms)],
              ]} />
            </Panel>
            <Panel title="Registo e cópias de segurança">
              <Rows head={['', 'Estado']} cols="220px" rows={[
                ['Eventos escritos desde o arranque', m.trail_written],
                ['Eventos perdidos', m.trail_dropped],
                ['Última cópia de segurança', m.last_backup ? new Date(m.last_backup).toLocaleString('pt-PT') : 'Sem registo'],
              ]} />
              <div style={{ padding: '4px 20px 16px' }}><button className="link-btn" onClick={() => nav('/admin/rasto')}>Ver os eventos recentes no Rasto</button></div>
            </Panel>
          </div>
        </>
      )}
    </div>
  )
}
