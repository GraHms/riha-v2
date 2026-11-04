import { lazy, Suspense, type ReactNode } from 'react'
import { Navigate, Route, Routes, useLocation } from 'react-router-dom'
import { useSession } from './lib/session'
import Landing from './pages/public/Landing'
import { Login, Signup } from './pages/public/Auth'
import Kyc from './pages/public/Kyc'
import Checkout from './pages/public/Checkout'
import LinkLanding from './pages/public/LinkLanding'
import AppShell from './pages/app/AppShell'
import Overview from './pages/app/Overview'
import Transactions from './pages/app/Transactions'
import Links from './pages/app/Links'
import Payouts from './pages/app/Payouts'
import Settings from './pages/app/Settings'
import NewLinkPage from './pages/app/NewLinkPage'
import AdminShell from './pages/admin/AdminShell'
import AdminOverview from './pages/admin/AdminOverview'
import Approvals from './pages/admin/Approvals'
import AdminPayouts from './pages/admin/AdminPayouts'
import Merchants from './pages/admin/Merchants'
import MerchantDetail from './pages/admin/MerchantDetail'
import { Spinner } from './components/ui'
import SubsShell from './pages/app/subs/SubsShell'
import Summary from './pages/app/subs/Summary'
import { PlanDetail, PlanForm, PlansList } from './pages/app/subs/Plans'
import Members from './pages/app/subs/Members'
import Member from './pages/app/subs/Member'
import Catalog from './pages/app/subs/Catalog'
import SubsSettings from './pages/app/subs/SubsSettings'
import { CatalogPage, PlanPage } from './pages/public/subs/Subscribe'
import Invoice from './pages/public/subs/Invoice'

const Docs = lazy(() => import('./pages/public/Docs'))

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

export default function App() {
  return (
    <Routes>
      <Route path="/" element={<Landing />} />
      <Route path="/docs" element={<Suspense fallback={<Loading />}><Docs /></Suspense>} />
      <Route path="/entrar" element={<Login />} />
      <Route path="/registo" element={<Signup />} />
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
  )
}
