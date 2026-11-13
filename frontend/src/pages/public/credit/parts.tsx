import { useEffect, useRef, useState, type ReactNode } from 'react'
import { Avatar, Icon, Spinner } from '../../../components/ui'

export function Head({ m, sub }: { m: { name: string; initials: string; color: string }; sub: string }) {
  return (
    <div style={{ display: 'flex', gap: 12, alignItems: 'center', marginTop: 18 }}>
      <Avatar text={m.initials} color={m.color || '#FFD21F'} size={44} radius={13} font={15} />
      <div style={{ minWidth: 0 }}><div style={{ fontWeight: 700, fontSize: 16 }}>{m.name}</div><div style={{ fontSize: 13, color: 'var(--muted)' }}>{sub}</div></div>
    </div>
  )
}

export function Stepper({ label, onDec, onInc, canDec, canInc }: { label: string; onDec: () => void; onInc: () => void; canDec: boolean; canInc: boolean }) {
  const b = (on: boolean) => ({ width: 52, height: '100%', border: 0, background: 'none', fontSize: 22, color: on ? 'var(--ink)' : '#C9CDC6' })
  return (
    <div style={{ display: 'flex', alignItems: 'center', height: 54, borderRadius: 16, border: '1px solid var(--line)', background: '#fff' }}>
      <button type="button" style={b(canDec)} disabled={!canDec} onClick={onDec} aria-label="Menos">−</button>
      <b style={{ flex: 1, textAlign: 'center', fontSize: 16 }}>{label}</b>
      <button type="button" style={b(canInc)} disabled={!canInc} onClick={onInc} aria-label="Mais">+</button>
    </div>
  )
}

export function Row({ k, v, strong }: { k: ReactNode; v: ReactNode; strong?: boolean }) {
  return <div className="between" style={{ padding: '5px 0', fontSize: 14 }}><span style={{ color: strong ? 'var(--ink)' : 'var(--muted)', fontWeight: strong ? 700 : 400 }}>{k}</span><span className="tnum" style={{ fontWeight: strong ? 700 : 500 }}>{v}</span></div>
}

export function Cta({ children, onClick, disabled, busy, tone = 'green' }: { children: ReactNode; onClick: () => void; disabled?: boolean; busy?: boolean; tone?: 'green' | 'ink' }) {
  return <button className={`btn btn-${tone} btn-lg btn-block`} disabled={disabled || busy} onClick={onClick}>{busy ? <Spinner /> : children}</button>
}

// The sheet for the 6-digit SMS code: the same for confirming a number and accepting an agreement.
export function CodeSheet({ phone, note, testCode, busy, err, onSubmit, onResend, onClose }: { phone: string; note: ReactNode; testCode?: string; busy: boolean; err: string; onSubmit: (code: string) => void; onResend: () => void; onClose: () => void }) {
  const [code, setCode] = useState('')
  const ref = useRef<HTMLInputElement>(null)
  useEffect(() => { ref.current?.focus() }, [])
  useEffect(() => { if (code.length === 6) onSubmit(code) }, [code]) // eslint-disable-line react-hooks/exhaustive-deps
  return (
    <div style={{ position: 'fixed', inset: 0, zIndex: 60, display: 'flex', alignItems: 'flex-end', justifyContent: 'center' }}>
      <div className="drawer-bg" onClick={onClose} />
      <div style={{ position: 'relative', width: '100%', maxWidth: 480, background: 'var(--paper)', borderRadius: '26px 26px 0 0', padding: '10px 22px calc(22px + env(safe-area-inset-bottom))' }}>
        <div style={{ width: 40, height: 4, borderRadius: 2, background: '#D6D4CB', margin: '0 auto 14px' }} />
        <div className="display" style={{ fontSize: 24 }}>Código de 6 dígitos</div>
        <div className="muted" style={{ fontSize: 14, marginTop: 4 }}>Enviado por SMS para o {phone}</div>
        <label style={{ position: 'relative', display: 'grid', gridTemplateColumns: 'repeat(6,1fr)', gap: 8, marginTop: 18 }}>
          {Array.from({ length: 6 }, (_, i) => (
            <span key={i} style={{ height: 56, borderRadius: 14, background: '#fff', border: i === code.length ? '2px solid var(--ink)' : '1px solid #E2E0D8', display: 'grid', placeItems: 'center', fontSize: 22, fontWeight: 700 }}>{code[i] || ''}</span>
          ))}
          <input ref={ref} value={code} inputMode="numeric" autoComplete="one-time-code" aria-label="Código" maxLength={6} onChange={e => setCode(e.target.value.replace(/\D/g, '').slice(0, 6))} style={{ position: 'absolute', inset: 0, opacity: 0, fontSize: 16 }} />
        </label>
        {testCode && <div style={{ marginTop: 10, fontSize: 12.5, padding: '8px 12px', borderRadius: 10, background: 'var(--sun-bg)', color: 'var(--sun-text)' }}>Modo de teste: o código é <b className="mono">{testCode}</b></div>}
        <div style={{ fontSize: 13, color: 'var(--muted)', marginTop: 12, lineHeight: 1.5 }}>{note}</div>
        {err && <div className="form-err" style={{ marginTop: 10 }} role="alert">{err}</div>}
        <div style={{ marginTop: 16 }}><Cta busy={busy} disabled={code.length !== 6} onClick={() => onSubmit(code)}>Confirmar</Cta></div>
        <button className="btn btn-ghost btn-block" style={{ marginTop: 4, textDecoration: 'underline', fontSize: 13.5 }} onClick={() => { setCode(''); onResend() }}>Não recebi o código</button>
      </div>
    </div>
  )
}

export function Talk({ contact, name, children }: { contact?: string; name: string; children?: ReactNode }) {
  if (!contact) return null
  return <a className="link-btn" style={{ color: 'var(--ink)', fontSize: 13.5 }} href={`https://wa.me/258${contact}`} target="_blank" rel="noreferrer">{children || `Falar com a ${name}`}</a>
}

export function Done({ title, text, children }: { title: string; text?: ReactNode; children?: ReactNode }) {
  return (
    <div style={{ marginTop: 22 }}>
      <div style={{ width: 64, height: 64, borderRadius: '50%', background: 'var(--green)', display: 'grid', placeItems: 'center' }}><Icon name="check" size={30} stroke={2.6} color="#fff" /></div>
      <h1 className="display" style={{ fontSize: 28, margin: '22px 0 0' }}>{title}</h1>
      {text && <p style={{ fontSize: 15, color: 'var(--text-2)', lineHeight: 1.5, margin: '8px 0 0' }}>{text}</p>}
      {children}
    </div>
  )
}
