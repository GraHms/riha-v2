import type { CSSProperties, ReactNode } from 'react'
import { Avatar, Icon, MethodIcon } from '../../../components/ui'
import { money } from '../../app/subs/common'

// What a customer sees of a merchant's plans: the catalog, a plan, and when
// they pay. The dashboard shows the same components as its preview, so what the
// merchant sees is what their customers get.

export type Page = { title?: string; description?: string; cover_url?: string; accent?: string; background?: string; url?: string }
export type Merchant = { name: string; initials: string; color?: string }
export type CustomerPlan = {
  id: string; name: string; amount: number; interval: string; highlights?: string[]; when_text?: string; duration_label?: string
  months_off?: string; fields?: string[]; dates?: { label: string; period: string }[]; first_today?: boolean; first_period?: string
}

export const per = (i: string) => (i === 'week' ? 'semana' : i === 'quarter' ? 'trimestre' : i === 'year' ? 'ano' : 'mês')

const BRAND = '#0A8205'
const PAPER = '#F4F3EE'

function hex(c: string | undefined, fallback: string) {
  return c && /^#[0-9a-fA-F]{6}$/.test(c) ? c : fallback
}

function luminance(c: string) {
  const v = [1, 3, 5].map(i => parseInt(c.slice(i, i + 2), 16) / 255).map(x => (x <= 0.03928 ? x / 12.92 : ((x + 0.055) / 1.055) ** 2.4))
  return 0.2126 * v[0] + 0.7152 * v[1] + 0.0722 * v[2]
}

// Text on a colour: white or ink, whichever reads better.
export const onColor = (c: string) => (luminance(c) > 0.45 ? '#0B120E' : '#FFFFFF')

export function brandVars(page?: Page): CSSProperties {
  const brand = hex(page?.accent, BRAND)
  const bg = hex(page?.background, PAPER)
  const dark = luminance(bg) < 0.3
  return {
    '--brand': brand, '--on-brand': onColor(brand), '--bg': bg,
    '--on-bg': dark ? '#FFFFFF' : '#0B120E', '--on-bg-2': dark ? 'rgba(255,255,255,.74)' : '#56605A',
    // Selected states use the brand colour unless it vanishes on white.
    '--brand-line': luminance(brand) > 0.75 ? '#0B120E' : brand,
  } as CSSProperties
}

export function BrandFrame({ page, test, children, host = true }: { page?: Page; test?: boolean; children: ReactNode; host?: boolean }) {
  return (
    <div className="bf" style={{ ...brandVars(page), minHeight: '100%', background: 'var(--bg)', color: 'var(--on-bg)', display: 'flex', justifyContent: 'center', padding: '0 0 28px' }}>
      <style>{CSS}</style>
      <div style={{ width: '100%', maxWidth: 480, display: 'flex', flexDirection: 'column' }}>
        {(host || test) && (
          <div style={{ display: 'flex', justifyContent: 'center', gap: 8, padding: '14px 16px 0' }}>
            {host && <span className="bf-host"><Icon name="lock" size={12} stroke={2.4} />{window.location.host}</span>}
            {test && <span className="bf-host" style={{ background: 'var(--sun-bg)', color: 'var(--sun-text)', fontWeight: 600 }}>Modo de teste</span>}
          </div>
        )}
        {children}
      </div>
    </div>
  )
}

export function Hero({ merchant, page, sub }: { merchant: Merchant; page?: Page; sub?: ReactNode }) {
  const title = page?.title && page.title !== merchant.name ? page.title : ''
  return (
    <header className="bf-hero">
      <div className="bf-cover" style={page?.cover_url ? { backgroundImage: `url("${page.cover_url}")` } : undefined}>
        {!page?.cover_url && <span className="bf-stripes" aria-hidden="true"><i /><i /><i /></span>}
      </div>
      <div className="bf-id">
        <span className="bf-avatar"><Avatar text={merchant.initials} color={merchant.color || '#FFD21F'} size={64} radius={18} font={21} /></span>
        {title ? <><div className="bf-name">{merchant.name}</div><h1 className="bf-title">{title}</h1></> : <h1 className="bf-title" style={{ marginTop: 8 }}>{merchant.name}</h1>}
        {page?.description && <p className="bf-desc">{page.description}</p>}
        {sub}
      </div>
    </header>
  )
}

export function Price({ amount, interval, size = 26 }: { amount: number; interval: string; size?: number }) {
  return (
    <span className="tnum" style={{ whiteSpace: 'nowrap' }}>
      <b className="display" style={{ fontSize: size, letterSpacing: '-0.02em' }}>{money(amount)}</b>
      <span style={{ fontSize: Math.max(12, size * 0.5), color: 'var(--muted)', letterSpacing: 0, marginLeft: 5 }}>MT / {per(interval)}</span>
    </span>
  )
}

export function Highlights({ items, max }: { items?: string[]; max?: number }) {
  if (!items?.length) return null
  const shown = max ? items.slice(0, max) : items
  return (
    <ul className="bf-hl">
      {shown.map(h => <li key={h}><Icon name="check" size={14} stroke={2.6} color="var(--brand-line)" /><span>{h}</span></li>)}
      {max && items.length > max && <li className="bf-more">+{items.length - max} {items.length - max === 1 ? 'outro' : 'outros'}</li>}
    </ul>
  )
}

