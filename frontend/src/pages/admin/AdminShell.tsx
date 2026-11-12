import { Outlet, useLocation, useNavigate } from 'react-router-dom'
import { useSession } from '../../lib/session'
import { useQuery } from '../../core/useQuery'
import { can } from '../../core/model'
import { Icon } from '../../components/ui'
import { Empty, Page, Shell, titleFor, type NavGroup } from '../../components/ops'

// The backoffice: each area shows only to staff with its permission.
export default function AdminShell() {
  const { me, logout } = useSession()
  const nav = useNavigate()
  const path = useLocation().pathname
  // The menu's counts: one small request, not the whole overview.
  const { data } = useQuery(g => g.admin.badges(), [], 30000)
  const has = (...perms: string[]) => !perms.length || perms.some(p => can(me, p))
  const areas: [NavGroup['title'], [string, string, string, number | undefined, boolean, string[]][]][] = [
    [undefined, [['/admin', 'chart', 'Visão geral', undefined, false, []]]],
    ['Dinheiro', [
      ['/admin/transacoes', 'txns', 'Transações', data?.unconfirmed_payments || undefined, true, ['transactions']],
      ['/admin/levantamentos', 'wallet', 'Levantamentos', data?.payout_stats?.pending_count, true, ['payouts']],
      ['/admin/liquidacao', 'bank', 'Liquidação', undefined, false, ['settlement']],
    ]],
    ['Comerciantes', [
      ['/admin/aprovacoes', 'shield', 'Aprovações', data?.pending_approvals, true, ['approvals']],
      ['/admin/comerciantes', 'store', 'Comerciantes', undefined, false, ['merchants', 'approvals', 'pricing']],
    ]],
    ['Sistema', [
      ['/admin/rasto', 'search', 'Rasto', undefined, false, ['transactions']],
      ['/admin/metricas', 'grid', 'Métricas', data?.alerts || undefined, false, ['metrics']],
      ['/admin/equipa', 'users', 'Equipa', undefined, false, ['team']],
    ]],
  ]
  const groups: NavGroup[] = areas.map(([title, items]) => ({
    title, items: items.filter(i => has(...i[5])).map(([to, icon, label, count, hot]) => ({ to, icon, label, count, hot, exact: to === '/admin' })),
  }))
  const here = areas.flatMap(a => a[1]).filter(i => i[0] !== '/admin').find(i => path.startsWith(i[0]))
  const blocked = here && !has(...here[5])
  const search = (
    <form className="o-search" onSubmit={e => { e.preventDefault(); const q = new FormData(e.currentTarget).get('q'); nav('/admin/comerciantes?q=' + encodeURIComponent(String(q || ''))) }}>
      <Icon name="search" size={16} stroke={2} /><input name="q" placeholder="Procurar comerciante, NUIT ou email" aria-label="Procurar comerciante" />
    </form>
  )
  return (
    <Shell env="admin" groups={groups} title={titleFor(groups, path, 'RIHA Ops')} top={has('merchants', 'approvals', 'pricing') ? search : undefined}
      me={{ name: me!.user.name, sub: me!.user.title || me!.user.email }} onLogout={async () => { await logout(); nav('/') }}>
      {blocked ? <Page><Empty icon="lock" title="Sem acesso a esta área" text="Peça a quem gere a equipa para lhe dar esta permissão." /></Page> : <Outlet />}
    </Shell>
  )
}
