import { useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { useQuery } from '../../core/useQuery'
import { bpsLabel, fmt, METHOD_NAME, MERCHANT_STATUS, monthName } from '../../lib/format'
import { Avatar, Icon, Skeleton } from '../../components/ui'
import { Amount, Badge, Empty, List, Page, PageHead, type Tone } from '../../components/ops'
import { V1Tag } from './common'

const TABS: [string, string][] = [['', 'Todos'], ['active', 'Ativos'], ['review', 'Em verificação'], ['suspended', 'Suspensos']]
const MERCHANT_TONE: Record<string, Tone> = { lead: undefined, in_review: 'warn', info_requested: 'info', active: 'ok', rejected: 'bad', suspended: 'bad' }

// The seg scrolls on its own if a phone is too narrow for it; on mid-size
// screens the list drops the "since" column so names keep their room.
const CSS = `
.mc-seg{max-width:100%;overflow-x:auto;scrollbar-width:none;margin-bottom:12px}
.mc-seg .seg{display:inline-flex;white-space:nowrap}
.mc-go{color:var(--o-muted);display:flex;justify-content:flex-end}
@media(max-width:1180px){
  .mc-list .o-list-head,.mc-list .o-row{grid-template-columns:minmax(0,1fr) 120px 120px 130px 110px 16px!important}
  .mc-list .o-list-head>:nth-child(6),.mc-list .o-row>:nth-child(6){display:none}
}
@media(max-width:1100px){
  .mc-list .o-list-head,.mc-list .o-row{grid-template-columns:minmax(0,1fr) 120px 130px 110px 16px!important}
  .mc-list .o-list-head>:nth-child(3),.mc-list .o-row>:nth-child(3){display:none}
}`

export default function Merchants() {
  const nav = useNavigate()
  const [sp] = useSearchParams()
  const [tab, setTab] = useState('')
  const q = sp.get('q') || ''
  const { data } = useQuery(g => g.admin.merchants(tab, q), [tab, q])
  const rows: any[] = data?.data || []
  const since = (r: any) => { const d = new Date(r.created_at); return `${monthName(d.getMonth()).slice(0, 3)} ${d.getFullYear()}` }
  const status = (r: any) => <Badge tone={MERCHANT_TONE[r.status]}>{MERCHANT_STATUS[r.status]?.t || r.status}</Badge>
  const feeLabel = (r: any) => <>{bpsLabel(r.fee_bps)}{r.fee_negotiated && <Badge tone="warn"><span title="Taxa negociada">NEG</span></Badge>}</>
  return (
    <Page>
      <style>{CSS}</style>
      <PageHead title="Comerciantes" sub={<>{data ? `${fmt(data.total * 100, false)} contas · ${fmt(data.active * 100, false)} ativas` : '…'}{q && <> · pesquisa “{q}”</>}</>} />
      <div className="mc-seg">
        <div className="seg" role="tablist">{TABS.map(([k, l]) => <button key={k} role="tab" aria-selected={tab === k} className={tab === k ? 'on' : ''} onClick={() => setTab(k)}>{l}</button>)}</div>
      </div>
      {!data ? <Skeleton h={360} r={16} /> : (
        <div className="mc-list">
          <List rows={rows} rowKey={r => r.id} onRow={r => nav('/admin/comerciantes/' + r.id)}
            empty={<Empty icon="store" title="Nenhum comerciante" text={q ? `Nada encontrado para “${q}”.` : undefined} />}
            cols={[
              { head: 'Comerciante', w: 'minmax(0,1fr)', cell: r => (
                <div style={{ display: 'flex', alignItems: 'center', gap: 12, minWidth: 0 }}>
                  <Avatar text={r.initials} color={r.avatar_color} size={34} radius={10} font={12} />
                  <div style={{ minWidth: 0 }}><div className="o-t1">{r.name}{r.imported_from === 'v1' && <V1Tag />}</div><div className="o-t2">{r.city || r.email}</div></div>
                </div>
              ) },
              { head: 'Estado', w: '120px', cell: status },
              { head: 'Volume 30 d', w: '120px', right: true, cell: r => r.volume_30d ? <Amount c={r.volume_30d} cents={false} /> : <span className="o-t2">—</span> },
              { head: 'Saldo', w: '130px', right: true, cell: r => <Amount c={r.balance} /> },
              { head: 'Taxa', w: '110px', cell: r => <><div className="o-t1" style={{ display: 'flex', alignItems: 'center', gap: 6 }}>{feeLabel(r)}</div><div className="o-t2">{METHOD_NAME[r.main_method] || '—'}</div></> },
              { head: 'Desde', w: '80px', cell: r => <span className="o-t2">{since(r)}</span> },
              { head: '', w: '16px', cell: () => <span className="mc-go"><Icon name="chevronRight" size={16} /></span> },
            ]}
            card={r => ({
              lead: <Avatar text={r.initials} color={r.avatar_color} size={38} radius={11} font={13} />,
              title: <>{r.name}{r.imported_from === 'v1' && <V1Tag />}</>,
              sub: r.city || r.email,
              amount: <Amount c={r.balance} />,
              meta: status(r),
              foot: (
                <>
                  <span className="o-chip">30 d · {r.volume_30d ? fmt(r.volume_30d, false) + ' MT' : '—'}</span>
                  <span className="o-chip">{bpsLabel(r.fee_bps)}{r.fee_negotiated && ' · negociada'}</span>
                  {METHOD_NAME[r.main_method] && <span className="o-chip">{METHOD_NAME[r.main_method]}</span>}
                  <span className="o-t2" style={{ marginLeft: 'auto' }}>desde {since(r)}</span>
                </>
              ),
            })} />
        </div>
      )}
    </Page>
  )
}
