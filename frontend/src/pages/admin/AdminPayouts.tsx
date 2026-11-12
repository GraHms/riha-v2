import { useState } from 'react'
import { errMsg } from '../../core/model'
import { useGateway } from '../../core/gateway'
import { useQuery } from '../../core/useQuery'
import { fmt, PAYOUT_STATUS, relDate } from '../../lib/format'
import { useSession } from '../../lib/session'
import { Icon, Modal, Skeleton, Spinner } from '../../components/ui'
import { Amount, Badge, Empty, List, Page, PageHead, Stat, type Tone } from '../../components/ops'
import { SlaBar } from './common'
import { PayModal } from '../partner/PartnerPayouts'

// Funding shows who sends each part of a withdrawal: the partners whose
// accounts hold the merchant's money, and RIHA for the rest. A part a partner
// says it paid waits for RIHA to see the transfer.
function Funding({ parts, approved, pay, review }: { parts?: any[]; approved: boolean; pay: (part: any) => void; review: (part: any) => void }) {
  const live = (parts || []).filter(f => f.status !== 'cancelled')
  if (!live.length) return null
  return (
    <div style={{ display: 'flex', gap: 4, flexWrap: 'wrap', marginTop: 6 }}>
      {live.map(f => (
        <span key={f.id} className={'o-chip' + (f.status === 'paid' ? ' paid' : f.status === 'claimed' ? ' claimed' : '')}
          title={f.status === 'paid' ? `Pago · ${f.reference}` : f.status === 'claimed' ? `Diz que pagou · ${f.reference}` : 'Por pagar'}>
          {f.status === 'paid' && <Icon name="check" size={11} stroke={3} />}
          {f.status === 'claimed' && <Icon name="clock" size={11} stroke={2.6} />}
          {f.source_name} · {fmt(f.send, false)}
          {f.status === 'claimed' && <button className="link-btn" style={{ fontSize: 11 }} onClick={e => { e.stopPropagation(); review(f) }}>confirmar</button>}
          {approved && f.status === 'open' && f.source !== 'riha' && <button className="link-btn" style={{ fontSize: 11 }} onClick={e => { e.stopPropagation(); pay(f) }} title="Registar que o parceiro pagou">pago?</button>}
        </span>
      ))}
    </div>
  )
}

// BankFlags asks staff to look twice: an account changed lately, or one in a
// name that isn't the merchant's.
function BankFlags({ p }: { p: any }) {
  const changed = p.bank_changed_at && Date.now() - new Date(p.bank_changed_at).getTime() < 30 * 864e5
  if (!changed && !p.holder_differs) return null
  return (
    <div style={{ display: 'flex', gap: 4, flexWrap: 'wrap', marginTop: 6 }}>
      {changed && <span className="o-chip warn" title="O comerciante mudou a conta bancária">Conta mudada {relDate(p.bank_changed_at)}</span>}
      {p.holder_differs && <span className="o-chip warn" title={`Titular: ${p.bank_account?.holder || ''}`}>Titular diferente</span>}
    </div>
  )
}

const CLAIM_CSS = `
.o-chip.warn{background:#FDECEC;color:#9B1C1C;border-color:#F6CACA}
.o-chip.claimed{background:#FFF4DB;color:#8A5A00;border-color:#F5DFAE}
.ap-claims{display:flex;align-items:center;gap:10px 12px;flex-wrap:wrap;padding:12px 16px;margin-bottom:14px;border-radius:14px;background:#FFF8E8;border:1px solid #F5DFAE;font-size:13.5px}
.ap-claims > span{flex:1 1 220px;min-width:0;display:flex;align-items:center;gap:8px}
.ap-claims b{font-variant-numeric:tabular-nums}
.cl-list{display:flex;flex-direction:column;gap:8px;margin-top:14px;max-height:52vh;overflow:auto}
.cl-row{display:grid;grid-template-columns:20px minmax(0,1fr);gap:10px;align-items:start;padding:12px;border-radius:12px;border:1px solid var(--o-line-2);background:#fff}
.cl-main{min-width:0;font-size:13px;line-height:1.45}
.cl-main .o-t1,.cl-main .o-t2{white-space:normal;overflow:visible}
.cl-main .o-t1{font-size:13.5px}
.cl-warn{margin-top:6px;padding:6px 8px;border-radius:8px;background:#FDECEC;color:#9B1C1C;font-size:12.5px}
.cl-acts{display:flex;align-items:center;gap:12px;flex-wrap:wrap;margin-top:8px}
.cl-rej{display:flex;gap:6px;margin-top:8px}
.cl-rej .input{height:34px;flex:1;min-width:0}
@media(max-width:640px){.ap-claims .btn{width:100%}}
`

