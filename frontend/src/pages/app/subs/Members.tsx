import { useEffect, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { Icon, Skeleton } from '../../../components/ui'
import { useGateway } from '../../../core/gateway'
import { useQuery } from '../../../core/useQuery'
import { money, StatusPill, SUB_STATUS, VIA } from './common'
import { useSubs } from './SubsShell'

const FILTERS: [string, string, string][] = [['', 'Todos', 'all'], ['past_due', 'Em dívida', 'past_due'], ['paused', 'Pausados', 'paused'], ['invited', 'Convites', 'invited']]

export default function Members() {
  const g = useGateway()
  const nav = useNavigate()
  const sx = useSubs()
  const [sp, setSp] = useSearchParams()
  const status = sp.get('status') || ''
  const plan = sp.get('plan') || ''
  const field = sp.get('field') || ''
  const value = sp.get('value') || ''
  const [q, setQ] = useState(sp.get('q') || '')
  const [debounced, setDebounced] = useState(q)
  const [offset, setOffset] = useState(0)
  useEffect(() => { const t = setTimeout(() => setDebounced(q), 250); return () => clearTimeout(t) }, [q])
  useEffect(() => setOffset(0), [status, plan, field, value, debounced])
  const filter = { status, plan, field, value, q: debounced, offset }
  const { data } = useQuery(g => g.subscriptions.members(filter), [status, plan, field, value, debounced, offset, sx.version])
  const setParam = (k: string, v: string) => { const n = new URLSearchParams(sp); if (v) n.set(k, v); else n.delete(k); setSp(n, { replace: true }) }
  const fieldDef = data?.fields?.[0]
  return (
    <div>
      <style>{`
        .mem-filters{display:flex;gap:8px;align-items:center;flex-wrap:wrap}
        .mem-filters .pf{height:36px;padding:0 14px;border-radius:999px;border:1px solid #E1DFD7;background:#fff;font-weight:600;font-size:13px;display:inline-flex;align-items:center;gap:6px}
        .mem-filters .pf.on{background:var(--ink);color:#fff;border-color:var(--ink)}
        .mem-filters select.pf{border-style:dashed;appearance:none;padding-right:26px;background:#fff url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='10' height='6'%3E%3Cpath d='M0 0l5 6 5-6z' fill='%230B120E'/%3E%3C/svg%3E") no-repeat right 10px center}
        .mem-row{grid-template-columns:minmax(180px,1.6fr) 1.3fr 70px 120px 80px 100px 130px}
        @media (max-width:860px){.mem-desk{display:none}.mem-cards{display:flex!important}}
      `}</style>
      <div className="mem-filters">
        {FILTERS.map(([k, l, c]) => <button key={k} className={'pf' + (status === k ? ' on' : '')} onClick={() => setParam('status', k)}>{l} <span style={{ opacity: .7 }}>{data?.counts?.[c] ?? ''}</span></button>)}
        <span style={{ width: 1, height: 22, background: '#E1DFD7', margin: '0 4px' }} />
        <select className="pf" value={plan} onChange={e => setParam('plan', e.target.value)} aria-label="Plano">
          <option value="">Plano: todos</option>
          {sx.plans.map((p: any) => <option key={p.id} value={p.id}>{p.name}</option>)}
        </select>
        {fieldDef && (
          <select className="pf" value={field === fieldDef.key ? value : ''} onChange={e => { const n = new URLSearchParams(sp); if (e.target.value) { n.set('field', fieldDef.key); n.set('value', e.target.value) } else { n.delete('field'); n.delete('value') } setSp(n, { replace: true }) }} aria-label={fieldDef.key}>
            <option value="">{fieldDef.key}</option>
            {fieldDef.values.map((v: string) => <option key={v} value={v}>{v}</option>)}
          </select>
        )}
        <div style={{ flex: 1 }} />
        <div className="search" style={{ width: 260, maxWidth: '100%', height: 38 }}><Icon name="search" size={16} /><input value={q} onChange={e => setQ(e.target.value)} placeholder="Nome ou número" aria-label="Pesquisar membros" /></div>
      </div>
      {!data ? <Skeleton h={360} r={22} style={{ marginTop: 14 }} /> : (
        <>
          <div className="table mem-desk" style={{ marginTop: 14, overflowX: 'auto' }}>
            <div style={{ minWidth: 900 }}>
              <div className="thead mem-row"><span>Membro</span><span>Plano</span><span>{fieldDef?.key || ''}</span><span>Estado</span><span>Próxima</span><span style={{ textAlign: 'right' }}>Em dívida</span><span>Último pagamento</span></div>
              {data.data.length === 0 && <div style={{ padding: 28, textAlign: 'center', color: 'var(--muted)', fontSize: 14 }}>Nenhum membro com estes filtros.</div>}
              {data.data.map((m: any) => (
                <div key={m.id} className="trow click mem-row" style={{ background: m.status === 'past_due' || m.status === 'suspended' ? '#FFFCF0' : undefined }} onClick={() => nav('/painel/assinaturas/membros/' + m.id)}>
                  <div style={{ minWidth: 0 }}><div style={{ fontWeight: 600, fontSize: 14 }}>{m.name}{m.duplicate_of && <span title="Inscreveu-se pelo link com o número e o nome de um membro que já tem. Veja se é a mesma pessoa." style={{ marginLeft: 6, fontSize: 11, fontWeight: 600, padding: '2px 6px', borderRadius: 6, background: '#FFF4DB', color: '#8A5A00' }}>Possível duplicado</span>}</div><div style={{ fontSize: 12.5, color: 'var(--muted)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{[m.payer_name, m.phone].filter(Boolean).join(' · ')}</div></div>
                  <span style={{ fontSize: 13.5, color: 'var(--text-2)' }}>{m.plans || '—'}</span>
                  <span style={{ fontSize: 13.5 }}>{fieldDef ? m.fields?.[fieldDef.key] || '—' : ''}</span>
                  <span><StatusPill map={SUB_STATUS} s={m.status} /></span>
                  <span style={{ fontSize: 13.5 }}>{m.status === 'invited' ? '—' : m.next_due || '—'}</span>
                  <span className="tnum" style={{ textAlign: 'right', fontWeight: m.debt ? 700 : 400, color: m.debt ? 'var(--sun-text)' : 'var(--muted)' }}>{m.debt ? money(m.debt) + ',00' : '—'}</span>
                  <span style={{ fontSize: 13.5, color: 'var(--text-2)' }}>{m.last_paid ? `${VIA[m.last_paid_via] || m.last_paid_via} · ${m.last_paid}` : '—'}</span>
                </div>
              ))}
              <div className="between" style={{ padding: '12px 20px', fontSize: 13, color: 'var(--muted)', borderTop: '1px solid var(--line-3)' }}>
                <span>{data.data.length ? `${offset + 1}–${offset + data.data.length} de ${data.total}` : `0 de ${data.total}`}</span>
                <span style={{ display: 'flex', gap: 14 }}>
                  {offset > 0 && <button className="link-btn" onClick={() => setOffset(Math.max(0, offset - 50))}>Anteriores</button>}
                  {offset + data.data.length < data.total && <button className="link-btn" onClick={() => setOffset(offset + 50)}>Seguintes</button>}
                  <a href={g.subscriptions.membersExportUrl({ status, plan, field, value, q: debounced })} style={{ color: 'var(--text-2)' }}>Exportar CSV</a>
                </span>
              </div>
            </div>
          </div>
          <div className="mem-cards" style={{ display: 'none', flexDirection: 'column', gap: 8, marginTop: 12 }}>
            {data.data.map((m: any) => (
              <button key={m.id} className="card" onClick={() => nav('/painel/assinaturas/membros/' + m.id)} style={{ textAlign: 'left', padding: '12px 14px', display: 'flex', gap: 10, alignItems: 'center', border: '1px solid var(--line)' }}>
                <div style={{ flex: 1, minWidth: 0 }}><div style={{ fontWeight: 600, fontSize: 14.5 }}>{m.name}{m.duplicate_of && <span style={{ marginLeft: 6, fontSize: 11, fontWeight: 600, padding: '2px 6px', borderRadius: 6, background: '#FFF4DB', color: '#8A5A00' }}>Possível duplicado</span>}</div><div style={{ fontSize: 12.5, color: 'var(--muted)' }}>{[fieldDef && m.fields?.[fieldDef.key], m.payer_name].filter(Boolean).join(' · ')}</div></div>
                <div style={{ textAlign: 'right' }}><StatusPill map={SUB_STATUS} s={m.status} /><div style={{ fontSize: 12, color: m.debt ? 'var(--sun-text)' : 'var(--muted)', marginTop: 4 }}>{m.debt ? `${money(m.debt)} MT em dívida` : m.next_due ? `próx. ${m.next_due}` : ''}</div></div>
              </button>
            ))}
          </div>
        </>
      )}
    </div>
  )
}
