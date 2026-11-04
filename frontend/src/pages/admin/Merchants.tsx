import { useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { useQuery } from '../../core/useQuery'
import { bpsLabel, fmt, METHOD_NAME, MERCHANT_STATUS, monthName } from '../../lib/format'
import { Avatar, Pill } from '../../components/ui'

const TABS: [string, string][] = [['', 'Todos'], ['active', 'Ativos'], ['review', 'Em verificação'], ['suspended', 'Suspensos']]
const GRID = 'minmax(0,1.5fr) 130px 140px 150px 90px 120px 110px 70px'

export default function Merchants() {
  const nav = useNavigate()
  const [sp] = useSearchParams()
  const [tab, setTab] = useState('')
  const q = sp.get('q') || ''
  const { data } = useQuery(g => g.admin.merchants(tab, q), [tab, q])
  const rows: any[] = data?.data || []
  return (
    <div className="page">
      <h1 className="h1">Comerciantes</h1>
      <div style={{ fontSize: 14, color: 'var(--muted)', marginTop: 4 }}>{data ? `${fmt(data.total * 100, false)} contas · ${fmt(data.active * 100, false)} ativas` : '…'}{q && <> · pesquisa “{q}”</>}</div>
      <div className="row" style={{ gap: 8, marginTop: 18, flexWrap: 'wrap' }}>
        {TABS.map(([k, l]) => <button key={k} onClick={() => setTab(k)} style={{ height: 36, padding: '0 14px', borderRadius: 999, border: `1px solid ${tab === k ? 'var(--ink)' : 'var(--line-2)'}`, background: tab === k ? 'var(--ink)' : '#fff', color: tab === k ? '#fff' : 'var(--ink)', fontSize: 13.5, fontWeight: 600 }}>{l}</button>)}
      </div>
      <div className="table" style={{ marginTop: 14, overflowX: 'auto' }}>
        <div style={{ minWidth: 980 }}>
          <div className="thead" style={{ gridTemplateColumns: GRID }}><span>Comerciante</span><span>Estado</span><span style={{ textAlign: 'right' }}>Volume 30 d</span><span style={{ textAlign: 'right' }}>Saldo</span><span>Taxa</span><span>Principal</span><span>Desde</span><span /></div>
          {rows.map(r => {
            const st = MERCHANT_STATUS[r.status]
            const d = new Date(r.created_at)
            return (
              <div key={r.id} className="trow click" style={{ gridTemplateColumns: GRID }} onClick={() => nav('/admin/comerciantes/' + r.id)}>
                <div className="row" style={{ minWidth: 0 }}><Avatar text={r.initials} color={r.avatar_color} size={36} radius={11} font={12.5} /><div style={{ minWidth: 0 }}><div style={{ fontWeight: 600, fontSize: 14 }}>{r.name}</div><div style={{ fontSize: 12, color: 'var(--muted)' }}>{r.city || r.email}</div></div></div>
                <div><Pill {...st} /></div>
                <div className="tnum" style={{ textAlign: 'right', fontSize: 14 }}>{r.volume_30d ? fmt(r.volume_30d, false) + ' MT' : '—'}</div>
                <div className="tnum" style={{ textAlign: 'right', fontSize: 14, fontWeight: 600 }}>{fmt(r.balance)} MT</div>
                <div style={{ fontSize: 13.5 }}>{bpsLabel(r.fee_bps)}{r.fee_negotiated && <span className="tag" style={{ marginLeft: 6, background: 'var(--sun-bg)', color: 'var(--sun-text)', fontSize: 10 }}>NEG</span>}</div>
                <div style={{ fontSize: 13.5, color: 'var(--text-2)' }}>{METHOD_NAME[r.main_method] || '—'}</div>
                <div style={{ fontSize: 13.5, color: 'var(--text-2)' }}>{monthName(d.getMonth()).slice(0, 3)} {d.getFullYear()}</div>
                <div style={{ display: 'flex', justifyContent: 'flex-end' }}><span className="btn btn-outline btn-sm">Ver</span></div>
              </div>
            )
          })}
          {data && !rows.length && <div className="empty">Nenhum comerciante.</div>}
        </div>
      </div>
    </div>
  )
}