const TABS: [string, string, string][] = [['pending', 'Pendentes', 'pending_count'], ['sent', 'No banco', 'sent_count'], ['paid', 'Pagos', 'paid_count'], ['all', 'Todos', 'all_count']]
const TONE: Record<string, Tone> = { new: 'info', review: 'warn', sent: 'info', paid: 'ok', rejected: 'bad', failed: 'bad' }
const label = (st: string) => (st === 'paid' ? 'Pago' : PAYOUT_STATUS[st]?.t || st)

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
  const [payPart, setPayPart] = useState<any | null>(null)
  const [claims, setClaims] = useState<any[] | null>(null)
  const { data, reload } = useQuery(g => g.admin.payouts(tab), [tab], 15000)
  const rows: any[] = data?.data || []
  const st = data?.stats
  const ids = rows.filter(p => picked[p.id]).map(p => p.id)
  // What goes to the bank is the net: the amount less the transfer fee.
  const sum = rows.filter(p => picked[p.id]).reduce((a, p) => a + (p.net ?? p.amount), 0)
  const allOn = rows.length > 0 && ids.length === rows.length
  const claimed = rows.flatMap(p => (p.funding || []).filter((f: any) => f.status === 'claimed').map((f: any) => ({ ...f, payout: p })))

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
  const box = (on: boolean, onClick: () => void) => <button onClick={e => { e.stopPropagation(); onClick() }} aria-pressed={on} style={{ width: 18, height: 18, borderRadius: 5, border: `1.5px solid ${on ? 'var(--green)' : '#C9D1CD'}`, background: on ? 'var(--green)' : '#fff', padding: 0, display: 'grid', placeItems: 'center' }}>{on && <Icon name="check" size={11} stroke={3.4} color="#fff" />}</button>

  const actions = (p: any) => {
    const pend = p.status === 'new' || p.status === 'review'
    return <>
      {(pend || p.status === 'sent') && <button className="btn btn-outline btn-sm" title="Ordem de transferência" onClick={() => openPdf(g.admin.payoutInstructionsUrl([p.id]))}><Icon name="doc" size={13} />PDF</button>}
      {pend && <button className="btn btn-green btn-sm" disabled={busy} onClick={() => act([p.id], 'approve')}>Aprovar</button>}
      {p.status === 'sent' && <button className="btn btn-green btn-sm" disabled={busy} onClick={() => setComplete([p.id])}>Concluir</button>}
      {p.status === 'paid' && <>
        <button className="btn btn-outline btn-sm" onClick={() => openPdf(g.admin.payoutStatementUrl(p.id))}><Icon name="doc" size={13} />PDF</button>
        {p.has_receipt && <a className="btn btn-outline btn-sm" href={g.admin.payoutReceiptUrl(p.id)} title={p.receipt_name}>Talão</a>}
        <button className="btn btn-outline btn-sm" onClick={() => email(p)} title={p.emailed_at ? 'Enviado ' + relDate(p.emailed_at) : 'Enviar ao comerciante'}>{p.emailed_at ? 'Reenviar' : 'Email'}</button>
      </>}
    </>
  }
  const amount = (p: any) => <><Amount c={p.net ?? p.amount} />{p.fee > 0 && <div className="o-t2">taxa {fmt(p.fee, false)}</div>}</>

  return (
    <Page>
      <PageHead title="Levantamentos" sub="Cada pedido é pago em até 72 horas."
        actions={<button className="btn btn-outline" onClick={() => exportFile([])}><Icon name="download" size={15} />Exportar</button>} />
      <div className="o-stats">
        <Stat label="Por processar" value={st?.pending_total} hint={`${st?.pending_count ?? 0} pedidos`} />
        <Stat label="Urgentes" tip="Vencem em menos de 8 horas." value={String(st?.urgent ?? 0)} tone={st?.urgent ? 'bad' : undefined} hint={st?.urgent ? 'Prioridade' : 'Nada urgente'} />
        <Stat label="No banco" value={st?.sent_total} hint="À espera de confirmação" />
      </div>
      <style>{CLAIM_CSS}</style>
      <div className="tabs" style={{ marginTop: 20 }}>{TABS.map(([k, l, c]) => <button key={k} className={tab === k ? 'on' : ''} onClick={() => { setTab(k); setPicked({}) }}>{l}<span className="count">{st?.[c] ?? '·'}</span></button>)}</div>
      {claimed.length > 0 && (
        <div className="ap-claims">
          <span><Icon name="clock" size={16} /><span><b>{claimed.length}</b> {claimed.length === 1 ? 'pagamento de parceiro' : 'pagamentos de parceiros'} por confirmar · <b>{fmt(claimed.reduce((a, f) => a + f.send, 0))} MT</b></span></span>
          <button className="btn btn-ink btn-sm" onClick={() => setClaims(claimed)}>Rever no extrato</button>
        </div>
      )}
      {ids.length > 0 && (
        <div className="row" style={{ position: 'sticky', top: 8, zIndex: 4, gap: 8, margin: '12px 0', padding: '8px 8px 8px 16px', borderRadius: 14, background: 'var(--o-ink)', color: '#fff', flexWrap: 'wrap', boxShadow: 'var(--o-shadow-lift)' }}>
          <span style={{ fontWeight: 650, fontSize: 13.5, flex: 1, minWidth: 160 }}>{ids.length} {ids.length === 1 ? 'selecionado' : 'selecionados'} · {fmt(sum)} MT</span>
          <button className="btn btn-sm" style={{ background: 'rgba(255,255,255,.1)', color: '#fff' }} onClick={() => exportFile(ids)}>CSV</button>
          {tab !== 'paid' && <button className="btn btn-sm" style={{ background: 'rgba(255,255,255,.1)', color: '#fff' }} onClick={() => openPdf(g.admin.payoutInstructionsUrl(ids))}>Ordens PDF</button>}
          {tab !== 'paid' && <button className="btn btn-sm" style={{ background: 'rgba(255,255,255,.1)', color: '#FF9C9F' }} onClick={() => setReject({ ids, action: tab === 'sent' ? 'fail' : 'reject' })}>{tab === 'sent' ? 'Falhou' : 'Rejeitar'}</button>}
          {tab !== 'paid' && <button className="btn btn-green btn-sm" disabled={busy} onClick={() => (tab === 'sent' ? setComplete(ids) : act(ids, 'approve'))}>{busy ? <Spinner /> : tab === 'sent' ? 'Marcar como pagos' : 'Aprovar'}</button>}
        </div>
      )}
      <div style={{ marginTop: ids.length ? 0 : 14 }}>
        {!data ? <Skeleton h={200} /> : (
          <List rows={rows} rowKey={p => p.id} selected={p => !!picked[p.id]}
            empty={<Empty icon="wallet" title="Nada nesta lista" text={tab === 'pending' ? 'Sem pedidos por aprovar.' : undefined} />}
            cols={[
              { head: box(allOn, () => setPicked(allOn ? {} : Object.fromEntries(rows.map(p => [p.id, true])))), w: '28px', cell: p => box(!!picked[p.id], () => setPicked({ ...picked, [p.id]: !picked[p.id] })) },
              { head: 'Comerciante', w: 'minmax(0,1.5fr)', cell: p => <><div className="o-t1">{p.merchant_name}</div><div className="o-t2"><span className="o-mono">{p.reference}</span> · {relDate(p.requested_at_iso)}</div><BankFlags p={p} /><Funding parts={p.funding} approved={p.status === 'sent'} pay={part => setPayPart({ ...part, payout: p })} review={part => setClaims([{ ...part, payout: p }])} /></> },
              { head: 'Conta', w: 'minmax(0,1fr)', cell: p => <><div className="o-t1" style={{ fontWeight: 500 }}>{p.bank_account.bank} <span className="o-mono">•• {p.nib_last4}</span></div><div className="o-t2">{p.bank_account.holder}</div></> },
              { head: 'Valor', w: '130px', right: true, cell: amount },
              { head: 'Prazo', w: '130px', cell: p => <SlaBar p={p} /> },
              { head: 'Estado', w: '110px', cell: p => <><Badge tone={TONE[p.status]} dot>{label(p.status)}</Badge>{p.failure_reason && <div className="o-t2" style={{ color: 'var(--o-bad)' }} title={p.failure_reason}>{p.failure_reason}</div>}</> },
              { head: '', w: '200px', right: true, cell: p => <div style={{ display: 'flex', gap: 6, justifyContent: 'flex-end', flexWrap: 'wrap' }}>{actions(p)}</div> },
            ]}
            card={p => ({
              lead: box(!!picked[p.id], () => setPicked({ ...picked, [p.id]: !picked[p.id] })),
              title: p.merchant_name,
              sub: <><span className="o-mono">{p.reference}</span> · {p.bank_account.bank} •• {p.nib_last4}<BankFlags p={p} /><Funding parts={p.funding} approved={p.status === 'sent'} pay={part => setPayPart({ ...part, payout: p })} review={part => setClaims([{ ...part, payout: p }])} /></>,
              amount: <Amount c={p.net ?? p.amount} />,
              meta: <Badge tone={TONE[p.status]} dot>{label(p.status)}</Badge>,
              foot: <><div style={{ flex: '1 1 120px' }}><SlaBar p={p} /></div><div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>{actions(p)}</div></>,
            })} />
        )}
      </div>
      {claims && <ClaimsModal parts={claims} onClose={() => setClaims(null)} onDone={() => { setClaims(null); reload() }} />}
      {payPart && <PayModal row={{ ...payPart, payout: { ...payPart.payout, holder: payPart.payout.bank_account?.holder || payPart.payout.merchant_name, bank_name: payPart.payout.bank_account?.bank } }}
        pay={(ref, file) => g.admin.payFunding(payPart.id, ref, file)} onClose={() => setPayPart(null)} onDone={() => { setPayPart(null); toast('Parte marcada como paga'); reload() }} />}
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
    </Page>
  )
}

