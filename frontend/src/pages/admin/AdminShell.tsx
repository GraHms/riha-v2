import { Outlet, useLocation, useNavigate } from 'react-router-dom'
import { useSession } from '../../lib/session'
import { useQuery } from '../../core/useQuery'
import { Avatar, Icon, Logo } from '../../components/ui'
import { initials } from '../../lib/format'

export default function AdminShell() {
  const { me, logout } = useSession()
  const nav = useNavigate()
  const path = useLocation().pathname
  const { data } = useQuery(g => g.admin.overview(), [path], 30000)
  const on = (p: string) => (p === '/admin' ? path === '/admin' : path.startsWith(p))
  const items: [string, string, string, number | undefined][] = [
    ['/admin', 'chart', 'Visão geral', undefined],
    ['/admin/aprovacoes', 'shield', 'Aprovações', data?.pending_approvals],
    ['/admin/levantamentos', 'wallet', 'Levantamentos', data?.payout_stats?.pending_count],
    ['/admin/comerciantes', 'store', 'Comerciantes', undefined],
    ['/admin/transacoes', 'txns', 'Transações', data?.unconfirmed_payments || undefined],
    ['/admin/rasto', 'search', 'Rasto', undefined],
    ['/admin/metricas', 'grid', 'Métricas', data?.alerts || undefined],
  ]
  const now = new Date()
  return (
    <div className="shell">
      <aside className="sidebar dark">
        <div className="stripe g" style={{ bottom: -60, left: -30, width: 30, height: 260, opacity: .9 }} />
        <div className="stripe r" style={{ bottom: -60, left: 12, width: 30, height: 260, opacity: .9 }} />
        <div className="row" style={{ position: 'relative', padding: '0 8px', gap: 9 }}><Logo dark /><span className="tag" style={{ marginLeft: 'auto', background: 'var(--red)', color: '#fff', fontSize: 10.5, letterSpacing: '.06em' }}>ADMIN</span></div>
        <nav style={{ position: 'relative', display: 'flex', flexDirection: 'column', gap: 2, marginTop: 28 }}>
          {items.map(([p, ic, l, n], i) => (
            <button key={p} className={'nav-btn' + (on(p) ? ' on' : '')} onClick={() => nav(p)}>
              <Icon name={ic} /><span style={{ flex: 1 }}>{l}</span>
              {n !== undefined && n > 0 && <span style={{ fontSize: 11.5, fontWeight: 700, padding: '2px 8px', borderRadius: 999, background: i === 1 ? 'var(--sun)' : 'rgba(255,255,255,.14)', color: i === 1 ? 'var(--ink)' : '#fff' }}>{n}</span>}
            </button>
          ))}
          <div className="nav-btn" style={{ color: '#5E6862', cursor: 'default' }} title="Em breve"><Icon name="alert" />Risco e alertas</div>
        </nav>
        <div style={{ flex: 1 }} />
        <div className="row" style={{ position: 'relative', gap: 10, padding: 10, borderRadius: 14, background: 'rgba(255,255,255,.07)', border: '1px solid rgba(255,255,255,.1)' }}>
          <Avatar text={initials(me!.user.name || 'A')} color="var(--lime)" />
          <div style={{ flex: 1, minWidth: 0 }}><div style={{ fontWeight: 600, fontSize: 13.5 }}>{me!.user.name}</div><div style={{ fontSize: 12, color: '#93A09A' }}>{me!.user.title || 'Administrador'}</div></div>
          <button aria-label="Terminar sessão" onClick={async () => { await logout(); nav('/') }} style={{ border: 0, background: 'none', color: '#93A09A', display: 'grid' }}><Icon name="logout" size={17} /></button>
        </div>
      </aside>
      <div className="main">
        <header className="topbar">
          <form className="search" style={{ width: 420 }} onSubmit={e => { e.preventDefault(); const q = new FormData(e.currentTarget).get('q'); nav('/admin/comerciantes?q=' + encodeURIComponent(String(q || ''))) }}>
            <Icon name="search" size={17} stroke={2} /><input name="q" placeholder="Pesquisar comerciante, NUIT ou email" />
          </form>
          <div style={{ flex: 1 }} />
          <span className="row" style={{ gap: 7, fontSize: 13, fontWeight: 600, padding: '8px 12px', borderRadius: 999, background: '#fff', border: '1px solid #E4E2DA' }}><span style={{ width: 8, height: 8, borderRadius: '50%', background: 'var(--green)' }} />Produção</span>
          <span style={{ fontSize: 13, color: 'var(--muted)' }}>{now.toLocaleDateString('pt-PT', { weekday: 'short', day: 'numeric', month: 'short' })}</span>
        </header>
        <header className="mobile-head" style={{ flexDirection: 'column', alignItems: 'stretch', background: 'var(--ink)', color: '#fff' }}>
          <div className="between"><Logo dark /><span className="tag" style={{ background: 'var(--red)', color: '#fff' }}>ADMIN</span></div>
          <div style={{ display: 'flex', gap: 6, overflowX: 'auto', marginTop: 10, scrollbarWidth: 'none' }}>
            {items.map(([p, , l, n]) => <button key={p} onClick={() => nav(p)} style={{ flex: 'none', height: 34, padding: '0 12px', borderRadius: 999, border: 0, background: on(p) ? '#fff' : 'rgba(255,255,255,.1)', color: on(p) ? 'var(--ink)' : '#fff', fontSize: 13, fontWeight: 600 }}>{l}{n ? ` · ${n}` : ''}</button>)}
          </div>
        </header>
        <div className="content"><Outlet /></div>
      </div>
    </div>
  )
}
