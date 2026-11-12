import { useState } from 'react'
import { errMsg } from '../../core/model'
import { relDate } from '../../lib/format'
import { useSession } from '../../lib/session'
import { Avatar, Icon, Modal, Spinner } from '../../components/ui'
import { Badge, Empty, List, type Col } from '../../components/ops'
import { initials } from '../../lib/format'

// A team's people and what each can see: RIHA's staff or one partner's users.
// The same list serves the backoffice and a partner's own account.

export type Perm = { key: string; label: string; help: string }
type Api = {
  invite: (input: any) => Promise<any>
  setAccess: (id: string, patch: any) => Promise<any>
  resend: (id: string) => Promise<any>
}

const CSS = `
.tl-name{display:flex;align-items:center;gap:6px;min-width:0}
.tl-name .o-t1{min-width:0}
.tl-you{flex:none;font-size:11px;font-weight:650;color:var(--o-muted);background:var(--o-line-2);padding:1px 6px;border-radius:5px}
.tl-off{opacity:.5}
.tl-mail{overflow-wrap:anywhere}
.tl-chips{display:flex;flex-wrap:wrap;gap:4px;min-width:0}
.tl-chips .o-chip.all{background:var(--o-ink);border-color:var(--o-ink);color:#fff}
.tl-acts{display:flex;gap:6px;justify-content:flex-end;flex-wrap:wrap}
.tl-acts .tl-close{color:var(--o-bad)}
.tl-foot{display:flex;flex-direction:column;gap:10px;width:100%;min-width:0}
.tl-invite{margin-top:12px}
.tl-mt{font-family:var(--display);font-size:20px;font-weight:700;letter-spacing:-.02em;line-height:1.2}
.tl-ms{font-size:13px;color:var(--o-muted);margin-top:4px}
.tl-ml{display:block;margin-top:14px}
.tl-mh{font-size:12.5px;font-weight:600;margin-top:16px}
.tl-mf{display:flex;gap:10px;margin-top:18px}
.tl-mf .btn{flex:1;height:42px}
.tl-perms{display:flex;flex-direction:column;gap:6px;margin-top:8px}
.tl-perm{display:flex;gap:12px;align-items:flex-start;padding:11px 12px;border:1px solid var(--o-line);border-radius:12px;cursor:pointer;transition:background .12s,border-color .12s}
.tl-perm:hover{background:var(--o-surface-2)}
.tl-perm.on{background:var(--o-brand-soft);border-color:#B9DDB0}
.tl-perm.dim{opacity:.5;cursor:default}
.tl-perm.dim:hover{background:transparent}
.tl-perm input{margin:2px 0 0;width:16px;height:16px;flex:none;accent-color:var(--o-brand)}
.tl-perm b{display:block;font-size:13.5px;font-weight:650}
.tl-perm small{display:block;font-size:12px;color:var(--o-muted);margin-top:1px;line-height:1.4}
@media(max-width:860px){
  .tl-acts{justify-content:stretch}
  .tl-acts .btn{flex:1 1 auto;height:38px}
  .tl-invite{width:100%;height:44px}
}`

