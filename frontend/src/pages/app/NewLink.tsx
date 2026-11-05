import { useState } from 'react'
import { errMsg } from '../../core/model'
import { useGateway } from '../../core/gateway'
import { copy, fmt, METHOD_NAME, parseMT } from '../../lib/format'
import { useSession } from '../../lib/session'
import { Icon, MethodIcon, QR, Spinner, Toggle } from '../../components/ui'

export type LinkObj = { id: string; name: string; slug: string; amount: number | null; custom_amount: boolean; reusable: boolean; url: string; active: boolean; type: string; payment_method_types: string[]; expires_at: number | null; payments_count: number; total_received: number; livemode: boolean; location?: string }

const EXPIRY: [string, number][] = [['Nunca', 0], ['24 horas', 1], ['7 dias', 7], ['30 dias', 30]]

export function useLinkForm(onCreated: (l: LinkObj) => void) {
  const { me, toast } = useSession()
  const g = useGateway()
  const bps = me?.merchant?.fee_bps ?? 600
  const [amount, setAmount] = useState('1250')
  const [name, setName] = useState('')
  const [methods, setMethods] = useState<string[]>(['mpesa', 'emola', 'bim', 'card'])
  const [reusable, setReusable] = useState(true)
  const [custom, setCustom] = useState(false)
  const [kind, setKind] = useState<'link' | 'qr'>('link')
  const [exp, setExp] = useState(0)
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')
  const c = parseMT(amount)
  const fee = Math.round((c * bps) / 10000)
  const submit = async () => {
    setBusy(true); setErr('')
    try {
      const body: Record<string, unknown> = { name, custom_amount: custom, reusable, payment_method_types: methods, type: kind }
      if (!custom) body.amount = c
      if (exp) body.expires_at = Math.floor(Date.now() / 1000) + exp * 86400
      const l = await g.links.create(body)
      toast('Link criado')
      onCreated(l)
    } catch (e) { setErr(errMsg(e)) } finally { setBusy(false) }
  }
  const toggleM = (m: string) => setMethods(ms => (ms.includes(m) ? (ms.length > 1 ? ms.filter(x => x !== m) : ms) : [...ms, m]))
  const valid = name.trim() !== '' && (custom || c >= 1000)

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
          style={{ width: `${Math.max(1, amount.length) * 0.62}em`, minWidth: 50, maxWidth: 260, border: 0, background: 'transparent', textAlign: 'right', fontFamily: 'var(--display)', fontWeight: 700, fontSize: big ? 58 : 48, letterSpacing: '-0.045em', padding: 0, caretColor: 'var(--green)' }} />
        <span className="display" style={{ fontSize: big ? 26 : 22, color: 'var(--muted-2)' }}>MT</span>
      </div>
      <div style={{ display: 'inline-flex', gap: 6, alignItems: 'center', marginTop: 8, fontSize: 13, color: 'var(--text-2)', background: big ? 'var(--paper-2)' : 'transparent', padding: big ? '6px 12px' : 0, borderRadius: 999 }}>Recebe <b className="tnum">{fmt(c - fee)} MT</b> após taxa de {me?.merchant?.fee_label}</div>
      {big && <div style={{ display: 'flex', justifyContent: 'center', gap: 8, marginTop: 16, flexWrap: 'wrap' }}>{[500, 1000, 2500, 5000].map(v => <button key={v} type="button" className="btn btn-outline" style={{ height: 36, borderRadius: 999, fontSize: 13.5, padding: '0 14px' }} onClick={() => setAmount(String(v))}>{fmt(v * 100, false)}</button>)}</div>}
    </div>
  )

  const fields = (
    <>
      <label><span className="label">Descrição</span><input className="input" style={{ height: 50 }} value={name} onChange={e => setName(e.target.value)} placeholder="Bolo de chocolate · 1 kg" maxLength={200} /></label>
      <div>
        <span className="label">Métodos aceites</span>
        <div style={{ display: 'flex', gap: 8 }}>
          {['mpesa', 'emola', 'bim', 'card'].map(m => (
            <button key={m} type="button" onClick={() => toggleM(m)} aria-pressed={methods.includes(m)} style={{ flex: 1, height: 46, borderRadius: 13, border: methods.includes(m) ? '2px solid var(--ink)' : '1px solid var(--line-2)', background: '#fff', display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 7, fontSize: 14, fontWeight: 600, opacity: methods.includes(m) ? 1 : 0.6 }}>
              <MethodIcon method={m} size={18} radius={6} />{METHOD_NAME[m]}
            </button>
          ))}
        </div>
      </div>
      <div className="card" style={{ borderRadius: 16 }}>
        <div className="row" style={{ padding: '13px 16px' }}><div style={{ flex: 1 }}><div style={{ fontWeight: 600, fontSize: 14 }}>Reutilizável</div><div style={{ fontSize: 12.5, color: 'var(--muted)' }}>Vários clientes podem pagar o mesmo link</div></div><Toggle on={reusable} onChange={setReusable} label="Reutilizável" /></div>
        <div style={{ height: 1, background: '#EEECE5', margin: '0 16px' }} />
        <div className="row" style={{ padding: '13px 16px' }}><div style={{ flex: 1 }}><div style={{ fontWeight: 600, fontSize: 14 }}>Cliente escolhe o valor</div><div style={{ fontSize: 12.5, color: 'var(--muted)' }}>Para gorjetas e doações</div></div><Toggle on={custom} onChange={setCustom} label="Cliente escolhe o valor" /></div>
        <div style={{ height: 1, background: '#EEECE5', margin: '0 16px' }} />
        <div className="row" style={{ padding: '13px 16px' }}><div style={{ flex: 1 }}><div style={{ fontWeight: 600, fontSize: 14 }}>QR para o balcão</div><div style={{ fontSize: 12.5, color: 'var(--muted)' }}>Marca os pagamentos como origem QR</div></div><Toggle on={kind === 'qr'} onChange={v => setKind(v ? 'qr' : 'link')} label="QR" /></div>
        <div style={{ height: 1, background: '#EEECE5', margin: '0 16px' }} />
        <div className="row" style={{ padding: '10px 16px' }}><div style={{ flex: 1, fontWeight: 600, fontSize: 14 }}>Expira</div>
          <select value={exp} onChange={e => setExp(Number(e.target.value))} style={{ border: 0, background: 'transparent', fontSize: 14, color: 'var(--text-2)', textAlign: 'right' }}>{EXPIRY.map(([l, d]) => <option key={d} value={d}>{l}</option>)}</select>
        </div>
      </div>
      {err && <div className="form-err">{err}</div>}
    </>
  )
  return { amountBlock, fields, submit, busy, valid }
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