export function PlanCard({ plan, on, multi, onToggle, compact }: { plan: CustomerPlan; on?: boolean; multi?: boolean; onToggle?: () => void; compact?: boolean }) {
  const body = (
    <>
      <div style={{ display: 'flex', gap: 12, alignItems: 'flex-start' }}>
        <div style={{ flex: 1, minWidth: 0 }}>
          <div className="bf-plan-name">{plan.name}</div>
          <div style={{ marginTop: 4 }}><Price amount={plan.amount} interval={plan.interval} size={22} /></div>
        </div>
        {onToggle && <span className={'bf-pick' + (multi ? ' sq' : '') + (on ? ' on' : '')} aria-hidden="true">{on && <Icon name="check" size={13} stroke={3} color="var(--on-brand)" />}</span>}
      </div>
      <div className="bf-when">
        {plan.when_text && <span>{plan.when_text}</span>}
        {plan.duration_label && <span className="bf-chip">{plan.duration_label}</span>}
        {plan.months_off && <span className="bf-chip">Sem pagamento em {plan.months_off}</span>}
      </div>
      {!compact && <Highlights items={plan.highlights} />}
    </>
  )
  if (!onToggle) return <div className="bf-card">{body}</div>
  return (
    <button type="button" className={'bf-card bf-card-btn' + (on ? ' on' : '')} onClick={onToggle} aria-pressed={on} role={multi ? 'checkbox' : 'radio'} aria-checked={on}>
      {body}
    </button>
  )
}

// PlanFacts tells a customer joining now what they pay today, and after.
export function PlanFacts({ plan }: { plan: CustomerPlan }) {
  const dates = plan.dates || []
  const later = dates.slice(1)
  return (
    <div className="bf-facts">
      <div className="bf-fact">
        <span>{plan.first_today === false ? `Primeiro, a ${dates[0]?.label}` : 'Hoje'}</span>
        <b className="tnum">{money(plan.amount)} MT</b>
        {plan.first_period && <em>{plan.first_period}</em>}
      </div>
      {plan.when_text && (later.length > 0 || !plan.duration_label) && (
        <div className="bf-fact">
          <span>Depois</span>
          <b>{plan.when_text}</b>
          {later.length > 0 && <em>{later.map(d => d.label).join(' · ')}{plan.duration_label ? '' : ' …'}</em>}
        </div>
      )}
      {plan.duration_label && (
        <div className="bf-fact">
          <span>Termina</span>
          <b>{dates.length === 1 ? 'É um só pagamento' : plan.duration_label.startsWith('Até') ? plan.duration_label : `Depois de ${plan.duration_label}`}</b>
          {dates.length > 1 && <em>Último: {dates[dates.length - 1].period}</em>}
        </div>
      )}
      {plan.months_off && <div className="bf-fact"><span>Pausa</span><b>Sem pagamento em {plan.months_off}</b></div>}
    </div>
  )
}

export function PaysWith() {
  return (
    <div className="bf-pays">
      <span style={{ display: 'flex' }}>{['mpesa', 'emola', 'bim'].map((m, i) => <span key={m} style={{ marginLeft: i ? -6 : 0, borderRadius: 9, boxShadow: '0 0 0 2px var(--bg)', display: 'inline-flex' }}><MethodIcon method={m} size={24} radius={8} /></span>)}</span>
      <span>Paga por M-Pesa, e-Mola ou Bim IZI. Nada é cobrado sem a sua confirmação.</span>
    </div>
  )
}

export function CatalogView({ merchant, page, plans, multi, sel, onToggle, onContinue }: {
  merchant: Merchant; page?: Page; plans: CustomerPlan[]; multi: boolean; sel: string[]; onToggle: (id: string) => void; onContinue: () => void
}) {
  const chosen = plans.filter(p => sel.includes(p.id))
  const total = chosen.reduce((a, p) => a + p.amount, 0)
  const sameInterval = chosen.every(p => p.interval === chosen[0]?.interval)
  return (
    <>
      <Hero merchant={merchant} page={page} />
      <div style={{ padding: '0 16px' }}>
        <div className="bf-label">{plans.length === 1 ? 'Plano' : multi ? 'Escolha um ou mais planos' : 'Escolha um plano'}</div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
          {plans.map(p => <PlanCard key={p.id} plan={p} multi={multi} on={sel.includes(p.id)} onToggle={() => onToggle(p.id)} />)}
        </div>
        <PaysWith />
      </div>
      <div className="bf-bar">
        {chosen.length > 0 && (
          <div className="between" style={{ fontSize: 14, marginBottom: 10 }}>
            <span>{chosen.length === 1 ? chosen[0].name : `${chosen.length} planos`}</span>
            <b className="tnum">{money(total)} MT{sameInterval && chosen.length ? ` / ${per(chosen[0].interval)}` : ''}</b>
          </div>
        )}
        <button className="bf-cta" disabled={!chosen.length} onClick={onContinue}>{chosen.length ? 'Continuar' : 'Escolha um plano para continuar'}</button>
      </div>
    </>
  )
}

