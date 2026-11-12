import { useNavigate, useSearchParams } from 'react-router-dom'
import { can } from '../../core/model'
import { useQuery } from '../../core/useQuery'
import { bpsLabel, fmt, initials, METHOD_NAME } from '../../lib/format'
import { useSession } from '../../lib/session'
import { Avatar, Icon, Skeleton } from '../../components/ui'
import { Amount, Badge, Empty, KV, List, Money, Page, PageHead, Panel, Section, Stat, Tip } from '../../components/ops'

// A partner's money with RIHA: its agreement, what its account collected, what
// it still holds for RIHA, and what it has paid.

const iso = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
const day = (s: string) => new Date(s + 'T00:00:00').toLocaleDateString('pt-PT', { day: 'numeric', month: 'short' })
const PERIODS: [string, string, () => [string, string]][] = [
  ['30', 'Últimos 30 dias', () => { const n = new Date(); return [iso(new Date(n.getFullYear(), n.getMonth(), n.getDate() - 29)), iso(n)] }],
  ['month', 'Este mês', () => { const n = new Date(); return [iso(new Date(n.getFullYear(), n.getMonth(), 1)), iso(n)] }],
  ['last', 'Mês passado', () => { const n = new Date(); return [iso(new Date(n.getFullYear(), n.getMonth() - 1, 1)), iso(new Date(n.getFullYear(), n.getMonth(), 0))] }],
]

const CSS = `
.ph-hero{display:grid;grid-template-columns:minmax(0,1.35fr) minmax(0,1fr);overflow:hidden}
.ph-hold{padding:22px 24px;display:flex;flex-direction:column;gap:6px;min-width:0}
.ph-hold .o-stat-v{font-size:38px;margin-top:4px}
.ph-hold .o-stat-v.neg{color:var(--o-bad)}
.ph-res{display:flex;align-items:center;gap:8px;flex-wrap:wrap;margin-top:10px;font-size:12.5px;color:var(--o-muted)}
.ph-res b{color:var(--o-ink);font-variant-numeric:tabular-nums}
.ph-pay{padding:22px 24px;display:flex;flex-direction:column;align-items:flex-start;gap:8px;min-width:0;background:var(--o-surface-2);border-left:1px solid var(--o-line-2)}
.ph-pay.hot{background:linear-gradient(180deg,#FFF8E8,#FFFDF8);border-left-color:#F5DFAE}
.ph-pay .o-stat-v{font-size:28px}
.ph-pay .btn{margin-top:auto;width:100%;height:42px}
.ph-kh{font-size:11.5px;font-weight:700;color:var(--o-muted);letter-spacing:.04em;text-transform:uppercase;margin-bottom:4px}
.ph-kh.sp{margin-top:14px}
.ph-note{margin-top:10px;white-space:normal}
.ph-deals{display:flex;flex-direction:column;gap:14px}
.ph-seg button{white-space:nowrap}
.ph-bar{width:100%;max-width:220px}
.ph-ico{width:34px;height:34px;border-radius:10px;display:grid;place-items:center;background:var(--o-brand-soft);color:var(--o-brand-ink)}
@media(max-width:860px){
  .ph-hero{grid-template-columns:minmax(0,1fr)}
  .ph-hold{padding:18px}
  .ph-hold .o-stat-v{font-size:30px}
  .ph-pay{padding:18px;border-left:0;border-top:1px solid var(--o-line-2)}
  .ph-pay.hot{border-top-color:#F5DFAE}
  .ph-pay .o-stat-v{font-size:24px}
  .ph-pay .btn{height:44px;margin-top:6px}
  .ph-seg{width:100%}
  .ph-seg button{flex:1;padding-left:4px;padding-right:4px}
  .ph-pstats > :first-child{grid-column:1/-1}
}`

