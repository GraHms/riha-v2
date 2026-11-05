import { Link, useNavigate } from 'react-router-dom'
import { useQuery } from '../../core/useQuery'
import { fmt, shortDate } from '../../lib/format'
import { Avatar, Icon, MethodIcon, Pill, Skeleton, Stripes } from '../../components/ui'
import { RiskPill, SlaBar, submittedAgo } from './common'

const HEALTH: Record<string, [string, string, string]> = { operational: ['Operacional', '#E2F3DE', '#0A6B05'], degraded: ['Lento', '#FFE58A', '#5B4600'], down: ['Indisponível', '#FFE3E1', '#C2080D'] }

export default function AdminOverview() {
  const nav = useNavigate()
  const { data: d } = useQuery(g => g.admin.overview(), [], 15000)
  if (!d) return <div className="page"><Skeleton h={40} w={300} /><Skeleton h={120} style={{ marginTop: 22 }} /></div>
  const growth = d.volume_week_ago > 0 ? Math.round(((d.volume_today - d.volume_week_ago) / d.volume_week_ago) * 100) : null
  const rate = d.tx_today ? ((d.tx_ok_today / d.tx_today) * 100).toFixed(1).replace('.', ',') : '—'
  const max = Math.max(1, ...d.stack.map((s: any) => s.mpesa + s.emola + (s.bim || 0) + s.card))
  const H = 190
  const oldest = d.approvals[0]?.submitted_at
  const st = d.payout_stats
  return (
    <div className="page">
      <h1 className="h1">Visão geral da plataforma</h1>
      <div className="grid-4" style={{ marginTop: 22 }}>
        <div className="dark-card" style={{ borderRadius: 22, padding: 20 }}>
          <Stripes top={-30} right={2} w={24} h={200} gap={8} />
          <div style={{ position: 'relative', fontSize: 13, color: '#A9B5AE' }}>Volume processado hoje</div>
          <div className="display" style={{ position: 'relative', fontSize: 30, letterSpacing: '-0.04em', marginTop: 6 }}>{fmt(d.volume_today, false)} <span style={{ fontSize: 14, color: '#A9B5AE' }}>MT</span></div>
          <div style={{ position: 'relative', fontSize: 12.5, color: growth !== null && growth < 0 ? '#FF8A8D' : 'var(--lime)', marginTop: 4 }}>{growth === null ? 'Sem dados da semana passada' : `${growth >= 0 ? '+' : ''}${growth}% vs. semana passada`}</div>
        </div>
        <div className="card" style={{ padding: 20 }}><div style={{ fontSize: 13, color: 'var(--muted)' }}>Transações hoje</div><div className="display" style={{ fontSize: 30, letterSpacing: '-0.04em', marginTop: 6 }}>{d.tx_today}</div><div style={{ fontSize: 12.5, color: 'var(--muted)', marginTop: 4 }}>{rate}% concluídas</div></div>
        <button onClick={() => nav('/admin/aprovacoes')} style={{ background: 'var(--sun)', border: 0, borderRadius: 22, padding: 20, textAlign: 'left' }}><div style={{ fontSize: 13, color: '#3B3410' }}>Contas por aprovar</div><div className="display" style={{ fontSize: 30, letterSpacing: '-0.04em', marginTop: 6 }}>{d.pending_approvals}</div><div style={{ fontSize: 12.5, color: '#3B3410', marginTop: 4, fontWeight: 600 }}>{oldest ? `Mais antiga ${submittedAgo(oldest)} →` : 'Fila vazia'}</div></button>
        <button onClick={() => nav('/admin/levantamentos')} className="card" style={{ padding: 20, textAlign: 'left' }}><div style={{ fontSize: 13, color: 'var(--muted)' }}>Levantamentos pendentes</div><div className="display" style={{ fontSize: 30, letterSpacing: '-0.04em', marginTop: 6 }}>{fmt(st.pending_total, false)} <span style={{ fontSize: 14, color: 'var(--muted)' }}>MT</span></div><div style={{ fontSize: 12.5, color: st.urgent ? 'var(--red-dark)' : 'var(--muted)', marginTop: 4, fontWeight: 600 }}>{st.urgent} {st.urgent === 1 ? 'vence' : 'vencem'} em menos de 8 h</div></button>
      </div>
      <div className="adm-row" style={{ marginTop: 14 }}>
        <style>{`.adm-row{display:grid;grid-template-columns:minmax(0,8fr) minmax(0,4fr);gap:14px}@media(max-width:1100px){.adm-row{grid-template-columns:1fr}}`}</style>
        <div className="card" style={{ padding: 22 }}>
          <div className="between" style={{ alignItems: 'flex-start', flexWrap: 'wrap' }}>
            <div><div className="h2">Volume por método · 14 dias</div><div style={{ fontSize: 13, color: 'var(--muted)', marginTop: 2 }}>{fmt(d.volume_14d, false)} MT processados</div></div>
            <div className="row" style={{ gap: 14, fontSize: 12.5, color: 'var(--text-2)' }}>{[['var(--mpesa)', 'M-Pesa'], ['var(--emola-bar)', 'e-Mola'], ['var(--bim-bar)', 'bim'], ['var(--ink)', 'Cartão']].map(([c, l]) => <span key={l} className="row" style={{ gap: 6 }}><span style={{ width: 9, height: 9, borderRadius: 3, background: c }} />{l}</span>)}</div>
          </div>
          <div style={{ display: 'flex', alignItems: 'flex-end', gap: 10, height: H, marginTop: 20 }}>
            {d.stack.map((s: any) => (
              <div key={s.date} title={`${shortDate(s.date)}: ${fmt(s.mpesa + s.emola + (s.bim || 0) + s.card, false)} MT`} style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 2, justifyContent: 'flex-end', height: '100%' }}>
                <div style={{ height: (s.card / max) * H, background: 'var(--ink)', borderRadius: '5px 5px 2px 2px' }} />
                <div style={{ height: (s.emola / max) * H, background: 'var(--emola-bar)', borderRadius: 2 }} />
                <div style={{ height: ((s.bim || 0) / max) * H, background: 'var(--bim-bar)', borderRadius: 2 }} />
                <div style={{ height: Math.max(2, (s.mpesa / max) * H), background: 'var(--mpesa)', borderRadius: '2px 2px 5px 5px' }} />
              </div>
            ))}
          </div>
          <div className="between" style={{ marginTop: 8, fontSize: 12, color: 'var(--muted-2)' }}><span>{shortDate(d.stack[0].date)}</span><span>{shortDate(d.stack[7].date)}</span><span>Hoje</span></div>
        </div>
        <div className="card" style={{ padding: 22 }}>
          <div className="h2">Estado dos operadores</div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 10, marginTop: 16 }}>
            {(['mpesa', 'emola', 'bim', 'card'] as const).map(m => {
              const h = d.health[m], hs = HEALTH[h.status]
              return (
                <div key={m} className="row" style={{ padding: 12, borderRadius: 14, background: h.status === 'operational' ? 'var(--paper)' : 'var(--sun-soft)' }}>
                  <MethodIcon method={m} size={34} radius={10} />
                  <div style={{ flex: 1 }}><div style={{ fontWeight: 600, fontSize: 14 }}>{({ card: 'Cartões', mpesa: 'M-Pesa', emola: 'e-Mola', bim: 'bim' } as Record<string, string>)[m]}</div><div style={{ fontSize: 12, color: 'var(--muted)' }}>{h.success_rate === null ? 'Sem tráfego nas últimas 24 h' : `${(h.success_rate * 100).toFixed(1).replace('.', ',')}% sucesso · ${h.total} tentativas`}</div></div>
                  <Pill t={hs[0]} bg={hs[1]} fg={hs[2]} />
                </div>
              )
            })}
          </div>
          <div style={{ fontSize: 11.5, color: 'var(--muted)', marginTop: 12 }}>Integrações simuladas: os operadores reais ainda não estão ligados.</div>
        </div>
      </div>
      <div className="grid-2" style={{ marginTop: 14, gap: 14 }}>
        <div className="card" style={{ padding: '8px 0' }}>
          <div className="between" style={{ padding: '14px 22px 10px' }}><div className="h2">Levantamentos a vencer</div><Link to="/admin/levantamentos" style={{ fontSize: 14, fontWeight: 600 }}>Ver fila</Link></div>
          {d.payouts_due.map((p: any) => (
            <div key={p.id} style={{ display: 'grid', gridTemplateColumns: 'minmax(0,1fr) 120px 130px', gap: 14, alignItems: 'center', padding: '12px 22px', borderTop: '1px solid var(--line-3)' }}>
              <div style={{ minWidth: 0 }}><div style={{ fontWeight: 600, fontSize: 14 }}>{p.merchant_name}</div><div className="mono" style={{ fontSize: 12, color: 'var(--muted)' }}>{p.reference} · {p.bank_account.bank}</div></div>
              <div className="tnum" style={{ fontWeight: 600, fontSize: 14, textAlign: 'right' }}>{fmt(p.amount, false)} MT</div>
              <SlaBar p={p} />
            </div>
          ))}
          {!d.payouts_due.length && <div className="empty">Nada pendente.</div>}
        </div>
        <div className="card" style={{ padding: '8px 0' }}>
          <div className="between" style={{ padding: '14px 22px 10px' }}><div className="h2">Fila de aprovações</div><Link to="/admin/aprovacoes" style={{ fontSize: 14, fontWeight: 600 }}>Rever</Link></div>
          {d.approvals.map((a: any) => (
            <button key={a.id} onClick={() => nav('/admin/aprovacoes?id=' + a.id)} className="row" style={{ width: '100%', border: 0, borderTop: '1px solid var(--line-3)', background: 'none', textAlign: 'left', padding: '12px 22px' }}>
              <Avatar text={a.initials} color={a.avatar_color} size={36} radius={11} />
              <div style={{ flex: 1, minWidth: 0 }}><div style={{ fontWeight: 600, fontSize: 14 }}>{a.name}</div><div style={{ fontSize: 12.5, color: 'var(--muted)' }}>{a.business_type === 'company' ? 'Empresa' : 'Em nome individual'} · {a.city}</div></div>
              <RiskPill risk={a.risk} /><span style={{ fontSize: 12.5, color: 'var(--muted)', width: 70, textAlign: 'right' }}>{submittedAgo(a.submitted_at)}</span>
            </button>
          ))}
          {!d.approvals.length && <div className="empty">Sem contas por aprovar. <Icon name="check" size={14} /></div>}
        </div>
      </div>
    </div>
  )
}
