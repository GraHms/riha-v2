import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { Icon, MethodIcon, Skeleton, Spinner } from '../../components/ui'
import { Amount, Badge, Empty, KV, List, Money, Page, PageHead, Panel, Section, Tip, type Tone } from '../../components/ops'
import { useQuery } from '../../core/useQuery'
import { useGateway } from '../../core/gateway'
import { errMsg } from '../../core/model'
import { copy, fmt, METHOD_NAME, STATUS } from '../../lib/format'
import { useSession } from '../../lib/session'

// The trail: everything that happened to a payment, in order and in one place.

const SOURCES: Record<string, { t: string; icon: string }> = {
  customer: { t: 'Cliente', icon: 'user' },
  riha: { t: 'RIHA', icon: 'shield' },
  operator: { t: 'mtoca · bim', icon: 'bank' },
  api: { t: 'API', icon: 'key' },
  merchant: { t: 'Comerciante', icon: 'store' },
}
const LEVEL: Record<string, { t: string; tone: Tone }> = {
  warn: { t: 'Aviso', tone: 'warn' },
  error: { t: 'Erro', tone: 'bad' },
}
const TONE: Record<string, Tone> = { succeeded: 'ok', failed: 'bad', unconfirmed: 'warn', processing: 'warn', refunded: 'info', partially_refunded: 'info' }

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

// The icon an event wears on the timeline: what kind of thing happened.
const kindIcon = (e: any) => e.kind === 'webhook.delivery' ? 'webhook' : e.kind === 'api.request' ? 'key' : (SOURCES[e.source] || SOURCES.riha).icon

function SourceChip({ s }: { s: string }) {
  const v = SOURCES[s] || SOURCES.riha
  return <span className="o-chip tr-src"><Icon name={v.icon} size={12} stroke={2.1} />{v.t}</span>
}

function LevelMark({ l }: { l: string }) {
  const v = LEVEL[l]
  if (!v) return null
  return <Badge tone={v.tone} dot>{v.t}</Badge>
}

function StatusBadge({ s }: { s: string }) {
  return <Badge tone={TONE[s]} dot>{STATUS[s]?.t || s}</Badge>
}

