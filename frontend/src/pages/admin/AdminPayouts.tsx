import { useState } from 'react'
import { errMsg } from '../../core/model'
import { useGateway } from '../../core/gateway'
import { useQuery } from '../../core/useQuery'
import { fmt, PAYOUT_STATUS, relDate } from '../../lib/format'
import { useSession } from '../../lib/session'
import { Icon, Modal, Pill, Spinner } from '../../components/ui'
import { SlaBar } from './common'

const TABS: [string, string, string][] = [['pending', 'Pendentes', 'pending_count'], ['sent', 'Enviados ao banco', 'sent_count'], ['paid', 'Pagos', 'paid_count'], ['all', 'Todos', 'all_count']]
const GRID = '40px 90px minmax(0,1.3fr) minmax(0,1fr) 130px 150px 130px 210px'
const ST = { ...PAYOUT_STATUS, paid: { t: 'Pago', bg: '#E2F3DE', fg: '#0A6B05' } }

export default function AdminPayouts() {
  const { toast } = useSession()
  const g = useGateway()
  const [tab, setTab] = useState('pending')
  const [picked, setPicked] = useState<Record<string, boolean>>({})
  const [busy, setBusy] = useState(false)
  const [reject, setReject] = useState<{ ids: string[]; action: 'reject' | 'fail' } | null>(null)
  const [complete, setComplete] = useState<string[] | null>(null)
  const [done, setDone] = useState<any | null>(null)
  const [reason, setReason] = useState('')
  const { data, reload } = useQuery(g => g.admin.payouts(tab), [tab], 15000)
  const rows: any[] = data?.data || []
  const st = data?.stats
  const ids = rows.filter(p => picked[p.id]).map(p => p.id)
  // What goes to the bank is the net: the amount less the transfer fee.
  const sum = rows.filter(p => picked[p.id]).reduce((a, p) => a + (p.net ?? p.amount), 0)
  const allOn = rows.length > 0 && ids.length === rows.length

  const act = async (list: string[], action: string, why = '') => {
    setBusy(true)
    try {
      const r = await g.admin.payoutAction(list, action, why)
      const verb: Record<string, string> = { approve: 'aprovados e enviados ao banco', paid: 'marcados como pagos', reject: 'rejeitados. O saldo volta aos comerciantes.', fail: 'marcados como falhados. O saldo volta aos comerciantes.' }
      toast(r.failed.length ? `${r.done.length} concluídos, ${r.failed.length} com erro: ${r.failed[0].error}` : r.done.length === 1 ? `${r.done[0]} ${verb[action].replace(/os\b/g, 'o').replace('marcados', 'marcado').replace('rejeitados', 'rejeitado')}` : `${r.done.length} ${verb[action]}`, r.failed.length > 0)
      setPicked({}); reload()
    } catch (e) { toast(errMsg(e), true) } finally { setBusy(false) }
  }
  const openPdf = (url: string) => window.open(url, '_blank', 'noopener')
  const email = async (p: any) => { try { const r = await g.admin.emailPayoutStatement(p.id); toast(`Comprovativo de ${p.reference} enviado para ${r.to}`); reload() } catch (e) { toast(errMsg(e), true) } }
  const exportFile = (list: string[]) => { window.location.href = g.admin.payoutExportUrl(list); toast(`Ficheiro bancário exportado (${list.length || 'todas as'} transferências)`) }
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
          <button className="btn" style={{ height: 38, border: '1px solid rgba(255,255,255,.2)', background: 'transparent', color: '#fff', fontSize: 13.5 }} onClick={() => exportFile(ids)}>Exportar CSV</button>
          {tab !== 'paid' && <button className="btn" style={{ height: 38, border: '1px solid rgba(255,255,255,.2)', background: 'transparent', color: '#fff', fontSize: 13.5 }} onClick={() => openPdf(g.admin.payoutInstructionsUrl(ids))}>Ordens em PDF</button>}
          {tab !== 'paid' && <button className="btn" style={{ height: 38, border: '1px solid rgba(255,255,255,.2)', background: 'transparent', color: '#FF9C9F', fontSize: 13.5 }} onClick={() => setReject({ ids, action: tab === 'sent' ? 'fail' : 'reject' })}>{tab === 'sent' ? 'Marcar falhados' : 'Rejeitar'}</button>}
          {tab !== 'paid' && <button className="btn btn-green" style={{ height: 38, fontSize: 13.5 }} disabled={busy} onClick={() => (tab === 'sent' ? setComplete(ids) : act(ids, 'approve'))}>{busy ? <Spinner /> : tab === 'sent' ? 'Marcar como pagos' : 'Aprovar e enviar'}</button>}
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
                <div className="tnum" style={{ textAlign: 'right', fontWeight: 600, fontSize: 14 }}>{fmt(p.net ?? p.amount)} MT{p.fee > 0 && <div style={{ fontSize: 11.5, fontWeight: 400, color: 'var(--muted)' }}>{fmt(p.amount)} − taxa {fmt(p.fee)}</div>}</div>
                <SlaBar p={p} />
                <div><Pill {...(ST as any)[p.status]} />{p.failure_reason && <div style={{ fontSize: 11.5, color: 'var(--red-dark)', marginTop: 3 }} title={p.failure_reason}>{p.failure_reason.slice(0, 28)}…</div>}</div>
                <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 6, flexWrap: 'wrap' }}>
                  {(pend || p.status === 'sent') && <button className="btn btn-outline btn-sm" title="Ordem de transferência com os dados bancários" onClick={() => openPdf(g.admin.payoutInstructionsUrl([p.id]))}><Icon name="doc" size={13} />PDF</button>}
                  {pend && <button className="btn btn-green btn-sm" disabled={busy} onClick={() => act([p.id], 'approve')}>Aprovar</button>}
                  {p.status === 'sent' && <button className="btn btn-green btn-sm" disabled={busy} onClick={() => setComplete([p.id])}>Concluir</button>}
                  {p.status === 'paid' && <>
                    <button className="btn btn-outline btn-sm" onClick={() => openPdf(g.admin.payoutStatementUrl(p.id))}><Icon name="doc" size={13} />PDF</button>
                    {p.has_receipt && <a className="btn btn-outline btn-sm" href={g.admin.payoutReceiptUrl(p.id)} title={p.receipt_name}>Talão</a>}
                    <button className="btn btn-outline btn-sm" onClick={() => email(p)} title={p.emailed_at ? 'Enviado ' + relDate(p.emailed_at) : 'Enviar ao comerciante'}>{p.emailed_at ? 'Reenviar' : 'Email'}</button>
                  </>}
                </div>
              </div>
            )
          })}
          {data && !rows.length && <div className="empty">Nada nesta lista.</div>}
        </div>
      </div>
      {complete && <CompleteModal ids={complete} rows={rows.filter(p => complete.includes(p.id))} onClose={() => setComplete(null)} onDone={r => { setComplete(null); setPicked({}); reload(); setDone(r) }} />}
      {done && (
        <Modal onClose={() => setDone(null)}>
          <div style={{ width: 52, height: 52, borderRadius: '50%', background: 'var(--green)', display: 'grid', placeItems: 'center', boxShadow: '0 0 0 8px #E2F3DE' }}><Icon name="check" size={24} stroke={2.6} color="#fff" /></div>
          <div className="display" style={{ fontSize: 22, marginTop: 16 }}>{done.length === 1 ? `${done[0].reference} concluído` : `${done.length} levantamentos concluídos`}</div>
          <div style={{ fontSize: 14, color: 'var(--muted)', marginTop: 6, lineHeight: 1.5 }}>O comprovativo bancário ficou anexado. Descarregue o comprovativo completo ou envie-o ao comerciante.</div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 8, marginTop: 18 }}>
            {done.map((p: any) => (
              <div key={p.id} className="row" style={{ gap: 8 }}>
                <span className="mono" style={{ flex: 1, fontSize: 13 }}>{p.reference} · {p.merchant_name}</span>
                <button className="btn btn-outline btn-sm" onClick={() => openPdf(g.admin.payoutStatementUrl(p.id) + '?download=1')}><Icon name="download" size={13} />PDF</button>
                <button className="btn btn-ink btn-sm" onClick={() => email(p)}>Enviar email</button>
              </div>
            ))}
          </div>
          <button className="btn btn-outline btn-block" style={{ marginTop: 18 }} onClick={() => setDone(null)}>Fechar</button>
        </Modal>
      )}
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

