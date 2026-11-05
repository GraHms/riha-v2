import { useState } from 'react'
import { Outlet, useLocation, useNavigate, useOutletContext } from 'react-router-dom'
import { Icon } from '../../../components/ui'
import { useQuery } from '../../../core/useQuery'
import { useSession } from '../../../lib/session'
import { NOT_APPROVED, useShell, type ShellCtx } from '../AppShell'
import AddMembers from './AddMembers'

export type SubsCtx = ShellCtx & {
  plans: any[]; reloadPlans: () => void; openAdd: (tab?: 'one' | 'import', planId?: string) => void; version: number; bump: () => void; catalogUrl: string; approved: boolean
  // The catalog page's settings, for the previews of what customers see.
  page: any; reloadPage: () => void; newPlan: () => void
}
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
  const ov = useQuery(g => g.subscriptions.overview(), [shell.version, version])
  const page = useQuery(g => g.subscriptions.catalog(), [shell.version, version])
  const plans = data?.data || []
  const rest = loc.pathname.replace(/^\/painel\/assinaturas\/?/, '')
  const tab = rest.split('/')[0]
  const guard = (f: () => void) => () => { if (!shell.approved) { toast(NOT_APPROVED, true); return } f() }
  const openAdd = (t: 'one' | 'import' = 'one', planId?: string) => guard(() => setAdd({ tab: t, planId }))()
  const newPlan = guard(() => nav('/painel/assinaturas/planos/novo'))
  const catalogUrl = page.data?.url || ov.data?.catalog_url || ''
  const ctx: SubsCtx = { ...shell, plans, reloadPlans: reload, openAdd, version: version + shell.version, bump: () => setVersion(v => v + 1), catalogUrl, approved: shell.approved,
    page: page.data, reloadPage: page.reload, newPlan }
  // One main action per tab, in the same place on a phone and on a computer.
  const inForm = rest.split('/').length > 1
  const action = inForm ? null
    : tab === 'planos' ? <button className="btn btn-green" onClick={newPlan}><Icon name="plus" size={17} stroke={2.4} />Novo plano</button>
    : tab === 'catalogo' ? (catalogUrl ? <a className="btn btn-outline" href={catalogUrl} target="_blank" rel="noreferrer"><Icon name="arrowUpRight" size={16} />Abrir página</a> : null)
    : tab === 'definicoes' ? null
    : <button className="btn btn-green" onClick={() => openAdd('one')}><Icon name="plus" size={17} stroke={2.4} />Adicionar membros</button>
  return (
    <div className="page">
      <style>{`
        .subs-head{display:flex;align-items:flex-end;gap:12px;flex-wrap:wrap}
        .subs-actions{display:flex;gap:8px;flex-wrap:wrap}
        @media (max-width:860px){.subs-title{font-size:26px!important}.subs-actions .btn{height:40px;padding:0 14px;font-size:14px}.subs-actions .secondary{display:none}}
      `}</style>
      <div className="subs-head">
        <div style={{ flex: 1, minWidth: 0 }}>
          <button className="only-mobile icon-btn" onClick={() => nav('/painel')} aria-label="Voltar" style={{ marginBottom: 10 }}><Icon name="arrowLeft" size={18} /></button>
          <div style={{ fontSize: 13, color: 'var(--muted)' }}>Serviços</div>
          <h1 className="display subs-title" style={{ margin: '2px 0 0', fontSize: 32, letterSpacing: '-0.03em' }}>Assinaturas</h1>
        </div>
        <div className="subs-actions">
          {tab === '' && <button className="btn btn-outline secondary" onClick={() => nav('/painel/assinaturas/catalogo')}>Partilhar catálogo</button>}
          {action}
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
