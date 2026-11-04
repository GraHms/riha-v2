import { useState } from 'react'
import { api, errMsg } from '../../lib/api'
import { useData } from '../../lib/useData'
import { copy, fmt, shortDate } from '../../lib/format'
import { useSession } from '../../lib/session'
import { Drawer, Icon, MethodIcon, Pill, Skeleton, Toggle } from '../../components/ui'
import { useShell } from './AppShell'
import { SharePanel, type LinkObj } from './NewLink'

const GRID = 'minmax(0,1.6fr) 120px 110px 100px 140px 110px 160px'

function linkState(l: LinkObj) {
  if (!l.active) return { t: 'Inativo', bg: '#EDECE6', fg: '#3E4742' }
  if (l.expires_at && l.expires_at * 1000 < Date.now()) return { t: 'Expirado', bg: '#EDECE6', fg: '#3E4742' }
  if (l.expires_at) return { t: 'Até ' + shortDate(l.expires_at), bg: '#FFF3C4', fg: '#7A5A00' }
  return { t: 'Ativo', bg: '#E2F3DE', fg: '#0A6B05' }
}

export default function Links() {
  const { openNewLink, version } = useShell()
  const { toast } = useSession()
  const { data, reload } = useData<{ data: LinkObj[] }>('/links', [version])
  const [copied, setCopied] = useState('')
  const [share, setShare] = useState<LinkObj | null>(null)
  const links = data?.data || []
  const doCopy = async (l: LinkObj) => { await copy(l.url); setCopied(l.id); setTimeout(() => setCopied(''), 1500) }
  const setActive = async (l: LinkObj, active: boolean) => {
    try { await api('/links/' + l.id, { method: 'PATCH', body: { active } }); toast(active ? 'Link ativado' : 'Link desativado'); reload() } catch (e) { toast(errMsg(e), true) }
  }
  const methods = (l: LinkObj) => (
    <div style={{ display: 'flex', gap: 4 }}>{['mpesa', 'emola', 'card'].map(m => <span key={m} style={{ opacity: l.payment_method_types.includes(m) ? 1 : 0.25 }}><MethodIcon method={m} size={22} radius={7} /></span>)}</div>
  )
  return (
    <div className="page">
      <div className="between" style={{ alignItems: 'flex-start' }}>
        <div><h1 className="h1">Links de pagamento</h1><div style={{ fontSize: 14.5, color: 'var(--muted)', marginTop: 4 }}>Partilhe por WhatsApp, SMS ou QR. Não precisa de site.</div></div>
        <button className="btn btn-ink only-desktop" onClick={openNewLink}><Icon name="plus" size={17} stroke={2.4} />Novo link</button>
      </div>
      <div className="table only-desktop" style={{ marginTop: 22 }}>
        <div className="thead" style={{ gridTemplateColumns: GRID }}><span>Link</span><span>Valor</span><span>Métodos</span><span>Pagamentos</span><span>Recebido</span><span>Estado</span><span /></div>
        {!data && Array.from({ length: 4 }, (_, i) => <div key={i} className="trow" style={{ gridTemplateColumns: '1fr' }}><Skeleton h={34} /></div>)}
        {links.map(l => (
          <div key={l.id} className="trow" style={{ gridTemplateColumns: GRID, padding: '14px 20px' }}>
            <div style={{ minWidth: 0 }}><div style={{ fontWeight: 600, fontSize: 14.5 }} className="row">{l.name}{l.type === 'qr' && <span className="tag" style={{ background: '#FFF3C4', color: '#7A5A00' }}>QR</span>}</div><div className="mono" style={{ fontSize: 12, color: 'var(--muted)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{l.url.replace(/^https?:\/\//, '')}</div></div>
            <div className="tnum" style={{ fontSize: 14, fontWeight: 500 }}>{l.custom_amount ? 'Livre' : fmt(l.amount)}</div>
            {methods(l)}
            <div style={{ fontSize: 14, color: 'var(--text-2)' }}>{l.payments_count}</div>
            <div className="tnum" style={{ fontSize: 14, fontWeight: 600 }}>{fmt(l.total_received)} MT</div>
            <div className="row" style={{ gap: 8 }}><Pill {...linkState(l)} /></div>
            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 6, alignItems: 'center' }}>
              <Toggle on={l.active} onChange={v => setActive(l, v)} label="Ativo" />
              <button className="btn btn-outline btn-sm" onClick={() => setShare(l)}>QR</button>
              <button className="btn btn-outline btn-sm" onClick={() => doCopy(l)}>{copied === l.id ? 'Copiado' : 'Copiar'}</button>
            </div>
          </div>
        ))}
        {data && !links.length && <div className="empty">Ainda não criou links. <button className="btn btn-green btn-sm" style={{ marginLeft: 8 }} onClick={openNewLink}>Criar o primeiro</button></div>}
      </div>
      <div className="only-mobile" style={{ marginTop: 16, display: 'flex', flexDirection: 'column', gap: 10 }}>
        {links.map(l => (
          <div key={l.id} className="card" style={{ borderRadius: 20, padding: 16 }} onClick={() => setShare(l)}>
            <div className="between" style={{ alignItems: 'flex-start' }}><div style={{ minWidth: 0 }}><div style={{ fontWeight: 600, fontSize: 15 }}>{l.name}</div><div className="mono" style={{ fontSize: 12, color: 'var(--muted)', marginTop: 2, wordBreak: 'break-all' }}>{l.url.replace(/^https?:\/\//, '')}</div></div><Pill {...linkState(l)} /></div>
            <div className="between" style={{ marginTop: 12 }}>{methods(l)}<div style={{ textAlign: 'right' }}><div className="display tnum" style={{ fontSize: 18 }}>{l.custom_amount ? 'Valor livre' : fmt(l.amount) + ' MT'}</div><div style={{ fontSize: 12, color: 'var(--muted)' }}>{l.payments_count} pagamentos · {fmt(l.total_received)} MT</div></div></div>
          </div>
        ))}
        <button className="btn btn-green btn-lg" onClick={openNewLink}><Icon name="plus" size={18} stroke={2.4} />Novo link</button>
      </div>
      {share && <Drawer title={share.name} onClose={() => setShare(null)}><div style={{ padding: '28px 26px' }}><SharePanel link={share} compact /></div></Drawer>}
    </div>
  )
}