const CSS = `
.tr-amt{overflow-wrap:anywhere}
.tr-amt small{font-size:.55em;font-weight:600;color:var(--o-muted);margin-left:4px;letter-spacing:0}
.tr-amt .o-cents{font-size:.7em;color:var(--o-muted)}
.tr-desc{color:var(--o-muted);font-weight:500}
.tr-src{color:var(--o-ink-2);gap:5px}
.tr-src svg{color:var(--o-muted)}
.tr-wrap{white-space:normal;overflow-wrap:anywhere}
.tr-who{display:flex;align-items:center;gap:8px;min-width:0;font-size:13px;color:var(--o-ink-2)}
.tr-tb .btn{height:38px}
.tr-meta{display:flex;gap:6px;flex-wrap:wrap;margin:-8px 0 18px}
.tr-meta .o-chip{color:var(--o-ink-2);max-width:100%;overflow:hidden;text-overflow:ellipsis}
.tr-pays{display:grid;grid-template-columns:repeat(auto-fill,minmax(min(100%,300px),1fr));gap:14px}
.tr-pay .o-panel-h h3{display:flex;align-items:center;gap:8px}
.tr-pay .o-kv span:last-child{min-width:0;overflow-wrap:anywhere}
.tr-flags{display:flex;gap:6px;flex-wrap:wrap;margin-top:10px}
.tr-note{font-size:12.5px;color:var(--o-ink-2);margin-top:8px;line-height:1.5}
.tr-check{margin-top:14px;padding-top:12px;border-top:1px dashed var(--o-line)}
.tr-fix{margin-top:10px;padding:12px;border-radius:var(--o-r-sm);font-size:13px;line-height:1.5}
.tr-fix.ok{background:var(--o-brand-soft)}.tr-fix.bad{background:var(--o-bad-soft)}
.tr-fix b{display:block;margin-bottom:2px}
.tr-acts{display:flex;gap:8px;flex-wrap:wrap;margin-top:10px}
.tr-filter-m{display:none}
.tr-tl{padding:6px 0}
.tr-ev{display:grid;grid-template-columns:84px 30px minmax(0,1fr);gap:0 14px;padding:0 18px}
.tr-ev-when{padding-top:13px;text-align:right}
.tr-ev-when b{display:block;font-family:var(--mono);font-size:12px;font-weight:600}
.tr-ev-when span{display:block;font-size:11.5px;color:var(--o-muted);margin-top:1px}
.tr-ev-rail{position:relative;display:flex;justify-content:center}
.tr-ev-rail::before{content:'';position:absolute;left:50%;top:0;bottom:0;width:2px;margin-left:-1px;background:var(--o-line-2)}
.tr-ev:first-child .tr-ev-rail::before{top:22px}
.tr-ev:last-child .tr-ev-rail::before{bottom:auto;height:22px}
.tr-ev:only-child .tr-ev-rail::before{display:none}
.tr-dot{position:relative;margin-top:9px;width:28px;height:28px;border-radius:50%;display:grid;place-items:center;background:var(--o-surface);border:1px solid var(--o-line);color:var(--o-ink-2);box-shadow:0 0 0 3px var(--o-surface)}
.tr-dot.warn{background:var(--o-warn-soft);border-color:transparent;color:var(--o-warn)}
.tr-dot.error{background:var(--o-bad-soft);border-color:transparent;color:var(--o-bad)}
.tr-ev-body{min-width:0;padding:12px 0 14px}
.tr-ev-t{font-size:13.5px;font-weight:600;line-height:1.4;overflow-wrap:anywhere}
.tr-ev-tags{display:flex;align-items:center;gap:6px;flex-wrap:wrap;margin-top:6px}
.tr-ev-m{display:none;font-size:11.5px;color:var(--o-muted);margin-bottom:3px}
.tr-ev-meta{font-size:12px;color:var(--o-muted);margin-top:5px;display:flex;flex-wrap:wrap;gap:2px 10px;overflow-wrap:anywhere}
.tr-ev-meta b{font-weight:600;color:var(--o-ink-2)}
.tr-more{margin-top:6px;margin-left:-8px;color:var(--o-ink-2)}
.tr-more svg{transition:transform .15s}
.tr-more[aria-expanded=true] svg{transform:rotate(180deg)}
.tr-block{margin-top:10px}
.tr-block-h{display:flex;justify-content:space-between;align-items:center;gap:8px;font-size:11.5px;font-weight:700;color:var(--o-muted);letter-spacing:.03em}
.tr-block pre{margin:6px 0 0;padding:12px;border-radius:var(--o-r-sm);background:var(--o-surface-2);border:1px solid var(--o-line-2);color:var(--o-ink-2);font-family:var(--mono);font-size:12px;line-height:1.5;overflow:auto;max-height:320px;white-space:pre-wrap;word-break:break-word}
.tr-ua{font-size:12px;color:var(--o-muted);margin-top:8px;overflow-wrap:anywhere}
@media(max-width:860px){
  .tr-tb .o-search{flex-basis:100%}
  .tr-tb .btn{flex:1}
  .tr-filter{display:none!important}
  .tr-filter-m{display:block;width:100%}
  .tr-ev{grid-template-columns:30px minmax(0,1fr);gap:0 12px;padding:0 14px}
  .tr-ev-when{display:none}
  .tr-ev-m{display:block}
  .tr-meta{margin-top:-4px}
}`

