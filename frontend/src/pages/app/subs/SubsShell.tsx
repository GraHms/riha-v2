import { useState } from 'react'
import { Outlet, useLocation, useNavigate, useOutletContext } from 'react-router-dom'
import { Icon } from '../../../components/ui'
import { useQuery } from '../../../core/useQuery'
import { useSession } from '../../../lib/session'
import { NOT_APPROVED, useShell, type ShellCtx } from '../AppShell'
import AddMembers from './AddMembers'

export type SubsCtx = ShellCtx & { plans: any[]; reloadPlans: () => void; openAdd: (tab?: 'one' | 'import', planId?: string) => void; version: number; bump: () => void; catalogUrl: string; approved: boolean }
export const useSubs = () => useOutletContext<SubsCtx>()

const TABS: [string, string][] = [['', 'Resumo'], ['planos', 'Planos'], ['membros', 'Membros'], ['catalogo', 'Catálogo'], ['definicoes', 'Definições']]

export default function SubsShell() {
  const nav = useNavigate()
  const loc = useLocation()
  const shell = useShell()
  const { toast } = useSession()
  const [version, setVersion] = useState(0)
  const [add, setAdd] = useState<null | { tab: 'one' | 'import'; planId?: string }>(null)
  const { data, reload } = useQuery(g => g.subscriptions.plans(), [shell.version, version])
  const ov = useQuery(g => g.subscriptions.overview(), [shell.version])
  const plans = data?.data || []
  const rest = loc.pathname.replace(/^\/painel\/assinaturas\/?/, '')
  const tab = rest.split('/')[0]
  const guard = (f: () => void) => () => { if (!shell.approved) { toast(NOT_APPROVED, true); return } f() }
  const openAdd = (t: 'one' | 'import' = 'one', planId?: string) => guard(() => setAdd({ tab: t, planId }))()
  const ctx: SubsCtx = { ...shell, plans, reloadPlans: reload, openAdd, version: version + shell.version, bump: () => setVersion(v => v + 1), catalogUrl: ov.data?.catalog_url || '', approved: shell.approved }
  return (
    <div className="page">
      <style>{`
        .subs-head{display:flex;align-items:flex-end;gap:12px;flex-wrap:wrap}
        .subs-actions{display:flex;gap:8px;flex-wrap:wrap}
        @media (max-width:860px){.subs-actions{display:none}.subs-title{font-size:26px!important}}
      `}</style>
      <div className="subs-head">
        <div style={{ flex: 1, minWidth: 200 }}>
          <button className="only-mobile icon-btn" onClick={() => nav('/painel')} aria-label="Voltar" style={{ marginBottom: 10 }}><Icon name="arrowLeft" size={18} /></button>
          <div style={{ fontSize: 13, color: 'var(--muted)' }}>Serviços</div>
          <h1 className="display subs-title" style={{ margin: '2px 0 0', fontSize: 32, letterSpacing: '-0.03em' }}>Assinaturas</h1>
        </div>
        <div className="subs-actions">
          <button className="btn btn-outline" onClick={() => nav('/painel/assinaturas/catalogo')}>Partilhar catálogo</button>
          <button className="btn btn-outline" onClick={guard(() => nav('/painel/assinaturas/planos/novo'))}>Novo plano</button>
          <button className="btn btn-green" onClick={() => openAdd('one')}><Icon name="plus" size={17} stroke={2.4} />Adicionar membros</button>
        </div>
      </div>
      <div className="tabs" style={{ marginTop: 18 }}>
        {TABS.map(([k, l]) => <button key={k} className={tab === k ? 'on' : ''} onClick={() => nav('/painel/assinaturas' + (k ? '/' + k : ''))}>{l}</button>)}
      </div>
      <div style={{ marginTop: 22 }}>
        <Outlet context={ctx} />
      </div>
      {add && <AddMembers initial={add.tab} planId={add.planId} plans={plans.filter((p: any) => p.status === 'active')} onClose={() => setAdd(null)} onDone={() => { setAdd(null); setVersion(v => v + 1); nav('/painel/assinaturas/membros') }} />}
    </div>
  )
}
