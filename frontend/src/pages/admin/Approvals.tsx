import { useEffect, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { api, errMsg } from '../../lib/api'
import { useData } from '../../lib/useData'
import { useSession } from '../../lib/session'
import { Avatar, Icon, Pill, Spinner } from '../../components/ui'
import { fileSize } from '../public/Kyc'
import { RISK, submittedAgo } from './common'

const DOC_LABEL: Record<string, string> = { id_front: 'BI · frente', id_back: 'BI · verso', registration: 'Certidão comercial', nib_proof: 'Comprovativo NIB' }
const CK: Record<string, [string, string, string]> = { ok: ['✓', '#E2F3DE', '#0A6B05'], warn: ['!', '#FFF3C4', '#7A5A00'], bad: ['×', '#FFE3E1', '#C2080D'] }

export default function Approvals() {
  const { toast } = useSession()
  const [sp, setSp] = useSearchParams()
  const { data, reload } = useData<any>('/admin/approvals')
  const [note, setNote] = useState('')
  const [busy, setBusy] = useState('')
  const list: any[] = data?.data || []
  const selId = sp.get('id') || list[0]?.id
  const app = list.find(a => a.id === selId) || list[0]
  useEffect(() => setNote(''), [selId])

  const decide = async (decision: string, msg: string) => {
    if (!app) return
    if (decision === 'reject' && !note.trim()) { toast('Escreva o motivo na nota interna antes de rejeitar.', true); return }
    setBusy(decision)
    try {
      await api(`/admin/merchants/${app.id}/decision`, { body: { decision, note } })
      toast(`${msg}: ${app.name}`)
      const next = list.find(a => a.id !== app.id && a.status === 'in_review')
      setSp(next ? { id: next.id } : {})
      reload()
    } catch (e) { toast(errMsg(e), true) } finally { setBusy('') }
  }

  return (
    <div className="appr">
      <style>{`.appr{display:flex;height:100%}.appr-list{width:380px;flex:none;border-right:1px solid var(--line);display:flex;flex-direction:column;height:100%}.appr-main{flex:1;min-width:0;display:flex;flex-direction:column;height:100%}
      @media(max-width:860px){.appr{flex-direction:column;height:auto}.appr-list{width:100%;height:auto;border-right:0}.appr-list .scroll{max-height:none;flex-direction:row;overflow-x:auto}.appr-list .scroll>button{flex:none;width:260px}.appr-main{height:auto}.appr-foot{flex-wrap:wrap;position:sticky;bottom:0}.appr-foot input{flex-basis:100%!important}}`}</style>
      <div className="appr-list">
        <div style={{ padding: '24px 22px 14px' }}><h1 className="h1" style={{ fontSize: 28 }}>Aprovações</h1><div style={{ fontSize: 13.5, color: 'var(--muted)', marginTop: 4 }}>{data?.pending ?? '…'} contas à espera de verificação</div></div>
        <div className="scroll" style={{ flex: 1, overflow: 'auto', padding: '0 12px 16px', display: 'flex', flexDirection: 'column', gap: 4 }}>
          {list.map(a => {
            const on = a.id === app?.id
            const pill = a.status === 'info_requested' ? ['Docs pedidos', '#EDECE6', '#3E4742'] : ['Risco ' + a.risk.toLowerCase(), ...RISK[a.risk]]
            return (
              <button key={a.id} onClick={() => setSp({ id: a.id })} className="row" style={{ padding: 12, borderRadius: 14, border: `1px solid ${on ? '#E2E0D8' : 'transparent'}`, background: on ? '#fff' : 'transparent', textAlign: 'left' }}>
                <Avatar text={a.initials} color={a.avatar_color} size={40} radius={12} font={13.5} />
                <div style={{ flex: 1, minWidth: 0 }}><div style={{ fontWeight: 600, fontSize: 14.5, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{a.name}</div><div style={{ fontSize: 12.5, color: 'var(--muted)', marginTop: 1 }}>{a.city || '—'} · {submittedAgo(a.submitted_at)}</div></div>
                <Pill t={pill[0]} bg={pill[1]} fg={pill[2]} style={{ fontSize: 11 }} />
              </button>
            )
          })}
          {data && !list.length && <div className="empty">Fila vazia. Bom trabalho.</div>}
        </div>
      </div>
      {app && (
        <div className="appr-main">
          <div style={{ flex: 1, overflow: 'auto', padding: '24px 32px' }}>
            <div className="row" style={{ gap: 16, flexWrap: 'wrap' }}>
              <Avatar text={app.initials} color={app.avatar_color} size={56} radius={16} font={18} />
              <div style={{ flex: 1, minWidth: 200 }}><div className="display" style={{ fontSize: 26, letterSpacing: '-0.025em' }}>{app.name}</div><div style={{ fontSize: 14, color: 'var(--muted)', marginTop: 2 }}>{app.owner_name} · Submetido {submittedAgo(app.submitted_at)}</div></div>
              <Pill t={app.status === 'info_requested' ? 'Docs pedidos' : 'Por rever'} bg={app.status === 'info_requested' ? '#EDECE6' : '#FFF3C4'} fg={app.status === 'info_requested' ? '#3E4742' : '#7A5A00'} style={{ fontSize: 12.5, padding: '5px 11px' }} />
            </div>
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit,minmax(180px,1fr))', gap: 1, marginTop: 22, background: 'var(--line)', border: '1px solid var(--line)', borderRadius: 18, overflow: 'hidden' }}>
              {[['Tipo', app.business_type === 'company' ? 'Empresa' : 'Em nome individual'], ['NUIT', app.nuit?.replace(/(\d{3})(\d{3})(\d{3})/, '$1 $2 $3'), true], ['Cidade', app.city], ['Telemóvel', app.phone], ['Email', app.email], ['Volume esperado', app.expected_volume]].map(([k, v, m]) => (
                <div key={k as string} style={{ background: '#fff', padding: '14px 16px' }}><div style={{ fontSize: 12, color: 'var(--muted)' }}>{k}</div><div className={m ? 'mono' : ''} style={{ fontWeight: m ? 500 : 600, fontSize: 14, marginTop: 3, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{v || '—'}</div></div>
              ))}
            </div>
            <div className="eyebrow" style={{ margin: '24px 0 10px' }}>Verificações automáticas</div>
            <div className="card" style={{ borderRadius: 18, padding: '4px 0' }}>
              {app.checks.map((c: any, i: number) => (
                <div key={i} style={{ display: 'flex', alignItems: 'flex-start', gap: 12, padding: '12px 16px' }}>
                  <div style={{ width: 24, height: 24, flex: 'none', borderRadius: '50%', background: CK[c.kind][1], color: CK[c.kind][2], display: 'grid', placeItems: 'center', fontWeight: 800, fontSize: 13 }}>{CK[c.kind][0]}</div>
                  <div><div style={{ fontWeight: 600, fontSize: 14 }}>{c.label}</div><div style={{ fontSize: 13, color: 'var(--muted)', marginTop: 2 }}>{c.note}</div></div>
                </div>
              ))}
            </div>
            <div className="eyebrow" style={{ margin: '24px 0 10px' }}>Documentos</div>
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill,minmax(160px,1fr))', gap: 12 }}>
              {app.documents.map((d: any) => (
                <a key={d.id} href={`/api/admin/documents/${d.id}`} target="_blank" rel="noreferrer" className="card" style={{ borderRadius: 16, overflow: 'hidden', color: 'var(--ink)' }}>
                  <div className="mono" style={{ height: 110, background: 'repeating-linear-gradient(135deg,#F0EEE8 0 10px,#F7F6F2 10px 20px)', display: 'grid', placeItems: 'center', fontSize: 11.5, color: 'var(--muted-2)', padding: 8, textAlign: 'center', wordBreak: 'break-all' }}>{d.file_name}<br />{fileSize(d.size)}</div>
                  <div className="between" style={{ padding: '10px 12px' }}><span style={{ fontWeight: 600, fontSize: 13 }}>{DOC_LABEL[d.kind] || d.kind}</span><span style={{ fontSize: 12, fontWeight: 600, color: 'var(--green-text)' }}>Abrir</span></div>
                </a>
              ))}
              {!app.documents.length && <div style={{ fontSize: 13.5, color: 'var(--muted)' }}>Sem documentos.</div>}
            </div>
            <div className="eyebrow" style={{ margin: '24px 0 10px' }}>Conta para levantamentos</div>
            <div className="card row" style={{ padding: '14px 16px', borderRadius: 16, flexWrap: 'wrap' }}>
              <Icon name="bank" size={20} />
              {app.bank_account ? <><span style={{ fontWeight: 600, fontSize: 14 }}>{app.bank_account.bank_name} · NIB •••• {app.bank_account.nib_last5}</span><span style={{ fontSize: 13, color: 'var(--muted)' }}>Titular: {app.bank_account.holder}</span></> : <span className="muted">Por indicar</span>}
            </div>
          </div>
          <div className="appr-foot row" style={{ flex: 'none', gap: 10, padding: '16px 32px', borderTop: '1px solid var(--line)', background: '#fff' }}>
            <input className="input" style={{ flex: 1, height: 44 }} placeholder="Nota interna (obrigatória para rejeitar ou pedir documentos)" value={note} onChange={e => setNote(e.target.value)} />
            <button className="btn btn-danger" onClick={() => decide('reject', 'Conta rejeitada')} disabled={!!busy}>{busy === 'reject' ? <Spinner dark /> : 'Rejeitar'}</button>
            <button className="btn btn-outline" onClick={() => decide('info', 'Pedido de documentos enviado')} disabled={!!busy}>{busy === 'info' ? <Spinner dark /> : 'Pedir documentos'}</button>
            <button className="btn btn-green" onClick={() => decide('approve', 'Conta aprovada')} disabled={!!busy}>{busy === 'approve' ? <Spinner /> : <><Icon name="check" size={16} stroke={2.6} />Aprovar conta</>}</button>
          </div>
        </div>
      )}
    </div>
  )
}
