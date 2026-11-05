import { lazy, Suspense, useEffect, type ReactNode } from 'react'
import { Navigate, Route, Routes, useLocation } from 'react-router-dom'
import { useSession } from './lib/session'
import { Login, Signup, ForgotPassword, ResetPassword } from './pages/public/Auth'
import Checkout from './pages/public/Checkout'
import LinkLanding from './pages/public/LinkLanding'
import { Spinner } from './components/ui'

const Landing = lazy(() => import('./pages/public/Landing'))
const SubsShell = lazy(() => import('./pages/app/subs/SubsShell'))
const Summary = lazy(() => import('./pages/app/subs/Summary'))
const PlanDetail = lazy(() => import('./pages/app/subs/Plans').then(m => ({ default: m.PlanDetail })))
const PlanForm = lazy(() => import('./pages/app/subs/Plans').then(m => ({ default: m.PlanForm })))
const PlansList = lazy(() => import('./pages/app/subs/Plans').then(m => ({ default: m.PlansList })))
const Members = lazy(() => import('./pages/app/subs/Members'))
const Member = lazy(() => import('./pages/app/subs/Member'))
const Catalog = lazy(() => import('./pages/app/subs/Catalog'))
const SubsSettings = lazy(() => import('./pages/app/subs/SubsSettings'))
const CatalogPage = lazy(() => import('./pages/public/subs/Subscribe').then(m => ({ default: m.CatalogPage })))
const PlanPage = lazy(() => import('./pages/public/subs/Subscribe').then(m => ({ default: m.PlanPage })))
const Invoice = lazy(() => import('./pages/public/subs/Invoice'))
const Kyc = lazy(() => import('./pages/public/Kyc'))
const AppShell = lazy(() => import('./pages/app/AppShell'))
const Overview = lazy(() => import('./pages/app/Overview'))
const Transactions = lazy(() => import('./pages/app/Transactions'))
const Links = lazy(() => import('./pages/app/Links'))
const Payouts = lazy(() => import('./pages/app/Payouts'))
const Settings = lazy(() => import('./pages/app/Settings'))
const NewLinkPage = lazy(() => import('./pages/app/NewLinkPage'))
const AdminShell = lazy(() => import('./pages/admin/AdminShell'))
const AdminOverview = lazy(() => import('./pages/admin/AdminOverview'))
const Approvals = lazy(() => import('./pages/admin/Approvals'))
const AdminPayouts = lazy(() => import('./pages/admin/AdminPayouts'))
const Merchants = lazy(() => import('./pages/admin/Merchants'))
const MerchantDetail = lazy(() => import('./pages/admin/MerchantDetail'))
const Docs = lazy(() => import('./pages/public/Docs'))

// On a catalog's own subdomain the server names the catalog in the page, and
// only the catalog, its plans and the invoices they lead to live there.
const meta = (name: string) => document.querySelector(`meta[name="${name}"]`)?.getAttribute('content') || ''
const CATALOG = meta('riha-catalog')

function ToMainSite() {
  const { pathname, search } = useLocation()
  useEffect(() => { window.location.replace((meta('riha-site') || '') + pathname + search) }, [pathname, search])
  return <Loading />
}

function CatalogSite() {
  return (
    <Suspense fallback={<Loading />}>
      <Routes>
        <Route path="/" element={<CatalogPage catalog={CATALOG} />} />
        <Route path="/i/:token" element={<Invoice />} />
        {['/entrar', '/registo', '/recuperar', '/docs', '/painel/*', '/admin/*', '/verificacao'].map(p => <Route key={p} path={p} element={<ToMainSite />} />)}
        <Route path="/:slug" element={<PlanPage catalog={CATALOG} />} />
        <Route path="*" element={<ToMainSite />} />
      </Routes>
    </Suspense>
  )
}

function Loading() {
  return <div style={{ height: '100%', display: 'grid', placeItems: 'center' }}><Spinner dark /></div>
}

