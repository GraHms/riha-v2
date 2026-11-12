import { Navigate, Outlet, useLocation, useNavigate } from 'react-router-dom'
import { can } from '../../core/model'
import { useQuery } from '../../core/useQuery'
import { useSession } from '../../lib/session'
import { Shell, titleFor, type NavGroup } from '../../components/ops'

// A partner's own account at RIHA: only its integrations, money and people.
export default function PartnerShell() {
  const { me, logout } = useSession()
  const nav = useNavigate()
  const path = useLocation().pathname
  const payouts = can(me, 'payouts')
  const { data } = useQuery(payouts ? g => g.partner.payouts('to_pay') : null, [path], 30000)
  const groups: NavGroup[] = [{
    items: [
      ...(can(me, 'money') ? [{ to: '/parceiro', icon: 'chart', label: 'Resumo', exact: true }] : []),
      ...(payouts ? [{ to: '/parceiro/levantamentos', icon: 'wallet', label: 'Levantamentos', count: data?.data?.length, hot: true }] : []),
      { to: '/parceiro/equipa', icon: 'users', label: 'Equipa' },
    ],
  }]
  if (path === '/parceiro' && !can(me, 'money')) return <Navigate to={payouts ? '/parceiro/levantamentos' : '/parceiro/equipa'} replace />
  return (
    <Shell env="partner" org={me?.partner?.name} groups={groups} title={titleFor(groups, path, me?.partner?.name || 'Parceiro')}
      me={{ name: me!.user.name, sub: me!.user.title || me!.user.email }} onLogout={async () => { await logout(); nav('/') }}>
      <Outlet />
    </Shell>
  )
}
