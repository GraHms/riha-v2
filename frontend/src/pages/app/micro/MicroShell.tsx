import { useState } from 'react'
import { Navigate, Outlet, useLocation, useNavigate } from 'react-router-dom'
import { Icon, Modal, QR } from '../../../components/ui'
import { useQuery } from '../../../core/useQuery'
import { useSession } from '../../../lib/session'
import { NOT_APPROVED, useShell } from '../AppShell'
import { CSS, type MicroCtx } from './common'
import { NewLoan } from './Loans'

const TABS: [string, string][] = [['', 'Resumo'], ['pedidos', 'Pedidos'], ['emprestimos', 'Empréstimos'], ['produtos', 'Produtos'], ['definicoes', 'Definições']]

export function RequestLink({ url, name, compact }: { url: string; name: string; compact?: boolean }) {
  const { toast } = useSession()
  const copy = async () => { try { await navigator.clipboard.writeText(url); toast('Link copiado') } catch { toast('Não foi possível copiar', true) } }
  const wa = 'https://wa.me/?text=' + encodeURIComponent(`Peça o seu crédito à ${name} aqui: ${url}`)
  const downloadQR = () => {
    const svg = document.querySelector('#mc-qr svg')
    if (!svg) return
    const blob = new Blob([new XMLSerializer().serializeToString(svg)], { type: 'image/svg+xml' })
    const a = document.createElement('a')
    a.href = URL.createObjectURL(blob); a.download = 'riha-credito-qr.svg'; a.click(); URL.revokeObjectURL(a.href)
  }
  return (
    <div style={{ display: 'flex', gap: 18, alignItems: 'center', flexWrap: 'wrap' }}>
      <div id="mc-qr" style={{ padding: 8, borderRadius: 14, border: '1px solid var(--line)', background: '#fff', flex: 'none' }}><QR value={url} size={compact ? 96 : 112} logo={false} /></div>
      <div style={{ flex: 1, minWidth: 220 }}>
        <div style={{ fontSize: 13, color: 'var(--muted)' }}>Link de pedido</div>
        <div className="mono" style={{ fontSize: 14.5, marginTop: 4, wordBreak: 'break-all' }}>{url.replace(/^https?:\/\//, '')}</div>
        <div style={{ display: 'flex', gap: 8, marginTop: 12, flexWrap: 'wrap' }}>
          <button className="btn btn-ink btn-sm" style={{ height: 38, padding: '0 14px' }} onClick={copy}>Copiar link</button>
          <a className="btn btn-outline btn-sm" style={{ height: 38, padding: '0 14px' }} href={wa} target="_blank" rel="noreferrer">WhatsApp</a>
          <button className="btn btn-outline btn-sm" style={{ height: 38, padding: '0 14px' }} onClick={downloadQR}>Descarregar QR</button>
        </div>
      </div>
    </div>
  )
}

export default function MicroShell() {
  const nav = useNavigate()
  const loc = useLocation()
  const shell = useShell()
  const { toast } = useSession()
  const [version, setVersion] = useState(0)
  const [newLoan, setNewLoan] = useState(false)
  const [share, setShare] = useState(false)
  const enabled = !!shell.merchant.microcredit
  const prods = useQuery(enabled ? g => g.microcredit.products() : null, [shell.version, version])
  const reqs = useQuery(enabled ? g => g.microcredit.requests('decidir', '', '') : null, [shell.version, version])
  if (!enabled) return <Navigate to="/painel" replace />
  const rest = loc.pathname.replace(/^\/painel\/microcredito\/?/, '')
  const tab = rest.split('/')[0]
  const guard = (f: () => void) => () => { if (!shell.approved) { toast(NOT_APPROVED, true); return } f() }
  const products = prods.data?.data || []
  const requestUrl = prods.data?.request_url || ''
  const ctx: MicroCtx = {
    ...shell, version: version + shell.version, bump: () => setVersion(v => v + 1), products, reloadProducts: prods.reload, requestUrl,
    pending: reqs.data?.counts?.decidir || 0, newLoan: guard(() => setNewLoan(true)), shareLink: () => setShare(true),
  }
  return (
    <div className="page">
      <style>{CSS}</style>
      <div className="mc-head">
        <div style={{ flex: 1, minWidth: 0 }}>
          <button className="only-mobile icon-btn" onClick={() => nav('/painel')} aria-label="Voltar" style={{ marginBottom: 10 }}><Icon name="arrowLeft" size={18} /></button>
          <div style={{ fontSize: 13, color: 'var(--muted)' }}>Serviços</div>
          <h1 className="display mc-title" style={{ margin: '2px 0 0', fontSize: 32, letterSpacing: '-0.03em' }}>Microcrédito</h1>
        </div>
        <div className="mc-actions">
          <button className="btn btn-outline secondary" onClick={() => setShare(true)}>Link de pedido</button>
          <button className="btn btn-outline secondary" onClick={guard(() => nav('/painel/microcredito/produtos/novo'))}>Novo produto</button>
          <button className="btn btn-green" onClick={ctx.newLoan}><Icon name="plus" size={17} stroke={2.4} />Novo empréstimo</button>
        </div>
      </div>
      <div className="tabs" style={{ marginTop: 18 }}>
        {TABS.map(([k, l]) => (
          <button key={k} className={tab === k ? 'on' : ''} onClick={() => nav('/painel/microcredito' + (k ? '/' + k : ''))}>
            {l}{k === 'pedidos' && ctx.pending > 0 && <span className="count" style={{ background: 'var(--sun)', color: '#3F3200' }}>{ctx.pending}</span>}
          </button>
        ))}
      </div>
      <div style={{ marginTop: 22 }}><Outlet context={ctx} /></div>
      {newLoan && <NewLoan products={products.filter((p: any) => p.status === 'active')} onClose={() => setNewLoan(false)} onDone={id => { setNewLoan(false); setVersion(v => v + 1); nav('/painel/microcredito/emprestimos/' + id) }} />}
      {share && (
        <Modal onClose={() => setShare(false)} width={560}>
          <div style={{ padding: 24 }}>
            <div className="between"><div className="display" style={{ fontSize: 22 }}>Link de pedido</div><button className="icon-btn" onClick={() => setShare(false)} aria-label="Fechar"><Icon name="close" size={16} /></button></div>
            <p className="muted" style={{ fontSize: 14, lineHeight: 1.5, margin: '8px 0 18px' }}>O cliente escolhe o produto, simula a prestação e envia o pedido. O QR pode ficar impresso no balcão.</p>
            {requestUrl ? <RequestLink url={requestUrl} name={shell.merchant.name} /> : <p className="muted">Crie um produto primeiro.</p>}
          </div>
        </Modal>
      )}
    </div>
  )
}