function RequireMerchant({ children }: { children: ReactNode }) {
  const { me, loading } = useSession()
  const loc = useLocation()
  if (loading) return <Loading />
  if (!me) return <Navigate to={'/entrar?next=' + encodeURIComponent(loc.pathname)} replace />
  if (me.user.role === 'admin') return <Navigate to="/admin" replace />
  return <>{children}</>
}

function RequireAdmin({ children }: { children: ReactNode }) {
  const { me, loading } = useSession()
  if (loading) return <Loading />
  if (!me) return <Navigate to="/entrar?next=/admin" replace />
  if (me.user.role !== 'admin') return <Navigate to="/painel" replace />
  return <>{children}</>
}

// The server writes the first title. These keep it right as people move around,
// and pages it wrote for a specific payment are left alone.
const TITLES: [string, string][] = [
  ['/docs', 'API de pagamentos M-Pesa para Moçambique: documentação · RIHA'],
  ['/registo', 'Criar conta para receber pagamentos M-Pesa · RIHA'],
  ['/entrar', 'Entrar · RIHA'], ['/recuperar', 'Recuperar palavra-passe · RIHA'], ['/verificacao', 'Verificação · RIHA'],
  ['/painel', 'Painel · RIHA'], ['/admin', 'Administração · RIHA'],
]

function PageTitle() {
  const { pathname } = useLocation()
  useEffect(() => {
    if (pathname === '/') document.title = 'RIHA: pagamentos online em Moçambique com M-Pesa'
    else {
      const t = TITLES.find(([p]) => pathname.startsWith(p))
      if (t) document.title = t[1]
    }
  }, [pathname])
  return null
}

export default function App() {
  if (CATALOG) return <CatalogSite />
  return (
    <Suspense fallback={<Loading />}>
    <PageTitle />
    <Routes>
      <Route path="/" element={<Suspense fallback={<Loading />}><Landing /></Suspense>} />
      <Route path="/docs" element={<Suspense fallback={<Loading />}><Docs /></Suspense>} />
      <Route path="/entrar" element={<Login />} />
      <Route path="/registo" element={<Signup />} />
      <Route path="/recuperar" element={<ForgotPassword />} />
      <Route path="/recuperar/nova" element={<ResetPassword />} />
      <Route path="/verificacao" element={<RequireMerchant><Kyc /></RequireMerchant>} />
      <Route path="/c/:id" element={<Checkout />} />
      <Route path="/p/:merchant/:slug" element={<LinkLanding />} />
      <Route path="/s/:merchant/:slug" element={<PlanPage />} />
      <Route path="/a/:merchant" element={<CatalogPage />} />
      <Route path="/i/:token" element={<Invoice />} />

      <Route path="/painel" element={<RequireMerchant><AppShell /></RequireMerchant>}>
        <Route index element={<Overview />} />
        <Route path="transacoes" element={<Transactions />} />
        <Route path="transacoes/:id" element={<Transactions />} />
        <Route path="links" element={<Links />} />
        <Route path="links/novo" element={<NewLinkPage />} />
        <Route path="saldo" element={<Payouts />} />
        <Route path="definicoes" element={<Settings />} />
        <Route path="definicoes/:tab" element={<Settings />} />
        <Route path="assinaturas" element={<SubsShell />}>
          <Route index element={<Summary />} />
          <Route path="planos" element={<PlansList />} />
          <Route path="planos/novo" element={<PlanForm />} />
          <Route path="planos/:id" element={<PlanDetail />} />
          <Route path="planos/:id/editar" element={<PlanForm />} />
          <Route path="membros" element={<Members />} />
          <Route path="membros/:id" element={<Member />} />
          <Route path="catalogo" element={<Catalog />} />
          <Route path="definicoes" element={<SubsSettings />} />
        </Route>
      </Route>

      <Route path="/admin" element={<RequireAdmin><AdminShell /></RequireAdmin>}>
        <Route index element={<AdminOverview />} />
        <Route path="aprovacoes" element={<Approvals />} />
        <Route path="levantamentos" element={<AdminPayouts />} />
        <Route path="comerciantes" element={<Merchants />} />
        <Route path="comerciantes/:id" element={<MerchantDetail />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
    </Suspense>
  )
}