export function TrailSearch() {
  const nav = useNavigate()
  const [sp, setSp] = useSearchParams()
  const q = sp.get('q') || ''
  const [text, setText] = useState(q)
  const [level, setLevel] = useState('warn,error')
  const hits = useQuery(q ? g => g.admin.trailSearch(q) : null, [q])
  const feed = useQuery(g => g.admin.trailEvents({ level: level.includes(',') ? '' : level, hours: 48 }), [level], 20000)
  const events = ((feed.data?.data || []) as any[]).filter(e => level === '' || level.split(',').includes(e.level))
  const open = (to: string) => to && nav('/admin/rasto/' + to)
  return (
    <Page>
      <style>{CSS}</style>
      <PageHead title="Rasto" sub={<>Tudo o que aconteceu a um pagamento, por ordem <Tip text="O cliente e a rede dele, o pedido ao mtoca e ao bim, o aviso do bim, os webhooks e a API." /></>} />
      <form className="o-toolbar tr-tb" onSubmit={e => { e.preventDefault(); setSp(text.trim() ? { q: text.trim() } : {}) }}>
        <label className="o-search">
          <Icon name="search" size={16} />
          <input value={text} onChange={e => setText(e.target.value)} placeholder="pay_…, RH-…, operação, req_…, cs_…, 4 dígitos" title="pay_…, RH-…, número da operação, req_…, cs_…, ou os 4 últimos dígitos" aria-label="Procurar um pagamento" />
        </label>
        <button className="btn btn-ink" type="submit"><Icon name="search" size={16} />Procurar</button>
      </form>

      {q && (
        !hits.data ? <div className="o-panel" style={{ padding: 16 }}><Skeleton h={60} /></div> : (
          <List rows={hits.data.data || []} rowKey={(h: any) => h.payment_id || h.session_id} onRow={(h: any) => open(h.payment_id || h.session_id)}
            empty={<Empty icon="search" title={`Nada encontrado para “${q}”`} />}
            cols={[
              { head: 'Pagamento', w: 'minmax(0,1.3fr)', cell: (h: any) => <><div className="o-t1">{h.merchant}</div><div className="o-t2 o-mono">{h.reference || h.session_id}{h.livemode ? '' : ' · teste'}</div></> },
              { head: 'Valor', w: '120px', right: true, cell: (h: any) => <Amount c={h.amount} /> },
              { head: 'Cliente', w: '140px', cell: (h: any) => <span className="tr-who">{h.method && <MethodIcon method={h.method} size={20} radius={6} />}{h.phone_last4 ? `${h.phone_prefix} ••• ${h.phone_last4}` : '—'}</span> },
              { head: 'Estado', w: '130px', cell: (h: any) => <StatusBadge s={h.status} /> },
              { head: 'Quando', w: 'minmax(0,1fr)', cell: (h: any) => <span className="o-t2">{dateTime(h.created_at)}{h.failure_code ? ` · ${h.failure_code}` : ''}</span> },
            ]}
            card={(h: any) => ({
              lead: h.method ? <MethodIcon method={h.method} size={32} radius={9} /> : undefined,
              title: h.merchant,
              sub: <><span className="o-mono">{h.reference || h.session_id}</span>{h.livemode ? '' : ' · teste'} · {dateTime(h.created_at)}{h.failure_code ? ` · ${h.failure_code}` : ''}</>,
              amount: <Amount c={h.amount} />,
              meta: <StatusBadge s={h.status} />,
            })} />
        )
      )}

      <Section title="Eventos recentes" sub="48 h"
        actions={<div className="seg">{[['warn,error', 'Avisos e erros'], ['error', 'Só erros'], ['', 'Tudo']].map(([k, l]) => <button key={k} className={level === k ? 'on' : ''} onClick={() => setLevel(k)}>{l}</button>)}</div>} />
      {!feed.data ? <div className="o-panel" style={{ padding: 16 }}><Skeleton h={80} /></div> : (
        <List rows={events} rowKey={e => e.id} onRow={e => open(e.payment_id || e.session_id)}
          empty={<Empty title="Nada nas últimas 48 horas" />}
          cols={[
            { head: 'Quando', w: '116px', cell: e => <span className="o-mono o-t2">{dateTime(e.created_at)}</span> },
            { head: 'Origem', w: '130px', cell: e => <SourceChip s={e.source} /> },
            { head: 'Evento', w: 'minmax(0,1fr)', cell: e => <div className="tr-wrap">{e.title}</div> },
            { head: '', w: '80px', right: true, cell: e => <LevelMark l={e.level} /> },
          ]}
          card={e => ({
            title: <span className="tr-wrap">{e.title}</span>,
            sub: <>{(SOURCES[e.source] || SOURCES.riha).t} · {dateTime(e.created_at)}</>,
            meta: <LevelMark l={e.level} />,
          })} />
      )}
    </Page>
  )
}

