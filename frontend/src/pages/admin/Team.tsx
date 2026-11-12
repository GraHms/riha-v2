import { useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { errMsg } from '../../core/model'
import { useGateway } from '../../core/gateway'
import { useQuery } from '../../core/useQuery'
import { initials } from '../../lib/format'
import { useSession } from '../../lib/session'
import { Avatar, Icon, Modal, Skeleton, Spinner } from '../../components/ui'
import { Page, PageHead, Tip } from '../../components/ops'
import { TeamList } from './TeamList'

// Who works on RIHA and what each sees: RIHA's staff, and the people at each
// partner, who see only their own partner's account.

const CSS = `
.tm-tabs{margin-bottom:16px}
.tm-tabs .count{background:var(--o-line-2);color:var(--o-ink-2);font-weight:650}
.tm-tabs button.on .count{background:var(--o-ink);color:#fff}
.tm-partners{display:flex;flex-direction:column;gap:30px}
.tm-ph{display:flex;align-items:center;gap:12px;margin-bottom:12px;min-width:0}
.tm-ph-main{min-width:0;flex:1}
.tm-pn{display:flex;align-items:baseline;gap:8px;flex-wrap:wrap;font-size:15.5px;font-weight:700;letter-spacing:-.01em}
.tm-pn .o-mono{color:var(--o-muted);font-weight:500}
.tm-ints{display:flex;flex-wrap:wrap;align-items:center;gap:6px;margin-top:5px;font-size:12.5px;color:var(--o-muted)}
.tm-ints .o-chip{max-width:100%;overflow:hidden;text-overflow:ellipsis}
.tm-mt{font-family:var(--display);font-size:20px;font-weight:700;letter-spacing:-.02em;line-height:1.2}
.tm-ms{display:flex;align-items:center;gap:6px;font-size:13px;color:var(--o-muted);margin-top:4px}
.tm-ml{display:block;margin-top:12px}
.tm-mf{display:flex;gap:10px;margin-top:18px}
.tm-mf .btn{flex:1;height:42px}
`

export default function AdminTeam() {
  const g = useGateway()
  const { me } = useSession()
  const [sp, setSp] = useSearchParams()
  const tab = sp.get('tab') === 'parceiros' ? 'parceiros' : 'riha'
  const staff = useQuery(g => g.admin.team())
  const partners = useQuery(g => g.admin.partners())
  const [adding, setAdding] = useState(false)
  return (
    <Page>
      <style>{CSS}</style>
      <PageHead title="Equipa" sub="Quem entra no backoffice e nas contas dos parceiros."
        actions={tab === 'parceiros' && <button className="btn btn-outline" onClick={() => setAdding(true)}><Icon name="plus" size={16} />Novo parceiro</button>} />
      <div className="tabs tm-tabs">
        <button className={tab === 'riha' ? 'on' : ''} onClick={() => setSp({})}>Equipa RIHA<span className="count">{staff.data?.data.length ?? '·'}</span></button>
        <button className={tab === 'parceiros' ? 'on' : ''} onClick={() => setSp({ tab: 'parceiros' })}>Parceiros<span className="count">{partners.data?.data.length ?? '·'}</span></button>
      </div>
      {tab === 'riha' && (
        !staff.data ? <Skeleton h={160} /> : (
          <TeamList members={staff.data.data} perms={staff.data.permissions} canManage meId={me!.user.id} allowAll={!!me?.user.permissions?.includes('*')} onChange={staff.reload}
            api={{ invite: g.admin.invite, setAccess: g.admin.setAccess, resend: g.admin.resendInvite }} />
        )
      )}
      {tab === 'parceiros' && (
        <div className="tm-partners">
          {!partners.data ? <Skeleton h={160} /> : partners.data.data.map((p: any) => (
            <section key={p.code}>
              <div className="tm-ph">
                <Avatar text={initials(p.name)} color="var(--o-info-soft)" size={38} radius={11} font={13} />
                <div className="tm-ph-main">
                  <div className="tm-pn">{p.name}<span className="o-mono">{p.code}</span></div>
                  <div className="tm-ints">
                    {p.integrations.length
                      ? p.integrations.map((i: any) => <span key={i.code || i.name} className="o-chip">{i.name} · {i.account}</span>)
                      : <>Sem integrações<Tip text="Escolha este parceiro no acordo de uma integração, em Liquidação." /></>}
                  </div>
                </div>
              </div>
              <TeamList members={p.users} perms={partners.data.permissions} canManage meId={me!.user.id} onChange={partners.reload} inviteLabel={`Convidar alguém de ${p.name}`}
                api={{ invite: input => g.admin.invitePartnerUser(p.code, input), setAccess: g.admin.setAccess, resend: g.admin.resendInvite }} />
            </section>
          ))}
        </div>
      )}
      {adding && <NewPartner onClose={() => setAdding(false)} onDone={() => { setAdding(false); partners.reload() }} />}
    </Page>
  )
}

function NewPartner({ onClose, onDone }: { onClose: () => void; onDone: () => void }) {
  const g = useGateway()
  const { toast } = useSession()
  const [f, setF] = useState({ name: '', code: '', email: '' })
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')
  const code = f.code || f.name.toLowerCase().normalize('NFD').replace(/[̀-ͯ]/g, '').replace(/[^a-z0-9]+/g, '_').replace(/^[^a-z]+|_+$/g, '').slice(0, 30)
  const save = async () => {
    setBusy(true); setErr('')
    try { await g.admin.createPartner({ ...f, code }); toast(`${f.name} criado`); onDone() } catch (e) { setErr(errMsg(e)) } finally { setBusy(false) }
  }
  return (
    <Modal onClose={onClose}>
      <style>{CSS}</style>
      <div className="tm-mt">Novo parceiro</div>
      <div className="tm-ms">Recebe o dinheiro dos clientes numa conta sua.<Tip text="Depois de criado, ligue-o a uma integração em Liquidação → Editar acordo." /></div>
      <label className="tm-ml" style={{ marginTop: 14 }}><span className="label">Nome</span><input className="input" value={f.name} onChange={e => setF({ ...f, name: e.target.value })} /></label>
      <label className="tm-ml"><span className="label">Código</span><input className="input mono" value={code} onChange={e => setF({ ...f, code: e.target.value.toLowerCase() })} /></label>
      <label className="tm-ml"><span className="label">Email de contacto (opcional)</span><input className="input" type="email" value={f.email} onChange={e => setF({ ...f, email: e.target.value })} /></label>
      {err && <div className="form-err" style={{ marginTop: 12 }}>{err}</div>}
      <div className="tm-mf">
        <button className="btn btn-outline" onClick={onClose}>Cancelar</button>
        <button className="btn btn-green" disabled={busy || !f.name.trim() || !code} onClick={save}>{busy ? <Spinner /> : 'Criar parceiro'}</button>
      </div>
    </Modal>
  )
}
