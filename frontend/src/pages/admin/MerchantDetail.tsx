import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { errMsg } from '../../core/model'
import { useGateway } from '../../core/gateway'
import { useQuery } from '../../core/useQuery'
import { bpsLabel, fmt, fullDate, MERCHANT_STATUS, methodLine, PAYOUT_STATUS, relDate, STATUS } from '../../lib/format'
import { useSession } from '../../lib/session'
import { Avatar, Icon, Modal, Pill, Spinner } from '../../components/ui'
import { RiskPill } from './common'

export default function MerchantDetail() {
  const { id } = useParams()
  const nav = useNavigate()
  const { toast } = useSession()
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
  if (!m) return <div className="page"><Spinner dark /></div>
  const toBps = (v: string) => Math.round(parseFloat(v.replace(',', '.')) * 100)
  const saveFee = async (reset = false) => {
    const bps = toBps(fee)
    if (!reset && (!isFinite(bps) || bps < 0 || bps > 2000)) { toast('Taxa entre 0% e 20%', true); return }
    setBusy(true)
    try { const r = await g.admin.setFee(id!, reset ? null : bps); setData(r); toast(reset ? 'Taxa reposta para o valor por omissão' : `Taxa negociada: ${bpsLabel(bps)}`) }
    catch (e) { toast(errMsg(e), true) } finally { setBusy(false) }
  }
  const saveDefault = async () => {
    try { await g.admin.saveSettings(toBps(def)); toast('Taxa por omissão atualizada'); settings.reload(); reload() } catch (e) { toast(errMsg(e), true) }
  }
  const decide = async (decision: string) => {
    setBusy(true)
    try { await g.admin.decide(id!, decision, note); toast('Estado atualizado'); setConfirm(null); setNote(''); reload() } catch (e) { toast(errMsg(e), true) } finally { setBusy(false) }
  }
  const st = MERCHANT_STATUS[m.status]
  const KV = ({ k, v, mono }: { k: string; v: any; mono?: boolean }) => <div className="between" style={{ padding: '9px 0', borderTop: '1px solid var(--line-3)', fontSize: 13.5 }}><span className="muted">{k}</span><span className={mono ? 'mono' : ''} style={{ fontWeight: 500, textAlign: 'right' }}>{v || '—'}</span></div>
  return (
    <div className="page">
      <button className="btn btn-ghost" style={{ padding: 0, height: 32 }} onClick={() => nav('/admin/comerciantes')}><Icon name="arrowLeft" size={16} stroke={2.2} />Comerciantes</button>
      <div className="row" style={{ gap: 16, marginTop: 10, flexWrap: 'wrap' }}>
        <Avatar text={m.initials} color={m.avatar_color} size={56} radius={16} font={18} />
        <div style={{ flex: 1, minWidth: 220 }}><div className="display" style={{ fontSize: 28 }}>{m.name}</div><div style={{ fontSize: 14, color: 'var(--muted)', marginTop: 2 }}>{m.legal_name || '—'} · {m.email}</div></div>
        <Pill {...st} style={{ fontSize: 12.5, padding: '5px 11px' }} /><RiskPill risk={m.risk} />
        {m.status === 'suspended' ? <button className="btn btn-green" onClick={() => setConfirm('reactivate')}>Reativar</button> : <button className="btn btn-danger" onClick={() => setConfirm('suspend')}>Suspender</button>}
        {(m.status === 'in_review' || m.status === 'info_requested') && <button className="btn btn-outline" onClick={() => nav('/admin/aprovacoes?id=' + m.id)}>Rever verificação</button>}
      </div>
      <div className="grid-4" style={{ marginTop: 22 }}>
        <div className="dark-card" style={{ borderRadius: 20, padding: 18 }}><div style={{ fontSize: 13, color: '#A9B5AE' }}>Saldo (produção)</div><div className="display" style={{ fontSize: 26, marginTop: 4 }}>{fmt(m.balance)} MT</div></div>
        <div className="card" style={{ padding: 18, borderRadius: 20 }}><div style={{ fontSize: 13, color: 'var(--muted)' }}>A caminho do banco</div><div className="display" style={{ fontSize: 26, marginTop: 4 }}>{fmt(m.in_transit)} MT</div></div>
        <div className="card" style={{ padding: 18, borderRadius: 20 }}><div style={{ fontSize: 13, color: 'var(--muted)' }}>Volume 30 dias</div><div className="display" style={{ fontSize: 26, marginTop: 4 }}>{fmt(m.volume_30d, false)} MT</div></div>
        <div className="card" style={{ padding: 18, borderRadius: 20 }}><div style={{ fontSize: 13, color: 'var(--muted)' }}>Taxa aplicada</div><div className="display" style={{ fontSize: 26, marginTop: 4 }}>{bpsLabel(m.fee_bps)}</div><div style={{ fontSize: 12.5, color: 'var(--muted)' }}>{m.fee_negotiated ? 'Negociada' : 'Por omissão da plataforma'}</div></div>
      </div>
      <div className="grid-2" style={{ marginTop: 16, alignItems: 'start' }}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          <div className="card" style={{ padding: 22 }}>
            <div style={{ fontWeight: 600, fontSize: 16 }}>Taxa negociada</div>
            <div style={{ fontSize: 13, color: 'var(--muted)', marginTop: 4, lineHeight: 1.5 }}>Aplicada a novos pagamentos deste comerciante. Pagamentos já feitos mantêm a taxa da altura. Por omissão: <b>{bpsLabel(m.default_fee_bps)}</b>.</div>
            <div className="row" style={{ marginTop: 14, gap: 8, flexWrap: 'wrap' }}>
              <div style={{ position: 'relative', width: 140 }}><input className="input" inputMode="decimal" value={fee} onChange={e => setFee(e.target.value.replace(/[^\d.,]/g, ''))} style={{ paddingRight: 30, fontWeight: 600 }} aria-label="Taxa em percentagem" /><span style={{ position: 'absolute', right: 14, top: 14, color: 'var(--muted)' }}>%</span></div>
              <button className="btn btn-green" onClick={() => saveFee(false)} disabled={busy}>Guardar taxa</button>
              {m.fee_negotiated && <button className="btn btn-outline" onClick={() => saveFee(true)} disabled={busy}>Repor {bpsLabel(m.default_fee_bps)}</button>}
            </div>
            <div style={{ fontSize: 12.5, color: 'var(--muted)', marginTop: 10 }}>Exemplo: num pagamento de 1 250,00 MT a taxa é {fmt(Math.round(125000 * (toBps(fee) || 0) / 10000))} MT.</div>
          </div>
          <div className="card" style={{ padding: 22 }}>
            <div style={{ fontWeight: 600, fontSize: 16 }}>Ficha</div>
            <div style={{ marginTop: 10 }}>
              <KV k="NUIT" v={m.nuit} mono /><KV k="Tipo" v={m.business_type === 'company' ? 'Empresa' : m.business_type ? 'Em nome individual' : ''} /><KV k="Cidade" v={m.city} /><KV k="Telemóvel" v={m.phone} /><KV k="Responsável" v={m.owner_name} /><KV k="Nome no extrato" v={m.statement_descriptor} mono />
              <KV k="Conta bancária" v={m.bank_account ? `${m.bank_account.bank_name} · •••• ${m.bank_account.nib_last5} · ${m.bank_account.holder}` : ''} /><KV k="Criada" v={fullDate(m.created_at)} />
              {m.review_note && <KV k="Nota interna" v={m.review_note} />}
            </div>
          </div>
          <div className="card" style={{ padding: 22 }}>
            <div style={{ fontWeight: 600, fontSize: 16 }}>Taxa por omissão da plataforma</div>
            <div style={{ fontSize: 13, color: 'var(--muted)', marginTop: 4 }}>Aplica-se a todos os comerciantes sem taxa negociada.</div>
            <div className="row" style={{ marginTop: 12, gap: 8 }}><div style={{ position: 'relative', width: 140 }}><input className="input" value={def} onChange={e => setDef(e.target.value.replace(/[^\d.,]/g, ''))} style={{ paddingRight: 30, fontWeight: 600 }} aria-label="Taxa por omissão" /><span style={{ position: 'absolute', right: 14, top: 14, color: 'var(--muted)' }}>%</span></div><button className="btn btn-outline" onClick={saveDefault}>Atualizar</button></div>
          </div>
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          <div className="card" style={{ padding: '8px 0' }}>
            <div className="h2" style={{ padding: '14px 22px 8px' }}>Pagamentos recentes</div>
            {m.recent_payments.map((p: any) => <div key={p.id} className="row" style={{ padding: '10px 22px', borderTop: '1px solid var(--line-3)' }}><div style={{ flex: 1, minWidth: 0 }}><div style={{ fontWeight: 600, fontSize: 13.5 }}>{p.customer_name || p.reference}</div><div style={{ fontSize: 12, color: 'var(--muted)' }}>{methodLine(p)} · {relDate(p.created_at)}</div></div><Pill {...(STATUS[p.status] || STATUS.processing)} /><span className="tnum" style={{ width: 110, textAlign: 'right', fontWeight: 600, fontSize: 13.5 }}>{fmt(p.amount)} MT</span></div>)}
            {!m.recent_payments.length && <div className="empty">Sem pagamentos em produção.</div>}
          </div>
          <div className="card" style={{ padding: '8px 0' }}>
            <div className="h2" style={{ padding: '14px 22px 8px' }}>Levantamentos</div>
            {m.payouts.map((p: any) => <div key={p.id} className="row" style={{ padding: '10px 22px', borderTop: '1px solid var(--line-3)' }}><span className="mono" style={{ fontSize: 12.5, width: 70 }}>{p.reference}</span><span style={{ flex: 1, fontSize: 12.5, color: 'var(--muted)' }}>{relDate(p.requested_at_iso)}</span><Pill {...PAYOUT_STATUS[p.status]} /><span className="tnum" style={{ width: 110, textAlign: 'right', fontWeight: 600, fontSize: 13.5 }}>{fmt(p.amount)} MT</span></div>)}
            {!m.payouts.length && <div className="empty">Sem levantamentos.</div>}
          </div>
          <div className="card" style={{ padding: '8px 0' }}>
            <div className="h2" style={{ padding: '14px 22px 8px' }}>Registo de auditoria</div>
            {m.audit.map((a: any, i: number) => <div key={i} style={{ padding: '10px 22px', borderTop: '1px solid var(--line-3)', fontSize: 13 }}><div className="between"><span className="mono" style={{ fontSize: 12.5 }}>{a.action}</span><span className="muted" style={{ fontSize: 12 }}>{relDate(a.created_at)}</span></div>{a.details && <div className="muted" style={{ fontSize: 12.5, marginTop: 2 }}>{a.details} · {a.actor_role}</div>}</div>)}
            {!m.audit.length && <div className="empty">Sem registos.</div>}
          </div>
        </div>
      </div>
      {confirm && (
        <Modal onClose={() => setConfirm(null)}>
          <div className="display" style={{ fontSize: 22 }}>{confirm === 'suspend' ? `Suspender ${m.name}?` : `Reativar ${m.name}?`}</div>
          <div style={{ fontSize: 14, color: 'var(--muted)', marginTop: 8, lineHeight: 1.5 }}>{confirm === 'suspend' ? 'O comerciante deixa de aceitar pagamentos e de pedir levantamentos até ser reativado.' : 'O comerciante volta a aceitar pagamentos.'}</div>
          <label style={{ display: 'block', marginTop: 14 }}><span className="label">Nota interna</span><input className="input" value={note} onChange={e => setNote(e.target.value)} /></label>
          <div style={{ display: 'flex', gap: 10, marginTop: 18 }}><button className="btn btn-outline" style={{ flex: 1 }} onClick={() => setConfirm(null)}>Cancelar</button><button className={'btn ' + (confirm === 'suspend' ? 'btn-ink' : 'btn-green')} style={{ flex: 1 }} disabled={busy} onClick={() => decide(confirm)}>{busy ? <Spinner /> : 'Confirmar'}</button></div>
        </Modal>
      )}
    </div>
  )
}
