import { useEffect, useState, type ReactNode } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { can, errMsg } from '../../core/model'
import { useGateway } from '../../core/gateway'
import { useQuery } from '../../core/useQuery'
import { bpsLabel, fmt, fullDate, MERCHANT_STATUS, methodLine, parseMT, PAYOUT_STATUS, relDate, STATUS } from '../../lib/format'
import { useSession } from '../../lib/session'
import { Avatar, Modal, Skeleton, Spinner } from '../../components/ui'
import { Amount, Badge, Empty, KV, List, Page, PageHead, Panel, Section, Stat, Tip, type Tone } from '../../components/ops'
import { RiskPill, V1Tag } from './common'
import { MerchantRouting, Suffixed } from './Routing'

const MERCHANT_TONE: Record<string, Tone> = { lead: undefined, in_review: 'warn', info_requested: 'info', active: 'ok', rejected: 'bad', suspended: 'bad' }
const PAY_TONE: Record<string, Tone> = { succeeded: 'ok', processing: 'warn', unconfirmed: 'warn', failed: 'bad', refunded: undefined, partially_refunded: undefined }
const PAYOUT_TONE: Record<string, Tone> = { new: undefined, review: 'warn', sent: 'ink', paid: 'ok', rejected: 'bad', failed: 'bad' }

const CSS = `
.md-id{display:inline-flex;align-items:center;gap:12px;min-width:0;max-width:100%}
.md-id>span{min-width:0;overflow-wrap:anywhere}
.md-sub{display:flex;align-items:center;gap:6px;flex-wrap:wrap}
.md-sub>span:last-child{margin-left:2px;overflow-wrap:anywhere}
.md-cols{align-items:start;margin-top:14px}
.md-main>.o-section:first-child{margin-top:4px}
.md-side{display:flex;flex-direction:column;gap:14px;min-width:0}
.md-h{display:inline-flex;align-items:center;gap:6px}
.md-form{display:flex;align-items:flex-end;gap:8px;flex-wrap:wrap}
.md-form label{display:flex;flex-direction:column;gap:5px;min-width:0}
.md-blk+.md-blk{margin-top:16px;padding-top:16px;border-top:1px dashed var(--o-line-2)}
.md-blk-t{display:flex;align-items:center;gap:6px;font-size:13px;font-weight:650;margin-bottom:10px}
.md-beta{display:flex;align-items:center;justify-content:space-between;gap:12px}
.md-m h3{margin:0;font-family:var(--display);font-size:20px;font-weight:700;letter-spacing:-.02em}
.md-m p{margin:6px 0 0;font-size:13.5px;line-height:1.5;color:var(--o-muted)}
.md-m-do{display:flex;gap:8px;margin-top:18px}
.md-m-do .btn{flex:1}
@media(max-width:860px){
  .md-id .md-av{display:none}
  .md-beta{flex-wrap:wrap}
  .md-beta .btn{flex:1 1 100%}
}`