export default function PartnerHome() {
  const nav = useNavigate()
  const { me } = useSession()
  const [sp, setSp] = useSearchParams()
  const period = sp.get('p') || '30'
  const [from, to] = (PERIODS.find(p => p[0] === period) || PERIODS[0])[2]()
  const { data } = useQuery(g => g.partner.home(from, to), [from, to])
  if (!data) return <Page><Skeleton h={30} w={200} /><Skeleton h={170} style={{ marginTop: 20 }} /><Skeleton h={220} style={{ marginTop: 14 }} /></Page>
  const n = data.to_pay_count || 0
  const maxDay = Math.max(1, ...data.days.map((d: any) => d.amount))
  return (
    <Page>
      <style>{CSS}</style>
      <PageHead title="Resumo" sub={data.partner.name} />

      <section className="o-panel ph-hero">
        <div className="ph-hold">
          <span className="o-stat-l">Ainda consigo, devido à RIHA<Tip text="Recebido de clientes, menos a sua parte e o que já pagou à RIHA e aos comerciantes." /></span>
          <div className={'o-stat-v' + (data.held < 0 ? ' neg' : '')}><Money c={data.held} /></div>
          <div className="ph-res">
            <span>Reservado para levantamentos</span><b>{fmt(data.reserved)} MT</b>
            <Tip text="Pedidos de comerciantes que vai pagar quando a RIHA aprovar." />
          </div>
        </div>
        <div className={'ph-pay' + (n ? ' hot' : '')}>
          <span className="o-stat-l">A pagar a comerciantes agora</span>
          <div className="o-stat-v"><Money c={data.to_pay} /></div>
          {n ? <Badge tone="warn" dot>{n} {n === 1 ? 'aprovado pela RIHA' : 'aprovados pela RIHA'}</Badge> : <Badge tone="ok" dot>Nada à sua espera</Badge>}
          {n > 0 && can(me, 'payouts') && <button className="btn btn-green" onClick={() => nav('/parceiro/levantamentos')}>Pagar levantamentos<Icon name="arrowRight" size={16} /></button>}
        </div>
      </section>

      <Section title="O seu acordo" />
      {data.integrations.length ? (
        <div className="ph-deals">
          {data.integrations.map((i: any) => {
            const ops = Object.entries(i.platform_bps || {}).filter(([, v]) => (v as number) > 0)
            return (
              <Panel key={i.code} title={i.name} sub={`${i.account}${i.methods.length ? ` · ${i.methods.map((m: string) => METHOD_NAME[m] || m).join(', ')}` : ''}`}>
                <div className="o-grid c2" style={{ gap: 24 }}>
                  <div>
                    <div className="ph-kh">Condições</div>
                    <KV k={<>A sua parte <Tip text="Da taxa da RIHA, depois da taxa do operador." /></>} v={`${bpsLabel(i.share_bps)} da taxa da RIHA`} />
                    <KV k="Taxa do operador" v={ops.length ? ops.map(([m, v]) => `${METHOD_NAME[m] || m} ${bpsLabel(v as number)}`).join(', ') : 'Nenhuma'} />
                    {i.deals.length > 0 && <div className="ph-kh sp">Por comerciante</div>}
                    {i.deals.map((d: any) => <KV key={d.merchant_name} k={d.merchant_name} v={bpsLabel(d.share_bps)} />)}
                    {i.note && <div className="o-t2 ph-note">{i.note}</div>}
                  </div>
                  <div>
                    <div className="ph-kh">Desde o início</div>
                    <KV k="Recebido de clientes" v={`${fmt(i.all_time.amount)} MT`} />
                    <KV k="Taxa do operador" v={`${fmt(i.all_time.platform_fee)} MT`} />
                    <KV k="A sua parte" v={`${fmt(i.all_time.partner_fee)} MT`} />
                    <KV k="Pago à RIHA" v={`${fmt(i.received)} MT`} />
                    <KV k="Pago a comerciantes" v={`${fmt(i.paid_out)} MT`} />
                    <KV total k="Ainda consigo" v={`${fmt(i.held)} MT`} />
                  </div>
                </div>
              </Panel>
            )
          })}
        </div>
      ) : (
        <div className="o-panel"><Empty icon="link" title="Sem integrações" text="Ainda sem integrações ligadas a esta conta." /></div>
      )}

      <Section title="No período" sub={`${day(from)} – ${day(to)}`}
        actions={<div className="seg ph-seg">{PERIODS.map(([k, l]) => <button key={k} className={period === k ? 'on' : ''} onClick={() => setSp({ p: k })}>{l}</button>)}</div>} />
      <div className="o-stats ph-pstats">
        <Stat label="A sua parte" value={data.period.partner_fee} cents hint={`${data.period.count} pagamentos`} />
        <Stat label="Recebido de clientes" value={data.period.amount} cents />
        <Stat label="Taxa do operador" value={data.period.platform_fee} cents />
      </div>

      <Section title="Por comerciante" />
      <List rows={data.merchants} rowKey={(m: any) => m.name}
        empty={<Empty icon="store" title="Sem pagamentos neste período" />}
        cols={[
          { head: 'Comerciante', w: 'minmax(0,1.6fr)', cell: (m: any) => <div className="row" style={{ gap: 10 }}><Avatar text={initials(m.name)} color="#DCEFD6" size={30} radius={9} font={11} /><span className="o-t1">{m.name}</span></div> },
          { head: 'Pag.', w: '70px', right: true, cell: (m: any) => <span className="o-t2">{m.count}</span> },
          { head: 'Recebido', w: 'minmax(0,1fr)', right: true, cell: (m: any) => <Amount c={m.amount} /> },
          { head: 'Operador', w: 'minmax(0,1fr)', right: true, cell: (m: any) => <span className="o-t2">{fmt(m.platform_fee)}</span> },
          { head: 'A sua parte', w: 'minmax(0,1fr)', right: true, cell: (m: any) => <Amount c={m.partner_fee} /> },
        ]}
        card={(m: any) => ({
          lead: <Avatar text={initials(m.name)} color="#DCEFD6" size={34} radius={10} font={12} />,
          title: m.name,
          sub: `${m.count} pag. · operador ${fmt(m.platform_fee)} MT`,
          amount: <Amount c={m.amount} />,
          meta: <span className="o-t2">sua parte {fmt(m.partner_fee)}</span>,
        })} />

      <div className="o-grid c2" style={{ alignItems: 'start' }}>
        <div>
          <Section title="Por dia" />
          <List rows={[...data.days].reverse()} rowKey={(d: any) => d.day}
            empty={<Empty icon="calendar" title="Sem pagamentos neste período" />}
            cols={[
              { head: 'Dia', w: '84px', cell: (d: any) => <b>{day(d.day)}</b> },
              { head: 'Recebido', w: 'minmax(0,1fr)', cell: (d: any) => <><Amount c={d.amount} /><div className="o-bar ph-bar" style={{ marginTop: 5 }}><i style={{ width: `${(d.amount / maxDay) * 100}%`, background: 'var(--o-brand)' }} /></div></> },
              { head: 'Pag.', w: '44px', right: true, cell: (d: any) => <span className="o-t2">{d.count}</span> },
              { head: 'A sua parte', w: 'minmax(0,.8fr)', right: true, cell: (d: any) => <Amount c={d.partner_fee} /> },
            ]}
            card={(d: any) => ({
              title: day(d.day),
              sub: `${d.count} pag. · sua parte ${fmt(d.partner_fee)}`,
              amount: <Amount c={d.amount} />,
            })} />
        </div>
        <div>
          <Section title="Pagamentos à RIHA" />
          <List rows={data.settlements} rowKey={(s: any) => s.id}
            empty={<Empty icon="bank" title="Nenhum pagamento registado" text="A RIHA ainda não registou pagamentos seus." />}
            cols={[
              { head: 'Data', w: '84px', cell: (s: any) => <b>{day(s.received_on)}</b> },
              { head: 'Referência', w: 'minmax(0,1fr)', cell: (s: any) => <span className="o-mono o-t1" style={{ display: 'block' }}>{s.reference}</span> },
              { head: 'Valor', w: 'auto', right: true, cell: (s: any) => <Amount c={s.amount} /> },
            ]}
            card={(s: any) => ({
              lead: <span className="ph-ico"><Icon name="check" size={16} /></span>,
              title: day(s.received_on),
              sub: <span className="o-mono">{s.reference}</span>,
              amount: <Amount c={s.amount} />,
            })} />
        </div>
      </div>
    </Page>
  )
}
