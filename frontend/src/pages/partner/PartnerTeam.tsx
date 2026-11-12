import { useGateway } from '../../core/gateway'
import { useQuery } from '../../core/useQuery'
import { useSession } from '../../lib/session'
import { Skeleton } from '../../components/ui'
import { Page, PageHead } from '../../components/ops'
import { TeamList } from '../admin/TeamList'

export default function PartnerTeam() {
  const g = useGateway()
  const { me } = useSession()
  const { data, reload } = useQuery(g => g.partner.team())
  return (
    <Page>
      <PageHead title="Equipa" sub={me?.partner?.name ? `Quem entra na conta de ${me.partner.name}.` : 'Quem entra nesta conta.'} />
      {!data ? <Skeleton h={140} /> : (
        <TeamList members={data.data} perms={data.permissions} canManage={data.can_manage} meId={me!.user.id} onChange={reload}
          api={{ invite: g.partner.invite, setAccess: g.partner.setAccess, resend: g.partner.resendInvite }} />
      )}
    </Page>
  )
}
