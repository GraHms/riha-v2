import { useEffect, useState } from 'react'
import { SUPPORT_PHONE } from '../../lib/support'
import { Link, Outlet, useLocation, useNavigate, useOutletContext } from 'react-router-dom'
import { useSession, type Merchant } from '../../lib/session'
import { Avatar, Icon, Logo } from '../../components/ui'
import { initials } from '../../lib/format'
import NewLinkDrawer from './NewLinkDrawer'

export type ShellCtx = { openNewLink: () => void; merchant: Merchant; live: boolean; version: number; bump: () => void; approved: boolean }

export const NOT_APPROVED = 'A sua conta ainda não foi aprovada. Nenhuma operação é possível até a equipa RIHA concluir a verificação.'
export const useShell = () => useOutletContext<ShellCtx>()

const NAV: [string, string, string][] = [
  ['/painel', 'home', 'Visão geral'],
  ['/painel/transacoes', 'txns', 'Transações'],
  ['/painel/links', 'link', 'Links de pagamento'],
  ['/painel/saldo', 'wallet', 'Saldo e levantamentos'],
]

export function ModeSwitch() {
  const { live, setLive } = useSession()
  return (
    <button className="mode-btn" onClick={() => setLive(!live)} aria-label="Alternar modo de teste">
      {live ? 'Produção' : 'Teste'}
      <span className="mode-sw" style={{ background: live ? 'var(--green)' : '#E0A800', justifyContent: live ? 'flex-end' : 'flex-start' }}><span /></span>
    </button>
  )
}

export const STATUS_TEXT: Record<string, string> = { active: 'Verificado', in_review: 'Em verificação', lead: 'Lead · por verificar', info_requested: 'Documentos pedidos', rejected: 'Verificação rejeitada', suspended: 'Suspenso' }

export default function AppShell() {
  const { me, live, toast } = useSession()
  const loc = useLocation()
  const nav = useNavigate()
  const [drawer, setDrawer] = useState(false)
  const [q, setQ] = useState('')
  const [version, setVersion] = useState(0)
  const m = me!.merchant!
  const path = loc.pathname
  const isOn = (p: string) => (p === '/painel' ? path === '/painel' : path.startsWith(p))
  const approved = m.status === 'active'
  const openNewLink = () => { if (!approved) { toast(NOT_APPROVED, true); return } if (window.matchMedia('(max-width: 860px)').matches) nav('/painel/links/novo'); else setDrawer(true) }
  useEffect(() => setVersion(v => v + 1), [live])

  const banner = m.status !== 'active' && (
    <div style={{ flex: 'none', display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 10, flexWrap: 'wrap', padding: '8px 16px', background: m.status === 'rejected' || m.status === 'suspended' ? 'var(--red-bg)' : 'var(--sun-soft)', fontSize: 13, fontWeight: 500, color: m.status === 'rejected' || m.status === 'suspended' ? '#8E0609' : '#5B4600', textAlign: 'center' }}>
      {m.status === 'in_review' && 'A sua conta está em verificação. Links, pagamentos, chaves API e levantamentos ficam disponíveis depois da aprovação.'}
      {m.status === 'lead' && <>A sua conta ainda não foi aprovada. Complete a verificação para começar a receber. <Link to="/verificacao" style={{ fontWeight: 700 }}>Verificar agora</Link></>}
      {m.status === 'info_requested' && <>A equipa RIHA pediu mais documentos. <Link to="/verificacao" style={{ fontWeight: 700 }}>Completar</Link></>}
      {m.status === 'rejected' && `A verificação foi rejeitada. Contacte o suporte da RIHA: ${SUPPORT_PHONE}.`}
      {m.status === 'suspended' && `Esta conta está suspensa. Contacte o suporte da RIHA: ${SUPPORT_PHONE}.`}
    </div>
  )

  return (
    <div className="shell">
      <aside className="sidebar">
        <div style={{ padding: '0 8px' }}><Link to="/painel"><Logo /></Link></div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginTop: 22, padding: 10, borderRadius: 14, background: 'var(--paper)' }}>
          <Avatar text={initials(m.name)} color={m.avatar_color} />
          <div style={{ flex: 1, minWidth: 0 }}><div style={{ fontWeight: 600, fontSize: 14, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{m.name}</div><div style={{ fontSize: 12, color: 'var(--muted)' }}>{STATUS_TEXT[m.status]}</div></div>
        </div>
        <nav style={{ display: 'flex', flexDirection: 'column', gap: 2, marginTop: 22 }}>
          {NAV.map(([p, ic, l]) => <button key={p} className={'nav-btn' + (isOn(p) ? ' on' : '')} onClick={() => nav(p)}><Icon name={ic} />{l}</button>)}
        </nav>
        <div style={{ flex: 1 }} />
        <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
          <button className={'nav-btn' + (isOn('/painel/definicoes') ? ' on' : '')} onClick={() => nav('/painel/definicoes')}><Icon name="settings" />Definições</button>
          <Link to="/docs" className="nav-btn"><Icon name="doc" />Documentação</Link>
        </div>
      </aside>

      <div className="main">
        <header className="topbar">
          <form className="search" onSubmit={e => { e.preventDefault(); nav('/painel/transacoes?q=' + encodeURIComponent(q)) }}>
            <Icon name="search" size={17} stroke={2} />
            <input value={q} onChange={e => setQ(e.target.value)} placeholder="Pesquisar pagamentos, referências, clientes" aria-label="Pesquisar" />
          </form>
          <div style={{ flex: 1 }} />
          <ModeSwitch />
          <button className="btn btn-green" onClick={openNewLink}><Icon name="plus" size={17} stroke={2.4} />Criar link</button>
        </header>
        <header className="mobile-head">
          <Avatar text={initials(m.name)} color={m.avatar_color} size={44} radius={14} font={15} />
          <div style={{ flex: 1, minWidth: 0 }}><div style={{ fontSize: 13, color: 'var(--muted)' }}>{STATUS_TEXT[m.status]}</div><div style={{ fontWeight: 600, fontSize: 17, letterSpacing: '-0.01em', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{m.name}</div></div>
          <ModeSwitch />
        </header>
        {!live && <div className="testbar">Modo de teste. Nenhum dinheiro real é movimentado.</div>}
        {banner}
        <div className="content">
          <Outlet context={{ openNewLink, merchant: m, live, version, bump: () => setVersion(v => v + 1), approved } satisfies ShellCtx} />
        </div>
        <nav className="tabbar">
          <button className={isOn('/painel') ? 'on' : ''} onClick={() => nav('/painel')}><Icon name="home" size={22} />Início</button>
          <button className={isOn('/painel/transacoes') ? 'on' : ''} onClick={() => nav('/painel/transacoes')}><Icon name="txns" size={22} />Transações</button>
          <button className="fab" onClick={openNewLink} aria-label="Criar link"><Icon name="plus" size={24} stroke={2.4} /></button>
          <button className={isOn('/painel/saldo') ? 'on' : ''} onClick={() => nav('/painel/saldo')}><Icon name="wallet" size={22} />Saldo</button>
          <button className={isOn('/painel/definicoes') || isOn('/painel/links') ? 'on' : ''} onClick={() => nav('/painel/definicoes')}><Icon name="settings" size={22} />Definições</button>
        </nav>
      </div>
      {drawer && <NewLinkDrawer onClose={() => { setDrawer(false); setVersion(v => v + 1) }} />}
    </div>
  )
}
