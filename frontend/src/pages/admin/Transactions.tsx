import { useEffect, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { Icon, MethodIcon, Pill, Skeleton } from '../../components/ui'
import { useQuery } from '../../core/useQuery'
import { fmt, METHOD_NAME, STATUS } from '../../lib/format'

// Every merchant's payments in one list, with what needs a look on top.

const VIEWS: [string, string][] = [['', 'Todas'], ['paid', 'Pagas'], ['unconfirmed', 'A confirmar'], ['processing', 'Pendentes'], ['failed', 'Falhadas']]
const ATTENTION: { key: string; count: string; t: string; d: string; bg: string; fg: string }[] = [
  { key: 'unconfirmed', count: 'unconfirmed', t: 'A confirmar', d: 'Sem resposta final do operador. A RIHA continua a perguntar.', bg: '#FFE9C7', fg: '#8A4B00' },
  { key: 'duplicates', count: 'duplicates', t: 'Possíveis duplicados', d: 'Pagos tarde por quem já tinha pago de novo. Ver se é para reembolsar.', bg: '#FFE3E1', fg: '#C2080D' },
  { key: 'late', count: 'paid_late', t: 'Pagos depois', d: 'Confirmados pelo operador depois de ficarem a confirmar.', bg: '#E6ECFB', fg: '#2A4AA8' },
]

const when = (iso: string) => new Date(iso).toLocaleString('pt-PT', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' })

export default function AdminTransactions() {
  const nav = useNavigate()
  const [sp, setSp] = useSearchParams()
  const q = sp.get('q') || ''
  const view = sp.get('view') || ''
  const method = sp.get('method') || ''
  const mode = sp.get('mode') === 'test' ? 'test' : ''
  const page = Math.max(1, Number(sp.get('page')) || 1)
  const go = (patch: Record<string, string | number>) => {
    const all: Record<string, string | number> = { q, view, method, mode, page, ...patch }
    setSp(Object.fromEntries(Object.entries(all).filter(([k, v]) => v !== '' && !(k === 'page' && Number(v) <= 1)).map(([k, v]) => [k, String(v)])))
  }
  const [term, setTerm] = useState(q)
  useEffect(() => setTerm(q), [q])
  useEffect(() => {
    if (term.trim() === q) return
    const t = setTimeout(() => go({ q: term.trim(), page: 1 }), 350)
    return () => clearTimeout(t)
  }, [term]) // eslint-disable-line react-hooks/exhaustive-deps
  const { data } = useQuery(g => g.admin.payments({ q, view, method, mode, page }), [q, view, method, mode, page], 30000)
  const rows: any[] = data?.data || []
  const total: number = data?.total ?? 0
  const per: number = data?.per_page ?? 50
  const from = (page - 1) * per

  return (
    <div className="page">
      <style>{`.tx-att{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:10px;margin-top:18px}
        .tx-att button{text-align:left;padding:14px 16px;border-radius:18px;border:1px solid var(--line);background:#fff;display:flex;flex-direction:column;gap:4px}
        .tx-att button.on{border-color:var(--ink);box-shadow:inset 0 0 0 1px var(--ink)}
        .tx-filters{display:flex;gap:8px;flex-wrap:wrap;align-items:center;margin-top:18px}
        .tx-search{flex:1;min-width:240px;height:40px;display:flex;align-items:center;gap:8px;padding:0 14px;border:1px solid #E1DFD7;border-radius:999px;background:#fff;color:var(--muted)}
        .tx-search:focus-within{border-color:var(--ink)}.tx-search input{flex:1;min-width:0;border:0;outline:0;background:transparent;font-size:14px;color:var(--ink)}
        .tx-filters select{height:40px;padding:0 30px 0 14px;border-radius:999px;border:1px dashed #E1DFD7;font-weight:600;font-size:13px;appearance:none;background:#fff url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='10' height='6'%3E%3Cpath d='M0 0l5 6 5-6z' fill='%230B120E'/%3E%3C/svg%3E") no-repeat right 12px center}
        .tx-row{grid-template-columns:118px minmax(0,1.2fr) minmax(0,1fr) 150px 110px minmax(0,1fr);padding:12px 20px;align-items:center}
        @media(max-width:900px){.tx-att{grid-template-columns:1fr}.tx-row{grid-template-columns:1fr 1fr;gap:4px 12px}.tx-head{display:none!important}}`}</style>
      <h1 className="h1">Transações</h1>
      <div style={{ fontSize: 14.5, color: 'var(--muted)', marginTop: 4, maxWidth: 680, lineHeight: 1.5 }}>Os pagamentos de todos os comerciantes. Abra um para ver o rasto completo e perguntar ao operador como está.</div>

      <div className="tx-att">
        {ATTENTION.map(x => {
          const n = data?.counts?.[x.count]
          const on = view === x.key
          return (
            <button key={x.key} className={on ? 'on' : ''} onClick={() => go({ view: on ? '' : x.key, page: 1 })} aria-pressed={on}>
              <span className="row" style={{ gap: 8 }}><span className="display tnum" style={{ fontSize: 26, letterSpacing: '-0.03em' }}>{n ?? '·'}</span><Pill t={x.t} bg={x.bg} fg={x.fg} /></span>
              <span style={{ fontSize: 12.5, color: 'var(--muted)', lineHeight: 1.45 }}>{x.d}</span>
            </button>
          )
        })}
      </div>

      <div className="tx-filters">
        <div className="tx-search"><Icon name="search" size={16} /><input type="search" value={term} onChange={e => setTerm(e.target.value)} placeholder="pay_…, RH-…, operação, ref. do operador, 4 dígitos" title="Procura por id ou referência RIHA, número da operação, referência do operador ou os 4 últimos dígitos do telefone" aria-label="Procurar transações" /></div>
        <div className="seg" style={{ flexWrap: 'wrap' }}>{VIEWS.map(([k, l]) => <button key={k} className={view === k ? 'on' : ''} onClick={() => go({ view: k, page: 1 })}>{l}</button>)}</div>
        <select value={method} onChange={e => go({ method: e.target.value, page: 1 })} aria-label="Método">
          <option value="">Todos os métodos</option>
          {['mpesa', 'emola', 'bim', 'card'].map(m => <option key={m} value={m}>{METHOD_NAME[m] || m}</option>)}
        </select>
        <select value={mode} onChange={e => go({ mode: e.target.value, page: 1 })} aria-label="Modo">
          <option value="">Produção</option>
          <option value="test">Teste</option>
        </select>
      </div>

      <div className="table" style={{ marginTop: 14, overflowX: 'auto' }}>
        <div className="trow tx-row tx-head" style={{ fontSize: 12, fontWeight: 700, color: 'var(--muted)', textTransform: 'uppercase', letterSpacing: '.05em' }}>
          <span>Quando</span><span>Comerciante</span><span>Referência</span><span>Cliente</span><span style={{ textAlign: 'right' }}>Valor</span><span>Estado</span>
        </div>
        {!data && <div style={{ padding: 16 }}><Skeleton h={120} /></div>}
        {data && rows.length === 0 && <div className="empty">{q ? `Nada encontrado para “${q}”.` : 'Sem transações com este filtro.'}</div>}
        {rows.map(p => {
          const st = STATUS[p.status] || { t: p.status, bg: '#EDECE6', fg: '#3E4742' }
          return (
            <div key={p.id} className="trow tx-row click" onClick={() => nav('/admin/rasto/' + p.id)}>
              <span style={{ fontSize: 13, color: 'var(--muted)' }}>{when(p.created_at)}</span>
              <span style={{ minWidth: 0 }}><div style={{ fontWeight: 600, fontSize: 14, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{p.merchant_name || p.merchant_id}</div><div style={{ fontSize: 12.5, color: 'var(--muted)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{p.description || '—'}</div></span>
              <span className="mono" style={{ fontSize: 12.5, minWidth: 0 }}><div>{p.reference}</div><div style={{ color: 'var(--muted)' }}>{p.rail_reference || p.id}</div></span>
              <span className="row" style={{ gap: 8, fontSize: 13 }}><MethodIcon method={p.method} size={22} radius={7} />{p.phone_last4 ? `${p.phone_prefix} ••• ${p.phone_last4}` : p.card_last4 ? `•••• ${p.card_last4}` : '—'}</span>
              <span className="tnum" style={{ fontWeight: 600, textAlign: 'right' }}>{fmt(p.amount)} MT</span>
              <span style={{ display: 'flex', gap: 6, flexWrap: 'wrap', alignItems: 'center' }}>
                <Pill {...st} />
                {p.paid_late && <Pill t="Pago depois" bg="#E6ECFB" fg="#2A4AA8" />}
                {p.duplicate_of && <Pill t="Duplicado?" bg="#FFE3E1" fg="#C2080D" />}
                {p.failure_code && p.status === 'failed' && <span style={{ fontSize: 12, color: 'var(--muted)' }}>{p.failure_code}</span>}
              </span>
            </div>
          )
        })}
        {data && total > per && (
          <div className="between" style={{ padding: '12px 20px', fontSize: 13, color: 'var(--muted)', borderTop: '1px solid var(--line-3)' }}>
            <span>{`${from + 1}–${from + rows.length} de ${total}`}</span>
            <span style={{ display: 'flex', gap: 14 }}>
              {page > 1 && <button className="link-btn" onClick={() => go({ page: page - 1 })}>Anteriores</button>}
              {from + rows.length < total && <button className="link-btn" onClick={() => go({ page: page + 1 })}>Seguintes</button>}
            </span>
          </div>
        )}
      </div>
    </div>
  )
}
