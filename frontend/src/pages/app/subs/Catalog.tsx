import { useEffect, useState } from 'react'
import { Avatar, Icon, QR, Skeleton, Spinner, Toggle } from '../../../components/ui'
import { useGateway } from '../../../core/gateway'
import { errMsg } from '../../../core/model'
import { useQuery } from '../../../core/useQuery'
import { copy, initials } from '../../../lib/format'
import { useSession } from '../../../lib/session'
import { useShell } from '../AppShell'
import { downloadSvg, money } from './common'
import { useSubs } from './SubsShell'

export default function Catalog() {
  const g = useGateway()
  const { toast } = useSession()
  const shell = useShell()
  const sx = useSubs()
  const settings = useQuery(g => g.subscriptions.settings(), [sx.version])
  const [rows, setRows] = useState<any[]>([])
  const [multi, setMulti] = useState(true)
  const [dirty, setDirty] = useState(false)
  const [busy, setBusy] = useState(false)
  useEffect(() => { setRows(sx.plans.filter((p: any) => p.status !== 'archived').map((p: any) => ({ ...p }))); setDirty(false) }, [sx.plans])
  useEffect(() => { if (settings.data) setMulti(settings.data.catalog_multi) }, [settings.data])
  const url = sx.catalogUrl
  const move = (i: number, d: number) => { const j = i + d; if (j < 0 || j >= rows.length) return; const n = rows.slice(); [n[i], n[j]] = [n[j], n[i]]; setRows(n); setDirty(true) }
  const save = async () => {
    setBusy(true)
    try { await g.subscriptions.saveCatalog(rows.map(r => ({ plan_id: r.id, in_catalog: r.in_catalog })), multi); toast('Catálogo guardado'); setDirty(false); sx.reloadPlans() }
    catch (e) { toast(errMsg(e), true) } finally { setBusy(false) }
  }
  if (!settings.data) return <Skeleton h={300} r={22} />
  const shown = rows.filter(r => r.in_catalog && r.status === 'active')
  return (
    <div style={{ display: 'grid', gridTemplateColumns: 'minmax(0,1.5fr) minmax(260px,1fr)', gap: 18, alignItems: 'start' }} className="cat-grid">
      <style>{`@media (max-width:1000px){.cat-grid{grid-template-columns:1fr!important}}`}</style>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 14, minWidth: 0 }}>
        <div className="card" style={{ padding: 16, display: 'flex', gap: 16, alignItems: 'center', flexWrap: 'wrap' }}>
          <div id="cat-qr" style={{ padding: 6, borderRadius: 12, boxShadow: 'inset 0 0 0 1px #EEECE5' }}><QR value={url || window.location.origin} size={96} logo={false} /></div>
          <div style={{ flex: 1, minWidth: 200 }}>
            <div style={{ fontSize: 13, color: 'var(--muted)' }}>Link do catálogo</div>
            <div className="mono" style={{ fontSize: 14, marginTop: 4, wordBreak: 'break-all' }}>{url.replace(/^https?:\/\//, '')}</div>
            <div style={{ display: 'flex', gap: 8, marginTop: 12, flexWrap: 'wrap' }}>
              <button className="btn btn-ink btn-sm" onClick={async () => { await copy(url); toast('Link copiado') }}><Icon name="copy" size={14} />Copiar link</button>
              <a className="btn btn-outline btn-sm" href={`https://wa.me/?text=${encodeURIComponent(`${shell.merchant.name}: escolha e pague aqui ${url}`)}`} target="_blank" rel="noreferrer"><Icon name="whatsapp" size={14} />WhatsApp</a>
              <button className="btn btn-outline btn-sm" onClick={() => downloadSvg('#cat-qr', 'qr-catalogo.svg')}>Descarregar QR</button>
            </div>
          </div>
        </div>
        <div className="card" style={{ overflow: 'hidden' }}>
          <div className="between" style={{ padding: '14px 20px', borderBottom: '1px solid var(--line-3)' }}><b>Planos no catálogo</b><span style={{ fontSize: 13, color: 'var(--muted)' }}>Use as setas para ordenar</span></div>
          {rows.length === 0 && <div style={{ padding: 20, color: 'var(--muted)', fontSize: 14 }}>Ainda não há planos.</div>}
          {rows.map((r, i) => (
            <div key={r.id} style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '12px 20px', borderBottom: '1px solid var(--line-3)' }}>
              <div style={{ display: 'flex', flexDirection: 'column' }}>
                <button className="icon-btn" style={{ width: 22, height: 18, border: 0 }} disabled={i === 0} onClick={() => move(i, -1)} aria-label="Subir">▲</button>
                <button className="icon-btn" style={{ width: 22, height: 18, border: 0 }} disabled={i === rows.length - 1} onClick={() => move(i, 1)} aria-label="Descer">▼</button>
              </div>
              <div style={{ flex: 1, minWidth: 0 }}><div style={{ fontWeight: 600, fontSize: 14 }}>{r.name}</div><div style={{ fontSize: 12.5, color: 'var(--muted)' }}>{money(r.amount)} MT · {r.status === 'draft' ? 'rascunho' : `${r.stats?.members || 0} membros`}</div></div>
              <Toggle on={r.in_catalog && r.status === 'active'} label={'Mostrar ' + r.name} onChange={v => { if (r.status !== 'active') { toast('Ative o plano primeiro.', true); return } setRows(rows.map(x => (x.id === r.id ? { ...x, in_catalog: v } : x))); setDirty(true) }} />
            </div>
          ))}
          <div style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '14px 20px' }}>
            <div style={{ flex: 1 }}><div style={{ fontWeight: 600, fontSize: 14 }}>Permitir escolher vários</div><div style={{ fontSize: 12.5, color: 'var(--muted)' }}>Por exemplo propina e transporte, pagos juntos no mesmo link.</div></div>
            <Toggle on={multi} label="Permitir escolher vários" onChange={v => { setMulti(v); setDirty(true) }} />
          </div>
        </div>
        {dirty && <div style={{ display: 'flex', justifyContent: 'flex-end' }}><button className="btn btn-green" disabled={busy} onClick={save}>{busy ? <Spinner /> : 'Guardar catálogo'}</button></div>}
      </div>
      <div>
        <div style={{ fontSize: 12, fontWeight: 700, color: 'var(--muted)', textTransform: 'uppercase', letterSpacing: '.06em', margin: '4px 0 10px' }}>O que o cliente vê</div>
        <div className="card" style={{ padding: 18 }}>
          <div style={{ display: 'flex', gap: 12, alignItems: 'center' }}><Avatar text={initials(shell.merchant.name)} color={shell.merchant.avatar_color} size={40} radius={12} /><div><div style={{ fontWeight: 700 }}>{shell.merchant.name}</div><div style={{ fontSize: 12.5, color: 'var(--muted)' }}>Planos disponíveis</div></div></div>
          <div className="display" style={{ fontSize: 18, marginTop: 16 }}>O que quer pagar?</div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 8, marginTop: 10 }}>
            {shown.length === 0 && <div style={{ fontSize: 13, color: 'var(--muted)' }}>Nenhum plano visível.</div>}
            {shown.map(r => <div key={r.id} className="between" style={{ padding: '12px 14px', borderRadius: 12, border: '1px solid #E7E5DD', fontSize: 13.5 }}><b>{r.name}</b><span className="tnum" style={{ fontWeight: 600 }}>{money(r.amount)} MT</span></div>)}
          </div>
        </div>
      </div>
    </div>
  )
}
