import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { errMsg } from '../../core/model'
import { useGateway } from '../../core/gateway'
import { useQuery } from '../../core/useQuery'
import { copy, fmt, shortDate } from '../../lib/format'
import { useSession } from '../../lib/session'
import { Drawer, Icon, MethodIcon, Pill, Skeleton, Toggle } from '../../components/ui'
import { useShell } from './AppShell'
import { SharePanel, type LinkObj } from './NewLink'

type Bucket = 'active' | 'inactive' | 'archived'

// linkState says where a link stands, in the words the merchant sees, and which
// tab of the list it belongs to.
export function linkState(l: LinkObj): { t: string; bg: string; fg: string; bucket: Bucket } {
  const grey = { bg: '#EDECE6', fg: '#3E4742' }
  if (l.archived) return { t: 'Arquivado', ...grey, bucket: 'archived' }
  if (!l.reusable && l.payments_count > 0) return { t: 'Já pago', ...grey, bucket: 'inactive' }
  if (!l.active) return { t: 'Inativo', ...grey, bucket: 'inactive' }
  if (l.expires_at && l.expires_at * 1000 < Date.now()) return { t: 'Expirado', ...grey, bucket: 'inactive' }
  if (l.expires_at) return { t: 'Até ' + shortDate(l.expires_at), bg: '#FFF3C4', fg: '#7A5A00', bucket: 'active' }
  return { t: 'Ativo', bg: '#E2F3DE', fg: '#0A6B05', bucket: 'active' }
}

export function useLinkActions(onChanged: () => void) {
  const g = useGateway()
  const nav = useNavigate()
  const { toast } = useSession()
  const patch = async (l: LinkObj, body: Record<string, unknown>, done: string) => {
    try { await g.links.update(l.id, body); toast(done); onChanged() } catch (e) { toast(errMsg(e), true) }
  }
  return {
    edit: (l: LinkObj) => nav(`/painel/links/${l.id}/editar`),
    duplicate: (l: LinkObj) => nav(`/painel/links/novo?from=${l.id}`),
    setActive: (l: LinkObj, active: boolean) => {
      if (!active && !confirm(`Desativar "${l.name}"?\n\nQuem abrir o link vê que já não está ativo. Pode voltar a ativá-lo quando quiser.`)) return
      patch(l, { active }, active ? 'Link ativado' : 'Link desativado')
    },
    archive: (l: LinkObj, archived: boolean) => {
      if (archived && !confirm(`Arquivar "${l.name}"?\n\nO link deixa de aceitar pagamentos e sai da lista. Os pagamentos ficam no histórico, e pode restaurá-lo em Arquivados.`)) return
      patch(l, { archived }, archived ? 'Link arquivado' : 'Link restaurado. Ative-o para voltar a receber.')
    },
  }
}

export function LinkMenu({ l, onShare, onChanged, align = 'right' }: { l: LinkObj; onShare: () => void; onChanged: () => void; align?: 'right' | 'left' }) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  const a = useLinkActions(onChanged)
  useEffect(() => {
    if (!open) return
    const close = (e: MouseEvent | KeyboardEvent) => { if (e instanceof KeyboardEvent ? e.key === 'Escape' : !ref.current?.contains(e.target as Node)) setOpen(false) }
    document.addEventListener('mousedown', close); document.addEventListener('keydown', close)
    return () => { document.removeEventListener('mousedown', close); document.removeEventListener('keydown', close) }
  }, [open])
  const item = (label: string, icon: string, fn: () => void, danger = false) => (
    <button role="menuitem" type="button" onClick={() => { setOpen(false); fn() }} style={{ display: 'flex', alignItems: 'center', gap: 10, width: '100%', padding: '10px 14px', border: 0, background: 'none', fontSize: 14, textAlign: 'left', color: danger ? 'var(--red-dark)' : 'var(--ink)', cursor: 'pointer', borderRadius: 10 }} className="menu-item">
      <Icon name={icon} size={16} />{label}
    </button>
  )
  return (
    <div ref={ref} style={{ position: 'relative' }} onClick={e => e.stopPropagation()}>
      <style>{`.menu-item:hover{background:var(--paper)}`}</style>
      <button type="button" className="icon-btn" aria-label={'Mais ações para ' + l.name} aria-haspopup="menu" aria-expanded={open} onClick={() => setOpen(!open)} style={{ width: 36, height: 36 }}><Icon name="dots" size={18} /></button>
      {open && (
        <div role="menu" style={{ position: 'absolute', top: 42, [align]: 0, zIndex: 20, minWidth: 200, padding: 6, borderRadius: 14, background: '#fff', border: '1px solid var(--line)', boxShadow: '0 18px 40px -16px rgba(11,18,14,.35)' }}>
          {item('Partilhar', 'share', onShare)}
          {!l.archived && item('Editar', 'settings', () => a.edit(l))}
          {item('Duplicar', 'copy', () => a.duplicate(l))}
          {!l.archived && (l.active ? item('Desativar', 'pause', () => a.setActive(l, false)) : item('Ativar', 'check', () => a.setActive(l, true)))}
          {l.archived ? item('Restaurar', 'refresh', () => a.archive(l, false)) : item('Arquivar', 'trash', () => a.archive(l, true), true)}
        </div>
      )}
    </div>
  )
}

