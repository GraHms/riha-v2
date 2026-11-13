import { Navigate, useNavigate } from 'react-router-dom'
import { Icon, Skeleton } from '../../../components/ui'
import { useQuery } from '../../../core/useQuery'
import { fullDate } from '../../../lib/format'
import { useShell } from '../AppShell'

export default function Microcredit() {
  const nav = useNavigate()
  const shell = useShell()
  const enabled = !!shell.merchant.microcredit
  const { data, error } = useQuery(enabled ? g => g.microcredit.status() : null, [shell.version])
  if (!enabled) return <Navigate to="/painel" replace />
  return (
    <div className="page">
      <button className="only-mobile icon-btn" onClick={() => nav('/painel')} aria-label="Voltar" style={{ marginBottom: 10 }}><Icon name="arrowLeft" size={18} /></button>
      <div style={{ fontSize: 13, color: 'var(--muted)' }}>Serviços</div>
      <h1 className="display" style={{ margin: '2px 0 0', fontSize: 32, letterSpacing: '-0.03em' }}>Microcrédito</h1>
      <div className="card card-pad" style={{ marginTop: 22, display: 'flex', gap: 16, alignItems: 'center' }}>
        <span style={{ width: 46, height: 46, borderRadius: 14, background: 'var(--sun-bg)', color: 'var(--sun-text)', display: 'grid', placeItems: 'center', flex: 'none' }}><Icon name="coins" size={22} /></span>
        <div style={{ minWidth: 0 }}>
          {error ? <div style={{ fontSize: 14, color: 'var(--muted)' }}>{error}</div>
            : !data ? <Skeleton h={40} w={260} />
            : <>
                <div style={{ fontWeight: 700, fontSize: 16 }}>O Microcrédito está ativo nesta conta</div>
                <div style={{ fontSize: 14, color: 'var(--muted)', marginTop: 4, lineHeight: 1.5 }}>Ativado pela equipa RIHA em {fullDate(data.since)}. Pedidos, empréstimos e produtos aparecem aqui.</div>
              </>}
        </div>
      </div>
    </div>
  )
}