export function TeamList({ members, perms, canManage, meId, api, onChange, inviteLabel = 'Convidar pessoa', allowAll = false }:
  { members: any[]; perms: Perm[]; canManage: boolean; meId: string; api: Api; onChange: () => void; inviteLabel?: string; allowAll?: boolean }) {
  const { toast } = useSession()
  const [edit, setEdit] = useState<any | null>(null)
  const [inviting, setInviting] = useState(false)
  const label = (k: string) => (k === '*' ? 'Tudo' : perms.find(p => p.key === k)?.label || k)
  const toggle = async (m: any) => {
    try { await api.setAccess(m.id, { disabled: !m.disabled }); toast(m.disabled ? `Conta de ${m.name} reaberta` : `Conta de ${m.name} fechada: já não entra`); onChange() } catch (e) { toast(errMsg(e), true) }
  }
  const resend = async (m: any) => {
    try { await api.resend(m.id); toast(`Novo convite enviado para ${m.email}`) } catch (e) { toast(errMsg(e), true) }
  }

  const avatar = (m: any, size = 36) => <span className={m.disabled ? 'tl-off' : undefined}><Avatar text={initials(m.name || m.email)} color="#DCEFD6" size={size} radius={10} font={12} /></span>
  const name = (m: any) => (
    <div className="tl-name">
      <span className="o-t1">{m.name}</span>
      {m.id === meId && <span className="tl-you">você</span>}
      {m.disabled && <Badge tone="bad">Fechada</Badge>}
    </div>
  )
  const seen = (m: any) => (m.last_login_at ? `entrou ${relDate(m.last_login_at).replace(/^./, c => c.toLowerCase())}` : 'ainda não entrou')
  const chips = (m: any) => (
    <div className={'tl-chips' + (m.disabled ? ' tl-off' : '')}>
      {m.permissions.map((k: string) => <span key={k} className={'o-chip' + (k === '*' ? ' all' : '')}>{label(k)}</span>)}
    </div>
  )
  const actions = (m: any) => (
    <div className="tl-acts">
      {!m.disabled && !m.last_login_at && <button className="btn btn-ghost btn-sm" onClick={() => resend(m)}>Reenviar convite</button>}
      {!m.disabled && m.id !== meId && <button className="btn btn-outline btn-sm" onClick={() => setEdit(m)}>Permissões</button>}
      {m.id !== meId && <button className={'btn btn-ghost btn-sm' + (m.disabled ? '' : ' tl-close')} onClick={() => toggle(m)}>{m.disabled ? 'Reabrir' : 'Fechar conta'}</button>}
    </div>
  )

  const cols: Col<any>[] = [
    { head: '', w: '36px', cell: m => avatar(m) },
    { head: 'Pessoa', w: 'minmax(0,1.3fr)', cell: m => <>{name(m)}<div className="o-t2">{m.email}{m.title ? ` · ${m.title}` : ''}</div></> },
    { head: 'Pode ver', w: 'minmax(0,1.4fr)', cell: chips },
    { head: 'Última entrada', w: '130px', cell: m => (m.last_login_at ? <span className="o-t2">{relDate(m.last_login_at)}</span> : <Badge tone="info">Ainda não entrou</Badge>) },
    ...(canManage ? [{ head: '', w: 'auto', right: true, cell: actions }] : []),
  ]

  return (
    <div>
      <style>{CSS}</style>
      <List rows={members} rowKey={m => m.id} cols={cols}
        empty={<Empty icon="users" title="Ainda ninguém" text={canManage ? 'Convide a primeira pessoa.' : undefined} />}
        card={m => ({
          lead: avatar(m, 38),
          title: name(m),
          sub: <><span className="tl-mail">{m.email}</span>{m.title ? ` · ${m.title}` : ''}<br />{seen(m)}</>,
          foot: <div className="tl-foot">{chips(m)}{canManage && actions(m)}</div>,
        })} />
      {canManage && <button className="btn btn-outline tl-invite" onClick={() => setInviting(true)}><Icon name="userPlus" size={16} />{inviteLabel}</button>}
      {edit && <AccessModal member={edit} perms={perms} allowAll={allowAll} onClose={() => setEdit(null)}
        onSave={async (permissions, title) => { await api.setAccess(edit.id, { permissions, title }); toast(`Permissões de ${edit.name} atualizadas`); setEdit(null); onChange() }} />}
      {inviting && <InviteModal perms={perms} allowAll={allowAll} onClose={() => setInviting(false)}
        onSave={async input => { await api.invite(input); toast(`Convite enviado para ${input.email}`); setInviting(false); onChange() }} />}
    </div>
  )
}