const CSS = `
.bf-host{display:inline-flex;align-items:center;gap:6px;padding:6px 12px;border-radius:999px;background:rgba(127,127,127,.14);font-size:12.5px;color:var(--on-bg-2)}
.bf-hero{margin:14px 16px 0;border-radius:24px;overflow:hidden;background:#fff;color:var(--ink);box-shadow:0 1px 0 rgba(11,18,14,.04),0 12px 32px -22px rgba(11,18,14,.35)}
.bf-cover{height:132px;background:var(--brand) center/cover no-repeat;position:relative;overflow:hidden}
.bf-stripes{position:absolute;inset:0}
.bf-stripes i{position:absolute;top:-40%;width:34px;height:180%;background:var(--on-brand);opacity:.13;transform:rotate(24deg);border-radius:6px}
.bf-stripes i:nth-child(1){right:22%}.bf-stripes i:nth-child(2){right:calc(22% - 48px);opacity:.22}.bf-stripes i:nth-child(3){right:calc(22% - 96px);opacity:.09}
.bf-id{padding:0 20px 20px;position:relative}
.bf-avatar{display:inline-block;margin-top:-34px;border-radius:22px;border:4px solid #fff;background:#fff;line-height:0}
.bf-name{font-weight:700;font-size:15px;margin-top:8px;color:#56605A}
.bf-title{font-family:var(--display);font-size:27px;line-height:1.12;letter-spacing:-0.025em;margin:4px 0 0;font-weight:800;text-wrap:balance}
.bf-desc{margin:10px 0 0;font-size:15px;line-height:1.55;color:#3E4742;white-space:pre-line}
.bf-label{font-size:13px;font-weight:700;color:var(--on-bg-2);margin:22px 4px 10px;letter-spacing:.01em}
.bf-card{display:block;width:100%;text-align:left;background:#fff;color:var(--ink);border:1px solid rgba(11,18,14,.08);border-radius:20px;padding:16px 18px;font:inherit}
.bf-card-btn{cursor:pointer;transition:border-color .15s,box-shadow .15s}
.bf-card-btn:hover{border-color:rgba(11,18,14,.22)}
.bf-card-btn.on{border-color:var(--brand-line);box-shadow:0 0 0 1px var(--brand-line)}
.bf-card-btn:focus-visible{outline:3px solid var(--brand-line);outline-offset:2px}
.bf-plan-name{font-weight:700;font-size:16px;line-height:1.3}
.bf-pick{width:24px;height:24px;flex:none;border-radius:50%;border:1.5px solid #C9C7BE;display:grid;place-items:center;margin-top:2px}
.bf-pick.sq{border-radius:8px}
.bf-pick.on{background:var(--brand);border-color:var(--brand-line)}
.bf-when{display:flex;flex-wrap:wrap;gap:6px 8px;align-items:center;margin-top:10px;font-size:13.5px;color:#3E4742}
.bf-chip{display:inline-flex;align-items:center;padding:3px 9px;border-radius:999px;background:#F1F0EA;font-size:12.5px;font-weight:600;color:#3E4742}
.bf-hl{list-style:none;margin:12px 0 0;padding:12px 0 0;border-top:1px dashed #E4E2DA;display:flex;flex-direction:column;gap:7px}
.bf-hl li{display:flex;gap:9px;align-items:flex-start;font-size:14px;line-height:1.4;color:#29312C}
.bf-hl li svg{margin-top:2px}
.bf-more{color:var(--muted);font-size:13px;padding-left:23px}
.bf-facts{margin-top:14px;border-radius:16px;background:#F7F6F1;padding:4px 14px}
.bf-fact{display:grid;grid-template-columns:76px 1fr;gap:2px 12px;padding:11px 0;border-bottom:1px solid #ECEAE3;font-size:14px}
.bf-fact:last-child{border-bottom:0}
.bf-fact span{color:var(--muted);grid-row:span 2}
.bf-fact em{font-style:normal;font-size:12.5px;color:var(--muted);grid-column:2}
.bf-pays{display:flex;gap:12px;align-items:center;margin:22px 4px 0;font-size:12.5px;line-height:1.45;color:var(--on-bg-2)}
.bf-bar{position:sticky;bottom:0;margin-top:18px;padding:12px 16px max(14px,env(safe-area-inset-bottom));background:var(--bg)}
.bf-bar::before{content:'';position:absolute;left:0;right:0;top:-22px;height:22px;background:linear-gradient(to top,var(--bg),transparent);pointer-events:none}
.bf-cta{width:100%;height:54px;border:0;border-radius:16px;background:var(--brand);color:var(--on-brand);font:inherit;font-weight:700;font-size:16px;cursor:pointer}
.bf-cta:disabled{opacity:.45;cursor:default}
.bf-cta:focus-visible{outline:3px solid var(--brand-line);outline-offset:3px}
@media (prefers-reduced-motion:reduce){.bf-card-btn{transition:none}}
`
