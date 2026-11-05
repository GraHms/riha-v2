import { useEffect, useMemo, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { Icon, Modal, QR, Skeleton, Spinner, Toggle } from '../../../components/ui'
import { useGateway } from '../../../core/gateway'
import { errMsg, type ApiError } from '../../../core/model'
import { copy, initials } from '../../../lib/format'
import { useSession } from '../../../lib/session'
import { BrandFrame, CatalogView, onColor, type CustomerPlan } from '../../public/subs/CatalogView'
import { useShell } from '../AppShell'
import { customerView, downloadSvg, money } from './common'
import { sharePlan } from './Plans'
import { useSubs } from './SubsShell'

const ACCENTS = ['#0A8205', '#0F766E', '#1D4ED8', '#6D28D9', '#BE185D', '#C2410C', '#CA8A04', '#0B120E']
const BACKGROUNDS = ['#F4F3EE', '#FFFFFF', '#EEF3FB', '#F3EFFA', '#FBEFF2', '#EEF7F0', '#FBF4E6', '#0B120E']
const NAMES: Record<string, string> = { '#0A8205': 'Verde', '#0F766E': 'Turquesa', '#1D4ED8': 'Azul', '#6D28D9': 'Roxo', '#BE185D': 'Rosa', '#C2410C': 'Laranja', '#CA8A04': 'Dourado', '#0B120E': 'Preto',
  '#F4F3EE': 'Areia', '#FFFFFF': 'Branco', '#EEF3FB': 'Azul claro', '#F3EFFA': 'Lilás', '#FBEFF2': 'Rosa claro', '#EEF7F0': 'Verde claro', '#FBF4E6': 'Creme' }

type Draft = { slug: string; title: string; description: string; accent: string; background: string }

function Swatches({ value, options, fallback, onChange, label }: { value: string; options: string[]; fallback: string; onChange: (v: string) => void; label: string }) {
  const cur = (value || fallback).toUpperCase()
  const custom = !options.includes(cur)
  return (
    <div role="radiogroup" aria-label={label} style={{ display: 'flex', flexWrap: 'wrap', gap: 8, alignItems: 'center' }}>
      {options.map(c => (
        <button key={c} type="button" role="radio" aria-checked={cur === c} aria-label={NAMES[c] || c} title={NAMES[c] || c} onClick={() => onChange(c === fallback ? '' : c)}
          style={{ width: 34, height: 34, borderRadius: 11, background: c, border: '1px solid rgba(11,18,14,.14)', boxShadow: cur === c ? '0 0 0 2px #fff, 0 0 0 4px var(--ink)' : 'none', display: 'grid', placeItems: 'center', cursor: 'pointer', padding: 0 }}>
          {cur === c && <Icon name="check" size={15} stroke={3} color={onColor(c)} />}
        </button>
      ))}
      <label title="Outra cor" style={{ position: 'relative', width: 34, height: 34, borderRadius: 11, border: '1.5px dashed #BDBBB2', display: 'grid', placeItems: 'center', cursor: 'pointer', background: custom ? cur : '#fff', boxShadow: custom ? '0 0 0 2px #fff, 0 0 0 4px var(--ink)' : 'none' }}>
        <Icon name="plus" size={15} color={custom ? onColor(cur) : 'var(--muted)'} />
        <input type="color" value={cur.toLowerCase()} onChange={e => onChange(e.target.value.toUpperCase())} aria-label={'Outra cor: ' + label} style={{ position: 'absolute', inset: 0, opacity: 0, cursor: 'pointer' }} />
      </label>
    </div>
  )
}

export default function Catalog() {
  const g = useGateway()
  const { toast } = useSession()
  const shell = useShell()
  const sx = useSubs()
  const page = sx.page
  const [rows, setRows] = useState<any[]>([])
  const [multi, setMulti] = useState(true)
  const [d, setD] = useState<Draft | null>(null)
  const [rowsDirty, setRowsDirty] = useState(false)
  const [slugState, setSlugState] = useState<{ ok: boolean; message?: string; checking?: boolean }>({ ok: true })
  const [busy, setBusy] = useState(false)
  const [coverBusy, setCoverBusy] = useState(false)
  const [err, setErr] = useState<ApiError | null>(null)
  const [advanced, setAdvanced] = useState(false)
  const [preview, setPreview] = useState(false)
  const [sel, setSel] = useState<string[]>([])
  const fileRef = useRef<HTMLInputElement>(null)
  const base = (p: any): Draft => ({ slug: p.slug, title: p.custom_title || '', description: p.description || '', accent: p.accent || '', background: p.background || '' })
  useEffect(() => { setRows(sx.plans.filter((p: any) => p.status !== 'archived').map((p: any) => ({ ...p }))); setRowsDirty(false) }, [sx.plans])
  useEffect(() => { if (page) { setD(base(page)); setMulti(page.multi); setAdvanced(a => a || !!(page.accent || page.background || page.cover_url)) } }, [page])
  useEffect(() => {
    if (!d || !page || d.slug === page.slug) { setSlugState({ ok: true }); return }
    setSlugState({ ok: false, checking: true })
    const t = setTimeout(() => g.subscriptions.checkCatalogSlug(d.slug).then(r => setSlugState({ ok: r.ok, message: r.message })).catch(() => setSlugState({ ok: false, message: 'Não foi possível confirmar o endereço.' })), 400)
    return () => clearTimeout(t)
  }, [d?.slug, page?.slug])
  const shown = rows.filter(r => r.in_catalog && r.status === 'active')
  const previewPlans: CustomerPlan[] = useMemo(() => shown.map(r => ({ id: r.id, name: r.name, amount: r.amount, interval: r.interval, highlights: r.highlights, ...customerView(r) })), [JSON.stringify(shown)])
  if (!page || !d) return <Skeleton h={300} r={22} />
  const pageDirty = JSON.stringify(d) !== JSON.stringify(base(page))
  const dirty = pageDirty || rowsDirty || multi !== page.multi
  const set = (patch: Partial<Draft>) => setD({ ...d, ...patch })
  const move = (i: number, dir: number) => { const j = i + dir; if (j < 0 || j >= rows.length) return; const n = rows.slice(); [n[i], n[j]] = [n[j], n[i]]; setRows(n); setRowsDirty(true) }
  const save = async () => {
    setBusy(true); setErr(null)
    try {
      await g.subscriptions.saveCatalog(rows.map(r => ({ plan_id: r.id, in_catalog: r.in_catalog })), multi, d)
      toast('Catálogo guardado'); setRowsDirty(false); sx.reloadPage(); sx.reloadPlans()
    } catch (e) { setErr(e as ApiError) } finally { setBusy(false) }
  }
  const upload = async (file?: File | null) => {
    if (!file) return
    setCoverBusy(true)
    try { await g.subscriptions.uploadCover(file); toast('Capa atualizada'); sx.reloadPage() }
    catch (e) { toast(errMsg(e), true) } finally { setCoverBusy(false); if (fileRef.current) fileRef.current.value = '' }
  }
  const removeCover = async () => { setCoverBusy(true); try { await g.subscriptions.removeCover(); toast('Capa removida'); sx.reloadPage() } catch (e) { toast(errMsg(e), true) } finally { setCoverBusy(false) } }
  const url = page.url as string
  const draftPage = { ...page, title: d.title || shell.merchant.name, description: d.description, accent: d.accent, background: d.background }
  const merchant = { name: shell.merchant.name, initials: initials(shell.merchant.name), color: shell.merchant.avatar_color }
  const toggleSel = (id: string) => setSel(multi ? (sel.includes(id) ? sel.filter(x => x !== id) : [...sel, id]) : sel.includes(id) ? [] : [id])
  const Preview = (
    <BrandFrame page={draftPage} host={false}>
      {previewPlans.length ? <CatalogView merchant={merchant} page={draftPage} plans={previewPlans} multi={multi} sel={sel} onToggle={toggleSel} onContinue={() => {}} />
        : <div style={{ padding: 40, textAlign: 'center', color: 'var(--on-bg-2)', fontSize: 14 }}>Nenhum plano visível. Ligue pelo menos um plano em Planos no catálogo.</div>}
    </BrandFrame>
  )
  const fe = (p: string) => (err?.param === p ? ' err' : '')
  return (
    <div className="cat-edit">
      <style>{`
        .cat-edit{display:grid;grid-template-columns:minmax(0,1fr) 380px;gap:20px;align-items:start}
        .ce-col{display:flex;flex-direction:column;gap:14px;min-width:0}
        .ce-block{padding:20px 22px}
        .ce-addr{display:flex;align-items:stretch;border:1px solid #E1DFD7;border-radius:12px;overflow:hidden;background:#fff}
        .ce-addr span{display:flex;align-items:center;padding:0 12px;background:var(--paper);color:var(--muted);font-size:14px;white-space:nowrap}
        .ce-addr input{flex:1;min-width:0;border:0;outline:0;padding:0 12px;height:44px;font:inherit;font-size:15px;font-family:var(--mono)}
        .ce-addr:focus-within{border-color:var(--ink);box-shadow:0 0 0 3px rgba(11,18,14,.08)}
        .ce-addr.err{border-color:var(--red)}
        .ce-preview{position:sticky;top:16px}
        .ce-phone{border-radius:30px;overflow:hidden;border:8px solid #0B120E;max-height:calc(100vh - 120px);overflow-y:auto;background:#0B120E}
        .ce-actions{position:sticky;bottom:0;z-index:2;display:flex;gap:10px;justify-content:flex-end;flex-wrap:wrap;padding:14px 0 calc(14px + env(safe-area-inset-bottom));background:linear-gradient(to top,var(--paper) 75%,transparent)}
        .ce-adv{display:flex;width:100%;align-items:center;gap:10px;background:none;border:0;padding:0;font:inherit;text-align:left;cursor:pointer}
        @media (max-width:1100px){.cat-edit{grid-template-columns:1fr}.ce-preview{display:none}}
        @media (max-width:700px){.ce-block{padding:18px 16px}.ce-addr span{padding:0 8px;font-size:13px}.ce-actions{flex-wrap:nowrap}.ce-actions .btn{flex:1;padding:0 12px}.ce-actions .ce-eye{flex:none;width:48px;padding:0}}
      `}</style>
      <div className="ce-col">
        <div className="card" style={{ padding: 16, display: 'flex', gap: 16, alignItems: 'center', flexWrap: 'wrap' }}>
          <div id="cat-qr" style={{ padding: 6, borderRadius: 12, boxShadow: 'inset 0 0 0 1px #EEECE5' }}><QR value={url || window.location.origin} size={88} logo={false} /></div>
          <div style={{ flex: 1, minWidth: 200 }}>
            <div style={{ fontSize: 13, color: 'var(--muted)' }}>Endereço do catálogo</div>
            <a href={url} target="_blank" rel="noreferrer" className="mono" style={{ fontSize: 14.5, marginTop: 4, wordBreak: 'break-all', display: 'block', color: 'var(--ink)', fontWeight: 600 }}>{url.replace(/^https?:\/\//, '')}</a>
            <div style={{ display: 'flex', gap: 8, marginTop: 12, flexWrap: 'wrap' }}>
              <button className="btn btn-ink btn-sm" onClick={() => sharePlan(page.title, url, toast)}><Icon name="share" size={14} />Partilhar</button>
              <a className="btn btn-outline btn-sm" href={`https://wa.me/?text=${encodeURIComponent(`${page.title}: escolha e pague aqui ${url}`)}`} target="_blank" rel="noreferrer"><Icon name="whatsapp" size={14} />WhatsApp</a>
              <button className="btn btn-outline btn-sm" onClick={async () => { await copy(url); toast('Link copiado') }}><Icon name="copy" size={14} />Copiar</button>
              <button className="btn btn-outline btn-sm" onClick={() => downloadSvg('#cat-qr', 'qr-catalogo.svg')}><Icon name="download" size={14} />QR</button>
            </div>
          </div>
        </div>

        <section className="card ce-block">
          <h3 style={{ margin: 0, fontSize: 16 }}>A página</h3>
          <label style={{ display: 'block', marginTop: 14 }}>
            <span className="label">Endereço</span>
            <div className={'ce-addr' + (!slugState.ok && !slugState.checking ? ' err' : '') + fe('slug')}>
              {!page.subdomain && <span>{page.host}/a/</span>}
              <input value={d.slug} onChange={e => set({ slug: e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, '').slice(0, 30) })} aria-label="Endereço do catálogo" spellCheck={false} autoCapitalize="off" />
              {page.subdomain && <span>.{page.host}</span>}
            </div>
            <div style={{ fontSize: 12.5, marginTop: 6, color: slugState.checking ? 'var(--muted)' : slugState.ok ? 'var(--green-text)' : 'var(--red-dark)' }}>
              {d.slug === page.slug ? 'Letras, números e hífenes. Quem tiver o endereço antigo continua a chegar aqui.' : slugState.checking ? 'A confirmar…' : slugState.ok ? 'Disponível. Guarde para começar a usar.' : slugState.message}
            </div>
          </label>
          <label style={{ display: 'block', marginTop: 14 }}>
            <span className="label">Título</span>
            <input className={'input' + fe('title')} value={d.title} maxLength={80} onChange={e => set({ title: e.target.value })} placeholder={shell.merchant.name} />
            <div style={{ fontSize: 12.5, color: 'var(--muted)', marginTop: 6 }}>Vazio, mostra o nome do negócio. Por exemplo: Propinas 2027.</div>
          </label>
          <label style={{ display: 'block', marginTop: 14 }}>
            <span className="between"><span className="label" style={{ margin: 0 }}>Descrição</span><span style={{ fontSize: 12, color: 'var(--muted)' }}>{d.description.length}/400</span></span>
            <textarea className={'input' + fe('description')} value={d.description} maxLength={400} rows={4} onChange={e => set({ description: e.target.value })} placeholder="Diga em duas frases o que oferece e para quem. Aparece no topo da página e no Google." style={{ height: 'auto', padding: '10px 14px', lineHeight: 1.5, resize: 'vertical', marginTop: 6 }} />
          </label>
        </section>

        <section className="card" style={{ overflow: 'hidden' }}>
          <div className="between" style={{ padding: '16px 20px', borderBottom: '1px solid var(--line-3)' }}>
            <div><h3 style={{ margin: 0, fontSize: 16 }}>Planos no catálogo</h3><div style={{ fontSize: 12.5, color: 'var(--muted)', marginTop: 2 }}>Ligue os que quer mostrar. Use as setas para os ordenar.</div></div>
            <button className="btn btn-outline btn-sm" onClick={sx.newPlan}><Icon name="plus" size={14} />Novo plano</button>
          </div>
          {rows.length === 0 && <div style={{ padding: 20, color: 'var(--muted)', fontSize: 14 }}>Ainda não há planos.</div>}
          {rows.map((r, i) => (
            <div key={r.id} style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '12px 16px 12px 12px', borderBottom: '1px solid var(--line-3)' }}>
              <div style={{ display: 'flex', flexDirection: 'column' }}>
                <button className="icon-btn" style={{ width: 30, height: 22, border: 0 }} disabled={i === 0} onClick={() => move(i, -1)} aria-label={'Subir ' + r.name}><Icon name="chevronDown" size={15} style={{ transform: 'rotate(180deg)' }} /></button>
                <button className="icon-btn" style={{ width: 30, height: 22, border: 0 }} disabled={i === rows.length - 1} onClick={() => move(i, 1)} aria-label={'Descer ' + r.name}><Icon name="chevronDown" size={15} /></button>
              </div>
              <div style={{ flex: 1, minWidth: 0 }}>
                <div style={{ fontWeight: 600, fontSize: 14.5, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{r.name}</div>
                <div style={{ fontSize: 12.5, color: 'var(--muted)' }}>{money(r.amount)} MT · {r.status === 'draft' ? 'rascunho, ative-o para mostrar' : `${r.stats?.members || 0} membros`} · <Link to={`/painel/assinaturas/planos/${r.id}/editar`}>Editar</Link></div>
              </div>
              <Toggle on={r.in_catalog && r.status === 'active'} label={'Mostrar ' + r.name} onChange={v => { if (r.status !== 'active') { toast('Ative o plano primeiro: abra-o e guarde.', true); return } setRows(rows.map(x => (x.id === r.id ? { ...x, in_catalog: v } : x))); setRowsDirty(true) }} />
            </div>
          ))}
          <div style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '14px 20px' }}>
            <div style={{ flex: 1 }}><div style={{ fontWeight: 600, fontSize: 14 }}>Permitir escolher vários</div><div style={{ fontSize: 12.5, color: 'var(--muted)' }}>Por exemplo propina e transporte, pagos juntos no mesmo link.</div></div>
            <Toggle on={multi} label="Permitir escolher vários" onChange={setMulti} />
          </div>
        </section>

        <section className="card ce-block">
          <button className="ce-adv" onClick={() => setAdvanced(!advanced)} aria-expanded={advanced}>
            <span style={{ flex: 1 }}><h3 style={{ margin: 0, fontSize: 16 }}>Avançado: capa e cores</h3><span style={{ fontSize: 12.5, color: 'var(--muted)' }}>Opcional. Dê à página o aspeto do seu negócio.</span></span>
            <Icon name="chevronDown" size={18} style={{ transform: advanced ? 'rotate(180deg)' : undefined, transition: 'transform .15s' }} />
          </button>
          {advanced && (
            <div style={{ marginTop: 16, display: 'flex', flexDirection: 'column', gap: 18 }}>
              <div>
                <span className="label">Imagem de capa</span>
                <div style={{ display: 'flex', gap: 14, alignItems: 'center', flexWrap: 'wrap' }}>
                  <div style={{ width: 160, height: 64, borderRadius: 12, background: page.cover_url ? `center/cover url("${page.cover_url}")` : d.accent || '#0A8205', border: '1px solid var(--line)', flex: 'none' }} />
                  <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
                    <button className="btn btn-outline btn-sm" disabled={coverBusy} onClick={() => fileRef.current?.click()}>{coverBusy ? <Spinner dark /> : <><Icon name="upload" size={14} />{page.cover_url ? 'Trocar imagem' : 'Escolher imagem'}</>}</button>
                    {page.cover_url && <button className="btn btn-ghost btn-sm" disabled={coverBusy} onClick={removeCover}>Remover</button>}
                  </div>
                  <input ref={fileRef} type="file" accept="image/jpeg,image/png,image/webp" hidden onChange={e => upload(e.target.files?.[0])} />
                </div>
                <div style={{ fontSize: 12.5, color: 'var(--muted)', marginTop: 8, lineHeight: 1.45 }}>JPG, PNG ou WebP, com pelo menos 600 × 200 pixels. Aparece no topo da página e na pré-visualização do link no WhatsApp. Guarda-se logo.</div>
              </div>
              <div>
                <span className="label">Cor principal</span>
                <Swatches label="Cor principal" value={d.accent} options={ACCENTS} fallback="#0A8205" onChange={v => set({ accent: v })} />
                <div style={{ fontSize: 12.5, color: 'var(--muted)', marginTop: 8 }}>Botões e planos escolhidos. O texto por cima fica branco ou preto, o que se ler melhor.</div>
              </div>
              <div>
                <span className="label">Fundo</span>
                <Swatches label="Fundo" value={d.background} options={BACKGROUNDS} fallback="#F4F3EE" onChange={v => set({ background: v })} />
              </div>
              {(d.accent || d.background) && <button type="button" className="link-btn" style={{ alignSelf: 'flex-start' }} onClick={() => set({ accent: '', background: '' })}>Repor as cores da RIHA</button>}
            </div>
          )}
        </section>

        {err && <div className="form-err">{err.message}</div>}
        <div className="ce-actions">
          <button className={'btn btn-outline only-mobile' + (dirty ? ' ce-eye' : '')} onClick={() => setPreview(true)} aria-label="Ver como o cliente vê" title="Ver como o cliente vê"><Icon name="eye" size={18} />{!dirty && 'Ver como o cliente vê'}</button>
          {dirty && <button className="btn btn-outline" disabled={busy} onClick={() => { setD(base(page)); setMulti(page.multi); setRows(sx.plans.filter((p: any) => p.status !== 'archived').map((p: any) => ({ ...p }))); setRowsDirty(false); setErr(null) }}>Desfazer</button>}
          {dirty && <button className="btn btn-green" disabled={busy || (pageDirty && d.slug !== page.slug && !slugState.ok)} onClick={save}>{busy ? <Spinner /> : 'Guardar catálogo'}</button>}
        </div>
      </div>
      <aside className="ce-preview">
        <div style={{ fontSize: 12, fontWeight: 700, color: 'var(--muted)', textTransform: 'uppercase', letterSpacing: '.06em', margin: '6px 0 10px' }}>O que o cliente vê</div>
        <div className="ce-phone">{Preview}</div>
      </aside>
      {preview && <Modal onClose={() => setPreview(false)} width={Math.min(420, window.innerWidth - 24)}>
        <div className="between" style={{ marginBottom: 12 }}><b>O que o cliente vê</b><button className="icon-btn" onClick={() => setPreview(false)} aria-label="Fechar"><Icon name="close" size={16} /></button></div>
        <div style={{ borderRadius: 22, overflow: 'hidden', margin: '0 -6px', maxHeight: '75vh', overflowY: 'auto' }}>{Preview}</div>
      </Modal>}
    </div>
  )
}
