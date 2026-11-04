import { useEffect, useState, type FormEvent } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { errMsg, type ApiError } from '../../core/model'
import { useGateway } from '../../core/gateway'
import { useQuery } from '../../core/useQuery'
import { bpsLabel, fmt, fullDate, methodLine, parseMT, relDate, SOURCE, STATUS } from '../../lib/format'
import { useSession } from '../../lib/session'
import { Icon, MethodIcon, Modal, Pill, Skeleton, Spinner } from '../../components/ui'
import { useShell } from './AppShell'
import { TxLine } from './Overview'

const TABS: [string, string][] = [['all', 'Todas'], ['paid', 'Pagas'], ['pending', 'Pendentes'], ['failed', 'Falhadas']]
const GRID = 'minmax(0,1.4fr) 70px minmax(0,1.1fr) 120px 110px 120px'

export default function Transactions() {
  const g = useGateway()
  const { id } = useParams()
  const [sp, setSp] = useSearchParams()
  const nav = useNavigate()
  const { version } = useShell()
  const tab = sp.get('estado') || 'all'
  const q = sp.get('q') || ''
  const [search, setSearch] = useState(q)
  useEffect(() => setSearch(q), [q])
  const filter = { status: tab, q, limit: 100 }
  const qs = new URLSearchParams({ status: tab === 'all' ? '' : tab, q, limit: '100' }).toString()
  const { data, loading, reload } = useQuery(g => g.payments.list(filter), [qs, version], 8000)
  const rows: any[] = data?.data || []
  const setParam = (k: string, v: string) => { const n = new URLSearchParams(sp); if (v) n.set(k, v); else n.delete(k); setSp(n) }
  const open = (pid: string) => nav('/painel/transacoes/' + pid + (sp.toString() ? '?' + sp : ''))
  const close = () => nav('/painel/transacoes' + (sp.toString() ? '?' + sp : ''))

  return (
    <div style={{ display: 'flex', minHeight: '100%' }}>
      <div className="page" style={{ flex: 1, minWidth: 0 }}>
        <div className="between" style={{ flexWrap: 'wrap' }}>
          <h1 className="h1">Transações</h1>
          <div className="row" style={{ gap: 8 }}>
            <form className="only-desktop" onSubmit={e => { e.preventDefault(); setParam('q', search) }}>
              <input className="input" style={{ height: 40, width: 220 }} placeholder="Nome, referência, pedido" value={search} onChange={e => setSearch(e.target.value)} />
            </form>
            <a className="btn btn-outline only-desktop" style={{ height: 40, fontWeight: 500, fontSize: 14 }} href={g.payments.exportUrl(filter)} download><Icon name="download" size={16} stroke={2} />Exportar CSV</a>
          </div>
        </div>
        <form className="only-mobile" style={{ marginTop: 14 }} onSubmit={e => { e.preventDefault(); setParam('q', search) }}>
          <div className="search" style={{ width: '100%' }}><Icon name="search" size={17} stroke={2} /><input value={search} onChange={e => setSearch(e.target.value)} placeholder="Pesquisar" /></div>
        </form>
        <div className="tabs" style={{ marginTop: 18 }}>
          {TABS.map(([k, l]) => <button key={k} className={tab === k ? 'on' : ''} onClick={() => setParam('estado', k === 'all' ? '' : k)}>{l}<span className="count">{data?.counts?.[k] ?? '·'}</span></button>)}
        </div>
        {q && <div style={{ marginTop: 12, fontSize: 13.5 }} className="row">Resultados para <b>“{q}”</b><button className="btn btn-ghost btn-sm" onClick={() => setParam('q', '')}>Limpar</button></div>}

        <div className="table only-desktop" style={{ marginTop: 16 }}>
          <div className="thead" style={{ gridTemplateColumns: GRID }}><span>Cliente</span><span>Origem</span><span>Método</span><span>Data</span><span>Estado</span><span style={{ textAlign: 'right' }}>Valor</span></div>
          {loading && !data && Array.from({ length: 6 }, (_, i) => <div key={i} className="trow" style={{ gridTemplateColumns: '1fr' }}><Skeleton h={34} /></div>)}
          {rows.map(t => {
            const st = STATUS[t.status] || STATUS.processing, src = SOURCE[t.source.type] || SOURCE.payment_link, sel = t.id === id, failed = t.status === 'failed'
            return (
              <div key={t.id} className="trow click" onClick={() => open(t.id)} style={{ gridTemplateColumns: GRID, background: sel ? '#F6FAF4' : undefined, boxShadow: sel ? 'inset 3px 0 0 var(--green)' : 'none' }}>
                <div className="row" style={{ minWidth: 0 }}><MethodIcon method={t.method} size={34} radius={10} /><div style={{ minWidth: 0 }}><div style={{ fontWeight: 600, fontSize: 14, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{t.customer_name || 'Cliente'}</div><div className="mono" style={{ fontSize: 12, color: "var(--muted)", whiteSpace: "nowrap" }}>{t.reference}</div></div></div>
                <div><span className="tag" style={{ background: src.bg, color: src.fg }}>{src.t}</span></div>
                <div style={{ fontSize: 13.5, color: 'var(--text-2)' }}>{methodLine(t)}</div>
                <div style={{ fontSize: 13.5, color: 'var(--text-2)' }}>{relDate(t.created_at)}</div>
                <div><Pill {...st} /></div>
                <div className="tnum" style={{ textAlign: 'right', fontWeight: 600, fontSize: 14, color: failed ? 'var(--muted-2)' : 'var(--ink)', textDecoration: failed ? 'line-through' : 'none' }}>{fmt(t.amount)} MT</div>
              </div>
            )
          })}
          {data && !rows.length && <div className="empty">Sem transações {tab !== 'all' ? 'neste estado' : 'ainda'}.</div>}
        </div>

        <div className="card only-mobile" style={{ marginTop: 14, borderRadius: 22, padding: '4px 0' }}>
          {rows.map(t => <TxLine key={t.id} p={t} onClick={() => open(t.id)} />)}
          {data && !rows.length && <div className="empty">Sem transações.</div>}
          {!data && <div className="empty"><Spinner dark /></div>}
        </div>
      </div>
      {id && <Detail id={id} onClose={close} onChanged={reload} />}
    </div>
  )
}

function Detail({ id, onClose, onChanged }: { id: string; onClose: () => void; onChanged: () => void }) {
  const { approved } = useShell()
  const merchant = useSession().me?.merchant
  const g = useGateway()
  const { toast } = useSession()
  const { data: p, reload } = useQuery(g => g.payments.get(id), [id], 5000)
  const [refund, setRefund] = useState(false)
  const [receipt, setReceipt] = useState(false)
  useEffect(() => { const h = (e: KeyboardEvent) => e.key === 'Escape' && onClose(); window.addEventListener('keydown', h); return () => window.removeEventListener('keydown', h) }, [onClose])

  const body = !p ? <div style={{ padding: 24 }}><Skeleton h={40} w="60%" /><Skeleton h={18} style={{ marginTop: 16 }} /><Skeleton h={200} style={{ marginTop: 16 }} /></div> : (() => {
    const st = STATUS[p.status] || STATUS.processing, src = SOURCE[p.source.type] || SOURCE.payment_link
    const failed = p.status === 'failed', api_ = p.source.type === 'api_checkout'
    const ok = 'var(--green)', y = '#E0A800', r = 'var(--red)', n = '#D6D4CB'
    const created = relDate(p.created_at), done = p.succeeded_at ? relDate(p.succeeded_at) : created
    const push = p.method === 'card' ? 'Pedido enviado ao banco emissor' : 'Pedido de PIN enviado ao telemóvel'
    const cb = (p.deliveries || []).filter((d: any) => d.target === 'callback_url')
    const cbDone = cb.find((d: any) => d.status === 'succeeded')
    let steps: [string, string, string][]
    if (p.status === 'processing') steps = [[api_ ? 'Sessão criada via API' : 'Checkout aberto pelo cliente', created, ok], [push, created, ok], ['A aguardar o cliente', 'Expira em poucos minutos', y], ['Creditado no saldo', '—', n]]
    else if (failed) steps = [[api_ ? 'Sessão criada via API' : 'Checkout aberto pelo cliente', created, ok], [push, created, ok], ['Falhou: ' + (p.failure_code || '').replace(/_/g, ' '), p.failed_at ? relDate(p.failed_at) : created, r]]
    else {
      steps = [[api_ ? 'Sessão criada via API' : p.source.type === 'qr' ? 'QR digitalizado pelo cliente' : 'Link aberto pelo cliente', created, ok], [push, created, ok], ['Confirmado pelo cliente', done, ok]]
      if (p.session?.callback_url) steps.push([cbDone ? `callback_url notificado · ${cbDone.last_status_code} OK` : cb.length ? `callback_url · ${cb[0].status === 'failed' ? 'falhou' : 'a tentar'}` : 'callback_url por notificar', cbDone ? relDate(cbDone.updated_at) : '—', cbDone ? ok : y])
      if (p.session?.success_url) steps.push(['Cliente redirecionado para success_url', done, ok])
      steps.push(['Creditado no saldo', done, ok])
      for (const rf of p.refunds || []) steps.push([`Reembolso de ${fmt(rf.amount)} MT`, relDate(rf.created * 1000), n])
    }
    const mono = "'Geist Mono',monospace"
    const srcRows: [string, string, boolean][] = api_ ? [
      ['Sessão', p.checkout_session, true], ['Domínio de origem', p.source.origin?.domain || '—', true], ['Chave API', p.api_key_label || p.source.api_key || '—', true],
      ['client_reference_id', p.client_reference_id || '—', true], ['IP do pedido', p.source.origin?.ip || '—', true],
      ...(p.session?.callback_url ? [['callback_url', p.session.callback_url.replace(/^https?:\/\//, ''), true] as [string, string, boolean]] : []),
      ...(cb.length ? [['Resposta do callback', cbDone ? `${cbDone.last_status_code} OK · ${cbDone.duration_ms} ms` : `${cb[0].last_status_code || '—'} · ${cb[0].attempts} tentativas`, false] as [string, string, boolean]] : []),
    ] : [
      ['Link', p.source.payment_link || '—', true], ['Criado por', (p.created_by_name || '—') + ' · painel', false],
      ['Tipo', `${p.link?.custom_amount ? 'Valor livre' : 'Valor fixo'} · ${p.link?.reusable ? 'reutilizável' : 'uso único'}`, false],
      ...(p.link?.location ? [['Local', p.link.location, false] as [string, string, boolean]] : []),
      ['Canal', ({ whatsapp: 'WhatsApp', sms: 'SMS', qr: 'QR', outro: 'Outro' } as any)[p.source.channel] || '—', false],
      ...(p.payer_user_agent ? [['Dispositivo', uaShort(p.payer_user_agent), false] as [string, string, boolean]] : []),
    ]
    const refundable = p.status === 'succeeded' || p.status === 'partially_refunded'
    return (
      <div style={{ padding: 24 }}>
        <div className="between"><span className="mono" style={{ fontSize: 12.5, color: 'var(--muted)' }}>{p.reference}</span><button className="icon-btn" style={{ width: 34, height: 34 }} onClick={onClose} aria-label="Fechar"><Icon name="close" size={16} stroke={2} /></button></div>
        <div className="display" style={{ fontSize: 38, letterSpacing: '-0.04em', marginTop: 18, color: failed ? 'var(--muted-2)' : 'var(--ink)', textDecoration: failed ? 'line-through' : 'none' }}>{fmt(p.amount)} <span style={{ fontSize: 17, color: 'var(--muted)' }}>MT</span></div>
        <div className="row" style={{ gap: 8, marginTop: 8 }}><Pill {...st} style={{ fontSize: 12, padding: '3px 9px' }} /><span style={{ fontSize: 13.5, color: 'var(--muted)' }}>{p.customer_name || 'Cliente'}</span></div>
        {failed && p.failure_message && <div style={{ marginTop: 14, padding: '12px 14px', borderRadius: 12, background: 'var(--red-bg)', color: '#8E0609', fontSize: 13, lineHeight: 1.45 }}>{p.failure_message}</div>}
        <div className="eyebrow" style={{ marginTop: 22 }}>Cronologia</div>
        <div style={{ marginTop: 12 }}>
          {steps.map(([l, t, c], i) => (
            <div key={i} style={{ display: 'flex', gap: 12 }}>
              <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center' }}><div style={{ width: 12, height: 12, borderRadius: '50%', background: c, marginTop: 3 }} />{i < steps.length - 1 && <div style={{ flex: 1, width: 2, background: '#EEECE5', margin: '4px 0' }} />}</div>
              <div style={{ paddingBottom: 14 }}><div style={{ fontWeight: 600, fontSize: 13.5 }}>{l}</div><div style={{ fontSize: 12.5, color: 'var(--muted)' }}>{t}</div></div>
            </div>
          ))}
        </div>
        <div className="eyebrow" style={{ marginTop: 8 }}>Origem</div>
        <div style={{ marginTop: 10, padding: 14, borderRadius: 14, background: 'var(--paper)' }}>
          <div className="row" style={{ gap: 8 }}><span className="tag" style={{ background: src.bg, color: src.fg }}>{src.t}</span><span style={{ fontWeight: 600, fontSize: 13.5 }}>{api_ ? 'Checkout Session via API' : p.source.type === 'qr' ? 'QR estático no balcão' : 'Link · ' + (p.link?.name || p.description || '')}</span></div>
          <div style={{ marginTop: 8 }}>{srcRows.map(([k, v, m]) => <div key={k} style={{ display: 'flex', justifyContent: 'space-between', gap: 12, padding: '7px 0', borderTop: '1px solid var(--line)', fontSize: 12.5 }}><span style={{ color: 'var(--muted)', flex: 'none' }}>{k}</span><span style={{ fontFamily: m ? mono : undefined, textAlign: 'right', wordBreak: 'break-all' }}>{v}</span></div>)}</div>
          {api_ && p.metadata && Object.keys(p.metadata).length > 0 && <><div style={{ fontSize: 12, color: 'var(--muted)', marginTop: 8 }}>metadata</div><pre style={{ margin: '6px 0 0', padding: '10px 12px', borderRadius: 10, background: 'var(--ink)', color: '#C9D3CD', fontFamily: mono, fontSize: 11.5, lineHeight: 1.6, overflowX: 'auto' }}>{JSON.stringify(p.metadata, null, 2)}</pre></>}
        </div>
        <div className="eyebrow" style={{ marginTop: 18 }}>Detalhes</div>
        <div style={{ marginTop: 6, fontSize: 13.5 }}>
          <div className="between" style={{ padding: '10px 0', borderBottom: '1px solid var(--line-3)' }}><span className="muted">Método</span><span style={{ fontWeight: 500 }}>{methodLine(p)}</span></div>
          {p.payment_method?.[p.method]?.operator_reference && <div className="between" style={{ padding: '10px 0', borderBottom: '1px solid var(--line-3)' }}><span className="muted">Ref. do operador</span><span className="mono" style={{ fontSize: 12.5 }}>{p.payment_method[p.method].operator_reference}</span></div>}
          <div className="between" style={{ padding: '10px 0', borderBottom: '1px solid var(--line-3)' }}><span className="muted">Data</span><span style={{ fontWeight: 500 }}>{fullDate(p.created_at)}</span></div>
          <div className="between" style={{ padding: '10px 0', borderBottom: '1px solid var(--line-3)' }}><span className="muted">Taxa RIHA ({bpsLabel(p.fee_bps)})</span><span className="tnum" style={{ fontWeight: 500 }}>{failed ? '—' : fmt(p.fee) + ' MT'}</span></div>
          {p.amount_refunded > 0 && <div className="between" style={{ padding: '10px 0', borderBottom: '1px solid var(--line-3)' }}><span className="muted">Reembolsado</span><span className="tnum" style={{ fontWeight: 500 }}>−{fmt(p.amount_refunded)} MT</span></div>}
          <div className="between" style={{ padding: '10px 0' }}><span className="muted">Líquido</span><span className="tnum" style={{ fontWeight: 600 }}>{failed ? '—' : fmt(p.net - p.amount_refunded) + ' MT'}</span></div>
        </div>
        <div style={{ display: 'flex', gap: 8, marginTop: 14 }}>
          <button className="btn btn-outline" style={{ flex: 1 }} disabled={p.status !== 'succeeded'} onClick={async () => {
            try { const r = await g.payments.sendReceipt(p.id); toast(r.message) }
            catch (e) { if ((e as ApiError).code === 'customer_email_missing') setReceipt(true); else toast(errMsg(e)) }
          }}>Enviar recibo</button>
          {refundable && approved && (p.livemode && merchant && !merchant.live_refunds
            ? <button className="btn btn-outline" style={{ flex: 1 }} disabled title="Os reembolsos em produção chegam em breve">Reembolsar · em breve</button>
            : <button className="btn btn-danger" style={{ flex: 1 }} onClick={() => setRefund(true)}>Reembolsar</button>)}
        </div>
        {refund && <RefundModal p={p} onClose={() => setRefund(false)} onDone={() => { setRefund(false); reload(); onChanged() }} />}
        {receipt && <ReceiptModal id={p.id} onClose={() => setReceipt(false)} />}
      </div>
    )
  })()

  return (
    <>
      <div className="only-desktop" style={{ width: 380, flex: 'none', borderLeft: '1px solid var(--line)', background: '#fff', position: 'sticky', top: 0, alignSelf: 'flex-start', maxHeight: '100%', overflowY: 'auto', height: '100%' }}>{body}</div>
      <div className="only-mobile" style={{ position: 'fixed', inset: 0, zIndex: 40, background: '#fff', overflowY: 'auto' }}>{body}</div>
    </>
  )
}

function uaShort(ua: string) {
  const os = /Android/.test(ua) ? 'Android' : /iPhone|iPad/.test(ua) ? 'iOS' : /Windows/.test(ua) ? 'Windows' : /Mac OS/.test(ua) ? 'macOS' : /Linux/.test(ua) ? 'Linux' : 'Outro'
  const br = /Edg\//.test(ua) ? 'Edge' : /Chrome\//.test(ua) ? 'Chrome' : /Firefox\//.test(ua) ? 'Firefox' : /Safari\//.test(ua) ? 'Safari' : 'Browser'
  return `${os} · ${br}`
}

function ReceiptModal({ id, onClose }: { id: string; onClose: () => void }) {
  const g = useGateway()
  const { toast } = useSession()
  const [email, setEmail] = useState('')
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState(false)
  const go = async (e: FormEvent) => {
    e.preventDefault(); setBusy(true); setErr('')
    try { const r = await g.payments.sendReceipt(id, email); toast(r.message); onClose() }
    catch (e) { setErr(errMsg(e)) } finally { setBusy(false) }
  }
  return (
    <Modal onClose={onClose}>
      <form onSubmit={go}>
        <div className="display" style={{ fontSize: 22 }}>Enviar recibo</div>
        <div style={{ fontSize: 13.5, color: 'var(--muted)', marginTop: 6, lineHeight: 1.5 }}>Este pagamento não tem email do cliente. O email que indicar fica guardado no pagamento.</div>
        <label style={{ display: 'block', marginTop: 16 }}><span className="label">Email do cliente</span><input className="input" type="email" required autoFocus value={email} onChange={e => setEmail(e.target.value)} /></label>
        {err && <div className="form-err" style={{ marginTop: 12 }}>{err}</div>}
        <div style={{ display: 'flex', gap: 10, marginTop: 18 }}><button type="button" className="btn btn-outline" style={{ flex: 1 }} onClick={onClose}>Cancelar</button><button className="btn btn-ink" style={{ flex: 1 }} disabled={busy}>{busy ? <Spinner /> : 'Enviar recibo'}</button></div>
      </form>
    </Modal>
  )
}

function RefundModal({ p, onClose, onDone }: { p: any; onClose: () => void; onDone: () => void }) {
  const g = useGateway()
  const { toast } = useSession()
  const max = p.amount - p.amount_refunded
  const [amount, setAmount] = useState(fmt(max).replace(/\u00A0/g, ''))
  const [reason, setReason] = useState('')
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState(false)
  const go = async () => {
    setBusy(true); setErr('')
    try { await g.payments.refund(p.id, parseMT(amount), reason); toast('Reembolso concluído'); onDone() }
    catch (e) { setErr(errMsg(e)) } finally { setBusy(false) }
  }
  return (
    <Modal onClose={onClose}>
      <div className="display" style={{ fontSize: 22 }}>Reembolsar pagamento</div>
      <div style={{ fontSize: 13.5, color: 'var(--muted)', marginTop: 6, lineHeight: 1.5 }}>O valor é devolvido ao cliente ({methodLine(p)}) e debitado do seu saldo. A taxa RIHA não é devolvida.</div>
      <label style={{ display: 'block', marginTop: 16 }}><span className="label">Valor (máx. {fmt(max)} MT)</span><input className="input" inputMode="decimal" value={amount} onChange={e => setAmount(e.target.value)} /></label>
      <label style={{ display: 'block', marginTop: 12 }}><span className="label">Motivo (opcional)</span><input className="input" value={reason} onChange={e => setReason(e.target.value)} placeholder="Produto esgotado" /></label>
      {err && <div className="form-err" style={{ marginTop: 12 }}>{err}</div>}
      <div style={{ display: 'flex', gap: 10, marginTop: 18 }}><button className="btn btn-outline" style={{ flex: 1 }} onClick={onClose}>Cancelar</button><button className="btn btn-ink" style={{ flex: 1 }} onClick={go} disabled={busy}>{busy ? <Spinner /> : 'Reembolsar'}</button></div>
    </Modal>
  )
}
