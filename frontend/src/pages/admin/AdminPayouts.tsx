import { useState } from 'react'
import { api, errMsg } from '../../lib/api'
import { useData } from '../../lib/useData'
import { fmt, PAYOUT_STATUS, relDate } from '../../lib/format'
import { useSession } from '../../lib/session'
import { Icon, Modal, Pill, Spinner } from '../../components/ui'
import { SlaBar } from './common'

const TABS: [string, string, string][] = [['pending', 'Pendentes', 'pending_count'], ['sent', 'Enviados ao banco', 'sent_count'], ['paid', 'Pagos', 'paid_count'], ['all', 'Todos', 'all_count']]
const GRID = '40px 90px minmax(0,1.3fr) minmax(0,1fr) 130px 150px 130px 130px'
const ST = { ...PAYOUT_STATUS, paid: { t: 'Pago', bg: '#E2F3DE', fg: '#0A6B05' } }

export default function AdminPayouts() {
  const { toast } = useSession()
  const [tab, setTab] = useState('pending')
  const [picked, setPicked] = useState<Record<string, boolean>>({})
  const [busy, setBusy] = useState(false)
  const [reject, setReject] = useState<{ ids: string[]; action: 'reject' | 'fail' } | null>(null)
  const [reason, setReason] = useState('')
  const { data, reload } = useData<any>('/admin/payouts?tab=' + tab, [tab], 15000)
  const rows: any[] = data?.data || []
  const st = data?.stats
  const ids = rows.filter(p => picked[p.id]).map(p => p.id)
  const sum = rows.filter(p => picked[p.id]).reduce((a, p) => a + p.amount, 0)
  const allOn = rows.length > 0 && ids.length === rows.length

  const act = async (list: string[], action: string, why = '') => {
    setBusy(true)
    try {
      const r = await api<any>('/admin/payouts/action', { body: { ids: list, action, reason: why } })
      const verb: Record<string, string> = { approve: 'aprovados e enviados ao banco', paid: 'marcados como pagos', reject: 'rejeitados. O saldo volta aos comerciantes.', fail: 'marcados como falhados. O saldo volta aos comerciantes.' }
      toast(r.failed.length ? `${r.done.length} concluídos, ${r.failed.length} com erro: ${r.failed[0].error}` : r.done.length === 1 ? `${r.done[0]} ${verb[action].replace(/os\b/g, 'o').replace('marcados', 'marcado').replace('rejeitados', 'rejeitado')}` : `${r.done.length} ${verb[action]}`, r.failed.length > 0)
      setPicked({}); reload()
    } catch (e) { toast(errMsg(e), true) } finally { setBusy(false) }
  }
  const exportFile = (list: string[]) => { window.location.href = '/api/admin/payouts/export.csv' + (list.length ? '?ids=' + list.join(',') : ''); toast(`Ficheiro bancário exportado (${list.length || 'todas as'} transferências)`) }
  const box = (on: boolean, onClick: () => void) => <button onClick={onClick} aria-pressed={on} style={{ width: 18, height: 18, borderRadius: 5, border: `1.5px solid ${on ? 'var(--green)' : '#CFCDC4'}`, background: on ? 'var(--green)' : '#fff', padding: 0, display: 'grid', placeItems: 'center' }}>{on && <Icon name="check" size={11} stroke={3.4} color="#fff" />}</button>

  return (
    <div className="page">
      <div className="between" style={{ alignItems: 'flex-end', flexWrap: 'wrap' }}>
        <div><h1 className="h1">Levantamentos</h1><div style={{ fontSize: 14, color: 'var(--muted)', marginTop: 4 }}>Cada pedido tem de ser transferido em até 72 horas.</div></div>
        <button className="btn btn-outline" style={{ height: 40, fontWeight: 500, fontSize: 14 }} onClick={() => exportFile([])}><Icon name="download" size={16} stroke={2} />Exportar relatório</button>
      </div>
      <div className="grid-3" style={{ marginTop: 20, gap: 14 }}>
        <div className="card" style={{ padding: 18, borderRadius: 20 }}><div style={{ fontSize: 13, color: 'var(--muted)' }}>Por processar</div><div className="display" style={{ fontSize: 28, letterSpacing: '-0.035em', marginTop: 4 }}>{fmt(st?.pending_total, false)} <span style={{ fontSize: 14, color: 'var(--muted)' }}>MT</span></div><div style={{ fontSize: 12.5, color: 'var(--muted)', marginTop: 2 }}>{st?.pending_count ?? 0} pedidos</div></div>
        <div style={{ background: 'var(--red-bg)', borderRadius: 20, padding: 18, color: '#8E0609' }}><div style={{ fontSize: 13 }}>Vencem em menos de 8 h</div><div className="display" style={{ fontSize: 28, letterSpacing: '-0.035em', marginTop: 4 }}>{st?.urgent ?? 0}</div><div style={{ fontSize: 12.5, marginTop: 2 }}>Priorize estes pedidos</div></div>
        <div className="card" style={{ padding: 18, borderRadius: 20 }}><div style={{ fontSize: 13, color: 'var(--muted)' }}>Enviados ao banco</div><div className="display" style={{ fontSize: 28, letterSpacing: '-0.035em', marginTop: 4 }}>{fmt(st?.sent_total, false)} <span style={{ fontSize: 14, color: 'var(--muted)' }}>MT</span></div><div style={{ fontSize: 12.5, color: 'var(--muted)', marginTop: 2 }}>À espera de confirmação</div></div>
      </div>
      <div className="tabs" style={{ marginTop: 22 }}>{TABS.map(([k, l, c]) => <button key={k} className={tab === k ? 'on' : ''} onClick={() => { setTab(k); setPicked({}) }}>{l}<span className="count">{st?.[c] ?? '·'}</span></button>)}</div>
      {ids.length > 0 && (
        <div className="row" style={{ gap: 10, marginTop: 14, padding: '10px 10px 10px 18px', borderRadius: 16, background: 'var(--ink)', color: '#fff', flexWrap: 'wrap' }}>
          <span style={{ fontWeight: 600, fontSize: 14, flex: 1 }}>{ids.length} {ids.length === 1 ? 'selecionado' : 'selecionados'} · {fmt(sum)} MT</span>
          <button className="btn" style={{ height: 38, border: '1px solid rgba(255,255,255,.2)', background: 'transparent', color: '#fff', fontSize: 13.5 }} onClick={() => exportFile(ids)}>Exportar ficheiro bancário</button>
          {tab !== 'paid' && <button className="btn" style={{ height: 38, border: '1px solid rgba(255,255,255,.2)', background: 'transparent', color: '#FF9C9F', fontSize: 13.5 }} onClick={() => setReject({ ids, action: tab === 'sent' ? 'fail' : 'reject' })}>{tab === 'sent' ? 'Marcar falhados' : 'Rejeitar'}</button>}
          {tab !== 'paid' && <button className="btn btn-green" style={{ height: 38, fontSize: 13.5 }} disabled={busy} onClick={() => act(ids, tab === 'sent' ? 'paid' : 'approve')}>{busy ? <Spinner /> : tab === 'sent' ? 'Marcar como pagos' : 'Aprovar e enviar'}</button>}
        </div>
      )}
      <div className="table" style={{ marginTop: 14, overflowX: 'auto' }}>
        <div style={{ minWidth: 1000 }}>
          <div className="thead" style={{ gridTemplateColumns: GRID, alignItems: 'center' }}>{box(allOn, () => setPicked(allOn ? {} : Object.fromEntries(rows.map(p => [p.id, true]))))}<span>Ref.</span><span>Comerciante</span><span>Conta</span><span style={{ textAlign: 'right' }}>Valor</span><span>Prazo 72 h</span><span>Estado</span><span /></div>
          {rows.map(p => {
            const on = !!picked[p.id], pend = p.status === 'new' || p.status === 'review'
            return (
              <div key={p.id} className="trow" style={{ gridTemplateColumns: GRID, background: on ? '#F6FAF4' : undefined }}>
                {box(on, () => setPicked({ ...picked, [p.id]: !on }))}
                <span className="mono" style={{ fontSize: 12.5 }}>{p.reference}</span>
                <div style={{ minWidth: 0 }}><div style={{ fontWeight: 600, fontSize: 14 }}>{p.merchant_name}</div><div style={{ fontSize: 12, color: 'var(--muted)' }}>Pedido {relDate(p.requested_at_iso)}</div></div>
                <div style={{ fontSize: 13, color: 'var(--text-2)', minWidth: 0 }}>{p.bank_account.bank} <span className="mono">•• {p.nib_last4}</span><div style={{ fontSize: 12, color: 'var(--muted)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{p.bank_account.holder}</div></div>
                <div className="tnum" style={{ textAlign: 'right', fontWeight: 600, fontSize: 14 }}>{fmt(p.amount)} MT</div>
                <SlaBar p={p} />
                <div><Pill {...(ST as any)[p.status]} />{p.failure_reason && <div style={{ fontSize: 11.5, color: 'var(--red-dark)', marginTop: 3 }} title={p.failure_reason}>{p.failure_reason.slice(0, 28)}…</div>}</div>
                <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
                  {pend && <button className="btn btn-green btn-sm" disabled={busy} onClick={() => act([p.id], 'approve')}>Aprovar</button>}
                  {p.status === 'sent' && <button className="btn btn-outline btn-sm" disabled={busy} onClick={() => act([p.id], 'paid')}>Marcar pago</button>}
                  {p.status === 'paid' && <button className="btn btn-outline btn-sm" onClick={() => exportFile([p.id])}>Comprovativo</button>}
                </div>
              </div>
            )
          })}
          {data && !rows.length && <div className="empty">Nada nesta lista.</div>}
        </div>
      </div>
      {reject && (
        <Modal onClose={() => setReject(null)}>
          <div className="display" style={{ fontSize: 22 }}>{reject.action === 'fail' ? 'Marcar como falhados' : 'Rejeitar levantamentos'}</div>
          <div style={{ fontSize: 14, color: 'var(--muted)', marginTop: 8, lineHeight: 1.5 }}>{reject.ids.length} pedido(s). O valor volta ao saldo de cada comerciante e o motivo é mostrado no painel deles.</div>
          <label style={{ display: 'block', marginTop: 14 }}><span className="label">Motivo</span><input autoFocus className="input" value={reason} onChange={e => setReason(e.target.value)} placeholder={reject.action === 'fail' ? 'NIB inválido: banco devolveu a transferência' : 'Titular da conta não coincide'} /></label>
          <div style={{ display: 'flex', gap: 10, marginTop: 18 }}><button className="btn btn-outline" style={{ flex: 1 }} onClick={() => setReject(null)}>Cancelar</button><button className="btn btn-ink" style={{ flex: 1 }} disabled={!reason.trim() || busy} onClick={async () => { await act(reject.ids, reject.action, reason); setReject(null); setReason('') }}>Confirmar</button></div>
        </Modal>
      )}
    </div>
  )
}
