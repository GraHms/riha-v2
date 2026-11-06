import { Avatar, Icon, MethodIcon } from '../../components/ui'
import { fmt, METHOD_NAME } from '../../lib/format'

// What a customer sees of a payment link. The dashboard shows the same pieces
// as its preview, so a merchant sees what their customer will.

export type LinkFace = {
  name: string; details?: string; image_url?: string
  merchant: { name: string; initials: string; avatar_color?: string }
}

export function LinkHeader({ link, compact }: { link: LinkFace; compact?: boolean }) {
  return (
    <div className="card" style={{ borderRadius: 24, overflow: 'hidden' }}>
      {link.image_url && <img src={link.image_url} alt="" style={{ display: 'block', width: '100%', aspectRatio: '16 / 9', objectFit: 'cover', background: 'var(--paper-2)' }} />}
      <div style={{ padding: compact ? 16 : 18 }}>
        <div className="row">
          <Avatar text={link.merchant.initials} color={link.merchant.avatar_color} size={40} radius={12} font={14} />
          <div style={{ minWidth: 0 }}>
            <div style={{ fontWeight: 600, fontSize: 15 }}>{link.merchant.name}</div>
            <div style={{ fontSize: 12.5, color: 'var(--muted)' }}>Pagamento seguro</div>
          </div>
        </div>
        <div style={{ fontWeight: 700, fontSize: 18, lineHeight: 1.3, marginTop: 14 }}>{link.name || 'O que está a cobrar'}</div>
        {link.details && <p style={{ margin: '6px 0 0', fontSize: 14.5, lineHeight: 1.55, color: 'var(--text-2)', whiteSpace: 'pre-line' }}>{link.details}</p>}
      </div>
    </div>
  )
}

const MIN = 1000 // 10 MT
const MAX = 100000000

// Quick amounts for a customer choosing how much to pay: the suggested one,
// and round amounts within the merchant's range.
export function quickAmounts(suggested: number, lo: number, hi: number) {
  const pool = [suggested, 10000, 25000, 50000, 100000, 250000, 500000].filter(v => v > 0 && v >= lo && v <= hi)
  const out = [...new Set(pool)].sort((a, b) => a - b).slice(0, 4)
  if (out.length < 2) return [...new Set([lo, ...out, hi].filter(v => v >= MIN && v <= MAX))].slice(0, 3)
  return out
}

export function rangeText(lo: number, hi: number) {
  const hasLo = lo > MIN, hasHi = hi < MAX
  if (hasLo && hasHi) return `Entre ${fmt(lo, false)} e ${fmt(hi, false)} MT`
  if (hasLo) return `A partir de ${fmt(lo, false)} MT`
  if (hasHi) return `Até ${fmt(hi, false)} MT`
  return ''
}

export type LinkDraft = LinkFace & {
  amount: number; custom: boolean; suggested: number; min: number; max: number; methods: string[]
  after: 'page' | 'redirect'; message: string; redirect: string
}

// LinkPreview shows a draft link as the customer will meet it: before paying,
// and after.
export function LinkPreview({ d }: { d: LinkDraft }) {
  const lo = d.min || MIN, hi = d.max || MAX
  const host = (() => { try { return new URL(d.redirect).host } catch { return '' } })()
  return (
    <div style={{ background: 'var(--paper)', borderRadius: 22, padding: 14, display: 'flex', flexDirection: 'column', gap: 12 }}>
      <LinkHeader link={d} compact />
      <div className="card" style={{ borderRadius: 20, padding: 16, textAlign: 'center' }}>
        {d.custom ? (
          <>
            <div style={{ fontSize: 13, color: 'var(--muted)' }}>Quanto quer pagar?</div>
            <div className="display tnum" style={{ fontSize: 34, marginTop: 4 }}>{d.suggested ? fmt(d.suggested, false) : '0'} <span style={{ fontSize: 16, color: 'var(--muted-2)' }}>MT</span></div>
            <div style={{ display: 'flex', justifyContent: 'center', gap: 6, marginTop: 10, flexWrap: 'wrap' }}>
              {quickAmounts(d.suggested, lo, hi).map(v => <span key={v} className="chip" style={{ fontFamily: 'var(--sans)', background: v === d.suggested ? 'var(--ink)' : 'var(--paper-2)', color: v === d.suggested ? '#fff' : undefined }}>{fmt(v, false)}</span>)}
            </div>
            {rangeText(lo, hi) && <div style={{ fontSize: 12.5, color: 'var(--muted)', marginTop: 10 }}>{rangeText(lo, hi)}</div>}
          </>
        ) : (
          <>
            <div style={{ fontSize: 13, color: 'var(--muted)' }}>Total a pagar</div>
            <div className="display tnum" style={{ fontSize: 34, marginTop: 4 }}>{fmt(d.amount, false)} <span style={{ fontSize: 16, color: 'var(--muted-2)' }}>MT</span></div>
          </>
        )}
        <div style={{ display: 'flex', justifyContent: 'center', gap: 6, marginTop: 14, flexWrap: 'wrap' }}>
          {d.methods.map(m => <span key={m} className="chip" style={{ fontFamily: 'var(--sans)', background: '#fff', border: '1px solid var(--line)', gap: 6 }}><MethodIcon method={m} size={16} radius={5} />{METHOD_NAME[m]}</span>)}
        </div>
      </div>
      <div style={{ fontSize: 12, fontWeight: 700, color: 'var(--muted)', textTransform: 'uppercase', letterSpacing: '.06em', margin: '4px 4px 0' }}>Depois de pagar</div>
      <div className="card" style={{ borderRadius: 20, padding: 16, display: 'flex', gap: 12, alignItems: 'flex-start' }}>
        <span style={{ width: 34, height: 34, borderRadius: '50%', background: 'var(--green)', display: 'grid', placeItems: 'center', flex: 'none' }}><Icon name="check" size={17} stroke={2.6} color="#fff" /></span>
        <div style={{ fontSize: 14, lineHeight: 1.5 }}>
          <b>Pagamento concluído</b>
          {d.after === 'redirect'
            ? <div style={{ color: 'var(--muted)' }}>{host ? <>Volta para <b style={{ color: 'var(--ink)' }}>{host}</b> em 8 segundos.</> : 'Indique o endereço do seu site.'}</div>
            : <div style={{ color: 'var(--text-2)', whiteSpace: 'pre-line' }}>{d.message || 'Vê o recibo com a referência do pagamento.'}</div>}
        </div>
      </div>
    </div>
  )
}
