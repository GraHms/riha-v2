import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { Icon, MethodIcon, Pill, Skeleton, Spinner } from '../../components/ui'
import { useQuery } from '../../core/useQuery'
import { useGateway } from '../../core/gateway'
import { errMsg } from '../../core/model'
import { copy, fmt, METHOD_NAME, STATUS } from '../../lib/format'
import { useSession } from '../../lib/session'

// The trail: everything that happened to a payment, in order and in one place.

const SOURCES: Record<string, { t: string; bg: string; fg: string }> = {
  customer: { t: 'Cliente', bg: '#E6ECFB', fg: '#2A4AA8' },
  riha: { t: 'RIHA', bg: '#EDECE6', fg: '#3E4742' },
  operator: { t: 'mtoca · bim', bg: '#FFF3C4', fg: '#7A5A00' },
  api: { t: 'API', bg: '#F3EEFF', fg: '#5B3FB8' },
  merchant: { t: 'Comerciante', bg: '#E2F3DE', fg: '#0A6B05' },
}
const LEVEL: Record<string, { t: string; c: string; icon: string }> = {
  warn: { t: 'Aviso', c: '#B87A00', icon: 'alert' },
  error: { t: 'Erro', c: '#C2080D', icon: 'alert' },
}

export function device(ua: string) {
  if (!ua) return ''
  const os = /Android ([\d.]+)/.exec(ua) ? 'Android ' + /Android ([\d.]+)/.exec(ua)![1].split('.')[0]
    : /iPhone OS ([\d_]+)/.exec(ua) ? 'iPhone, iOS ' + /iPhone OS ([\d_]+)/.exec(ua)![1].split('_')[0]
    : /iPad/.test(ua) ? 'iPad' : /Windows/.test(ua) ? 'Windows' : /Mac OS X/.test(ua) ? 'Mac' : /Linux/.test(ua) ? 'Linux' : ''
  const br = /SamsungBrowser/.test(ua) ? 'Samsung Internet' : /OPR\/|Opera/.test(ua) ? 'Opera' : /Edg\//.test(ua) ? 'Edge' : /Firefox\//.test(ua) ? 'Firefox'
    : /Chrome\//.test(ua) ? 'Chrome' : /Safari\//.test(ua) ? 'Safari' : /^Java|curl|Go-http|python|node|axios|okhttp|Guzzle/i.test(ua) ? ua.split(/[ /]/)[0] : ''
  return [os, br].filter(Boolean).join(' · ')
}