function CompleteModal({ ids, rows, onClose, onDone }: { ids: string[]; rows: any[]; onClose: () => void; onDone: (rows: any[]) => void }) {
  const g = useGateway()
  const { toast } = useSession()
  const [file, setFile] = useState<File | null>(null)
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')
  const total = rows.reduce((a, p) => a + (p.net ?? p.amount), 0)
  const go = async () => {
    if (!file) return
    setBusy(true); setErr('')
    try {
      const r = await g.admin.completePayouts(ids, file)
      toast(`${r.done.length} ${r.done.length === 1 ? 'levantamento concluído' : 'levantamentos concluídos'}`)
      onDone(rows.map(p => ({ ...p, status: 'paid' })))
    } catch (e) { setErr(errMsg(e)) } finally { setBusy(false) }
  }
  return (
    <Modal onClose={onClose} width={480}>
      <div className="display" style={{ fontSize: 22 }}>Concluir {ids.length === 1 ? rows[0]?.reference : `${ids.length} levantamentos`}</div>
      <div style={{ fontSize: 14, color: 'var(--muted)', marginTop: 6, lineHeight: 1.5 }}>Carregue o comprovativo do banco como evidência da transferência de <b style={{ color: 'var(--ink)' }}>{fmt(total)} MT</b>. Sem comprovativo não é possível marcar como pago.</div>
      <label style={{ display: 'flex', alignItems: 'center', gap: 12, marginTop: 16, padding: '14px 16px', borderRadius: 14, border: '1.5px dashed #D6D4CB', background: '#FAFAF7', cursor: 'pointer' }}>
        <Icon name={file ? 'check' : 'upload'} size={18} color={file ? 'var(--green)' : 'currentColor'} />
        <span style={{ flex: 1, fontSize: 14 }}>{file ? <><b>{file.name}</b> <span className="muted">· {Math.round(file.size / 1000)} KB</span></> : 'Escolher comprovativo (PDF, JPG ou PNG)'}</span>
        <input type="file" accept=".pdf,.jpg,.jpeg,.png" hidden onChange={e => setFile(e.target.files?.[0] || null)} />
      </label>
      {err && <div className="form-err" style={{ marginTop: 12 }}>{err}</div>}
      <div style={{ display: 'flex', gap: 10, marginTop: 18 }}><button className="btn btn-outline" style={{ flex: 1 }} onClick={onClose}>Cancelar</button><button className="btn btn-green" style={{ flex: 1 }} disabled={!file || busy} onClick={go}>{busy ? <Spinner /> : 'Marcar como pago'}</button></div>
    </Modal>
  )
}