function PermPicker({ perms, value, onChange, allowAll }: { perms: Perm[]; value: string[]; onChange: (v: string[]) => void; allowAll: boolean }) {
  const all = value.includes('*')
  const flip = (k: string) => onChange(value.includes(k) ? value.filter(x => x !== k) : [...value.filter(x => x !== '*'), k])
  return (
    <div className="tl-perms">
      {allowAll && (
        <label className={'tl-perm' + (all ? ' on' : '')}>
          <input type="checkbox" checked={all} onChange={() => onChange(all ? [] : ['*'])} />
          <span><b>Tudo</b><small>Todas as áreas, incluindo as que vierem a existir.</small></span>
        </label>
      )}
      {perms.map(p => {
        const on = all || value.includes(p.key)
        return (
          <label key={p.key} className={'tl-perm' + (all ? ' dim' : on ? ' on' : '')}>
            <input type="checkbox" checked={on} disabled={all} onChange={() => flip(p.key)} />
            <span><b>{p.label}</b><small>{p.help}</small></span>
          </label>
        )
      })}
    </div>
  )
}

function AccessModal({ member, perms, allowAll, onClose, onSave }: { member: any; perms: Perm[]; allowAll: boolean; onClose: () => void; onSave: (p: string[], title: string) => Promise<void> }) {
  const [value, setValue] = useState<string[]>(member.permissions)
  const [title, setTitle] = useState(member.title || '')
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')
  const save = async () => { setBusy(true); setErr(''); try { await onSave(value, title) } catch (e) { setErr(errMsg(e)) } finally { setBusy(false) } }
  return (
    <Modal onClose={onClose} width={520}>
      <div className="tl-mt">O que {member.name} pode ver</div>
      <div className="tl-ms">{member.email}</div>
      <label className="tl-ml"><span className="label">Função (opcional)</span><input className="input" value={title} onChange={e => setTitle(e.target.value)} placeholder="Finanças, Suporte…" /></label>
      <div className="tl-mh">Pode ver</div>
      <PermPicker perms={perms} value={value} onChange={setValue} allowAll={allowAll} />
      {err && <div className="form-err" style={{ marginTop: 12 }}>{err}</div>}
      <div className="tl-mf">
        <button className="btn btn-outline" onClick={onClose}>Cancelar</button>
        <button className="btn btn-green" disabled={busy || !value.length} onClick={save}>{busy ? <Spinner /> : 'Guardar'}</button>
      </div>
    </Modal>
  )
}

function InviteModal({ perms, allowAll, onClose, onSave }: { perms: Perm[]; allowAll: boolean; onClose: () => void; onSave: (input: any) => Promise<void> }) {
  const [f, setF] = useState({ name: '', email: '', title: '' })
  const [value, setValue] = useState<string[]>([])
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')
  const save = async () => { setBusy(true); setErr(''); try { await onSave({ ...f, permissions: value }) } catch (e) { setErr(errMsg(e)) } finally { setBusy(false) } }
  return (
    <Modal onClose={onClose} width={520}>
      <div className="tl-mt">Convidar pessoa</div>
      <div className="tl-ms">Recebe um email para criar a palavra-passe. O link vale 7 dias.</div>
      <div className="o-grid c2" style={{ gap: 12, marginTop: 14 }}>
        <label><span className="label">Nome</span><input className="input" value={f.name} onChange={e => setF({ ...f, name: e.target.value })} /></label>
        <label><span className="label">Email</span><input className="input" type="email" value={f.email} onChange={e => setF({ ...f, email: e.target.value })} /></label>
      </div>
      <label className="tl-ml" style={{ marginTop: 12 }}><span className="label">Função (opcional)</span><input className="input" value={f.title} onChange={e => setF({ ...f, title: e.target.value })} placeholder="Finanças, Suporte…" /></label>
      <div className="tl-mh">Pode ver</div>
      <PermPicker perms={perms} value={value} onChange={setValue} allowAll={allowAll} />
      {err && <div className="form-err" style={{ marginTop: 12 }}>{err}</div>}
      <div className="tl-mf">
        <button className="btn btn-outline" onClick={onClose}>Cancelar</button>
        <button className="btn btn-green" disabled={busy || !value.length || !f.name.trim() || !f.email.trim()} onClick={save}>{busy ? <Spinner /> : 'Enviar convite'}</button>
      </div>
    </Modal>
  )
}
