import { useEffect, type CSSProperties, type ReactNode } from 'react'
import { METHOD_NAME } from '../lib/format'
import { QRCodeSVG } from 'qrcode.react'

const P: Record<string, ReactNode> = {
  home: <path d="M3 10.5 12 3l9 7.5V20a1 1 0 0 1-1 1h-5v-6H9v6H4a1 1 0 0 1-1-1z" />,
  txns: <path d="M7 4v16M7 20l-3-3M7 20l3-3M17 20V4M17 4l-3 3M17 4l3 3" />,
  link: <><path d="M10 13a5 5 0 0 0 7.5.5l3-3a5 5 0 0 0-7-7l-1.7 1.7" /><path d="M14 11a5 5 0 0 0-7.5-.5l-3 3a5 5 0 0 0 7 7l1.7-1.7" /></>,
  wallet: <><rect x="3" y="6" width="18" height="14" rx="3" /><path d="M3 10h18M16 15h1.5M7 6V4.5A1.5 1.5 0 0 1 8.5 3h7A1.5 1.5 0 0 1 17 4.5V6" /></>,
  settings: <><circle cx="12" cy="12" r="3" /><path d="M19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.8-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.5 1.7 1.7 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.8 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1.1 1.7 1.7 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.8.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.8V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z" /></>,
  doc: <><path d="M14 3H6a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9z" /><path d="M14 3v6h6" /></>,
  search: <><circle cx="11" cy="11" r="7" /><path d="m21 21-4.3-4.3" /></>,
  bell: <><path d="M6 8a6 6 0 0 1 12 0c0 7 3 9 3 9H3s3-2 3-9" /><path d="M10 21a2 2 0 0 0 4 0" /></>,
  menu: <path d="M4 7h16M4 12h16M4 17h16" />,
  plus: <path d="M12 5v14M5 12h14" />,
  arrowUpRight: <path d="M7 17 17 7M8 7h9v9" />,
  arrowRight: <path d="M5 12h14M13 5l7 7-7 7" />,
  arrowLeft: <path d="M19 12H5M12 19l-7-7 7-7" />,
  close: <path d="M18 6 6 18M6 6l12 12" />,
  check: <path d="M20 6 9 17l-5-5" />,
  clock: <><circle cx="12" cy="12" r="9" /><path d="M12 7v5l3 2" /></>,
  card: <><rect x="2" y="5" width="20" height="14" rx="2.5" /><path d="M2 10h20" /></>,
  bank: <path d="M3 21h18M5 21V10M19 21V10M9 21V10M15 21V10M2 10l10-7 10 7z" />,
  lock: <><rect x="4" y="11" width="16" height="10" rx="2" /><path d="M8 11V7a4 4 0 0 1 8 0v4" /></>,
  shield: <><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" /><path d="m9 12 2 2 4-4" /></>,
  download: <path d="M12 3v12M7 10l5 5 5-5M5 21h14" />,
  upload: <path d="M12 19V5M5 12l7-7 7 7" />,
  calendar: <><rect x="3" y="5" width="18" height="16" rx="2" /><path d="M3 10h18M8 3v4M16 3v4" /></>,
  chevronDown: <path d="m6 9 6 6 6-6" />,
  chevronRight: <path d="m9 18 6-6-6-6" />,
  updown: <path d="m7 15 5 5 5-5M7 9l5-5 5 5" />,
  copy: <><rect x="9" y="9" width="12" height="12" rx="2" /><path d="M5 15H4a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1h10a1 1 0 0 1 1 1v1" /></>,
  eye: <><path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12z" /><circle cx="12" cy="12" r="3" /></>,
  whatsapp: <path d="M21 12a9 9 0 0 1-13.5 7.8L3 21l1.2-4.5A9 9 0 1 1 21 12z" />,
  sms: <path d="M4 5h16v11H8l-4 4z" />,
  share: <path d="M4 12v7a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-7M16 6l-4-4-4 4M12 2v13" />,
  chart: <><path d="M3 3v18h18" /><path d="M7 15l4-4 3 3 5-6" /></>,
  store: <path d="M3 9l1.5-5h15L21 9M3 9v11h18V9M3 9h18M9 20v-6h6v6" />,
  alert: <path d="M10.3 3.9 1.8 18a2 2 0 0 0 1.7 3h17a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0zM12 9v4M12 17h.01" />,
  key: <><circle cx="7.5" cy="15.5" r="4.5" /><path d="m10.7 12.3 9.3-9.3M17 6l3 3M14 9l2 2" /></>,
  webhook: <><path d="M18 16.98h-5.99c-1.1 0-1.95.94-2.48 1.9A4 4 0 0 1 2 17c.01-.7.2-1.4.57-2" /><path d="m6 17 3.13-5.78c.53-.97.1-2.18-.5-3.1a4 4 0 1 1 6.89-4.06" /><path d="m12 6 3.13 5.73C15.66 12.7 16.9 13 18 13a4 4 0 0 1 0 8" /></>,
  user: <><circle cx="12" cy="8" r="4" /><path d="M4 21a8 8 0 0 1 16 0" /></>,
  logout: <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4M16 17l5-5-5-5M21 12H9" />,
  refresh: <path d="M21 12a9 9 0 1 1-3-6.7L21 8M21 3v5h-5" />,
  trash: <path d="M3 6h18M8 6V4h8v2M19 6l-1 14H6L5 6" />,
  repeat: <><path d="M17 2l4 4-4 4" /><path d="M3 11v-1a4 4 0 0 1 4-4h14" /><path d="M7 22l-4-4 4-4" /><path d="M21 13v1a4 4 0 0 1-4 4H3" /></>,
  users: <><circle cx="9" cy="8" r="4" /><path d="M2 21a7 7 0 0 1 14 0M16 3.5a4 4 0 0 1 0 9M22 21a7 7 0 0 0-4-6.3" /></>,
  userPlus: <><circle cx="9" cy="8" r="4" /><path d="M2 21a7 7 0 0 1 14 0M19 8v6M16 11h6" /></>,
  grid: <><rect x="3" y="3" width="7" height="7" rx="1.5" /><rect x="14" y="3" width="7" height="7" rx="1.5" /><rect x="3" y="14" width="7" height="7" rx="1.5" /><rect x="14" y="14" width="7" height="7" rx="1.5" /></>,
  pause: <path d="M8 5v14M16 5v14" />,
  dots: <path d="M5 12h.01M12 12h.01M19 12h.01" />,
  qr: <><rect x="3" y="3" width="7" height="7" rx="1" /><rect x="14" y="3" width="7" height="7" rx="1" /><rect x="3" y="14" width="7" height="7" rx="1" /><path d="M14 14h3v3h-3zM20 14v.01M14 20h.01M17 20h4v-3" /></>,
}

