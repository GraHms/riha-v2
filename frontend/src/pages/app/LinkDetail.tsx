import { useState, type ReactNode } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { Drawer, Icon, MethodIcon, Pill, Skeleton } from '../../components/ui'
import { useQuery } from '../../core/useQuery'
import { copy, fmt, METHOD_NAME, shortDate } from '../../lib/format'
import { useSession } from '../../lib/session'
import { useShell } from './AppShell'
import { LinkMenu, linkState, useLinkActions } from './Links'
import { SharePanel, type LinkObj } from './NewLink'
import { submitWords } from '../public/LinkCard'
import { TxLine } from './Overview'

function Row({ k, children, last }: { k: string; children: ReactNode; last?: boolean }) {
  return (
    <div style={{ display: 'grid', gridTemplateColumns: 'minmax(96px,150px) minmax(0,1fr)', gap: 14, padding: '12px 0', borderBottom: last ? 0 : '1px solid var(--line-3)', fontSize: 14 }}>
      <span style={{ color: 'var(--muted)' }}>{k}</span><span style={{ minWidth: 0, overflowWrap: 'anywhere' }}>{children}</span>
    </div>
  )
}

export default function LinkDetail() {
  const { id } = useParams()
  const nav = useNavigate()
  const { version } = useShell()
  const { toast } = useSession()
  const link = useQuery(g => g.links.get(id!), [id, version])
  const pays = useQuery(g => g.payments.list({ link: id, limit: 50 }), [id, version])
  const [share, setShare] = useState(false)
  const a = useLinkActions(link.reload)
  const l = link.data as LinkObj | undefined
  if (!l) return <div className="page"><Skeleton h={420} r={22} /></div>
  const st = linkState(l)
  const rows: any[] = pays.data?.data || []
  const range = l.min_amount || l.max_amount ? ` · entre ${fmt(l.min_amount || 1000, false)} e ${fmt(l.max_amount || 100000000, false)} MT` : ''
  return (
    <div className="page">
      <style>{`
        .ld-head{display:flex;gap:14px;align-items:flex-start;justify-content:space-between;flex-wrap:wrap;margin-top:8px}
        .ld-title{min-width:0;flex:1}
        @media(max-width:700px){.ld-title{flex-basis:100%}.ld-actions{width:100%}.ld-actions>.btn{flex:1}}
        .ld-stats{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:12px;margin-top:20px}
        .ld-grid{display:grid;grid-template-columns:minmax(0,1.5fr) minmax(0,1fr);gap:16px;margin-top:16px;align-items:start}
        @media(max-width:1000px){.ld-grid{grid-template-columns:minmax(0,1fr)}}
        @media(max-width:600px){.ld-stats{gap:8px}.ld-stats .card{padding:14px!important}.ld-stats .display{font-size:19px!important}}
      `}</style>
      <Link to="/painel/links" style={{ fontSize: 13.5, color: 'var(--text-2)' }}>‹ Links de pagamento</Link>
      <div className="ld-head">
        <div className="ld-title">
          <div className="row" style={{ gap: 10, flexWrap: 'wrap' }}><h1 className="h1" style={{ margin: 0 }}>{l.name}</h1><Pill {...st} /></div>
          <button onClick={async () => { await copy(l.url); toast('Link copiado') }} className="mono" style={{ border: 0, background: 'none', padding: 0, marginTop: 6, fontSize: 13, color: 'var(--muted)', cursor: 'pointer', textAlign: 'left', wordBreak: 'break-all' }} title="Copiar">{l.url.replace(/^https?:\/\//, '')} <Icon name="copy" size={12} /></button>
        </div>
        <div className="row ld-actions" style={{ gap: 8 }}>
          <button className="btn btn-ink" onClick={() => setShare(true)}><Icon name="share" size={16} />Partilhar</button>
          {!l.archived && <button className="btn btn-outline" onClick={() => a.edit(l)}>Editar</button>}
          <LinkMenu l={l} onShare={() => setShare(true)} onChanged={link.reload} />
        </div>
      </div>
      {l.archived && <div className="form-err" style={{ marginTop: 14, background: 'var(--paper-2)', color: 'var(--text-2)' }}>Este link está arquivado e não aceita pagamentos. <button className="link-btn" onClick={() => a.archive(l, false)}>Restaurar</button></div>}
      {!l.archived && !l.active && <div className="form-err" style={{ marginTop: 14, background: 'var(--sun-soft)', color: '#5B4600' }}>Este link está desativado. Quem o abrir vê que já não está ativo. <button className="link-btn" onClick={() => a.setActive(l, true)}>Ativar</button></div>}

      <div className="ld-stats">
        <div className="card" style={{ padding: 18 }}><div style={{ fontSize: 13, color: 'var(--muted)' }}>Pagamentos</div><div className="display tnum" style={{ fontSize: 26, marginTop: 4 }}>{l.payments_count}</div></div>
        <div className="card" style={{ padding: 18 }}><div style={{ fontSize: 13, color: 'var(--muted)' }}>Recebido</div><div className="display tnum" style={{ fontSize: 26, marginTop: 4 }}>{fmt(l.total_received, false)} <span style={{ fontSize: 13, color: 'var(--muted)' }}>MT</span></div></div>
        <div className="card" style={{ padding: 18 }}><div style={{ fontSize: 13, color: 'var(--muted)' }}>Último pagamento</div><div className="display" style={{ fontSize: 26, marginTop: 4 }}>{l.last_payment_at ? shortDate(l.last_payment_at) : '—'}</div></div>
      </div>

      <div className="ld-grid">
        <div className="table" style={{ borderRadius: 22, padding: '6px 0' }}>
          <div className="between" style={{ padding: '14px 20px 10px' }}><div className="h2">Pagamentos por este link</div>{rows.length > 0 && <span style={{ fontSize: 13, color: 'var(--muted)' }}>{pays.data?.total} no total</span>}</div>
          <div style={{ borderTop: '1px solid var(--line-3)' }}>
            {!pays.data && <div style={{ padding: 16 }}><Skeleton h={120} /></div>}
            {rows.map(p => (
              <div key={p.id}>
                <div className="only-desktop"><TxLine p={p} desktop onClick={() => nav('/painel/transacoes/' + p.id)} /></div>
                <div className="only-mobile"><TxLine p={p} onClick={() => nav('/painel/transacoes/' + p.id)} /></div>
              </div>
            ))}
            {pays.data && !rows.length && <div className="empty">Ainda ninguém pagou por este link. <button className="link-btn" onClick={() => setShare(true)}>Partilhe-o</button></div>}
          </div>
        </div>
        <div className="card" style={{ padding: '6px 20px' }}>
          <Row k="Valor">{l.custom_amount ? <>O cliente escolhe{l.suggested_amount ? ` · sugerido ${fmt(l.suggested_amount, false)} MT` : ''}{range}</> : <b className="tnum">{fmt(l.amount)} MT</b>}</Row>
          <Row k="Métodos"><span style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>{l.payment_method_types.map(m => <span key={m} className="row" style={{ gap: 5 }}><MethodIcon method={m} size={18} radius={6} />{METHOD_NAME[m]}</span>)}</span></Row>
          <Row k="Uso">{l.reusable ? 'Vários clientes podem pagar' : 'Pago uma só vez'}{l.type === 'qr' ? ' · QR para o balcão' : ''}</Row>
          <Row k="Botão">{submitWords(l.submit_type).label}</Row>
          <Row k="Expira">{l.expires_at ? new Date(l.expires_at * 1000).toLocaleDateString('pt-PT', { day: 'numeric', month: 'long', year: 'numeric' }) : 'Nunca'}</Row>
          {l.details && <Row k="Descrição"><span style={{ whiteSpace: 'pre-line' }}>{l.details}</span></Row>}
          {l.image_url && <Row k="Imagem"><img src={l.image_url} alt="" style={{ width: 140, borderRadius: 10, aspectRatio: '16 / 9', objectFit: 'cover', display: 'block' }} /></Row>}
          <Row k="Depois de pagar" last>{l.redirect_url ? <>Volta para <b>{(() => { try { return new URL(l.redirect_url).host } catch { return l.redirect_url } })()}</b></> : l.after_message ? <span style={{ whiteSpace: 'pre-line' }}>{l.after_message}</span> : 'Vê a confirmação e o recibo'}</Row>
        </div>
      </div>
      {share && <Drawer title={l.name} onClose={() => setShare(false)}><div style={{ padding: '28px 26px' }}><SharePanel link={l} compact /></div></Drawer>}
    </div>
  )
}