function Block({ label, text }: { label: string; text: string }) {
  const { toast } = useSession()
  if (!text) return null
  return (
    <div className="tr-block">
      <div className="tr-block-h"><span>{label}</span><button className="btn btn-ghost btn-sm" onClick={async () => { await copy(text); toast('Copiado') }}><Icon name="copy" size={13} />Copiar</button></div>
      <pre>{text}</pre>
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
  if (e.network) bits.push(<b key="n">{e.network}</b>)
  const d = device(e.user_agent)
  if (d) bits.push(d)
  if (e.data?.connection) bits.push(`ligação ${e.data.connection}${e.data.rtt_ms ? `, ${e.data.rtt_ms} ms` : ''}`)
  if (e.status) bits.push(`HTTP ${e.status}`)
  if (e.duration_ms) bits.push(e.duration_ms >= 1000 ? `${(e.duration_ms / 1000).toFixed(1)} s` : `${e.duration_ms} ms`)
  if (e.data?.questions > 1) bits.push(`${e.data.questions} perguntas em ${e.data.seconds} s`)
  if (e.request_id) bits.push(<span key="r" className="o-mono">{e.request_id}</span>)
  if (!bits.length) return null
  return <div className="tr-ev-meta">{bits.map((b, i) => <span key={i}>{b}</span>)}</div>
}

const SOURCE_TYPE = (t: string) => (t === 'api_checkout' ? 'API' : t === 'payment_link' ? 'Link de pagamento' : t)

export function TrailThread() {
  const { id } = useParams()
  const nav = useNavigate()
  const { toast } = useSession()
  const { data, reload } = useQuery(g => g.admin.trailThread(id!), [id], 0)
  const [filter, setFilter] = useState('all')
  const [open, setOpen] = useState<Record<string, boolean>>({})
  useEffect(() => { const t = setInterval(reload, 5000); return () => clearInterval(t) }, [id]) // eslint-disable-line react-hooks/exhaustive-deps
  const events: any[] = data?.events || []
  const start = events.length ? new Date(events[0].created_at).getTime() : 0
  const shown = useMemo(() => events.filter((FILTERS.find(f => f[0] === filter) || FILTERS[0])[2]), [events, filter])
  if (!data) return <Page><Skeleton h={30} w={240} /><Skeleton h={160} style={{ marginTop: 20 }} /><Skeleton h={320} style={{ marginTop: 14 }} /></Page>
  const s = data.session
  const since = (iso: string) => { const sec = Math.round((new Date(iso).getTime() - start) / 1000); return sec < 60 ? `+${sec} s` : `+${Math.floor(sec / 60)} min ${sec % 60} s` }
  const network = (pid: string) => events.find(e => e.payment_id === pid && e.kind === 'payment.submitted')
  return (
    <Page>
      <style>{CSS}</style>
      <PageHead back={{ label: 'Rasto', onClick: () => nav('/admin/rasto') }}
        title={<span className="tr-amt"><Money c={s.amount} />{s.description ? <span className="tr-desc"> · {s.description}</span> : null}</span>}
        sub={<><Link to={'/admin/comerciantes/' + data.merchant.id}>{data.merchant.name}</Link> · {SOURCE_TYPE(s.source_type)}{s.origin_domain ? ` · ${s.origin_domain}` : ''}{s.livemode ? '' : ' · teste'}</>}
        actions={<button className="btn btn-outline btn-sm" onClick={async () => { await copy(window.location.href); toast('Endereço copiado') }}><Icon name="copy" size={14} />Copiar endereço</button>} />
      <div className="tr-meta">
        <span className="o-chip o-mono">{s.id}</span>
        <span className="o-chip"><Icon name="clock" size={12} />Criado {dateTime(s.created_at)}</span>
        <span className="o-chip">{s.status}</span>
      </div>

      {data.payments.length === 0
        ? <Panel><Empty icon="clock" title="Sem tentativas" text="O cliente ainda não tentou pagar." /></Panel>
        : (
          <div className="tr-pays">
            {data.payments.map((p: any, i: number) => {
              const sub = network(p.id)
              const end = p.succeeded_at || p.failed_at
              const net = sub ? [sub.network, device(sub.user_agent)].filter(Boolean).join(' · ') : ''
              return (
                <section key={p.id} className="o-panel tr-pay">
                  <div className="o-panel-h">
                    <h3><MethodIcon method={p.method} size={24} radius={7} />Tentativa {i + 1} · {METHOD_NAME[p.method] || p.method}</h3>
                    <StatusBadge s={p.status} />
                  </div>
                  <div className="o-panel-b">
                    <KV k="Referência" v={<span className="o-mono">{p.reference}</span>} />
                    <KV k="Telefone" v={p.phone || '—'} />
                    <KV k="Operação" v={<span className="o-mono">{p.operation || '—'}</span>} />
                    {end && <KV k={p.status === 'succeeded' ? 'Pago em' : 'Fechado em'} v={<>{took(p.created_at, end)}{p.failure_code ? <span className="o-mono" style={{ color: 'var(--o-muted)' }}> · {p.failure_code}</span> : null}</>} />}
                    {p.status === 'unconfirmed' && p.recheck_at && <KV k="Próxima pergunta" v={<>{time(p.recheck_at)}{p.rechecks ? <span style={{ color: 'var(--o-muted)', fontWeight: 500 }}> · já {p.rechecks}×</span> : null}</>} />}
                    {net && <KV k="Rede" v={net} />}
                    {(p.paid_late || p.duplicate_of) && (
                      <div className="tr-flags">
                        {p.paid_late && <Badge tone="info">Pago depois</Badge>}
                        {p.duplicate_of && <Badge tone="bad" dot>Possível duplicado</Badge>}
                      </div>
                    )}
                    {p.duplicate_of && (
                      <div className="tr-note">
                        Já tinha pago em <Link to={'/admin/rasto/' + p.duplicate_of} className="o-mono" style={{ overflowWrap: 'anywhere' }}>{p.duplicate_of}</Link>. Veja com o comerciante se é para reembolsar.
                      </div>
                    )}
                    {p.method !== 'card' && p.operation && <OperatorCheck p={p} onApplied={reload} />}
                  </div>
                </section>
              )
            })}
          </div>
        )}

      <Section title="O que aconteceu" sub={`${events.length} eventos`}
        actions={<>
          <div className="seg tr-filter" style={{ flexWrap: 'wrap' }}>{FILTERS.map(([k, l]) => <button key={k} className={filter === k ? 'on' : ''} onClick={() => setFilter(k)}>{l}</button>)}</div>
          <select className="o-select tr-filter-m" value={filter} onChange={e => setFilter(e.target.value)} aria-label="Filtrar eventos">
            {FILTERS.map(([k, l]) => <option key={k} value={k}>{l}</option>)}
          </select>
        </>} />
      <section className="o-panel">
        {shown.length === 0
          ? <Empty icon="clock" title={'Sem eventos' + (filter !== 'all' ? ' com este filtro' : '')} text="Os pagamentos anteriores a este registo não têm rasto." />
          : (
            <ol className="tr-tl" style={{ listStyle: 'none', margin: 0 }}>
              {shown.map(e => {
                const more = e.request || e.response || (e.data && Object.keys(e.data).length > 0)
                return (
                  <li key={e.id} className="tr-ev">
                    <div className="tr-ev-when"><b>{time(e.created_at)}</b><span>{since(e.created_at)}</span></div>
                    <div className="tr-ev-rail"><span className={'tr-dot ' + e.level} title={LEVEL[e.level]?.t}><Icon name={LEVEL[e.level] ? 'alert' : kindIcon(e)} size={14} stroke={2.1} /></span></div>
                    <div className="tr-ev-body">
                      <div className="tr-ev-m"><span className="o-mono">{time(e.created_at)}</span> · {since(e.created_at)}</div>
                      <div className="tr-ev-t">{e.title}</div>
                      <div className="tr-ev-tags"><SourceChip s={e.source} /><LevelMark l={e.level} /></div>
                      <Meta e={e} />
                      {more && (
                        <button className="btn btn-ghost btn-sm tr-more" onClick={() => setOpen({ ...open, [e.id]: !open[e.id] })} aria-expanded={!!open[e.id]}>
                          {open[e.id] ? 'Esconder' : e.request || e.response ? 'Enviado e recebido' : 'Detalhes'}<Icon name="chevronDown" size={14} />
                        </button>
                      )}
                      {open[e.id] && (
                        <div>
                          <Block label={e.kind === 'webhook.delivery' || e.kind === 'rail.request' ? 'Enviado' : e.kind === 'rail.callback' ? 'Recebido do bim' : 'Pedido'} text={e.request} />
                          <Block label={e.kind === 'webhook.delivery' ? 'Resposta do comerciante' : 'Resposta'} text={e.response} />
                          <Block label="Dados" text={e.data && Object.keys(e.data).length ? JSON.stringify(e.data, null, 2) : ''} />
                          {e.user_agent && <div className="tr-ua">{e.ip} · {e.user_agent}</div>}
                        </div>
                      )}
                    </div>
                  </li>
                )
              })}
            </ol>
          )}
      </section>
    </Page>
  )
}

// What the operator answers when asked, against what RIHA has.
const ANSWER: Record<string, { t: string; tone: Tone }> = {
  paid: { t: 'Pago', tone: 'ok' },
  failed: { t: 'Recusado', tone: 'bad' },
  pending: { t: 'Ainda sem resposta final', tone: 'warn' },
  none: { t: 'Sem registo deste pagamento', tone: undefined },
  unreachable: { t: 'Não respondeu', tone: 'bad' },
  unsupported: { t: 'Este método não pode ser consultado', tone: undefined },
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
  const credit = res?.fix === 'credit'
  return (
    <div className="tr-check">
      {!res && <button className="btn btn-outline btn-sm" onClick={() => run(false)} disabled={!!busy}>{busy ? <Spinner dark /> : <><Icon name="refresh" size={14} />Verificar no operador</>}</button>}
      {res && (
        <div style={{ fontSize: 13, lineHeight: 1.55 }}>
          <div className="between" style={{ gap: 8, flexWrap: 'wrap' }}>
            <span className="row" style={{ gap: 8 }}><span style={{ color: 'var(--o-muted)' }}>bim diz</span><Badge tone={a?.tone} dot>{a?.t || res.answer}</Badge></span>
            <button className="btn btn-ghost btn-sm" onClick={() => run(false)} disabled={!!busy}><Icon name="refresh" size={13} />{busy === 'check' ? 'A perguntar…' : 'Perguntar de novo'}</button>
          </div>
          {res.operator_reference && <KV k="Ref. do operador" v={<span className="o-mono">{res.operator_reference}</span>} />}
          {res.failure_message && <div className="tr-note">{res.failure_message}</div>}
          {res.error && <div className="tr-note" style={{ color: 'var(--o-muted)', overflowWrap: 'anywhere' }}>{res.error}</div>}
          {!res.fix && !res.applied && <div className="tr-note" style={{ color: 'var(--o-muted)' }}>{res.answer === 'paid' && p.status === 'succeeded' ? 'Bate certo com a RIHA.' : res.answer === 'paid' ? 'O pagamento foi recusado antes pelo operador, por isso não é reaberto aqui.' : 'Nada a mudar na RIHA.'}</div>}
          {res.fix && !res.applied && (
            <div className={'tr-fix ' + (credit ? 'ok' : 'bad')}>
              <b>{credit ? `A RIHA tem-no como ${STATUS[p.status]?.t.toLowerCase() || p.status}, mas o cliente pagou.` : 'O operador recusou-o, e a RIHA ainda o tem em aberto.'}</b>
              <span style={{ color: 'var(--o-ink-2)' }}>{credit ? 'Creditar põe os ' + fmt(p.amount) + ' MT no saldo do comerciante e envia-lhe payment.succeeded. Se o cliente já pagou de novo, fica marcado como possível duplicado.' : 'Marcar como falhado envia payment.failed ao comerciante.'}</span>
              <div className="tr-acts">
                {!sure
                  ? <button className={'btn btn-sm ' + (credit ? 'btn-green' : 'btn-danger')} onClick={() => setSure(true)}>{credit ? 'Creditar ' + fmt(p.amount) + ' MT' : 'Marcar como falhado'}</button>
                  : <><button className={'btn btn-sm ' + (credit ? 'btn-green' : 'btn-danger')} onClick={() => run(true)} disabled={!!busy}>{busy === 'apply' ? <Spinner /> : 'Confirmar'}</button><button className="btn btn-outline btn-sm" onClick={() => setSure(false)}>Cancelar</button></>}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