export default function MerchantDetail() {
  const { id } = useParams()
  const nav = useNavigate()
  const { toast, me } = useSession()
  const pricing = can(me, 'pricing'), manage = can(me, 'merchants')
  const g = useGateway()
  const { data: m, setData, reload } = useQuery(g => g.admin.merchant(id!), [id])
  const [fee, setFee] = useState('')
  const [busy, setBusy] = useState(false)
  const [confirm, setConfirm] = useState<string | null>(null)
  const [note, setNote] = useState('')
  const settings = useQuery(g => g.admin.settings())
  const [def, setDef] = useState('')
  useEffect(() => { if (m) setFee((m.fee_bps / 100).toString().replace('.', ',')) }, [m?.fee_bps]) // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => { if (settings.data) setDef((settings.data.default_fee_bps / 100).toString().replace('.', ',')) }, [settings.data])
  const [tfee, setTfee] = useState('')
  const [tfrom, setTfrom] = useState('')
  useEffect(() => { if (settings.data?.transfer_fee !== undefined) { setTfee(String(settings.data.transfer_fee / 100)); setTfrom(String(settings.data.free_transfer_from / 100)) } }, [settings.data])
  if (!m) return <Page><Skeleton h={30} w={260} /><Skeleton h={96} style={{ marginTop: 20 }} /><Skeleton h={320} style={{ marginTop: 14 }} /></Page>
  const toBps = (v: string) => Math.round(parseFloat(v.replace(',', '.')) * 100)
  const saveFee = async (reset = false) => {
    const bps = toBps(fee)
    if (!reset && (!isFinite(bps) || bps < 0 || bps > 2000)) { toast('Taxa entre 0% e 20%', true); return }
    setBusy(true)
    try { const r = await g.admin.setFee(id!, reset ? null : bps); setData(r); toast(reset ? 'Taxa reposta para o valor por omissão' : `Taxa negociada: ${bpsLabel(bps)}`) }
    catch (e) { toast(errMsg(e), true) } finally { setBusy(false) }
  }
  const saveBeta = async (beta: boolean) => {
    setBusy(true)
    try { const r = await g.admin.setBeta(id!, beta); setData(r); toast(beta ? 'No programa beta: valores a partir de 1 MT' : 'Fora do programa beta') }
    catch (e) { toast(errMsg(e), true) } finally { setBusy(false) }
  }
  const saveTransfer = async () => {
    const fee = parseMT(tfee), from = parseMT(tfrom)
    try { await g.admin.saveTransferFee(fee, from); toast('Taxa de transferência atualizada'); settings.reload() } catch (e) { toast(errMsg(e), true) }
  }
  const saveDefault = async () => {
    try { await g.admin.saveSettings(toBps(def)); toast('Taxa por omissão atualizada'); settings.reload(); reload() } catch (e) { toast(errMsg(e), true) }
  }
  const decide = async (decision: string) => {
    setBusy(true)
    try { await g.admin.decide(id!, decision, note); toast('Estado atualizado'); setConfirm(null); setNote(''); reload() } catch (e) { toast(errMsg(e), true) } finally { setBusy(false) }
  }
  const st = MERCHANT_STATUS[m.status]
  const or = (v: ReactNode) => v || '—'
  const mono = (v: string | null | undefined) => (v ? <span className="o-mono">{v}</span> : '—')
  const payTag = (p: any) => <Badge tone={PAY_TONE[p.status] ?? 'warn'}>{(STATUS[p.status] || STATUS.processing).t}</Badge>
  const payoutTag = (p: any) => <Badge tone={PAYOUT_TONE[p.status]}>{PAYOUT_STATUS[p.status]?.t || p.status}</Badge>
  const audit = m.audit.map((a: any, i: number) => ({ ...a, _k: String(i) }))

  return (
    <Page>
      <style>{CSS}</style>
      <PageHead back={{ label: 'Comerciantes', onClick: () => nav('/admin/comerciantes') }}
        title={<span className="md-id"><span className="md-av"><Avatar text={m.initials} color={m.avatar_color} size={40} radius={12} font={14} /></span><span>{m.name}{m.imported_from === 'v1' && <V1Tag big />}</span></span>}
        sub={<span className="md-sub"><Badge tone={MERCHANT_TONE[m.status]} dot>{st?.t || m.status}</Badge><RiskPill risk={m.risk} /><span>{m.legal_name || '—'} · {m.email}</span></span>}
        actions={<>
          {(m.status === 'in_review' || m.status === 'info_requested') && <button className="btn btn-outline" onClick={() => nav('/admin/aprovacoes?id=' + m.id)}>Rever verificação</button>}
          {!manage ? null : m.status === 'suspended' ? <button className="btn btn-green" onClick={() => setConfirm('reactivate')}>Reativar</button> : <button className="btn btn-danger" onClick={() => setConfirm('suspend')}>Suspender</button>}
        </>} />

      <div className="o-stats">
        <Stat tone="ink" label="Saldo em produção" value={m.balance} cents />
        <Stat label="A caminho do banco" value={m.in_transit} cents />
        <Stat label="Volume 30 dias" value={m.volume_30d} />
        <Stat label="Taxa aplicada" value={bpsLabel(m.fee_bps)} hint={m.fee_negotiated ? <Badge tone="warn">Negociada</Badge> : 'Por omissão da plataforma'} />
      </div>

      <div className="o-grid main-side md-cols">
        <div className="md-main" style={{ minWidth: 0 }}>
          <Section title="Pagamentos recentes" />
          <List rows={m.recent_payments} rowKey={(p: any) => p.id}
            empty={<Empty icon="card" title="Sem pagamentos" text="Nada em produção ainda." />}
            cols={[
              { head: 'Cliente', w: 'minmax(0,1fr)', cell: (p: any) => <><div className="o-t1">{p.customer_name || p.reference}</div><div className="o-t2">{methodLine(p)} · {relDate(p.created_at)}</div></> },
              { head: 'Estado', w: '120px', cell: payTag },
              { head: 'Valor', w: '120px', right: true, cell: (p: any) => <Amount c={p.amount} /> },
            ]}
            card={(p: any) => ({ title: p.customer_name || p.reference, sub: `${methodLine(p)} · ${relDate(p.created_at)}`, amount: <Amount c={p.amount} />, meta: payTag(p) })} />

          <Section title="Levantamentos" />
          <List rows={m.payouts} rowKey={(p: any) => p.id}
            empty={<Empty icon="bank" title="Sem levantamentos" />}
            cols={[
              { head: 'Referência', w: '96px', cell: (p: any) => <span className="o-mono">{p.reference}</span> },
              { head: 'Pedido', w: 'minmax(0,1fr)', cell: (p: any) => <span className="o-t2">{relDate(p.requested_at_iso)}</span> },
              { head: 'Estado', w: '140px', cell: payoutTag },
              { head: 'Valor', w: '120px', right: true, cell: (p: any) => <Amount c={p.amount} /> },
            ]}
            card={(p: any) => ({ title: <span className="o-mono">{p.reference}</span>, sub: relDate(p.requested_at_iso), amount: <Amount c={p.amount} />, meta: payoutTag(p) })} />

          <Section title="Registo de auditoria" />
          <List rows={audit} rowKey={(a: any) => a._k}
            empty={<Empty icon="clock" title="Sem registos" />}
            cols={[
              { head: 'Ação', w: 'minmax(0,1fr)', cell: (a: any) => <><div className="o-mono o-t1">{a.action}</div>{a.details && <div className="o-t2" style={{ whiteSpace: 'normal' }}>{a.details} · {a.actor_role}</div>}</> },
              { head: 'Quando', w: '120px', right: true, cell: (a: any) => <span className="o-t2">{relDate(a.created_at)}</span> },
            ]}
            card={(a: any) => ({ title: <span className="o-mono">{a.action}</span>, sub: a.details ? `${a.details} · ${a.actor_role}` : undefined, meta: <span className="o-t2">{relDate(a.created_at)}</span> })} />
        </div>

        <div className="md-side">
          <Panel title="Ficha">
            <KV k="NUIT" v={mono(m.nuit)} />
            <KV k="Tipo" v={or(m.business_type === 'company' ? 'Empresa' : m.business_type ? 'Em nome individual' : '')} />
            <KV k="Cidade" v={or(m.city)} />
            <KV k="Telemóvel" v={or(m.phone)} />
            <KV k="Responsável" v={or(m.owner_name)} />
            <KV k="Nome no extrato" v={mono(m.statement_descriptor)} />
            <KV k="Conta bancária" v={m.bank_account ? `${m.bank_account.bank_name} · •••• ${m.bank_account.nib_last5} · ${m.bank_account.holder}` : '—'} />
            <KV k="Criada" v={fullDate(m.created_at)} />
            {m.review_note && <KV k="Nota interna" v={m.review_note} />}
          </Panel>

          {pricing && (
            <Panel title={<span className="md-h">Taxa negociada<Tip text="Vale para novos pagamentos deste comerciante. Os já feitos mantêm a taxa da altura." /></span>} sub={`Por omissão ${bpsLabel(m.default_fee_bps)}`}>
              <div className="md-form">
                <Suffixed suffix="%" width={110} inputMode="decimal" value={fee} onChange={e => setFee(e.target.value.replace(/[^\d.,]/g, ''))} aria-label="Taxa em percentagem" />
                <button className="btn btn-green" onClick={() => saveFee(false)} disabled={busy}>Guardar taxa</button>
                {m.fee_negotiated && <button className="btn btn-outline" onClick={() => saveFee(true)} disabled={busy}>Repor {bpsLabel(m.default_fee_bps)}</button>}
              </div>
              <div className="o-t2" style={{ marginTop: 10, whiteSpace: 'normal' }}>Em 1 250,00 MT, a taxa é {fmt(Math.round(125000 * (toBps(fee) || 0) / 10000))} MT.</div>
            </Panel>
          )}

          {pricing && <MerchantRouting merchantId={id!} />}

          {manage && (
            <Panel title={<span className="md-h">Programa beta<Tip text="No beta, tudo a partir de 1 MT, para testar com dinheiro real. Fora dele: 10 MT por pagamento e 500 MT por plano de assinatura." /></span>}
              action={m.beta ? <Badge tone="info" dot>No beta</Badge> : undefined}>
              <div className="md-beta">
                <span className="o-t2" style={{ whiteSpace: 'normal', marginTop: 0 }}>{m.beta ? 'Cobra a partir de 1 MT, também em assinaturas.' : 'Mínimos normais: 10 MT e 500 MT por plano.'}</span>
                <button className={'btn ' + (m.beta ? 'btn-outline' : 'btn-green')} style={{ flex: 'none' }} onClick={() => saveBeta(!m.beta)} disabled={busy}>{m.beta ? 'Tirar do beta' : 'Pôr no beta'}</button>
              </div>
            </Panel>
          )}

          {pricing && (
            <Panel title="Taxas da plataforma" sub="Para todos os comerciantes">
              <div className="md-blk">
                <div className="md-blk-t">Taxa por omissão<Tip text="Aplica-se a todos os comerciantes sem taxa negociada." /></div>
                <div className="md-form">
                  <Suffixed suffix="%" width={110} value={def} onChange={e => setDef(e.target.value.replace(/[^\d.,]/g, ''))} aria-label="Taxa por omissão" />
                  <button className="btn btn-outline" onClick={saveDefault}>Atualizar</button>
                </div>
              </div>
              <div className="md-blk">
                <div className="md-blk-t">Levantamentos para outros bancos<Tip text="O dinheiro da RIHA está no Millennium bim. Cada transferência para outro banco custa-nos uma taxa; abaixo do limite, o comerciante paga-a. Contas bim nunca pagam." /></div>
                <div className="md-form">
                  <label style={{ width: 100 }}><span className="label">Taxa</span><Suffixed suffix="MT" inputMode="decimal" value={tfee} onChange={e => setTfee(e.target.value.replace(/[^\d.,\s]/g, ''))} /></label>
                  <label style={{ width: 132 }}><span className="label">Grátis a partir de</span><Suffixed suffix="MT" inputMode="decimal" value={tfrom} onChange={e => setTfrom(e.target.value.replace(/[^\d.,\s]/g, ''))} /></label>
                  <button className="btn btn-outline" onClick={saveTransfer}>Atualizar</button>
                </div>
              </div>
            </Panel>
          )}
        </div>
      </div>

      {confirm && (
        <Modal onClose={() => setConfirm(null)}>
          <div className="md-m">
            <h3>{confirm === 'suspend' ? `Suspender ${m.name}?` : `Reativar ${m.name}?`}</h3>
            <p>{confirm === 'suspend' ? 'Deixa de aceitar pagamentos e de pedir levantamentos até ser reativado.' : 'Volta a aceitar pagamentos.'}</p>
            <label style={{ display: 'block', marginTop: 14 }}><span className="label">Nota interna</span><input className="input" value={note} onChange={e => setNote(e.target.value)} /></label>
            <div className="md-m-do"><button className="btn btn-outline" onClick={() => setConfirm(null)}>Cancelar</button><button className={'btn ' + (confirm === 'suspend' ? 'btn-ink' : 'btn-green')} disabled={busy} onClick={() => decide(confirm)}>{busy ? <Spinner /> : 'Confirmar'}</button></div>
          </div>
        </Modal>
      )}
    </Page>
  )
}
