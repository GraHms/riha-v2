import { useEffect, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { Icon, MethodIcon, Skeleton } from '../../components/ui'
import { Amount, Badge, Empty, List, Page, PageHead, Tip, type Tone } from '../../components/ops'
import { useQuery } from '../../core/useQuery'
import { METHOD_NAME, STATUS } from '../../lib/format'

// Every merchant's payments in one list, with what needs a look on top.

const VIEWS: [string, string][] = [['', 'Todas'], ['paid', 'Pagas'], ['unconfirmed', 'A confirmar'], ['processing', 'Pendentes'], ['failed', 'Falhadas']]
const ATTENTION: { key: string; count: string; t: string; hint: string; d: string; tone: Tone }[] = [
  { key: 'unconfirmed', count: 'unconfirmed', t: 'A confirmar', hint: 'Sem resposta final', d: 'Sem resposta final do operador. A RIHA continua a perguntar.', tone: 'warn' },
  { key: 'duplicates', count: 'duplicates', t: 'Possíveis duplicados', hint: 'Ver se é para reembolsar', d: 'Pagos tarde por quem já tinha pago de novo. Ver se é para reembolsar.', tone: 'bad' },
  { key: 'late', count: 'paid_late', t: 'Pagos depois', hint: 'Confirmados tarde', d: 'Confirmados pelo operador depois de ficarem a confirmar.', tone: 'info' },
]
const TONE: Record<string, Tone> = { succeeded: 'ok', failed: 'bad', unconfirmed: 'warn', processing: 'warn', refunded: 'info', partially_refunded: 'info' }

const when = (iso: string) => new Date(iso).toLocaleString('pt-PT', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' })
const customer = (p: any) => (p.phone_last4 ? `${p.phone_prefix} ••• ${p.phone_last4}` : p.card_last4 ? `•••• ${p.card_last4}` : '—')

function StatusBadge({ s }: { s: string }) {
  return <Badge tone={TONE[s]} dot>{STATUS[s]?.t || s}</Badge>
}

// Flags are what a payment's status alone doesn't say.
function Flags({ p }: { p: any }) {
  return (
    <>
      {p.paid_late && <Badge tone="info">Pago depois</Badge>}
      {p.duplicate_of && <Badge tone="bad">Duplicado?</Badge>}
      {p.failure_code && p.status === 'failed' && <span className="o-mono tx-code">{p.failure_code}</span>}
    </>
  )
}

const CSS = `
.tx-att{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:12px;margin-bottom:18px}
.tx-att .o-stat{border-width:1px}
.tx-att .o-stat.tx-on{border-color:var(--o-ink);box-shadow:inset 0 0 0 1px var(--o-ink),var(--o-shadow)}
.tx-att .o-stat-l{justify-content:space-between}
.tx-code{color:var(--o-muted);font-size:11.5px}
.tx-flags{display:flex;gap:6px;flex-wrap:wrap;align-items:center;min-width:0}
.tx-who{display:flex;align-items:center;gap:8px;min-width:0;font-size:13px;color:var(--o-ink-2)}
.tx-views-m{display:none}
.tx-pager{display:flex;gap:6px}
@media(max-width:860px){
  .tx-att{grid-template-columns:minmax(0,1fr);gap:8px;margin-bottom:14px}
  .tx-att .o-stat{flex-direction:row;align-items:center;justify-content:space-between;padding:11px 14px}
  .tx-att .o-stat-h{display:none}
  .tx-att .o-stat-v{font-size:20px;order:2}
  .tx-views{display:none!important}
  .tx-views-m{display:block;flex:1 1 140px;min-width:0}
  .ops .tx-tb .o-select{flex:1 1 120px;min-width:0}
}`

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
    <Page>
      <style>{CSS}</style>
      <PageHead title="Transações" sub={<>Os pagamentos de todos os comerciantes <Tip text="Abra um para ver o rasto completo e perguntar ao operador como está." /></>} />

      <div className="tx-att">
        {ATTENTION.map(x => {
          const n = data?.counts?.[x.count]
          const on = view === x.key
          return (
            <button key={x.key} className={'o-stat' + (on ? ' tx-on' : '')} onClick={() => go({ view: on ? '' : x.key, page: 1 })} aria-pressed={on} title={x.d}>
              <span className="o-stat-l"><Badge tone={x.tone} dot>{x.t}</Badge>{on && <Icon name="close" size={14} />}</span>
              <span className="o-stat-v">{n ?? '·'}</span>
              <span className="o-stat-h">{x.hint}</span>
            </button>
          )
        })}
      </div>

      <div className="o-toolbar tx-tb">
        <label className="o-search">
          <Icon name="search" size={16} />
          <input type="search" value={term} onChange={e => setTerm(e.target.value)} placeholder="pay_…, RH-…, operação, ref. do operador, 4 dígitos" title="Procura por id ou referência RIHA, número da operação, referência do operador ou os 4 últimos dígitos do telefone" aria-label="Procurar transações" />
        </label>
        <div className="seg tx-views">{VIEWS.map(([k, l]) => <button key={k} className={view === k ? 'on' : ''} onClick={() => go({ view: k, page: 1 })}>{l}</button>)}</div>
        <select className="o-select tx-views-m" value={view} onChange={e => go({ view: e.target.value, page: 1 })} aria-label="Estado">
          {VIEWS.map(([k, l]) => <option key={k} value={k}>{l}</option>)}
          <option value="duplicates">Possíveis duplicados</option>
          <option value="late">Pagos depois</option>
        </select>
        <select className="o-select" value={method} onChange={e => go({ method: e.target.value, page: 1 })} aria-label="Método">
          <option value="">Todos os métodos</option>
          {['mpesa', 'emola', 'bim', 'card'].map(m => <option key={m} value={m}>{METHOD_NAME[m] || m}</option>)}
        </select>
        <select className="o-select" value={mode} onChange={e => go({ mode: e.target.value, page: 1 })} aria-label="Modo">
          <option value="">Produção</option>
          <option value="test">Teste</option>
        </select>
      </div>

      {!data ? <div className="o-panel" style={{ padding: 16 }}><Skeleton h={180} /></div> : (
        <List rows={rows} rowKey={p => p.id} onRow={p => nav('/admin/rasto/' + p.id)}
          empty={<Empty icon="search" title={q ? `Nada encontrado para “${q}”` : 'Sem transações'} text={q ? undefined : 'Nada com este filtro.'} />}
          cols={[
            { head: 'Quando', w: '96px', cell: p => <span className="o-t2">{when(p.created_at)}</span> },
            { head: 'Comerciante', w: 'minmax(0,1.3fr)', cell: p => <><div className="o-t1">{p.merchant_name || p.merchant_id}</div><div className="o-t2">{p.description || '—'}</div></> },
            { head: 'Referência', w: 'minmax(0,1fr)', cell: p => <><div className="o-t1 o-mono">{p.reference}</div><div className="o-t2 o-mono">{p.rail_reference || p.id}</div></> },
            { head: 'Cliente', w: '136px', cell: p => <span className="tx-who"><MethodIcon method={p.method} size={22} radius={7} /><span className="o-t1" style={{ fontWeight: 500 }}>{customer(p)}</span></span> },
            { head: 'Valor', w: '112px', right: true, cell: p => <Amount c={p.amount} /> },
            { head: 'Estado', w: 'minmax(120px,1fr)', cell: p => <span className="tx-flags"><StatusBadge s={p.status} /><Flags p={p} /></span> },
          ]}
          card={p => ({
            lead: <MethodIcon method={p.method} size={32} radius={9} />,
            title: p.merchant_name || p.merchant_id,
            sub: <><span className="o-mono">{p.reference}</span> · {customer(p)} · {when(p.created_at)}</>,
            amount: <Amount c={p.amount} />,
            meta: <StatusBadge s={p.status} />,
            foot: (p.paid_late || p.duplicate_of || (p.failure_code && p.status === 'failed')) ? <Flags p={p} /> : undefined,
          })}
          footer={total > per ? (
            <div className="o-more">
              <span>{`${from + 1}–${from + rows.length} de ${total}`}</span>
              <span className="tx-pager">
                {page > 1 && <button className="btn btn-outline btn-sm" onClick={() => go({ page: page - 1 })}><Icon name="arrowLeft" size={14} />Anteriores</button>}
                {from + rows.length < total && <button className="btn btn-outline btn-sm" onClick={() => go({ page: page + 1 })}>Seguintes<Icon name="arrowRight" size={14} /></button>}
              </span>
            </div>
          ) : undefined} />
      )}
    </Page>
  )
}
