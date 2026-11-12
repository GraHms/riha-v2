import { useEffect, useState } from 'react'
import { errMsg } from '../../core/model'
import { useGateway } from '../../core/gateway'
import { copy, fmt, initials, METHOD_NAME, parseMT } from '../../lib/format'
import { useSession } from '../../lib/session'
import { Icon, MethodIcon, QR, Spinner, Toggle } from '../../components/ui'
import { LinkPreview, SUBMIT, type LinkDraft } from '../public/LinkCard'

export type LinkObj = { id: string; name: string; slug: string; amount: number | null; custom_amount: boolean; reusable: boolean; url: string; active: boolean; type: string; payment_method_types: string[]; expires_at: number | null; payments_count: number; total_received: number; livemode: boolean; location?: string; details?: string; image_url?: string
  suggested_amount?: number | null; min_amount?: number | null; max_amount?: number | null; after_message?: string; redirect_url?: string; archived?: boolean; last_payment_at?: string | null; submit_type?: string }

const EXPIRY: [string, number][] = [['Nunca', 0], ['24 horas', 1], ['7 dias', 7], ['30 dias', 30]]

const mtText = (v?: number | null) => (v ? String(v / 100).replace('.', ',') : '')

// useLinkForm creates a link, or edits one: "from" fills the form with a link's
// values, and "edit" saves them back to it. Duplicating fills the form and creates.
export function useLinkForm(onCreated: (l: LinkObj) => void, from?: LinkObj, edit = false) {
  const { me, toast } = useSession()
  const g = useGateway()
  const bps = me?.merchant?.fee_bps ?? 600
  const { live } = useSession()
  // What this mode can't take yet: the test list is shorter while new methods are tried there.
  const soon: string[] = (live ? me?.merchant?.coming_soon : me?.merchant?.test_coming_soon ?? me?.merchant?.coming_soon) ?? []
  const abroadOK = !soon.includes('abroad')
  const [amount, setAmount] = useState(from && !from.custom_amount ? mtText(from.amount) : '1250')
  const [name, setName] = useState(from ? (edit ? from.name : from.name + ' (cópia)') : '')
  const [methods, setMethods] = useState<string[]>(from ? from.payment_method_types.filter(m => !soon.includes(m)) : ['mpesa', 'emola', 'bim', 'card'].filter(m => !soon.includes(m)))
  const [reusable, setReusable] = useState(from ? from.reusable : true)
  // Advanced: paying from abroad, an option a merchant turns on per link.
  const [advanced, setAdvanced] = useState(!!from?.payment_method_types.includes('abroad'))
  const [custom, setCustom] = useState(from ? from.custom_amount : false)
  const [kind, setKind] = useState<'link' | 'qr'>(from?.type === 'qr' ? 'qr' : 'link')
  // -1 keeps the expiry the link has.
  const [exp, setExp] = useState(edit && from?.expires_at ? -1 : 0)
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')
  const [details, setDetails] = useState(from?.details || '')
  const [image, setImage] = useState<File | null>(null)
  const [keptImage, setKeptImage] = useState(edit ? from?.image_url || '' : '')
  const [imageUrl, setImageUrl] = useState(keptImage)
  const [suggested, setSuggested] = useState(mtText(from?.suggested_amount))
  const [min, setMin] = useState(mtText(from?.min_amount))
  const [max, setMax] = useState(mtText(from?.max_amount))
  const [after, setAfter] = useState<'page' | 'redirect'>(from?.redirect_url ? 'redirect' : 'page')
  const [message, setMessage] = useState(from?.after_message || '')
  const [redirect, setRedirect] = useState(from?.redirect_url || '')
  const [submitType, setSubmitType] = useState(from?.submit_type || 'pay')
  const [tab, setTab] = useState<'edit' | 'preview'>('edit')
  useEffect(() => { if (!image) { setImageUrl(keptImage); return } const u = URL.createObjectURL(image); setImageUrl(u); return () => URL.revokeObjectURL(u) }, [image, keptImage])
  const c = parseMT(amount)
  const cents = (v: string) => (v.trim() ? parseMT(v) : 0)
  const fee = Math.round((c * bps) / 10000)
  const submit = async () => {
    setBusy(true); setErr('')
    try {
      const body: Record<string, unknown> = { name, custom_amount: custom, reusable, payment_method_types: methods, type: kind, details, submit_type: submitType,
        after_message: after === 'page' ? message : '', redirect_url: after === 'redirect' ? redirect.trim() : '' }
      if (!custom) body.amount = c
      else Object.assign(body, { suggested_amount: cents(suggested), min_amount: cents(min), max_amount: cents(max) })
      if (edit && from) {
        delete body.custom_amount; delete body.type
        if (exp === 0) body.expires_at = 0
        else if (exp > 0) body.expires_at = Math.floor(Date.now() / 1000) + exp * 86400
        let l = await g.links.update(from.id, body)
        try {
          if (image) l = await g.links.uploadImage(from.id, image)
          else if (from.image_url && !keptImage) l = await g.links.removeImage(from.id)
        } catch (e) { toast('Link guardado, mas a imagem não: ' + errMsg(e), true); onCreated(l); return }
        toast('Link guardado')
        onCreated(l)
        return
      }
      if (exp > 0) body.expires_at = Math.floor(Date.now() / 1000) + exp * 86400
      let l = await g.links.create(body)
      if (image) {
        try { l = await g.links.uploadImage(l.id, image) }
        catch (e) { toast('Link criado, mas a imagem não foi guardada: ' + errMsg(e), true); onCreated(l); return }
      }
      toast('Link criado')
      onCreated(l)
    } catch (e) { setErr(errMsg(e)) } finally { setBusy(false) }
  }
  const toggleM = (m: string) => setMethods(ms => (ms.includes(m) ? (ms.length > 1 ? ms.filter(x => x !== m) : ms) : [...ms, m]))
  const valid = name.trim() !== '' && (custom || c >= 1000) && (after === 'page' || /^https?:\/\/\S+\.\S+/.test(redirect.trim()))
  const draft: LinkDraft = { name, details, image_url: imageUrl, merchant: { name: me?.merchant?.name || '', initials: initials(me?.merchant?.name || ''), avatar_color: me?.merchant?.avatar_color },
    amount: c, custom, suggested: cents(suggested), min: cents(min), max: cents(max), methods, after, message, redirect, submit: submitType }
  const tabs = (
    <div className="seg" style={{ display: 'flex' }} role="tablist">
      <button type="button" role="tab" aria-selected={tab === 'edit'} style={{ flex: 1 }} className={tab === 'edit' ? 'on' : ''} onClick={() => setTab('edit')}>Editar</button>
      <button type="button" role="tab" aria-selected={tab === 'preview'} style={{ flex: 1 }} className={tab === 'preview' ? 'on' : ''} onClick={() => setTab('preview')}>O que o cliente vê</button>
    </div>
  )
  const preview = <LinkPreview d={draft} />
  const mt = (label: string, v: string, set: (v: string) => void, hint: string) => (
    <label style={{ minWidth: 0 }}><span className="label">{label} <span style={{ fontWeight: 400, color: 'var(--muted)' }}>MT</span></span>
      <input className="input tnum" inputMode="decimal" value={v} onChange={e => set(e.target.value.replace(/[^\d.,]/g, '').slice(0, 9))} placeholder={hint} style={{ width: '100%', minWidth: 0, textAlign: 'right' }} />
    </label>
  )

  const amountBlock = (big: boolean) => custom ? (
    <div style={{ textAlign: 'center', padding: big ? '18px 0 4px' : 6 }}>
      <div style={{ fontSize: 13, color: 'var(--muted)' }}>Valor a cobrar</div>
      <div className="display" style={{ fontSize: big ? 44 : 36, marginTop: 6 }}>Cliente escolhe</div>
      <div style={{ fontSize: 13, color: 'var(--text-2)', marginTop: 6 }}>Taxa de {me?.merchant?.fee_label} sobre cada pagamento</div>
    </div>
  ) : (
    <div style={{ textAlign: 'center' }}>
      <div style={{ fontSize: 13.5, color: 'var(--muted)' }}>Valor a cobrar</div>
      <div style={{ display: 'flex', alignItems: 'baseline', justifyContent: 'center', gap: 6, marginTop: 4 }}>
        <input aria-label="Valor" value={amount} onChange={e => setAmount(e.target.value.replace(/[^\d.,]/g, '').slice(0, 9))} inputMode="decimal"
          style={{ width: `${Math.max(1, amount.length) * 0.74}em`, minWidth: 50, maxWidth: 260, border: 0, background: 'transparent', textAlign: 'right', fontFamily: 'var(--display)', fontWeight: 700, fontSize: big ? 58 : 48, letterSpacing: '-0.045em', padding: 0, caretColor: 'var(--green)' }} />
        <span className="display" style={{ fontSize: big ? 26 : 22, color: 'var(--muted-2)' }}>MT</span>
      </div>
      <div style={{ display: 'inline-flex', gap: 6, alignItems: 'center', marginTop: 8, fontSize: 13, color: 'var(--text-2)', background: big ? 'var(--paper-2)' : 'transparent', padding: big ? '6px 12px' : 0, borderRadius: 999 }}>Recebe <b className="tnum">{fmt(c - fee)} MT</b> após taxa de {me?.merchant?.fee_label}</div>
      {big && <div style={{ display: 'flex', justifyContent: 'center', gap: 8, marginTop: 16, flexWrap: 'wrap' }}>{[500, 1000, 2500, 5000].map(v => <button key={v} type="button" className="btn btn-outline" style={{ height: 36, borderRadius: 999, fontSize: 13.5, padding: '0 14px' }} onClick={() => setAmount(String(v))}>{fmt(v * 100, false)}</button>)}</div>}
    </div>
  )

  const fields = (
    <>
      <label><span className="label">O que está a cobrar</span><input className="input" style={{ height: 50 }} value={name} onChange={e => setName(e.target.value)} placeholder="Bolo de chocolate · 1 kg" maxLength={200} /></label>
      <label>
        <span className="between"><span className="label" style={{ margin: 0 }}>Descrição <span style={{ fontWeight: 400, color: 'var(--muted)' }}>(opcional)</span></span><span style={{ fontSize: 12, color: 'var(--muted)' }}>{details.length}/500</span></span>
        <textarea className="input" rows={3} maxLength={500} value={details} onChange={e => setDetails(e.target.value)} placeholder="Mais pormenores para o cliente: o que inclui, quando entrega." style={{ height: 'auto', padding: '10px 14px', lineHeight: 1.5, resize: 'vertical', marginTop: 6 }} />
      </label>
      <div>
        <span className="label">Imagem <span style={{ fontWeight: 400, color: 'var(--muted)' }}>(opcional)</span></span>
        <div className="row" style={{ gap: 12 }}>
          <div style={{ width: 96, height: 54, borderRadius: 10, border: '1px solid var(--line)', background: imageUrl ? `center/cover url("${imageUrl}")` : 'var(--paper-2)', display: 'grid', placeItems: 'center', flex: 'none' }}>{!imageUrl && <Icon name="upload" size={16} color="var(--muted)" />}</div>
          <label className="btn btn-outline btn-sm" style={{ cursor: 'pointer' }}>{image || keptImage ? 'Trocar' : 'Escolher imagem'}<input type="file" accept="image/jpeg,image/png,image/webp" hidden onChange={e => setImage(e.target.files?.[0] || null)} /></label>
          {(image || keptImage) && <button type="button" className="btn btn-ghost btn-sm" onClick={() => { setImage(null); setKeptImage('') }}>Remover</button>}
        </div>
        <div style={{ fontSize: 12, color: 'var(--muted)', marginTop: 6 }}>JPG, PNG ou WebP, com pelo menos 200 × 200 pixels. Aparece por cima do pagamento.</div>
      </div>
      <div>
        <span className="label">Métodos aceites</span>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit,minmax(120px,1fr))', gap: 8 }}>
          {['mpesa', 'emola', 'bim', 'card'].map(m => (
            <button key={m} type="button" disabled={soon.includes(m)} title={soon.includes(m) ? 'Em breve' : undefined} onClick={() => toggleM(m)} aria-pressed={methods.includes(m)} style={{ flex: 1, whiteSpace: 'nowrap', height: 46, borderRadius: 13, border: methods.includes(m) ? '2px solid var(--ink)' : '1px solid var(--line-2)', background: '#fff', display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 7, fontSize: 14, fontWeight: 600, opacity: soon.includes(m) ? 0.5 : methods.includes(m) ? 1 : .6 }}>
              <MethodIcon method={m} size={18} radius={6} />{soon.includes(m) ? 'Em breve' : METHOD_NAME[m]}
            </button>
          ))}
        </div>
      </div>
      <div className="card" style={{ borderRadius: 16 }}>
        <button type="button" className="row" onClick={() => setAdvanced(!advanced)} aria-expanded={advanced} style={{ width: '100%', padding: '13px 16px', background: 'none', border: 0, textAlign: 'left' }}>
          <div style={{ flex: 1 }}><div style={{ fontWeight: 600, fontSize: 14 }}>Opções avançadas</div><div style={{ fontSize: 12.5, color: 'var(--muted)' }}>Pagamentos do estrangeiro</div></div>
          <Icon name="updown" size={16} />
        </button>
        {advanced && (
          <>
            <div style={{ height: 1, background: '#EEECE5', margin: '0 16px' }} />
            <div className="row" style={{ padding: '13px 16px', gap: 12 }}>
              <MethodIcon method="abroad" size={30} radius={9} />
              <div style={{ flex: 1 }}>
                <div style={{ fontWeight: 600, fontSize: 14 }}>Aceitar pagamentos do estrangeiro</div>
                <div style={{ fontSize: 12.5, color: 'var(--muted)', lineHeight: 1.45 }}>{abroadOK ? 'Clientes noutros países pagam com a carteira do país deles, na página segura da U2opia. Recebe em meticais.' : live ? 'Disponível primeiro em modo de teste.' : 'Ainda não disponível.'}</div>
              </div>
              <Toggle on={methods.includes('abroad')} onChange={() => abroadOK && toggleM('abroad')} label="Aceitar pagamentos do estrangeiro" />
            </div>
          </>
        )}
      </div>
      {edit && from && !custom && c !== from.amount && c >= 1000 && (
        <div className="form-err" style={{ background: 'var(--sun-soft)', color: '#5B4600' }}>Quem abrir o link a partir de agora paga {fmt(c)} MT em vez de {fmt(from.amount || 0)} MT. {from.payments_count ? 'Os pagamentos já feitos não mudam.' : ''}</div>
      )}
      {edit && <div style={{ fontSize: 12.5, color: 'var(--muted)', lineHeight: 1.45, padding: '0 2px' }}>O endereço, o tipo de valor e o QR não mudam depois de criado, para que o link já partilhado continue a funcionar. Para os mudar, duplique o link.</div>}
      <div className="card" style={{ borderRadius: 16 }}>
        <div className="row" style={{ padding: '13px 16px' }}><div style={{ flex: 1 }}><div style={{ fontWeight: 600, fontSize: 14 }}>Reutilizável</div><div style={{ fontSize: 12.5, color: 'var(--muted)' }}>Vários clientes podem pagar o mesmo link</div></div><Toggle on={reusable} onChange={setReusable} label="Reutilizável" /></div>
        <div style={{ height: 1, background: '#EEECE5', margin: '0 16px' }} />
        <div className="row" style={{ padding: '13px 16px' }}><div style={{ flex: 1 }}><div style={{ fontWeight: 600, fontSize: 14 }}>Cliente escolhe o valor</div><div style={{ fontSize: 12.5, color: 'var(--muted)' }}>Para gorjetas e doações</div></div><span style={{ opacity: edit ? 0.45 : 1 }}><Toggle on={custom} onChange={v => !edit && setCustom(v)} label="Cliente escolhe o valor" /></span></div>
        <div style={{ height: 1, background: '#EEECE5', margin: '0 16px' }} />
        <div className="row" style={{ padding: '13px 16px' }}><div style={{ flex: 1 }}><div style={{ fontWeight: 600, fontSize: 14 }}>QR para o balcão</div><div style={{ fontSize: 12.5, color: 'var(--muted)' }}>Marca os pagamentos como origem QR</div></div><span style={{ opacity: edit ? 0.45 : 1 }}><Toggle on={kind === 'qr'} onChange={v => !edit && setKind(v ? 'qr' : 'link')} label="QR" /></span></div>
        <div style={{ height: 1, background: '#EEECE5', margin: '0 16px' }} />
        <div className="row" style={{ padding: '10px 16px' }}><div style={{ flex: 1, fontWeight: 600, fontSize: 14 }}>Expira</div>
          <select value={exp} onChange={e => setExp(Number(e.target.value))} style={{ border: 0, background: 'transparent', fontSize: 14, color: 'var(--text-2)', textAlign: 'right' }}>{edit && from?.expires_at && <option value={-1}>{from.expires_at * 1000 < Date.now() ? 'Expirou' : 'Manter'} · {new Date(from.expires_at * 1000).toLocaleDateString('pt-PT')}</option>}{EXPIRY.map(([l, d]) => <option key={d} value={d}>{edit && d ? 'Daqui a ' + l : l}</option>)}</select>
        </div>
      </div>
      {custom && (
        <div className="card" style={{ borderRadius: 16, padding: 16 }}>
          <div style={{ fontWeight: 600, fontSize: 14 }}>Valor que o cliente escolhe</div>
          <div style={{ fontSize: 12.5, color: 'var(--muted)', marginTop: 2 }}>Opcional. O sugerido aparece já escrito; o cliente pode mudar dentro dos limites.</div>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3,minmax(0,1fr))', gap: 8, marginTop: 12 }}>
            {mt('Sugerido', suggested, setSuggested, '500')}
            {mt('Mínimo', min, setMin, '10')}
            {mt('Máximo', max, setMax, 'livre')}
          </div>
        </div>
      )}
      <div className="card" style={{ borderRadius: 16, padding: 16 }}>
        <div style={{ fontWeight: 600, fontSize: 14 }}>Texto do botão</div>
        <div style={{ fontSize: 12.5, color: 'var(--muted)', marginTop: 2 }}>A palavra no botão do checkout. Com Doar, a página pergunta "Quanto quer doar?".</div>
        <div className="seg" role="radiogroup" aria-label="Texto do botão" style={{ display: 'flex', marginTop: 10 }}>
          {Object.entries(SUBMIT).map(([k, w]) => <button key={k} type="button" role="radio" aria-checked={submitType === k} style={{ flex: 1 }} className={submitType === k ? 'on' : ''} onClick={() => setSubmitType(k)}>{w.label}</button>)}
        </div>
      </div>
      <div className="card" style={{ borderRadius: 16, padding: 16 }}>
        <div style={{ fontWeight: 600, fontSize: 14 }}>Depois do pagamento</div>
        <div role="radiogroup" style={{ display: 'flex', flexDirection: 'column', gap: 10, marginTop: 10 }}>
          <label className="row" style={{ gap: 10, fontSize: 14, cursor: 'pointer' }}><input type="radio" checked={after === 'page'} onChange={() => setAfter('page')} style={{ accentColor: 'var(--ink)', width: 18, height: 18 }} />Mostrar a confirmação, com uma mensagem sua</label>
          {after === 'page' && <textarea className="input" rows={2} maxLength={300} value={message} onChange={e => setMessage(e.target.value)} placeholder="Obrigado! Entregamos amanhã até às 12h. Opcional." style={{ height: 'auto', padding: '10px 14px', lineHeight: 1.5, resize: 'vertical', marginLeft: 28, width: 'calc(100% - 28px)' }} />}
          <label className="row" style={{ gap: 10, fontSize: 14, cursor: 'pointer' }}><input type="radio" checked={after === 'redirect'} onChange={() => setAfter('redirect')} style={{ accentColor: 'var(--ink)', width: 18, height: 18 }} />Enviar o cliente para o seu site</label>
          {after === 'redirect' && <input className="input" inputMode="url" value={redirect} onChange={e => setRedirect(e.target.value)} placeholder="https://www.exemplo.co.mz/obrigado" style={{ marginLeft: 28, width: 'calc(100% - 28px)' }} />}
        </div>
      </div>
      {err && <div className="form-err">{err}</div>}
    </>
  )
  return { amountBlock, fields, submit, busy, valid, tabs, tab, preview }
}