function CompleteModal({ ids, rows, onClose, onDone }: { ids: string[]; rows: any[]; onClose: () => void; onDone: (rows: any[]) => void }) {
  const g = useGateway()
  const { toast } = useSession()
  const [file, setFile] = useState<File | null>(null)
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')
  const total = rows.reduce((a, p) => a + (p.net ?? p.amount), 0)
  const partnerParts = rows.flatMap(p => (p.funding || []).filter((f: any) => f.status === 'open' && f.source !== 'riha'))
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
      {partnerParts.length > 0 && <div style={{ fontSize: 13, marginTop: 12, padding: '10px 12px', borderRadius: 12, background: '#FFF6E5', lineHeight: 1.5 }}>Inclui {partnerParts.length} parte(s) que os parceiros iam pagar ({partnerParts.map((f: any) => `${f.source_name} ${fmt(f.send)} MT`).join(', ')}). Concluir aqui marca-as também como pagas com este comprovativo.</div>}
      {err && <div className="form-err" style={{ marginTop: 12 }}>{err}</div>}
      <div style={{ display: 'flex', gap: 10, marginTop: 18 }}><button className="btn btn-outline" style={{ flex: 1 }} onClick={onClose}>Cancelar</button><button className="btn btn-green" style={{ flex: 1 }} disabled={!file || busy} onClick={go}>{busy ? <Spinner /> : 'Marcar como pago'}</button></div>
    </Modal>
  )
}