export function Icon({ name, size = 19, stroke = 1.9, color = 'currentColor', style }: { name: keyof typeof P | string; size?: number; stroke?: number; color?: string; style?: CSSProperties }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" aria-hidden="true" style={{ fill: 'none', stroke: color, strokeWidth: stroke, strokeLinecap: 'round', strokeLinejoin: 'round', flex: 'none', ...style }}>
      {P[name]}
    </svg>
  )
}

export function Logo({ dark = false, size = 28, text = 16 }: { dark?: boolean; size?: number; text?: number }) {
  return (
    <span style={{ display: 'inline-flex', alignItems: 'center', gap: 9 }}>
      <img src={dark ? '/assets/riha-icon-light.png' : '/assets/riha-icon-dark.png'} alt="" style={{ width: size, height: size, objectFit: 'contain' }} />
      <span style={{ fontFamily: 'var(--display)', fontWeight: 800, fontSize: text, letterSpacing: '.2em', color: dark ? '#fff' : 'var(--ink)' }}>RIHA</span>
    </span>
  )
}

// The stripes sit behind content, so they stay soft. Wider ones need more blur to look the same.
export const stripeBlur = (width: number) => `blur(${Math.round(Math.max(6, width * 0.18))}px)`

export function Stripes({ top = -40, right = 0, w = 44, h = 320, gap = 12, fade }: { top?: number; right?: number; w?: number; h?: number; gap?: number; fade?: string }) {
  return (
    <>
      <div className="stripe g" style={{ top, right: right + w + gap, width: w, height: h, filter: stripeBlur(w) }} />
      <div className="stripe r" style={{ top, right, width: w, height: h, filter: stripeBlur(w) }} />
      {fade && <div style={{ position: 'absolute', inset: 0, background: fade, pointerEvents: 'none' }} />}
    </>
  )
}

export function MethodIcon({ method, size = 36, radius = 11 }: { method: string; size?: number; radius?: number }) {
  const s: CSSProperties = { width: size, height: size, borderRadius: radius, fontSize: Math.round(size * 0.4) }
  if (method === 'mpesa' || method === 'emola' || method === 'bim') return <img className="method-ico" src={`/assets/methods/${method}.png`} alt={METHOD_NAME[method]} width={size} height={size} style={{ ...s, display: 'block', objectFit: 'cover', flex: 'none' }} />
  if (method === 'bank') return <span className="method-ico" style={{ ...s, background: 'var(--paper)', color: 'var(--ink)' }}><Icon name="bank" size={Math.round(size * 0.46)} /></span>
  return <span className="method-ico" style={{ ...s, background: 'var(--ink)', color: '#fff' }}><Icon name="card" size={Math.round(size * 0.46)} stroke={2} /></span>
}

export function Avatar({ text, color, size = 34, radius = 10, font = 13 }: { text: string; color?: string; size?: number; radius?: number; font?: number }) {
  return <div style={{ width: size, height: size, flex: 'none', borderRadius: radius, background: color || 'var(--sun)', display: 'grid', placeItems: 'center', fontWeight: 700, fontSize: font, color: 'var(--ink)' }}>{text}</div>
}

export function Pill({ t, bg, fg, style }: { t: string; bg: string; fg: string; style?: CSSProperties }) {
  return <span className="pill" style={{ background: bg, color: fg, ...style }}>{t}</span>
}