export function SharePanel({ link, compact }: { link: LinkObj; compact?: boolean }) {
  const [copied, setCopied] = useState(false)
  const display = link.url.replace(/^https?:\/\//, '')
  const share = (ch: string) => link.url + (link.url.includes('?') ? '&' : '?') + 'c=' + ch
  const msg = encodeURIComponent(`${link.name}${link.amount ? ` · ${fmt(link.amount)} MT` : ''}\nPague aqui: ${share('whatsapp')}`)
  const doCopy = async () => { await copy(link.url); setCopied(true); setTimeout(() => setCopied(false), 1500) }
  const downloadQR = () => {
    const svg = document.querySelector('#riha-qr svg') as SVGSVGElement | null
    if (!svg) return
    const blob = new Blob([new XMLSerializer().serializeToString(svg)], { type: 'image/svg+xml' })
    const a = document.createElement('a'); a.href = URL.createObjectURL(blob); a.download = `qr-${link.slug}.svg`; a.click(); URL.revokeObjectURL(a.href)
  }
  const native = () => (navigator as any).share ? (navigator as any).share({ title: link.name, url: share('outro') }).catch(() => {}) : doCopy()
  return (
    <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', textAlign: 'center' }}>
      <div style={{ width: 56, height: 56, borderRadius: '50%', background: 'var(--green)', display: 'grid', placeItems: 'center', boxShadow: '0 0 0 8px #E2F3DE' }}><Icon name="check" size={26} stroke={2.6} color="#fff" /></div>
      <div className="display" style={{ fontSize: compact ? 24 : 28, marginTop: 18 }}>Link pronto a partilhar</div>
      <div style={{ fontSize: 14, color: 'var(--muted)', marginTop: 4 }}>{link.name} · <b style={{ color: 'var(--ink)' }}>{link.custom_amount ? 'Valor livre' : `${fmt(link.amount)} MT`}</b></div>
      <div className="card" style={{ width: '100%', marginTop: 20, borderRadius: 26, padding: 20, display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
        <div id="riha-qr" style={{ padding: 12, borderRadius: 18, boxShadow: 'inset 0 0 0 1px #EEECE5' }}><QR value={share('qr')} size={compact ? 156 : 172} /></div>
        <div style={{ fontSize: 13, color: 'var(--muted)', marginTop: 12 }}>Mostre no balcão para o cliente digitalizar</div>
        <div style={{ width: '100%', display: 'flex', alignItems: 'center', gap: 10, marginTop: 16, padding: '6px 6px 6px 14px', borderRadius: 14, background: 'var(--paper)' }}>
          <span className="mono" style={{ flex: 1, minWidth: 0, fontSize: 12.5, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', textAlign: 'left' }}>{display}</span>
          <button className="btn btn-ink" style={{ height: 38, padding: '0 14px', borderRadius: 10, fontSize: 13 }} onClick={doCopy}><Icon name="copy" size={14} stroke={2.2} />{copied ? 'Copiado' : 'Copiar'}</button>
        </div>
      </div>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4,1fr)', gap: 8, marginTop: 16, width: '100%' }}>
        <a href={`https://wa.me/?text=${msg}`} target="_blank" rel="noreferrer" className="share-btn"><span style={{ background: '#25D366', color: '#fff', border: 0 }}><Icon name="whatsapp" size={26} stroke={2} /></span>WhatsApp</a>
        <a href={`sms:?&body=${msg}`} className="share-btn"><span><Icon name="sms" size={24} /></span>SMS</a>
        <button className="share-btn" onClick={native}><span><Icon name="share" size={24} /></span>Mais</button>
        <button className="share-btn" onClick={downloadQR}><span><Icon name="download" size={24} /></span>QR</button>
      </div>
      <a href={link.url} target="_blank" rel="noreferrer" style={{ marginTop: 16, fontSize: 14, fontWeight: 600 }}>Ver como o cliente vê</a>
      <style>{`.share-btn{display:flex;flex-direction:column;align-items:center;gap:7px;font-size:12.5px;font-weight:500;background:none;border:0;padding:0;color:var(--ink)}.share-btn span{width:58px;height:58px;border-radius:18px;background:#fff;border:1px solid #E4E2DA;display:grid;place-items:center}`}</style>
    </div>
  )
}

export { Spinner }