// ClaimsModal is RIHA checking, on its statement, the transfers partners say
// they made: confirming one pays that part, and the last part pays the
// withdrawal and tells the merchant. One not on the statement goes back to
// the partner, with why.
function ClaimsModal({ parts, onClose, onDone }: { parts: any[]; onClose: () => void; onDone: () => void }) {
  const g = useGateway()
  const { toast } = useSession()
  const [on, setOn] = useState<Record<string, boolean>>(() => Object.fromEntries(parts.map(f => [f.id, !f.warning])))
  const [rejecting, setRejecting] = useState<string | null>(null)
  const [why, setWhy] = useState('')
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')
  const ids = parts.filter(f => on[f.id]).map(f => f.id)
  const total = parts.filter(f => on[f.id]).reduce((a, f) => a + f.send, 0)
  const confirm = async () => {
    setBusy(true); setErr('')
    try { await g.admin.confirmFunding(ids); toast(ids.length === 1 ? 'Pagamento confirmado' : `${ids.length} pagamentos confirmados`); onDone() } catch (e) { setErr(errMsg(e)) } finally { setBusy(false) }
  }
  const reject = async (f: any) => {
    setBusy(true); setErr('')
    try { await g.admin.rejectClaim(f.id, why); toast(`Devolvido a ${f.source_name}`); onDone() } catch (e) { setErr(errMsg(e)) } finally { setBusy(false) }
  }
  return (
    <Modal onClose={onClose} width={560}>
      <style>{CLAIM_CSS}</style>
      <div className="display" style={{ fontSize: 22 }}>Confirmar pagamentos de parceiros</div>
      <div style={{ fontSize: 14, color: 'var(--muted)', marginTop: 6, lineHeight: 1.5 }}>Confirme só o que vê no extrato do banco. O comerciante é avisado quando todas as partes do levantamento estão confirmadas.</div>
      <div className="cl-list">
        {parts.map(f => (
          <div key={f.id} className="cl-row">
            <input type="checkbox" checked={!!on[f.id]} onChange={e => setOn({ ...on, [f.id]: e.target.checked })} style={{ width: 18, height: 18, accentColor: 'var(--green)', marginTop: 2 }} aria-label={`Confirmar ${f.payout.reference}`} />
            <div className="cl-main">
              <div className="o-t1">{f.source_name} · <b>{fmt(f.send)} MT</b></div>
              <div className="o-t2"><span className="o-mono">{f.payout.reference}</span> · {f.payout.merchant_name} · ref. <span className="o-mono">{f.reference}</span></div>
              {f.warning && <div className="cl-warn">{f.warning}</div>}
              <div className="cl-acts">
                {f.has_receipt && <a className="btn btn-outline btn-sm" href={g.admin.fundingReceiptUrl(f.id)} target="_blank" rel="noopener">Ver comprovativo</a>}
                {rejecting !== f.id && <button className="link-btn" style={{ fontSize: 12.5, color: 'var(--o-bad)' }} onClick={() => { setRejecting(f.id); setWhy('') }}>Não está no extrato</button>}
              </div>
              {rejecting === f.id && (
                <div className="cl-rej">
                  <input autoFocus className="input" value={why} onChange={e => setWhy(e.target.value)} placeholder="Porque não aparece" aria-label="Motivo" />
                  <button className="btn btn-ink btn-sm" disabled={busy || !why.trim()} onClick={() => reject(f)}>Devolver ao parceiro</button>
                </div>
              )}
            </div>
          </div>
        ))}
      </div>
      {err && <div className="form-err" style={{ marginTop: 12 }}>{err}</div>}
      <div style={{ display: 'flex', gap: 10, marginTop: 18 }}>
        <button className="btn btn-outline" style={{ flex: 1 }} onClick={onClose}>Fechar</button>
        <button className="btn btn-green" style={{ flex: 1 }} disabled={busy || !ids.length} onClick={confirm}>{busy ? <Spinner /> : ids.length ? `Confirmar ${fmt(total)} MT` : 'Confirmar'}</button>
      </div>
    </Modal>
  )
}