export function Toggle({ on, onChange, label }: { on: boolean; onChange: (v: boolean) => void; label?: string }) {
  return <button type="button" role="switch" aria-checked={on} aria-label={label} className={'toggle' + (on ? ' on' : '')} onClick={() => onChange(!on)}><span /></button>
}

export function Spinner({ dark }: { dark?: boolean }) { return <span className={'spinner' + (dark ? ' dark' : '')} /> }

export function Drawer({ title, onClose, children, footer }: { title: string; onClose: () => void; children: ReactNode; footer?: ReactNode }) {
  useEffect(() => {
    const h = (e: KeyboardEvent) => e.key === 'Escape' && onClose()
    window.addEventListener('keydown', h)
    return () => window.removeEventListener('keydown', h)
  }, [onClose])
  return (
    <div className="drawer-wrap">
      <div className="drawer-bg" onClick={onClose} />
      <div className="drawer" role="dialog" aria-label={title}>
        <div className="drawer-head">
          <div className="display" style={{ fontSize: 22, letterSpacing: '-0.02em' }}>{title}</div>
          <button className="icon-btn" onClick={onClose} aria-label="Fechar"><Icon name="close" size={16} stroke={2} /></button>
        </div>
        <div style={{ flex: 1, overflow: 'auto' }}>{children}</div>
        {footer && <div style={{ display: 'flex', gap: 10, padding: '18px 26px', borderTop: '1px solid var(--line)' }}>{footer}</div>}
      </div>
    </div>
  )
}

export function Modal({ onClose, children, width }: { onClose: () => void; children: ReactNode; width?: number }) {
  useEffect(() => {
    const h = (e: KeyboardEvent) => e.key === 'Escape' && onClose()
    window.addEventListener('keydown', h)
    return () => window.removeEventListener('keydown', h)
  }, [onClose])
  return (
    <div className="modal-wrap">
      <div className="drawer-bg" onClick={onClose} />
      <div className="modal" role="dialog" style={width ? { width } : undefined}>{children}</div>
    </div>
  )
}

export function QR({ value, size = 156, logo = true }: { value: string; size?: number; logo?: boolean }) {
  const l = Math.round(size * 0.24)
  return (
    <div style={{ position: 'relative', width: size, height: size }}>
      <QRCodeSVG value={value} size={size} fgColor="#0B120E" bgColor="transparent" level="H" />
      {logo && (
        <div style={{ position: 'absolute', left: '50%', top: '50%', width: l, height: l, margin: `${-l / 2}px 0 0 ${-l / 2}px`, borderRadius: Math.round(l * 0.27), background: 'var(--ink)', display: 'grid', placeItems: 'center', boxShadow: '0 0 0 4px #fff' }}>
          <img src="/assets/riha-icon-light.png" alt="" style={{ width: l * 0.6, height: l * 0.6, objectFit: 'contain' }} />
        </div>
      )}
    </div>
  )
}

export function Bars({ values, height = 150, gap = 5, highlightFrom, radius = 5, labels }: { values: number[]; height?: number; gap?: number; highlightFrom?: number; radius?: number; labels?: string[] }) {
  const max = Math.max(1, ...values)
  const n = values.length
  return (
    <div style={{ display: 'flex', alignItems: 'flex-end', gap, height }}>
      {values.map((v, i) => {
        const bg = i === n - 1 ? 'var(--green)' : highlightFrom !== undefined && i >= highlightFrom ? '#BFD8B8' : '#E4E8E1'
        const h = Math.max(4, Math.round((v / max) * (height - (labels ? 20 : 0))))
        return (
          <div key={i} title={labels?.[i]} style={{ flex: 1, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'flex-end', gap: 6, height: '100%' }}>
            <div style={{ width: '100%', height: h, borderRadius: radius, background: bg }} />
            {labels && <span style={{ fontSize: 11, color: i === n - 1 ? 'var(--ink)' : 'var(--muted-2)', fontWeight: 500 }}>{labels[i]}</span>}
          </div>
        )
      })}
    </div>
  )
}

export function Skeleton({ h = 20, w = '100%', r = 8, style }: { h?: number; w?: number | string; r?: number; style?: CSSProperties }) {
  return <div className="skeleton" style={{ height: h, width: w, borderRadius: r, ...style }} />
}

export function Field({ label, children, error, hint }: { label: string; children: ReactNode; error?: string; hint?: ReactNode }) {
  return (
    <label style={{ display: 'block' }}>
      <span className="label">{label}</span>
      {children}
      {error ? <div className="field-err">{error}</div> : hint ? <div style={{ fontSize: 12.5, color: 'var(--muted)', marginTop: 6 }}>{hint}</div> : null}
    </label>
  )
}

export function Check({ ok = true }: { ok?: boolean }) {
  return <Icon name={ok ? 'check' : 'close'} size={18} stroke={2.4} color={ok ? 'var(--green)' : 'var(--red-dark)'} />
}