const time = (iso: string) => new Date(iso).toLocaleTimeString('pt-PT', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
const took = (from: string, to: string) => {
  const sec = Math.round((new Date(to).getTime() - new Date(from).getTime()) / 1000)
  return sec < 120 ? `${sec} s` : sec < 7200 ? `${Math.round(sec / 60)} min` : `${Math.round(sec / 3600)} h`
}
const dateTime = (iso: string) => new Date(iso).toLocaleString('pt-PT', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' })

function SourceChip({ s }: { s: string }) {
  const v = SOURCES[s] || SOURCES.riha
  return <span style={{ display: 'inline-flex', padding: '2px 8px', borderRadius: 999, background: v.bg, color: v.fg, fontSize: 11.5, fontWeight: 700, whiteSpace: 'nowrap' }}>{v.t}</span>
}

function LevelMark({ l }: { l: string }) {
  const v = LEVEL[l]
  if (!v) return null
  return <span style={{ display: 'inline-flex', alignItems: 'center', gap: 4, color: v.c, fontSize: 12, fontWeight: 700 }}><Icon name={v.icon} size={13} stroke={2.4} />{v.t}</span>
}

function StatusPill({ s }: { s: string }) {
  const v = STATUS[s] || { t: s, bg: '#EDECE6', fg: '#3E4742' }
  return <Pill {...v} />
}

export function TrailSearch() {
  const nav = useNavigate()
  const [sp, setSp] = useSearchParams()
  const q = sp.get('q') || ''
  const [text, setText] = useState(q)
  const [level, setLevel] = useState('warn,error')
  const hits = useQuery(q ? g => g.admin.trailSearch(q) : null, [q])
  const feed = useQuery(g => g.admin.trailEvents({ level: level.includes(',') ? '' : level, hours: 48 }), [level], 20000)
  const events = ((feed.data?.data || []) as any[]).filter(e => level === '' || level.split(',').includes(e.level))
  return (
    <div className="page">
      <h1 className="h1">Rasto</h1>
      <div style={{ fontSize: 14.5, color: 'var(--muted)', marginTop: 4, maxWidth: 680, lineHeight: 1.5 }}>Tudo o que aconteceu a um pagamento, por ordem: o cliente e a rede dele, o pedido ao mtoca e ao bim, o aviso do bim, os webhooks e a API.</div>
      <form onSubmit={e => { e.preventDefault(); setSp(text.trim() ? { q: text.trim() } : {}) }} style={{ display: 'flex', gap: 8, marginTop: 20, maxWidth: 720, flexWrap: 'wrap' }}>
        <input className="input" style={{ flex: 1, minWidth: 240 }} value={text} onChange={e => setText(e.target.value)} placeholder="pay_…, RH-…, número da operação, req_…, cs_…, ou os 4 últimos dígitos" aria-label="Procurar um pagamento" />
        <button className="btn btn-ink" type="submit"><Icon name="search" size={16} />Procurar</button>
      </form>
      {q && (
        <div className="table" style={{ marginTop: 16 }}>
          {!hits.data && <div style={{ padding: 16 }}><Skeleton h={60} /></div>}
          {hits.data?.data?.length === 0 && <div className="empty">Nada encontrado para "{q}".</div>}
          {(hits.data?.data || []).map((h: any) => (
            <div key={(h.payment_id || h.session_id)} className="trow click" style={{ gridTemplateColumns: 'minmax(0,1.3fr) 110px 120px 110px minmax(0,1fr)', padding: '12px 20px' }} onClick={() => nav('/admin/rasto/' + (h.payment_id || h.session_id))}>
              <div style={{ minWidth: 0 }}><div style={{ fontWeight: 600, fontSize: 14 }}>{h.merchant}</div><div className="mono" style={{ fontSize: 12, color: 'var(--muted)' }}>{h.reference || h.session_id}{h.livemode ? '' : ' · teste'}</div></div>
              <div className="tnum" style={{ fontWeight: 600 }}>{fmt(h.amount)} MT</div>
              <div className="row" style={{ gap: 6, fontSize: 13 }}>{h.method && <MethodIcon method={h.method} size={20} radius={6} />}{h.phone_last4 ? `${h.phone_prefix} ••• ${h.phone_last4}` : '—'}</div>
              <div><StatusPill s={h.status} /></div>
              <div style={{ fontSize: 13, color: 'var(--muted)' }}>{dateTime(h.created_at)}{h.failure_code ? ` · ${h.failure_code}` : ''}</div>
            </div>
          ))}
        </div>
      )}
      <div className="between" style={{ marginTop: 28, flexWrap: 'wrap', gap: 10 }}>
        <div className="h2">Eventos recentes</div>
        <div className="seg">{[['warn,error', 'Avisos e erros'], ['error', 'Só erros'], ['', 'Tudo']].map(([k, l]) => <button key={k} className={level === k ? 'on' : ''} onClick={() => setLevel(k)}>{l}</button>)}</div>
      </div>
      <div className="table" style={{ marginTop: 12 }}>
        {!feed.data && <div style={{ padding: 16 }}><Skeleton h={80} /></div>}
        {feed.data && events.length === 0 && <div className="empty">Nada nas últimas 48 horas.</div>}
        {events.map(e => {
          const to = e.payment_id || e.session_id
          return (
            <div key={e.id} className={'trow' + (to ? ' click' : '')} style={{ gridTemplateColumns: '120px 120px minmax(0,1fr) 90px', padding: '11px 20px', alignItems: 'start' }} onClick={() => to && nav('/admin/rasto/' + to)}>
              <span className="mono" style={{ fontSize: 12.5, color: 'var(--muted)' }}>{dateTime(e.created_at)}</span>
              <span><SourceChip s={e.source} /></span>
              <span style={{ fontSize: 13.5, minWidth: 0, overflowWrap: 'anywhere' }}>{e.title}</span>
              <span style={{ textAlign: 'right' }}><LevelMark l={e.level} /></span>
            </div>
          )
        })}
      </div>
    </div>
  )
}

function Block({ label, text }: { label: string; text: string }) {
  if (!text) return null
  return (
    <div style={{ marginTop: 10 }}>
      <div className="between"><span style={{ fontSize: 12, fontWeight: 700, color: 'var(--muted)', textTransform: 'uppercase', letterSpacing: '.05em' }}>{label}</span><button className="link-btn" style={{ fontSize: 12 }} onClick={() => copy(text)}>Copiar</button></div>
      <pre style={{ margin: '6px 0 0', padding: 12, borderRadius: 12, background: '#0B120E', color: '#D7DED9', fontSize: 12, lineHeight: 1.5, overflow: 'auto', maxHeight: 320, whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>{text}</pre>
    </div>
  )
}

const FILTERS: [string, string, (e: any) => boolean][] = [
  ['all', 'Tudo', () => true],
  ['customer', 'Cliente', e => e.source === 'customer'],
  ['operator', 'mtoca · bim', e => e.kind.startsWith('rail.')],
  ['webhook', 'Webhooks', e => e.kind === 'webhook.delivery'],
  ['api', 'API', e => e.kind === 'api.request'],
  ['problems', 'Avisos e erros', e => e.level !== 'info'],
]

function Meta({ e }: { e: any }) {
  const bits: ReactNode[] = []
  if (e.network) bits.push(<b key="n" style={{ fontWeight: 600, color: 'var(--text-2)' }}>{e.network}</b>)
  const d = device(e.user_agent)
  if (d) bits.push(d)
  if (e.data?.connection) bits.push(`ligação ${e.data.connection}${e.data.rtt_ms ? `, ${e.data.rtt_ms} ms` : ''}`)
  if (e.status) bits.push(`HTTP ${e.status}`)
  if (e.duration_ms) bits.push(e.duration_ms >= 1000 ? `${(e.duration_ms / 1000).toFixed(1)} s` : `${e.duration_ms} ms`)
  if (e.data?.questions > 1) bits.push(`${e.data.questions} perguntas em ${e.data.seconds} s`)
  if (e.request_id) bits.push(<span key="r" className="mono">{e.request_id}</span>)
  if (!bits.length) return null
  return <div style={{ fontSize: 12.5, color: 'var(--muted)', marginTop: 3, display: 'flex', flexWrap: 'wrap', gap: '2px 10px' }}>{bits.map((b, i) => <span key={i}>{b}</span>)}</div>
}

export function TrailThread() {
  const { id } = useParams()
  const { toast } = useSession()
  const { data, reload } = useQuery(g => g.admin.trailThread(id!), [id], 0)
  const [filter, setFilter] = useState('all')
  const [open, setOpen] = useState<Record<string, boolean>>({})
  useEffect(() => { const t = setInterval(reload, 5000); return () => clearInterval(t) }, [id]) // eslint-disable-line react-hooks/exhaustive-deps
  const events: any[] = data?.events || []
  const start = events.length ? new Date(events[0].created_at).getTime() : 0
  const shown = useMemo(() => events.filter((FILTERS.find(f => f[0] === filter) || FILTERS[0])[2]), [events, filter])
  if (!data) return <div className="page"><Skeleton h={420} r={22} /></div>
  const s = data.session
  const since = (iso: string) => { const sec = Math.round((new Date(iso).getTime() - start) / 1000); return sec < 60 ? `+${sec} s` : `+${Math.floor(sec / 60)} min ${sec % 60} s` }
  const network = (pid: string) => events.find(e => e.payment_id === pid && e.kind === 'payment.submitted')
  return (
    <div className="page">
      <style>{`.tl-row{display:grid;grid-template-columns:92px minmax(0,1fr);gap:14px;padding:12px 20px;border-top:1px solid var(--line-3)}
        .tl-row.warn{box-shadow:inset 3px 0 0 #E0A800}.tl-row.error{box-shadow:inset 3px 0 0 #FF0F15}
        @media(max-width:700px){.tl-row{grid-template-columns:1fr;gap:4px}.tl-row>div:first-child{display:flex;gap:8px;align-items:baseline}.tl-pays{grid-template-columns:1fr!important}}`}</style>
      <Link to="/admin/rasto" style={{ fontSize: 13.5, color: 'var(--text-2)' }}>‹ Rasto</Link>
      <div className="between" style={{ marginTop: 8, alignItems: 'flex-start', flexWrap: 'wrap', gap: 12 }}>
        <div style={{ minWidth: 0 }}>
          <div style={{ fontSize: 13.5, color: 'var(--muted)' }}><Link to={'/admin/comerciantes/' + data.merchant.id}>{data.merchant.name}</Link> · {s.source_type === 'api_checkout' ? 'API' : s.source_type === 'payment_link' ? 'Link de pagamento' : s.source_type}{s.origin_domain ? ` · ${s.origin_domain}` : ''}{s.livemode ? '' : ' · teste'}</div>
          <h1 className="h1" style={{ margin: '4px 0 0' }}>{fmt(s.amount)} MT{s.description ? <span style={{ color: 'var(--muted)', fontWeight: 500 }}> · {s.description}</span> : null}</h1>
          <div className="mono" style={{ fontSize: 12.5, color: 'var(--muted)', marginTop: 6 }}>{s.id} · criado {dateTime(s.created_at)} · {s.status}</div>
        </div>
        <button className="btn btn-outline btn-sm" onClick={async () => { await copy(window.location.href); toast('Endereço copiado') }}><Icon name="copy" size={14} />Copiar endereço</button>
      </div>

      <div className="tl-pays" style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill,minmax(260px,1fr))', gap: 10, marginTop: 18 }}>
        {data.payments.length === 0 && <div className="card" style={{ padding: 16, fontSize: 14, color: 'var(--muted)' }}>O cliente ainda não tentou pagar.</div>}
        {data.payments.map((p: any, i: number) => {
          const sub = network(p.id)
          const end = p.succeeded_at || p.failed_at
          return (
            <div key={p.id} className="card" style={{ padding: 16 }}>
              <div className="between"><span className="row" style={{ gap: 8, fontWeight: 600 }}><MethodIcon method={p.method} size={22} radius={7} />Tentativa {i + 1} · {METHOD_NAME[p.method] || p.method}</span><StatusPill s={p.status} /></div>
              <div style={{ fontSize: 13, color: 'var(--text-2)', marginTop: 8, lineHeight: 1.6 }}>
                <div><span className="mono">{p.reference}</span> · {p.phone}</div>
                <div>Operação <span className="mono">{p.operation || '—'}</span></div>
                {end && <div>{p.status === 'succeeded' ? 'Pago' : 'Fechado'} em {took(p.created_at, end)}{p.failure_code ? ` · ${p.failure_code}` : ''}</div>}
                {p.status === 'unconfirmed' && p.recheck_at && <div>Próxima pergunta ao operador às {time(p.recheck_at)}{p.rechecks ? ` · já perguntou ${p.rechecks}×` : ''}</div>}
                {sub && <div>{[sub.network, device(sub.user_agent)].filter(Boolean).join(' · ')}</div>}
              </div>
              {(p.paid_late || p.duplicate_of) && (
                <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap', marginTop: 10 }}>
                  {p.paid_late && <Pill t="Pago depois" bg="#E6ECFB" fg="#2A4AA8" />}
                  {p.duplicate_of && <Pill t="Possível duplicado" bg="#FFE3E1" fg="#C2080D" />}
                </div>
              )}
              {p.duplicate_of && (
                <div style={{ fontSize: 12.5, color: 'var(--text-2)', marginTop: 6, lineHeight: 1.5 }}>
                  O mesmo cliente já tinha pago em <Link to={'/admin/rasto/' + p.duplicate_of} className="mono" style={{ overflowWrap: 'anywhere' }}>{p.duplicate_of}</Link>. Veja com o comerciante se é para reembolsar.
                </div>
              )}
              {p.method !== 'card' && p.operation && <OperatorCheck p={p} onApplied={reload} />}
            </div>
          )
        })}
      </div>

      <div className="between" style={{ marginTop: 24, flexWrap: 'wrap', gap: 10 }}>
        <div className="h2">O que aconteceu <span style={{ fontSize: 14, color: 'var(--muted)', fontWeight: 500 }}>· {events.length} eventos</span></div>
        <div className="seg" style={{ flexWrap: 'wrap' }}>{FILTERS.map(([k, l]) => <button key={k} className={filter === k ? 'on' : ''} onClick={() => setFilter(k)}>{l}</button>)}</div>
      </div>
      <div className="table" style={{ marginTop: 12, padding: 0, overflow: 'hidden' }}>
        {shown.length === 0 && <div className="empty">Sem eventos{filter !== 'all' ? ' com este filtro' : ''}. Os pagamentos anteriores a este registo não têm rasto.</div>}
        {shown.map(e => {
          const more = e.request || e.response || (e.data && Object.keys(e.data).length > 0)
          return (
            <div key={e.id} className={'tl-row ' + e.level}>
              <div><div className="mono" style={{ fontSize: 12.5 }}>{time(e.created_at)}</div><div style={{ fontSize: 12, color: 'var(--muted)' }}>{since(e.created_at)}</div></div>
              <div style={{ minWidth: 0 }}>
                <div className="row" style={{ gap: 8, flexWrap: 'wrap' }}><SourceChip s={e.source} /><LevelMark l={e.level} /><span style={{ fontSize: 14, fontWeight: 500, overflowWrap: 'anywhere' }}>{e.title}</span></div>
                <Meta e={e} />
                {more && <button className="link-btn" style={{ fontSize: 12.5, marginTop: 6 }} onClick={() => setOpen({ ...open, [e.id]: !open[e.id] })} aria-expanded={!!open[e.id]}>{open[e.id] ? 'Esconder' : e.request || e.response ? 'Ver o que foi enviado e recebido' : 'Ver detalhes'}</button>}
                {open[e.id] && (
                  <div>
                    <Block label={e.kind === 'webhook.delivery' || e.kind === 'rail.request' ? 'Enviado' : e.kind === 'rail.callback' ? 'Recebido do bim' : 'Pedido'} text={e.request} />
                    <Block label={e.kind === 'webhook.delivery' ? 'Resposta do comerciante' : 'Resposta'} text={e.response} />
                    <Block label="Dados" text={e.data && Object.keys(e.data).length ? JSON.stringify(e.data, null, 2) : ''} />
                    {e.user_agent && <div style={{ fontSize: 12, color: 'var(--muted)', marginTop: 8, overflowWrap: 'anywhere' }}>{e.ip} · {e.user_agent}</div>}
                  </div>
                )}
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}

// What the operator answers when asked, against what RIHA has.
const ANSWER: Record<string, { t: string; c: string }> = {
  paid: { t: 'Pago', c: '#0A6B05' },
  failed: { t: 'Recusado', c: '#C2080D' },
  pending: { t: 'Ainda sem resposta final', c: '#7A5A00' },
  none: { t: 'Sem registo deste pagamento', c: '#3E4742' },
  unreachable: { t: 'Não respondeu', c: '#C2080D' },
  unsupported: { t: 'Este método não pode ser consultado', c: '#3E4742' },
}

// OperatorCheck asks bim, through mtoca, about one payment now, and offers to
// apply an answer RIHA doesn't have yet.
function OperatorCheck({ p, onApplied }: { p: any; onApplied: () => void }) {
  const g = useGateway()
  const { toast } = useSession()
  const [res, setRes] = useState<any>(null)
  const [busy, setBusy] = useState<'' | 'check' | 'apply'>('')
  const [sure, setSure] = useState(false)
  const run = async (apply: boolean) => {
    setBusy(apply ? 'apply' : 'check')
    try {
      const r = await g.admin.checkPayment(p.id, apply)
      setRes(r); setSure(false)
      if (r.applied) { toast(r.fix === 'credit' ? 'Pagamento creditado ao comerciante' : 'Pagamento marcado como falhado'); onApplied() }
    } catch (e) { toast(errMsg(e), true) } finally { setBusy('') }
  }
  const a = res && ANSWER[res.answer]
  return (
    <div style={{ marginTop: 12, paddingTop: 12, borderTop: '1px dashed var(--line-2)' }}>
      {!res && <button className="btn btn-outline btn-sm" onClick={() => run(false)} disabled={!!busy}>{busy ? <Spinner dark /> : <><Icon name="refresh" size={14} />Verificar no operador</>}</button>}
      {res && (
        <div style={{ fontSize: 13, lineHeight: 1.55 }}>
          <div className="between" style={{ gap: 8 }}>
            <span>bim diz: <b style={{ color: a?.c }}>{a?.t || res.answer}</b></span>
            <button className="link-btn" style={{ fontSize: 12.5 }} onClick={() => run(false)} disabled={!!busy}>{busy === 'check' ? 'A perguntar…' : 'Perguntar de novo'}</button>
          </div>
          {res.operator_reference && <div>Referência do operador <span className="mono">{res.operator_reference}</span></div>}
          {res.failure_message && <div>{res.failure_message}</div>}
          {res.error && <div style={{ color: 'var(--muted)', overflowWrap: 'anywhere' }}>{res.error}</div>}
          {!res.fix && !res.applied && <div style={{ color: 'var(--muted)' }}>{res.answer === 'paid' && p.status === 'succeeded' ? 'Bate certo com a RIHA.' : res.answer === 'paid' ? 'O pagamento foi recusado antes pelo operador, por isso não é reaberto aqui.' : 'Nada a mudar na RIHA.'}</div>}
          {res.fix && !res.applied && (
            <div style={{ marginTop: 8, padding: 10, borderRadius: 12, background: res.fix === 'credit' ? '#E2F3DE' : '#FFE3E1' }}>
              <div style={{ fontWeight: 600 }}>{res.fix === 'credit' ? `A RIHA tem-no como ${STATUS[p.status]?.t.toLowerCase() || p.status}, mas o cliente pagou.` : 'O operador recusou-o, e a RIHA ainda o tem em aberto.'}</div>
              <div style={{ marginTop: 2 }}>{res.fix === 'credit' ? 'Creditar põe os ' + fmt(p.amount) + ' MT no saldo do comerciante e envia-lhe payment.succeeded. Se o cliente já pagou de novo, fica marcado como possível duplicado.' : 'Marcar como falhado envia payment.failed ao comerciante.'}</div>
              {!sure
                ? <button className={'btn btn-sm ' + (res.fix === 'credit' ? 'btn-green' : 'btn-danger')} style={{ marginTop: 8 }} onClick={() => setSure(true)}>{res.fix === 'credit' ? 'Creditar ' + fmt(p.amount) + ' MT' : 'Marcar como falhado'}</button>
                : <div className="row" style={{ gap: 8, marginTop: 8 }}><button className={'btn btn-sm ' + (res.fix === 'credit' ? 'btn-green' : 'btn-danger')} onClick={() => run(true)} disabled={!!busy}>{busy === 'apply' ? <Spinner /> : 'Confirmar'}</button><button className="btn btn-outline btn-sm" onClick={() => setSure(false)}>Cancelar</button></div>}
            </div>
          )}
        </div>
      )}
    </div>
  )
}
