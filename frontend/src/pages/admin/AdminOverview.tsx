import { useNavigate } from 'react-router-dom'
import { can } from '../../core/model'
import { useQuery } from '../../core/useQuery'
import { fmt, shortDate } from '../../lib/format'
import { useSession } from '../../lib/session'
import { Avatar, MethodIcon, Skeleton } from '../../components/ui'
import { Amount, Badge, Empty, HOLDER_COLORS, List, MoneyMap, Page, PageHead, Panel, Stat } from '../../components/ops'
import { RiskPill, SlaBar, submittedAgo } from './common'

const HEALTH: Record<string, [string, 'ok' | 'warn' | 'bad']> = { operational: ['Operacional', 'ok'], degraded: ['Lento', 'warn'], down: ['Em baixo', 'bad'] }
const METHODS: [string, string, string][] = [['mpesa', 'M-Pesa', 'var(--mpesa)'], ['emola', 'e-Mola', 'var(--emola-bar)'], ['bim', 'bim', 'var(--bim-bar)'], ['card', 'Cartão', '#2B3631']]

export default function AdminOverview() {
  const nav = useNavigate()
  const { me } = useSession()
  const { data: d } = useQuery(g => g.admin.overview(), [], 15000)
  const money = useQuery(can(me, 'settlement') ? g => g.admin.settlement() : null, [], 60000)
  if (!d) return <Page><Skeleton h={30} w={240} /><Skeleton h={110} style={{ marginTop: 20 }} /><Skeleton h={260} style={{ marginTop: 14 }} /></Page>
  const growth = d.volume_week_ago > 0 ? Math.round(((d.volume_today - d.volume_week_ago) / d.volume_week_ago) * 100) : null
  const rate = d.tx_today ? ((d.tx_ok_today / d.tx_today) * 100).toFixed(1).replace('.', ',') : null
  const total = (s: any) => s.mpesa + s.emola + (s.bim || 0) + s.card
  const max = Math.max(1, ...d.stack.map(total))
  const st = d.payout_stats
  const m = money.data
  const today = new Date().toLocaleDateString('pt-PT', { weekday: 'long', day: 'numeric', month: 'long' })
  return (
    <Page>
      <PageHead title="Visão geral" sub={today.charAt(0).toUpperCase() + today.slice(1)} />
      <div className="o-stats">
        <Stat tone="ink" label="Volume hoje" value={d.volume_today} delta={growth} hint={growth === null ? 'sem dados da semana passada' : 'vs. mesmo dia da semana passada'} />
        <Stat label="Transações hoje" value={String(d.tx_today)} hint={rate ? <><Badge tone={Number(rate.replace(',', '.')) >= 90 ? 'ok' : 'warn'}>{rate}%</Badge>com sucesso</> : 'nenhuma ainda'} />
        <Stat label="Levantamentos por pagar" value={st?.pending_total} tone={st?.urgent ? 'bad' : undefined} onClick={can(me, 'payouts') ? () => nav('/admin/levantamentos') : undefined}
          hint={st?.urgent ? <Badge tone="bad" dot title="Vencem em menos de 8 horas">{st.urgent} {st.urgent === 1 ? 'urgente' : 'urgentes'}</Badge> : `${st?.pending_count ?? 0} pedidos`} />
        <Stat label="Contas por aprovar" value={String(d.pending_approvals ?? d.approvals.length)} tone={d.pending_approvals ? 'warn' : undefined}
          onClick={can(me, 'approvals') ? () => nav('/admin/aprovacoes') : undefined} hint={d.approvals[0] ? `a mais antiga ${submittedAgo(d.approvals[0].submitted_at)}` : 'fila vazia'} />
      </div>

      {m && (
        <div style={{ marginTop: 14 }}>
          <MoneyMap owed={m.position.owed_to_merchants}
            holders={[
              ...m.partners.map((p: any, i: number) => ({ key: p.integration.code, label: p.partner_name, amount: p.held, color: HOLDER_COLORS[i % HOLDER_COLORS.length] })),
              { key: 'riha', label: 'Conta RIHA', amount: m.position.at_riha, color: 'var(--o-riha)' },
            ]}
            action={<button className="btn btn-outline btn-sm" onClick={() => nav('/admin/liquidacao')}>Liquidação</button>} />
        </div>
      )}

      <div className="o-grid main-side" style={{ marginTop: 14 }}>
        <Panel title="Volume por método" sub={`14 dias · ${fmt(d.volume_14d, false)} MT`}
          action={<div style={{ display: 'flex', gap: 12, flexWrap: 'wrap' }}>{METHODS.map(([, l, c]) => <span key={l} style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 12, color: 'var(--o-ink-2)' }}><i className="o-dot" style={{ background: c }} />{l}</span>)}</div>}>
          <div style={{ display: 'flex', alignItems: 'flex-end', gap: 6, height: 180 }}>
            {d.stack.map((s: any, i: number) => (
              <div key={s.date} title={`${shortDate(s.date)} · ${fmt(total(s), false)} MT`} style={{ flex: 1, height: '100%', display: 'flex', flexDirection: 'column', justifyContent: 'flex-end', gap: 2, opacity: i === d.stack.length - 1 ? 1 : 0.9 }}>
                {[['card', '#2B3631'], ['bim', 'var(--bim-bar)'], ['emola', 'var(--emola-bar)'], ['mpesa', 'var(--mpesa)']].map(([k, c], j) => {
                  const v = s[k] || 0
                  if (!v && !(k === 'mpesa' && !total(s))) return null
                  return <div key={k} style={{ height: total(s) ? `${(v / max) * 100}%` : 3, minHeight: v ? 3 : 0, background: total(s) ? c : 'var(--o-line)', borderRadius: j === 0 ? '4px 4px 1px 1px' : 1 }} />
                })}
              </div>
            ))}
          </div>
          <div className="between" style={{ marginTop: 8, fontSize: 11.5, color: 'var(--o-muted)' }}><span>{shortDate(d.stack[0].date)}</span><span>{shortDate(d.stack[7].date)}</span><span>Hoje</span></div>
        </Panel>
        <Panel title="Operadores" sub="Últimos 30 minutos" pad={false}>
          {METHODS.map(([k, l]) => {
            const h = d.health[k], [t, tone] = HEALTH[h.status]
            return (
              <div key={k} className="row" style={{ padding: '11px 18px', borderTop: '1px solid var(--o-line-2)', gap: 12 }}>
                <MethodIcon method={k} size={30} radius={9} />
                <div style={{ flex: 1, minWidth: 0 }}><div className="o-t1">{l}</div><div className="o-t2">{h.total ? `${h.total} tentativas · ${Math.round((h.success_rate || 0) * 100)}% sucesso` : 'Sem tentativas'}</div></div>
                <Badge tone={tone} dot>{t}</Badge>
              </div>
            )
          })}
        </Panel>
      </div>

      <div className="o-grid c2" style={{ marginTop: 14 }}>
        <Panel title="Levantamentos a vencer" pad={false} action={can(me, 'payouts') && <button className="btn btn-ghost btn-sm" onClick={() => nav('/admin/levantamentos')}>Ver todos</button>}>
          <List head={false} rows={d.payouts_due} rowKey={(p: any) => p.id} onRow={can(me, 'payouts') ? () => nav('/admin/levantamentos') : undefined}
            empty={<Empty title="Nada por pagar" text="Todos os levantamentos estão em dia." />}
            cols={[
              { head: '', w: 'minmax(0,1fr)', cell: (p: any) => <><div className="o-t1">{p.merchant_name}</div><div className="o-t2"><span className="o-mono">{p.reference}</span> · {p.bank_account.bank}</div></> },
              { head: '', w: '120px', right: true, cell: (p: any) => <Amount c={p.amount} cents={false} /> },
              { head: '', w: '130px', cell: (p: any) => <SlaBar p={p} /> },
            ]}
            card={(p: any) => ({ title: p.merchant_name, sub: <><span className="o-mono">{p.reference}</span> · {p.bank_account.bank}</>, amount: <Amount c={p.amount} cents={false} />, foot: <div style={{ flex: 1 }}><SlaBar p={p} /></div> })} />
        </Panel>
        <Panel title="Fila de aprovações" pad={false} action={can(me, 'approvals') && <button className="btn btn-ghost btn-sm" onClick={() => nav('/admin/aprovacoes')}>Rever</button>}>
          <List head={false} rows={d.approvals} rowKey={(a: any) => a.id} onRow={can(me, 'approvals') ? (a: any) => nav('/admin/aprovacoes?id=' + a.id) : undefined}
            empty={<Empty title="Fila vazia" text="Sem contas à espera de aprovação." />}
            cols={[
              { head: '', w: '36px', cell: (a: any) => <Avatar text={a.initials} color={a.avatar_color} size={32} radius={9} font={12} /> },
              { head: '', w: 'minmax(0,1fr)', cell: (a: any) => <><div className="o-t1">{a.name}</div><div className="o-t2">{a.business_type === 'company' ? 'Empresa' : 'Em nome individual'}{a.city ? ' · ' + a.city : ''}</div></> },
              { head: '', w: 'auto', cell: (a: any) => <RiskPill risk={a.risk} /> },
              { head: '', w: '64px', right: true, cell: (a: any) => <span className="o-t2">{submittedAgo(a.submitted_at)}</span> },
            ]}
            card={(a: any) => ({ lead: <Avatar text={a.initials} color={a.avatar_color} size={34} radius={10} font={12} />, title: a.name, sub: `${a.business_type === 'company' ? 'Empresa' : 'Em nome individual'} · ${submittedAgo(a.submitted_at)}`, meta: <RiskPill risk={a.risk} /> })} />
        </Panel>
      </div>
    </Page>
  )
}
