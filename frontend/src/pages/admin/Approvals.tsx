import { useEffect, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { errMsg } from '../../core/model'
import { useGateway } from '../../core/gateway'
import { useQuery } from '../../core/useQuery'
import { useSession } from '../../lib/session'
import { Avatar, Icon, Skeleton, Spinner } from '../../components/ui'
import { Badge, Empty, KV, List, Page, PageHead, Panel, type Tone } from '../../components/ops'
import { fileSize } from '../public/Kyc'
import { RiskPill, submittedAgo, V1Tag } from './common'

const DOC_LABEL: Record<string, string> = { id_front: 'BI · frente', id_back: 'BI · verso', registration: 'Certidão comercial', nib_proof: 'Comprovativo NIB' }
// The queue's own order comes first: submissions in the order they arrived,
// leads newest first.
const SORTS: Record<'review' | 'leads', [string, string][]> = {
  review: [['', 'Por ordem de chegada'], ['newest', 'Mais recentes'], ['name', 'Nome (A–Z)']],
  leads: [['', 'Mais recentes'], ['oldest', 'Mais antigas'], ['name', 'Nome (A–Z)']],
}
const CHECK: Record<string, [string, string]> = { ok: ['check', 'ok'], warn: ['alert', 'warn'], bad: ['close', 'bad'] }
const STAGE: Record<string, [string, Tone]> = { lead: ['Lead', undefined], info_requested: ['Docs pedidos', 'info'] }

// The list sits beside the account on wide screens; on phones it is one or
// the other, and the address says which.
const CSS = `
.ap{display:grid;grid-template-columns:360px minmax(0,1fr);gap:16px;align-items:start}
.ap.solo{grid-template-columns:minmax(0,1fr)}
.ap-side{position:sticky;top:12px;display:flex;flex-direction:column;max-height:calc(100vh - 84px);min-height:0}
.ap-side .tabs{margin-bottom:12px}
.ap-side .o-toolbar .o-search{flex:1 1 120px}
.ap-side .o-toolbar .o-select{flex:none;max-width:200px}
.ap-scroll{flex:1;min-height:0;overflow:auto;border-radius:var(--o-r)}
.ap-main{display:flex;flex-direction:column;gap:14px;min-width:0}
.ap-id{display:flex;align-items:center;gap:14px;min-width:0}
.ap-id h2{margin:0;font-family:var(--display);font-size:21px;font-weight:700;letter-spacing:-.02em;line-height:1.2;overflow-wrap:anywhere}
.ap-facts{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));column-gap:28px}
.ap-facts>div>.o-kv:first-child{border-top:0}
.ap-check{display:flex;gap:12px;align-items:flex-start;padding:12px 18px;border-top:1px solid var(--o-line-2)}
.ap-check:first-child{border-top:0}
.ap-ck{width:26px;height:26px;flex:none;border-radius:8px;display:grid;place-items:center}
.ap-ck.ok{background:var(--o-brand-soft);color:var(--o-brand-ink)}
.ap-ck.warn{background:var(--o-warn-soft);color:var(--o-warn)}
.ap-ck.bad{background:var(--o-bad-soft);color:var(--o-bad)}
.ap-docs{display:grid;grid-template-columns:repeat(auto-fill,minmax(150px,1fr));gap:10px}
.ap-doc{display:flex;flex-direction:column;border:1px solid var(--o-line);border-radius:12px;overflow:hidden;color:var(--o-ink);text-decoration:none;background:var(--o-surface);transition:box-shadow .15s,border-color .15s}
.ap-doc:hover{border-color:#CFD6D2;box-shadow:var(--o-shadow)}
.ap-doc-p{height:84px;display:grid;place-items:center;gap:4px;padding:10px;text-align:center;background:var(--o-surface-2);color:var(--o-muted);border-bottom:1px solid var(--o-line-2)}
.ap-doc-p .o-mono{font-size:11px;word-break:break-all;line-height:1.35;display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical;overflow:hidden}
.ap-doc-f{display:flex;align-items:center;justify-content:space-between;gap:8px;padding:9px 12px;font-size:12.5px;font-weight:650}
.ap-doc-f span:last-child{color:var(--o-brand-ink);display:inline-flex;align-items:center;gap:3px;flex:none}
.ap-bar{position:sticky;bottom:12px;z-index:3;display:flex;flex-wrap:wrap;gap:8px;align-items:center;padding:10px;background:var(--o-surface);border:1px solid var(--o-line);border-radius:var(--o-r);box-shadow:var(--o-shadow-lift)}
.ap-bar .input{flex:1 1 240px;min-width:0}
.ap-bar .btn{flex:none}
.ap-back{display:none}
@media(max-width:1100px){.ap{grid-template-columns:300px minmax(0,1fr)}}
@media(max-width:860px){
  .ap,.ap.solo{grid-template-columns:minmax(0,1fr)}
  .ap-side{position:static;max-height:none}
  .ap-scroll{overflow:visible}
  .ap .o-card-row.sel{border-color:var(--o-line);box-shadow:var(--o-shadow)}
  .ap-side .o-toolbar .o-select{max-width:none;flex:1 1 140px}
  .ap.has .ap-side{display:none}
  .ap:not(.has) .ap-main{display:none}
  .ap-back{display:inline-flex;align-self:flex-start;margin:-4px 0 -4px -6px}
  .ap-facts{grid-template-columns:minmax(0,1fr)}
  .ap-facts>div+div>.o-kv:first-child{border-top:1px dashed var(--o-line-2)}
  .ap-docs{grid-template-columns:repeat(2,minmax(0,1fr))}
  .ap-bar{bottom:0;margin:0 -14px;padding:12px 14px max(12px,env(safe-area-inset-bottom));border-radius:16px 16px 0 0;border-width:1px 0 0}
  .ap-bar .input{flex-basis:100%}
  .ap-bar .btn{flex:1 1 0;min-width:0;padding:0 8px}
  .ap-bar .btn-green{order:1;flex-basis:100%}
}`

export default function Approvals() {
  const { toast } = useSession()
  const g = useGateway()
  const [sp, setSp] = useSearchParams()
  const tab = sp.get('tab') === 'leads' ? 'leads' : 'review'
  const q = sp.get('q') || ''
  const sort = sp.get('sort') || ''
  const page = Math.max(1, Number(sp.get('page')) || 1)
  // Changes some of the list's settings and keeps the rest in the address.
  const go = (patch: Record<string, string | number>) => {
    const all: Record<string, string | number> = { tab: tab === 'leads' ? 'leads' : '', q, sort, page, id: sp.get('id') || '', ...patch }
    setSp(Object.fromEntries(Object.entries(all).filter(([k, v]) => v !== '' && !(k === 'page' && Number(v) <= 1)).map(([k, v]) => [k, String(v)])))
  }
  const [term, setTerm] = useState(q)
  useEffect(() => setTerm(q), [q])
  useEffect(() => {
    if (term.trim() === q) return
    const t = setTimeout(() => go({ q: term.trim(), page: 1, id: '' }), 300)
    return () => clearTimeout(t)
  }, [term])
  const { data, reload } = useQuery(g => g.admin.approvals(tab, { q, sort, page }), [tab, q, sort, page])
  const total: number = data?.total ?? 0
  const perPage: number = data?.per_page ?? 50
  const from = (page - 1) * perPage
  const [note, setNote] = useState('')
  const [busy, setBusy] = useState('')
  const list: any[] = data?.data || []
  const selId = sp.get('id') || list[0]?.id
  const app = list.find(a => a.id === selId) || list[0]
  useEffect(() => setNote(''), [selId])

  const decide = async (decision: string, msg: string) => {
    if (!app) return
    if (decision === 'reject' && !note.trim()) { toast('Escreva o motivo na nota interna antes de rejeitar.', true); return }
    setBusy(decision)
    try {
      await g.admin.decide(app.id, decision, note)
      toast(`${msg}: ${app.name}`)
      go({ id: list.find(a => a.id !== app.id)?.id || '' })
      reload()
    } catch (e) { toast(errMsg(e), true) } finally { setBusy('') }
  }

  const stage = (a: any) => (STAGE[a.status] ? <Badge tone={STAGE[a.status][1]}>{STAGE[a.status][0]}</Badge> : <RiskPill risk={a.risk} />)
  const where = (a: any) => `${a.city || a.email} · ${submittedAgo(a.submitted_at || a.created_at)}`
  const pager = data && total > 0 && (q || total > perPage) && (
    <div className="o-more">
      <span>{`${from + 1}–${from + list.length} de ${total}`}</span>
      <span style={{ display: 'flex', gap: 6 }}>
        {page > 1 && <button className="btn btn-ghost btn-sm" onClick={() => go({ page: page - 1, id: '' })}>Anteriores</button>}
        {from + list.length < total && <button className="btn btn-ghost btn-sm" onClick={() => go({ page: page + 1, id: '' })}>Seguintes</button>}
      </span>
    </div>
  )
  const nuit = app?.nuit?.replace(/(\d{3})(\d{3})(\d{3})/, '$1 $2 $3')

  return (
    <Page>
      <style>{CSS}</style>
      <PageHead title="Aprovações" sub={`${data?.pending ?? '…'} por rever · ${data?.leads ?? '…'} leads`} />
      <div className={'ap' + (sp.get('id') && app ? ' has' : '') + (data && !app ? ' solo' : '')}>
        <div className="ap-side">
          <div className="tabs">
            <button className={tab === 'review' ? 'on' : ''} onClick={() => setSp(q ? { q } : {})}>Por rever<span className="count">{data?.pending ?? '·'}</span></button>
            <button className={tab === 'leads' ? 'on' : ''} onClick={() => setSp(q ? { tab: 'leads', q } : { tab: 'leads' })}>Leads<span className="count">{data?.leads ?? '·'}</span></button>
          </div>
          <div className="o-toolbar">
            <label className="o-search"><Icon name="search" size={15} /><input type="search" value={term} onChange={e => setTerm(e.target.value)} placeholder="Nome, email ou telefone" title="Pesquisa por nome, dono, email, NUIT ou telefone" aria-label="Pesquisar contas" /></label>
            <select className="o-select" value={sort} onChange={e => go({ sort: e.target.value, page: 1, id: '' })} aria-label="Ordenar">
              {SORTS[tab].map(([k, l]) => <option key={k} value={k}>{l}</option>)}
            </select>
          </div>
          <div className="ap-scroll">
            {!data ? <Skeleton h={320} r={16} /> : (
              <List head={false} rows={list} rowKey={a => a.id} onRow={a => go({ id: a.id })} selected={a => a.id === app?.id}
                empty={q ? <Empty icon="search" title="Sem resultados" text={`Nenhuma conta para “${q}”.`} /> : tab === 'leads' ? <Empty title="Sem leads" text="Nenhuma lead por contactar." /> : <Empty title="Fila vazia" text="Bom trabalho." />}
                footer={pager}
                cols={[
                  { head: '', w: '36px', cell: a => <Avatar text={a.initials} color={a.avatar_color} size={36} radius={10} font={12.5} /> },
                  { head: '', w: 'minmax(0,1fr)', cell: a => <><div className="o-t1">{a.name}{a.imported_from === 'v1' && <V1Tag />}</div><div className="o-t2">{where(a)}</div></> },
                  { head: '', w: 'auto', cell: stage },
                ]}
                card={a => ({ lead: <Avatar text={a.initials} color={a.avatar_color} size={38} radius={11} font={13} />, title: <>{a.name}{a.imported_from === 'v1' && <V1Tag />}</>, sub: where(a), meta: stage(a) })} />
            )}
          </div>
        </div>

        {app && (
          <div className="ap-main">
            <button className="btn btn-ghost btn-sm ap-back" onClick={() => go({ id: '' })}><Icon name="arrowLeft" size={15} stroke={2.2} />Fila</button>
            <Panel>
              <div className="ap-id">
                <Avatar text={app.initials} color={app.avatar_color} size={48} radius={14} font={16} />
                <div style={{ flex: 1, minWidth: 0 }}>
                  <h2>{app.name}</h2>
                  <div className="o-t2" style={{ whiteSpace: 'normal' }}>{app.owner_name} · {app.submitted_at ? 'submetido ' + submittedAgo(app.submitted_at) : 'registado ' + submittedAgo(app.created_at) + ' · verificação por submeter'}</div>
                </div>
                <Badge tone={app.status === 'lead' ? undefined : app.status === 'info_requested' ? 'info' : 'warn'} dot>{app.status === 'lead' ? 'Lead' : app.status === 'info_requested' ? 'Docs pedidos' : 'Por rever'}</Badge>
              </div>
              <div className="ap-facts" style={{ marginTop: 14 }}>
                <div>
                  <KV k="Tipo" v={app.business_type === 'company' ? 'Empresa' : 'Em nome individual'} />
                  <KV k="NUIT" v={nuit ? <span className="o-mono">{nuit}</span> : '—'} />
                  <KV k="Cidade" v={app.city || '—'} />
                </div>
                <div>
                  <KV k="Telemóvel" v={app.phone || '—'} />
                  <KV k="Email" v={<span style={{ overflowWrap: 'anywhere' }}>{app.email || '—'}</span>} />
                  <KV k="Volume esperado" v={app.expected_volume || '—'} />
                </div>
              </div>
            </Panel>

            <Panel title="Verificações automáticas" sub={`${app.checks.filter((c: any) => c.kind === 'ok').length} de ${app.checks.length} em ordem`} pad={false}>
              {app.checks.map((c: any, i: number) => (
                <div key={i} className="ap-check">
                  <span className={'ap-ck ' + (CHECK[c.kind]?.[1] || '')}><Icon name={CHECK[c.kind]?.[0] || 'alert'} size={14} stroke={2.6} /></span>
                  <div style={{ minWidth: 0 }}><div className="o-t1" style={{ whiteSpace: 'normal' }}>{c.label}</div><div className="o-t2" style={{ whiteSpace: 'normal' }}>{c.note}</div></div>
                </div>
              ))}
              {!app.checks.length && <Empty title="Sem verificações" />}
            </Panel>

            <div className="o-grid c2">
              <Panel title="Documentos" sub={app.documents.length ? String(app.documents.length) : undefined}>
                {app.documents.length ? (
                  <div className="ap-docs">
                    {app.documents.map((d: any) => (
                      <a key={d.id} href={g.admin.documentUrl(d.id)} target="_blank" rel="noreferrer" className="ap-doc">
                        <div className="ap-doc-p"><Icon name="doc" size={20} /><span className="o-mono">{d.file_name}</span><span style={{ fontSize: 11 }}>{fileSize(d.size)}</span></div>
                        <div className="ap-doc-f"><span className="o-t1">{DOC_LABEL[d.kind] || d.kind}</span><span>Abrir<Icon name="arrowUpRight" size={13} stroke={2.2} /></span></div>
                      </a>
                    ))}
                  </div>
                ) : <div className="o-t2">Sem documentos.</div>}
              </Panel>
              <Panel title="Conta para levantamentos">
                {app.bank_account ? (
                  <>
                    <KV k="Banco" v={app.bank_account.bank_name} />
                    <KV k="NIB" v={<span className="o-mono">•••• {app.bank_account.nib_last5}</span>} />
                    <KV k="Titular" v={app.bank_account.holder} />
                  </>
                ) : <div className="o-t2" style={{ display: 'flex', alignItems: 'center', gap: 8 }}><Icon name="bank" size={16} />Por indicar</div>}
              </Panel>
            </div>

            <div className="ap-bar">
              <input className="input" placeholder="Nota interna (obrigatória para rejeitar ou pedir docs)" value={note} onChange={e => setNote(e.target.value)} aria-label="Nota interna" />
              <button className="btn btn-danger" onClick={() => decide('reject', 'Conta rejeitada')} disabled={!!busy}>{busy === 'reject' ? <Spinner dark /> : 'Rejeitar'}</button>
              <button className="btn btn-outline" onClick={() => decide('info', 'Pedido de documentos enviado')} disabled={!!busy}>{busy === 'info' ? <Spinner dark /> : 'Pedir documentos'}</button>
              <button className="btn btn-green" onClick={() => decide('approve', 'Conta aprovada')} disabled={!!busy}>{busy === 'approve' ? <Spinner /> : <><Icon name="check" size={16} stroke={2.6} />Aprovar conta</>}</button>
            </div>
          </div>
        )}
      </div>
    </Page>
  )
}
