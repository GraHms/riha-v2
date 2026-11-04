import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useQuery } from '../../core/useQuery'
import { fmt, greeting, longToday, methodLine, monthName, PAYOUT_STATUS, shortDate, SOURCE, STATUS, weekday } from '../../lib/format'
import { Bars, Icon, MethodIcon, Pill, Skeleton, Stripes } from '../../components/ui'
import { useShell } from './AppShell'

export function TxLine({ p, onClick, desktop }: { p: any; onClick?: () => void; desktop?: boolean }) {
  const st = STATUS[p.status] || STATUS.processing
  const failed = p.status === 'failed'
  const origin = p.source?.type === 'api_checkout' ? 'API · ' + (p.source?.origin?.domain || 'loja online') : p.source?.type === 'qr' ? 'QR · ' + (p.link_name || 'Balcão') : 'Link · ' + (p.link_name || p.description || '')
  if (desktop) return (
    <div onClick={onClick} className="trow click" style={{ gridTemplateColumns: 'minmax(0,1fr) 150px 100px 120px', padding: '11px 22px' }}>
      <div className="row" style={{ minWidth: 0 }}><MethodIcon method={p.method} size={36} /><div style={{ minWidth: 0 }}><div style={{ fontWeight: 600, fontSize: 14, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{p.customer_name || 'Cliente'}</div><div style={{ fontSize: 12.5, color: 'var(--muted)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{origin}</div></div></div>
      <div style={{ fontSize: 13, color: 'var(--muted)' }}>{methodLine(p)}</div>
      <div><Pill {...st} /></div>
      <div className="tnum" style={{ textAlign: 'right', fontWeight: 600, fontSize: 14, color: failed ? 'var(--muted-2)' : 'var(--ink)', textDecoration: failed ? 'line-through' : 'none' }}>{fmt(p.amount)} MT</div>
    </div>
  )
  return (
    <button onClick={onClick} style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '12px 16px', width: '100%', border: 0, background: 'none', textAlign: 'left' }}>
      <MethodIcon method={p.method} size={42} radius={13} />
      <div style={{ flex: 1, minWidth: 0 }}><div style={{ fontWeight: 600, fontSize: 14.5, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{p.customer_name || 'Cliente'}</div><div style={{ fontSize: 12.5, color: 'var(--muted)', marginTop: 2, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{methodLine(p)}</div></div>
      <div style={{ textAlign: 'right' }}><div className="tnum" style={{ fontWeight: 600, fontSize: 14.5, color: failed ? 'var(--muted-2)' : 'var(--ink)', textDecoration: failed ? 'line-through' : 'none' }}>{fmt(p.amount)}</div><Pill {...st} style={{ marginTop: 3, fontSize: 11, padding: '2px 7px' }} /></div>
    </button>
  )
}

export function PayoutProgress({ p }: { p: any }) {
  const st = PAYOUT_STATUS[p.status]
  const steps = p.status === 'new' ? 1 : p.status === 'review' ? 2 : 3
  return (
    <div style={{ background: 'var(--sun-soft)', borderRadius: 24, padding: 22 }}>
      <div className="between"><div style={{ fontWeight: 600, fontSize: 15 }}>Levantamento {p.reference}</div><span className="pill" style={{ background: '#FFE58A', color: '#5B4600' }}>{st.t}</span></div>
      <div className="display" style={{ fontSize: 26, marginTop: 8 }}>{fmt(p.amount)} MT</div>
      <div style={{ display: 'flex', gap: 4, marginTop: 14 }}>{[1, 2, 3].map(i => <div key={i} style={{ flex: 1, height: 5, borderRadius: 3, background: i <= steps ? 'var(--ink)' : '#EAD78C' }} />)}</div>
      <div className="between" style={{ marginTop: 8, fontSize: 12, color: '#5B4600' }}><span>Pedido</span><span>{p.status === 'sent' ? 'No banco' : 'Em análise'}</span><span>Até {shortDate(p.due_at)}</span></div>
    </div>
  )
}

const RANGES: [number, string][] = [[7, '7 dias'], [30, '30 dias'], [365, '12 meses']]

export default function Overview() {
  const { openNewLink, merchant, version } = useShell()
  const nav = useNavigate()
  const [days, setDays] = useState(30)
  const { data: d } = useQuery(g => g.payments.overview(days), [days, version], 15000)
  const mobile = useQuery(g => g.payments.overview(7), [version], 15000).data

  const change = (o: any) => (o && o.previous > 0 ? Math.round(((o.received - o.previous) / o.previous) * 100) : null)
  const ch = change(d)
  const bars: number[] = d?.bars?.map((b: any) => b.amount) || []
  const bm = d?.by_method || { mpesa: 0, emola: 0, card: 0 }
  const totalM = bm.mpesa + bm.emola + bm.card || 1
  const pct = (v: number) => Math.round((v / totalM) * 100)
  const first = d?.bars?.[0]?.date, mid = d?.bars?.[Math.floor(bars.length / 2)]?.date
  const inTransit = d?.in_transit > 0

  const balanceCard = (mob: boolean) => (
    <div className="dark-card" style={{ padding: mob ? 22 : 26, borderRadius: mob ? 28 : 26, display: 'flex', flexDirection: 'column' }}>
      <Stripes top={-40} right={mob ? -10 : 0} w={mob ? 34 : 44} h={mob ? 240 : 320} fade={`linear-gradient(90deg,#0B120E ${mob ? 45 : 50}%,rgba(11,18,14,.35))`} />
      <div style={{ position: 'relative', fontSize: 14, color: '#A9B5AE' }}>Saldo disponível</div>
      <div style={{ position: 'relative', display: 'flex', alignItems: 'baseline', gap: 8, marginTop: 8 }}>
        {d ? <span className="display tnum" style={{ fontSize: mob ? 42 : 54, letterSpacing: '-0.045em' }}>{fmt(d.available)}</span> : <Skeleton h={50} w={220} style={{ background: 'rgba(255,255,255,.1)' }} />}
        <span style={{ fontSize: mob ? 17 : 20, fontWeight: 600, color: '#A9B5AE' }}>MT</span>
      </div>
      {inTransit && <div style={{ position: 'relative', display: 'inline-flex', alignSelf: 'flex-start', alignItems: 'center', gap: 7, marginTop: 10, fontSize: 13.5, color: '#D7DED9', padding: '6px 12px', borderRadius: 999, background: 'rgba(255,255,255,.08)' }}><Icon name="clock" size={14} stroke={2.2} color="var(--sun)" />{fmt(d.in_transit)} MT a caminho {d.bank_account ? 'do ' + d.bank_account.bank_name : 'do banco'}{d.pending_payout ? ` · chega até ${shortDate(d.pending_payout.due_at)}` : ''}</div>}
      <div style={{ flex: 1, minHeight: 24 }} />
      <div style={{ position: 'relative', display: mob ? 'grid' : 'flex', gridTemplateColumns: '1fr 1fr', gap: 10 }}>
        <button className="btn btn-white" style={{ height: mob ? 50 : 48, borderRadius: 14, fontSize: 15 }} onClick={() => nav('/painel/saldo')}><Icon name="arrowUpRight" size={17} stroke={2.2} />Levantar</button>
        <button className="btn btn-green" style={{ height: mob ? 50 : 48, borderRadius: 14, fontSize: 15 }} onClick={openNewLink}><Icon name="plus" size={17} stroke={2.2} />Criar link</button>
      </div>
    </div>
  )

  return (
    <>
      <div className="page only-desktop">
        <div style={{ display: 'flex', alignItems: 'flex-end', justifyContent: 'space-between', gap: 16 }}>
          <div><div style={{ fontSize: 14, color: 'var(--muted)' }}>{longToday()}</div><h1 className="h1" style={{ marginTop: 4 }}>{greeting()}, {merchant.name}</h1></div>
          <div className="seg">{RANGES.map(([k, l]) => <button key={k} className={days === k ? 'on' : ''} onClick={() => setDays(k)}>{l}</button>)}</div>
        </div>
        <div style={{ display: 'grid', gridTemplateColumns: 'minmax(0,5fr) minmax(0,7fr)', gap: 16, marginTop: 22 }}>
          {balanceCard(false)}
          <div className="card" style={{ padding: 24, borderRadius: 26 }}>
            <div className="between" style={{ alignItems: 'flex-start' }}>
              <div><div style={{ fontSize: 14, color: 'var(--muted)' }}>Recebido {days === 365 ? 'nos últimos 12 meses' : `nos últimos ${days} dias`}</div><div className="display" style={{ fontSize: 32, letterSpacing: '-0.035em', marginTop: 4 }}>{d ? fmt(d.received) : '—'} <span style={{ fontSize: 15, color: 'var(--muted)' }}>MT</span></div></div>
              {ch !== null && <span className="pill" style={{ fontSize: 13, padding: '5px 10px', background: ch >= 0 ? 'var(--green-bg)' : 'var(--red-bg)', color: ch >= 0 ? 'var(--green-dark)' : 'var(--red-dark)' }}>{ch >= 0 ? '+' : ''}{ch}% vs. período anterior</span>}
            </div>
            <div style={{ marginTop: 20 }}>{d ? <Bars values={bars} height={150} highlightFrom={bars.length - 7} /> : <Skeleton h={150} />}</div>
            <div className="between" style={{ marginTop: 8, fontSize: 12, color: 'var(--muted-2)' }}>
              {days === 365 && d ? <><span>{monthName(new Date(first).getMonth())}</span><span>{monthName(new Date(mid).getMonth())}</span><span>Este mês</span></> : d ? <><span>{shortDate(first)}</span><span>{shortDate(mid)}</span><span>Hoje</span></> : <span />}
            </div>
          </div>
        </div>
        <div style={{ display: 'grid', gridTemplateColumns: 'minmax(0,8fr) minmax(0,4fr)', gap: 16, marginTop: 16 }}>
          <div className="table" style={{ borderRadius: 24, padding: '8px 0' }}>
            <div className="between" style={{ padding: '14px 22px 10px' }}><div className="h2">Atividade recente</div><Link to="/painel/transacoes" style={{ fontSize: 14, fontWeight: 600 }}>Ver todas</Link></div>
            <div style={{ borderTop: '1px solid var(--line-3)' }}>
              {d?.recent?.map((p: any) => <TxLine key={p.id} p={p} desktop onClick={() => nav('/painel/transacoes/' + p.id)} />)}
              {d && !d.recent.length && <EmptyTx onNew={openNewLink} />}
            </div>
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
            <div className="card" style={{ borderRadius: 24, padding: 22 }}>
              <div className="h2">Por método</div>
              <div style={{ display: 'flex', flexDirection: 'column', gap: 14, marginTop: 16 }}>
                {([['mpesa', 'M-Pesa', 'var(--mpesa)'], ['emola', 'e-Mola', 'var(--emola-bar)'], ['card', 'Cartão', 'var(--ink)']] as const).map(([k, l, c]) => (
                  <div key={k}><div className="between" style={{ fontSize: 13.5 }}><span style={{ fontWeight: 600 }}>{l}</span><span className="muted tnum">{fmt(bm[k])} MT</span></div><div className="bar-track" style={{ marginTop: 7 }}><div className="bar-fill" style={{ width: pct(bm[k]) + '%', background: c }} /></div></div>
                ))}
              </div>
            </div>
            {d?.pending_payout ? <PayoutProgress p={d.pending_payout} /> : (
              <div className="card" style={{ borderRadius: 24, padding: 22 }}>
                <div style={{ fontWeight: 600, fontSize: 15 }}>Sem levantamentos em curso</div>
                <div style={{ fontSize: 13.5, color: 'var(--muted)', marginTop: 6, lineHeight: 1.5 }}>Peça um levantamento e o dinheiro chega ao seu banco em até 72 horas.</div>
                <button className="btn btn-outline btn-sm" style={{ marginTop: 14 }} onClick={() => nav('/painel/saldo')}>Levantar</button>
              </div>
            )}
          </div>
        </div>
      </div>

      <div className="page only-mobile">
        {balanceCard(true)}
        <div className="card" style={{ marginTop: 12, borderRadius: 24, padding: 18 }}>
          <div className="between" style={{ alignItems: 'flex-start' }}>
            <div><div style={{ fontSize: 13, color: 'var(--muted)' }}>Recebido nos últimos 7 dias</div><div className="display" style={{ fontSize: 24, marginTop: 3 }}>{mobile ? fmt(mobile.received) : '—'} <span style={{ fontSize: 13, color: 'var(--muted)' }}>MT</span></div></div>
            {change(mobile) !== null && <span className="pill" style={{ fontSize: 12.5, padding: '5px 9px', background: change(mobile)! >= 0 ? 'var(--green-bg)' : 'var(--red-bg)', color: change(mobile)! >= 0 ? 'var(--green-dark)' : 'var(--red-dark)' }}>{change(mobile)! >= 0 ? '+' : ''}{change(mobile)}%</span>}
          </div>
          <div style={{ marginTop: 14 }}>{mobile && <Bars values={mobile.bars.map((b: any) => b.amount)} height={96} gap={8} radius={8} labels={mobile.bars.map((b: any, i: number) => (i === 6 ? 'Hoje' : weekday(b.date).slice(0, 3)))} />}</div>
          {mobile && (() => { const m = mobile.by_method; const t = m.mpesa + m.emola + m.card || 1; const p = (v: number) => Math.round((v / t) * 100); return (
            <>
              <div style={{ display: 'flex', height: 8, borderRadius: 4, overflow: 'hidden', gap: 2, marginTop: 16 }}><div style={{ width: p(m.mpesa) + '%', background: 'var(--mpesa)' }} /><div style={{ width: p(m.emola) + '%', background: 'var(--emola-bar)' }} /><div style={{ width: p(m.card) + '%', background: 'var(--ink)' }} /></div>
              <div style={{ display: 'flex', gap: 14, marginTop: 10, fontSize: 12, color: 'var(--text-2)', flexWrap: 'wrap' }}>
                <span className="row" style={{ gap: 5 }}><span style={{ width: 8, height: 8, borderRadius: 2, background: 'var(--mpesa)' }} />M-Pesa {p(m.mpesa)}%</span>
                <span className="row" style={{ gap: 5 }}><span style={{ width: 8, height: 8, borderRadius: 2, background: 'var(--emola-bar)' }} />e-Mola {p(m.emola)}%</span>
                <span className="row" style={{ gap: 5 }}><span style={{ width: 8, height: 8, borderRadius: 2, background: 'var(--ink)' }} />Cartão {p(m.card)}%</span>
              </div>
            </>) })()}
        </div>
        <div className="between" style={{ margin: '24px 2px 10px', alignItems: 'baseline' }}><div className="display" style={{ fontSize: 20, letterSpacing: '-0.02em' }}>Atividade recente</div><Link to="/painel/transacoes" style={{ fontSize: 14, fontWeight: 600 }}>Ver tudo</Link></div>
        <div className="card" style={{ borderRadius: 22, padding: '4px 0' }}>
          {mobile?.recent?.slice(0, 4).map((p: any) => <TxLine key={p.id} p={p} onClick={() => nav('/painel/transacoes/' + p.id)} />)}
          {mobile && !mobile.recent.length && <EmptyTx onNew={openNewLink} />}
        </div>
        <div style={{ marginTop: 12 }}>{mobile?.pending_payout && <PayoutProgress p={mobile.pending_payout} />}</div>
      </div>
    </>
  )
}

function EmptyTx({ onNew }: { onNew: () => void }) {
  return (
    <div className="empty">
      <div style={{ fontWeight: 600, color: 'var(--ink)', fontSize: 15 }}>Ainda sem pagamentos</div>
      <div style={{ marginTop: 4 }}>Crie um link e partilhe por WhatsApp para receber o primeiro.</div>
      <button className="btn btn-green btn-sm" style={{ marginTop: 14 }} onClick={onNew}><Icon name="plus" size={14} stroke={2.4} />Criar link</button>
    </div>
  )
}

export { SOURCE }
