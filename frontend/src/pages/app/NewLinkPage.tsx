import { useEffect, useState } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { Icon, Skeleton, Spinner } from '../../components/ui'
import { useGateway } from '../../core/gateway'
import { errMsg } from '../../core/model'
import { useSession } from '../../lib/session'
import { SharePanel, useLinkForm, type LinkObj } from './NewLink'
import { NOT_APPROVED, useShell } from './AppShell'

// One page for a new link, a copy of one (?from=id), and editing one (/:id/editar).
export default function NewLinkPage({ edit }: { edit?: boolean }) {
  const { id } = useParams()
  const [sp] = useSearchParams()
  const g = useGateway()
  const { toast } = useSession()
  const fromId = edit ? id : sp.get('from')
  const [from, setFrom] = useState<LinkObj | null>(null)
  const { approved } = useShell()
  useEffect(() => { if (fromId) g.links.get(fromId).then(l => setFrom(l as LinkObj)).catch(e => toast(errMsg(e), true)) }, [fromId])
  if (!approved) return <div className="page" style={{ maxWidth: 520, margin: '0 auto' }}><div className="form-err" style={{ background: 'var(--sun-soft)', color: '#5B4600', marginTop: 20 }}>{NOT_APPROVED}</div></div>
  if (fromId && !from) return <div className="page" style={{ maxWidth: 560, margin: '0 auto' }}><Skeleton h={480} r={22} /></div>
  return <LinkFormPage key={fromId || 'new'} from={from || undefined} edit={!!edit} />
}

function LinkFormPage({ from, edit }: { from?: LinkObj; edit: boolean }) {
  const nav = useNavigate()
  const [done, setDone] = useState<LinkObj | null>(null)
  const f = useLinkForm(l => (edit ? nav('/painel/links/' + l.id) : setDone(l)), from, edit)
  if (done) return (
    <div className="page" style={{ maxWidth: 520, margin: '0 auto' }}>
      <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8 }}>
        <button className="btn btn-outline" style={{ height: 40, borderRadius: 999 }} onClick={() => nav('/painel/links/' + done.id)}>Ver o link</button>
        <button className="btn btn-outline" style={{ height: 40, borderRadius: 999 }} onClick={() => nav('/painel/links')}>Concluir</button>
      </div>
      <SharePanel link={done} />
    </div>
  )
  const title = edit ? 'Editar link' : from ? 'Duplicar link' : 'Novo link de pagamento'
  return (
    <div style={{ display: 'flex', flexDirection: 'column', minHeight: '100%', maxWidth: 560, margin: '0 auto' }}>
      <div className="row" style={{ padding: '6px 16px 0' }}>
        <button className="icon-btn" style={{ width: 42, height: 42, borderRadius: 14 }} onClick={() => nav(-1)} aria-label="Fechar"><Icon name="close" size={20} stroke={2} /></button>
        <div style={{ fontWeight: 600, fontSize: 17 }}>{title}</div>
      </div>
      <div style={{ padding: '14px 16px 0' }}>{f.tabs}</div>
      {f.tab === 'edit' ? (
        <>
          <div style={{ padding: '22px 16px 8px' }}>{f.amountBlock(true)}</div>
          <div style={{ padding: '18px 16px 0', display: 'flex', flexDirection: 'column', gap: 14 }}>{f.fields}</div>
        </>
      ) : <div style={{ padding: '14px 16px 0' }}>{f.preview}</div>}
      <div style={{ flex: 1, minHeight: 20 }} />
      <div style={{ position: 'sticky', bottom: 0, padding: '14px 16px 12px', background: 'linear-gradient(180deg,rgba(244,243,238,0),#F4F3EE 30%)' }}>
        <button className="btn btn-green btn-lg btn-block" onClick={f.submit} disabled={f.busy || !f.valid}>{f.busy ? <Spinner /> : edit ? 'Guardar alterações' : 'Criar link'}</button>
      </div>
    </div>
  )
}