const GRID = 'minmax(0,1.6fr) 110px 106px 96px 130px 112px 150px'
const TABS: [Bucket, string][] = [['active', 'Ativos'], ['inactive', 'Inativos'], ['archived', 'Arquivados']]

export default function Links() {
  const { openNewLink, version } = useShell()
  const nav = useNavigate()
  const { data, reload } = useQuery(g => g.links.list(), [version])
  const [copied, setCopied] = useState('')
  const [share, setShare] = useState<LinkObj | null>(null)
  const [tab, setTab] = useState<Bucket>('active')
  const [q, setQ] = useState('')
  const a = useLinkActions(reload)
  const all = (data?.data || []) as LinkObj[]
  const count = (b: Bucket) => all.filter(l => linkState(l).bucket === b).length
  const needle = q.trim().toLowerCase()
  const links = all.filter(l => linkState(l).bucket === tab && (!needle || l.name.toLowerCase().includes(needle) || l.slug.includes(needle)))
  const doCopy = async (l: LinkObj) => { await copy(l.url); setCopied(l.id); setTimeout(() => setCopied(''), 1500) }
  const methods = (l: LinkObj) => (
    <div style={{ display: 'flex', gap: 4 }}>{['mpesa', 'emola', 'bim', 'card'].map(m => <span key={m} style={{ opacity: l.payment_method_types.includes(m) ? 1 : 0.25 }}><MethodIcon method={m} size={22} radius={7} /></span>)}</div>
  )
  const amount = (l: LinkObj) => (l.custom_amount ? 'Valor livre' : fmt(l.amount) + ' MT')
  const open = (l: LinkObj) => nav('/painel/links/' + l.id)
  const empty = !data ? null : all.length === 0
    ? <div className="empty">Ainda não criou links. <button className="btn btn-green btn-sm" style={{ marginLeft: 8 }} onClick={openNewLink}>Criar o primeiro</button></div>
    : <div className="empty">{needle ? `Nenhum link com "${q.trim()}".` : tab === 'archived' ? 'Não há links arquivados.' : tab === 'inactive' ? 'Não há links inativos.' : 'Não há links ativos.'}</div>
  return (
    <div className="page">
      <style>{`
        .lk-tools{display:flex;gap:10px;align-items:center;flex-wrap:wrap;margin-top:20px}
        .lk-search{flex:1;min-width:180px;max-width:320px;position:relative}
        .lk-search input{padding-left:38px;height:40px}
        .lk-search svg{position:absolute;left:12px;top:11px}
        @media(max-width:860px){.lk-search{max-width:none;flex-basis:100%}.lk-tools .seg{flex:1}.lk-tools .seg button{flex:1}}
      `}</style>
      <div className="between" style={{ alignItems: 'flex-start' }}>
        <div><h1 className="h1">Links de pagamento</h1><div style={{ fontSize: 14.5, color: 'var(--muted)', marginTop: 4 }}>Partilhe por WhatsApp, SMS ou QR. Não precisa de site.</div></div>
        <button className="btn btn-ink only-desktop" onClick={openNewLink}><Icon name="plus" size={17} stroke={2.4} />Novo link</button>
      </div>
      <div className="lk-tools">
        <div className="seg" role="tablist">{TABS.map(([k, l]) => <button key={k} role="tab" aria-selected={tab === k} className={tab === k ? 'on' : ''} onClick={() => setTab(k)}>{l}{data ? ` · ${count(k)}` : ''}</button>)}</div>
        <label className="lk-search"><Icon name="search" size={16} color="var(--muted)" /><input className="input" value={q} onChange={e => setQ(e.target.value)} placeholder="Procurar um link" aria-label="Procurar um link" /></label>
      </div>

      <div className="table only-desktop" style={{ marginTop: 16 }}>
        <div className="thead" style={{ gridTemplateColumns: GRID }}><span>Link</span><span>Valor</span><span>Métodos</span><span>Pagamentos</span><span>Recebido</span><span>Estado</span><span /></div>
        {!data && Array.from({ length: 4 }, (_, i) => <div key={i} className="trow" style={{ gridTemplateColumns: '1fr' }}><Skeleton h={34} /></div>)}
        {links.map(l => (
          <div key={l.id} className="trow click" style={{ gridTemplateColumns: GRID, padding: '14px 20px' }} onClick={() => open(l)}>
            <div style={{ minWidth: 0 }}><div style={{ fontWeight: 600, fontSize: 14.5 }} className="row">{l.name}{l.type === 'qr' && <span className="tag" style={{ background: '#FFF3C4', color: '#7A5A00' }}>QR</span>}</div><div className="mono" style={{ fontSize: 12, color: 'var(--muted)', marginTop: 2, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{l.url.replace(/^https?:\/\//, '')}</div></div>
            <div className="tnum" style={{ fontSize: 14, fontWeight: 500 }}>{l.custom_amount ? 'Livre' : fmt(l.amount)}</div>
            {methods(l)}
            <div style={{ fontSize: 14, color: 'var(--text-2)' }}>{l.payments_count}</div>
            <div className="tnum" style={{ fontSize: 14, fontWeight: 600 }}>{fmt(l.total_received)} MT</div>
            <div className="row" style={{ gap: 8 }}><Pill {...linkState(l)} /></div>
            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 6, alignItems: 'center' }} onClick={e => e.stopPropagation()}>
              {!l.archived && <Toggle on={l.active} onChange={v => a.setActive(l, v)} label={(l.active ? 'Desativar ' : 'Ativar ') + l.name} />}
              <button className="btn btn-outline btn-sm" onClick={() => doCopy(l)}>{copied === l.id ? 'Copiado' : 'Copiar'}</button>
              <LinkMenu l={l} onShare={() => setShare(l)} onChanged={reload} />
            </div>
          </div>
        ))}
        {!links.length && empty}
      </div>

      <div className="only-mobile" style={{ marginTop: 14, display: 'flex', flexDirection: 'column', gap: 10 }}>
        {!data && Array.from({ length: 3 }, (_, i) => <Skeleton key={i} h={120} r={20} />)}
        {links.map(l => (
          <div key={l.id} className="card" role="link" tabIndex={0} style={{ borderRadius: 20, padding: 16, cursor: 'pointer' }} onClick={() => open(l)} onKeyDown={e => e.key === 'Enter' && open(l)}>
            <div style={{ display: 'flex', gap: 10, alignItems: 'flex-start' }}>
              <div style={{ flex: 1, minWidth: 0 }}>
                <div style={{ fontWeight: 600, fontSize: 15.5, lineHeight: 1.3 }}>{l.name}{l.type === 'qr' && <span className="tag" style={{ background: '#FFF3C4', color: '#7A5A00', marginLeft: 6 }}>QR</span>}</div>
                <div className="display tnum" style={{ fontSize: 19, marginTop: 4 }}>{amount(l)}</div>
              </div>
              <Pill {...linkState(l)} />
            </div>
            <div style={{ fontSize: 13, color: 'var(--muted)', marginTop: 6 }}>{l.payments_count} {l.payments_count === 1 ? 'pagamento' : 'pagamentos'} · {fmt(l.total_received)} MT{l.last_payment_at ? ` · último a ${shortDate(l.last_payment_at)}` : ''}</div>
            <div className="between" style={{ marginTop: 12, paddingTop: 12, borderTop: '1px solid var(--line-3)' }} onClick={e => e.stopPropagation()}>
              {methods(l)}
              <div className="row" style={{ gap: 6 }}>
                <button className="btn btn-outline btn-sm" onClick={() => setShare(l)}><Icon name="share" size={14} />Partilhar</button>
                <LinkMenu l={l} onShare={() => setShare(l)} onChanged={reload} />
              </div>
            </div>
          </div>
        ))}
        {!links.length && empty && <div className="card" style={{ borderRadius: 20 }}>{empty}</div>}
        {tab === 'active' && <button className="btn btn-green btn-lg" onClick={openNewLink}><Icon name="plus" size={18} stroke={2.4} />Novo link</button>}
      </div>
      {share && <Drawer title={share.name} onClose={() => setShare(null)}><div style={{ padding: '28px 26px' }}><SharePanel link={share} compact /></div></Drawer>}
    </div>
  )
}
