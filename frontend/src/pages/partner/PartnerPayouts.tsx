import { useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { errMsg } from '../../core/model'
import { useGateway } from '../../core/gateway'
import { useQuery } from '../../core/useQuery'
import { fmt, initials, relDate } from '../../lib/format'
import { useSession } from '../../lib/session'
import { Avatar, Icon, Modal, Skeleton, Spinner } from '../../components/ui'
import { Badge, Empty, Money, Page, PageHead, Tip, type Tone } from '../../components/ops'

// The withdrawals a partner pays: merchants' money that landed in its account,
// sent to their bank once RIHA approves.

const TABS: [string, string][] = [['to_pay', 'A pagar'], ['waiting', 'À espera da RIHA'], ['paid', 'Pagos']]

const rel = (d: string) => relDate(d).replace(/^./, c => c.toLowerCase())

/** deadline says how close a withdrawal is to its due time. */
function deadline(due: string | null | undefined, urgent: boolean): [string, Tone] | null {
  if (!due) return null
  const h = (new Date(due).getTime() - Date.now()) / 3600e3
  if (Number.isNaN(h)) return null
  if (urgent && h < 0) return ['Em atraso', 'bad']
  if (urgent && h < 8) return [h < 1 ? 'Vence em menos de 1 h' : `Vence em ${Math.floor(h)} h`, 'warn']
  return ['Até ' + rel(due), undefined]
}

const CSS = `
.pp-tabs{margin-bottom:16px}
.pp-sum{display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap;padding:16px 20px;margin-bottom:14px}
.pp-sum .o-stat-v{font-size:24px;margin-top:2px}
.pp-list{display:grid;grid-template-columns:repeat(auto-fill,minmax(360px,1fr));gap:14px}
.pp-card{display:flex;flex-direction:column;gap:14px;padding:16px 18px 18px;min-width:0;background:var(--o-surface);border:1px solid var(--o-line);border-radius:var(--o-r);box-shadow:var(--o-shadow)}
.pp-card.late{border-color:#F7C9C5;box-shadow:0 0 0 3px var(--o-bad-soft),var(--o-shadow)}
.pp-top{display:flex;align-items:center;gap:12px;min-width:0}
.pp-who{flex:1;min-width:0}
.pp-who .o-t1{font-size:15px}
.pp-amt{padding:14px 16px;border-radius:12px;background:var(--o-surface-2);border:1px solid var(--o-line-2)}
.pp-amt .o-stat-v{font-size:28px;margin-top:3px}
.pp-amt .o-t2{white-space:normal;margin-top:3px}
.pp-bank{display:flex;flex-direction:column}
.pp-kv{display:grid;grid-template-columns:64px minmax(0,1fr) auto;align-items:center;gap:10px;min-height:38px;padding:4px 0;font-size:13.5px}
.pp-kv + .pp-kv{border-top:1px dashed var(--o-line-2)}
.pp-kv > span{font-size:12.5px;color:var(--o-muted);font-weight:600}
.pp-kv > b{font-weight:650;min-width:0;overflow-wrap:anywhere}
.pp-kv > b.o-mono{font-size:13px;letter-spacing:.02em}
.pp-foot{display:flex;align-items:center;gap:10px;flex-wrap:wrap;margin-top:auto}
.pp-foot .pp-main{flex:1 1 auto;height:44px;font-size:14.5px}
.pp-foot .o-t2{flex:1;min-width:0;white-space:normal}
@media(max-width:860px){
  .pp-list{grid-template-columns:minmax(0,1fr);gap:12px}
  .pp-card{padding:14px 14px 16px}
  .pp-amt .o-stat-v{font-size:26px}
  .pp-sum{padding:14px 16px}
  .pp-top{flex-wrap:wrap}
  .pp-who{flex:1 1 170px}
  .pp-kv .btn{height:36px}
}`

export default function PartnerPayouts() {
  const g = useGateway()
  const { toast } = useSession()
  const [sp, setSp] = useSearchParams()
  const tab = TABS.some(t => t[0] === sp.get('tab')) ? sp.get('tab')! : 'to_pay'
  const { data, reload } = useQuery(g => g.partner.payouts(tab), [tab], 30000)
  const [paying, setPaying] = useState<any | null>(null)
  const rows: any[] = data?.data || []
  const total = rows.reduce((a, r) => a + r.send, 0)
  const copy = (v: string) => { navigator.clipboard?.writeText(v); toast('Copiado') }
  return (
    <Page>
      <style>{CSS}</style>
      <PageHead title="Levantamentos"
        sub={<span className="row" style={{ gap: 6, display: 'inline-flex' }}>Pague os pedidos que a RIHA aprovou.<Tip text="Transfira para o banco do comerciante e marque como pago com o comprovativo." /></span>} />
      <div className="tabs pp-tabs">{TABS.map(([k, l]) => <button key={k} className={tab === k ? 'on' : ''} onClick={() => setSp(k === 'to_pay' ? {} : { tab: k })}>{l}</button>)}</div>

      {tab === 'to_pay' && rows.length > 0 && (
        <div className="o-panel pp-sum">
          <div>
            <div className="o-stat-l">Total a transferir</div>
            <div className="o-stat-v"><Money c={total} /></div>
          </div>
          <Badge tone="warn" dot>{rows.length} por pagar</Badge>
        </div>
      )}

      {!data && <Skeleton h={220} />}
      {data && !rows.length && (
        <div className="o-panel">
          {tab === 'to_pay' ? <Empty icon="check" title="Nada por pagar agora" />
            : tab === 'waiting' ? <Empty icon="clock" title="Nenhum pedido à espera da RIHA" />
            : <Empty icon="wallet" title="Ainda não pagou levantamentos" />}
        </div>
      )}
      {rows.length > 0 && (
        <div className="pp-list">
          {rows.map(r => {
            const p = r.payout
            const dl = tab !== 'paid' ? deadline(p.due_at, tab === 'to_pay') : null
            const note = r.send !== r.amount ? `${fmt(r.amount)} − taxa de transferência ${fmt(r.amount - r.send)}` : r.amount !== p.amount - p.fee ? `A sua parte de ${fmt(p.amount - p.fee)} MT` : ''
            return (
              <article key={r.id} className={'pp-card' + (dl?.[1] === 'bad' ? ' late' : '')}>
                <div className="pp-top">
                  <Avatar text={initials(r.merchant_name)} color="#DCEFD6" size={40} radius={11} font={13} />
                  <div className="pp-who">
                    <div className="o-t1">{r.merchant_name}</div>
                    <div className="o-t2"><span className="o-mono">{p.reference}</span> · pedido {rel(p.requested_at)}</div>
                  </div>
                  {dl && <Badge tone={dl[1]} dot>{dl[0]}</Badge>}
                  {tab === 'paid' && <Badge tone="ok" dot>Pago</Badge>}
                </div>

                <div className="pp-amt">
                  <div className="o-stat-l">{tab === 'paid' ? 'Transferido' : 'A transferir'}</div>
                  <div className="o-stat-v"><Money c={r.send} /></div>
                  {note && <div className="o-t2">{note}</div>}
                </div>

                <div className="pp-bank">
                  <div className="pp-kv"><span>Banco</span><b>{p.bank_name}</b><i /></div>
                  <div className="pp-kv"><span>Titular</span><b>{p.holder}</b><i /></div>
                  <div className="pp-kv">
                    <span>NIB</span><b className="o-mono" style={{ fontSize: 12.5, letterSpacing: 0, wordBreak: 'break-all' }}>{p.nib}</b>
                    <button className="btn btn-outline btn-sm" style={{ width: 32, padding: 0, flex: 'none' }} onClick={() => copy(p.nib)} aria-label="Copiar NIB" title="Copiar NIB"><Icon name="copy" size={14} /></button>
                  </div>
                </div>

                <div className="pp-foot">
                  {tab === 'to_pay' && <button className="btn btn-green pp-main" onClick={() => setPaying(r)}><Icon name="check" size={17} stroke={2.4} />Marcar como pago</button>}
                  {tab === 'waiting' && <Badge tone="info" dot>A RIHA ainda está a rever</Badge>}
                  {tab === 'paid' && (
                    <>
                      <span className="o-t2">{r.paid_at ? <>Pago {rel(r.paid_at)} · <span className="o-mono">{r.reference}</span></> : null}</span>
                      {r.has_receipt && <a className="btn btn-outline btn-sm" href={g.partner.fundingReceiptUrl(r.id)}><Icon name="download" size={14} />Comprovativo</a>}
                    </>
                  )}
                </div>
              </article>
            )
          })}
        </div>
      )}
      {paying && <PayModal row={paying} pay={(ref, file) => g.partner.payFunding(paying.id, ref, file)} onClose={() => setPaying(null)} onDone={() => { setPaying(null); toast(`${paying.payout.reference} marcado como pago`); reload() }} />}
    </Page>
  )
}

const PM_CSS = `
.pm-t{font-family:var(--display);font-size:20px;font-weight:700;letter-spacing:-.02em;line-height:1.2}
.pm-sum{margin-top:14px;padding:14px 16px;border-radius:12px;background:var(--o-surface-2);border:1px solid var(--o-line-2)}
.pm-sum .o-stat-v{font-size:26px;margin-top:3px}
.pm-sum .o-t2{white-space:normal;margin-top:3px}
.pm-l{display:block;margin-top:14px}
.pm-drop{display:flex;align-items:center;gap:12px;margin-top:12px;padding:14px 16px;border-radius:12px;border:1.5px dashed var(--o-line);background:var(--o-surface-2);cursor:pointer;transition:border-color .12s,background .12s}
.pm-drop:hover{border-color:var(--o-muted)}
.pm-drop.on{border-style:solid;border-color:#B9DDB0;background:var(--o-brand-soft)}
.pm-drop-i{width:34px;height:34px;flex:none;border-radius:10px;display:grid;place-items:center;background:var(--o-surface);border:1px solid var(--o-line)}
.pm-drop.on .pm-drop-i{background:var(--o-brand);border-color:var(--o-brand);color:#fff}
.pm-drop-t{flex:1;min-width:0;font-size:13.5px}
.pm-drop-t b{display:block;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.pm-drop-t small{display:block;font-size:12px;color:var(--o-muted)}
.pm-f{display:flex;gap:10px;margin-top:18px}
.pm-f .btn{flex:1;height:42px}
`

export function PayModal({ row, pay, onClose, onDone }: { row: any; pay: (ref: string, file: File) => Promise<any>; onClose: () => void; onDone: () => void }) {
  const [ref, setRef] = useState('')
  const [file, setFile] = useState<File | null>(null)
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')
  const go = async () => {
    if (!file) return
    setBusy(true); setErr('')
    try { await pay(ref, file); onDone() } catch (e) { setErr(errMsg(e)) } finally { setBusy(false) }
  }
  return (
    <Modal onClose={onClose} width={480}>
      <style>{PM_CSS}</style>
      <div className="pm-t">Marcar {row.payout.reference} como pago</div>
      <div className="pm-sum">
        <div className="o-stat-l">Transferiu</div>
        <div className="o-stat-v"><Money c={row.send} /></div>
        <div className="o-t2">para {row.payout.holder}{row.payout.bank_name ? ` · ${row.payout.bank_name}` : ''}</div>
      </div>
      <label className="pm-l"><span className="label">Referência da transferência</span><input className="input" value={ref} onChange={e => setRef(e.target.value)} placeholder="Como aparece no comprovativo" /></label>
      <label className={'pm-drop' + (file ? ' on' : '')}>
        <span className="pm-drop-i"><Icon name={file ? 'check' : 'upload'} size={17} /></span>
        <span className="pm-drop-t">
          {file ? <><b>{file.name}</b><small>{Math.round(file.size / 1000)} KB</small></> : <><b>Juntar comprovativo do banco</b><small>PDF, JPG ou PNG</small></>}
        </span>
        <input type="file" accept=".pdf,.jpg,.jpeg,.png" hidden onChange={e => setFile(e.target.files?.[0] || null)} />
      </label>
      {err && <div className="form-err" style={{ marginTop: 12 }}>{err}</div>}
      <div className="pm-f">
        <button className="btn btn-outline" onClick={onClose}>Cancelar</button>
        <button className="btn btn-green" disabled={!file || !ref.trim() || busy} onClick={go}>{busy ? <Spinner /> : 'Marcar como pago'}</button>
      </div>
    </Modal>
  )
}
