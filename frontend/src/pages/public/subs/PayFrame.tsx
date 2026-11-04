import type { ReactNode } from 'react'
import { Avatar, Icon } from '../../../components/ui'

export function PayFrame({ children, test }: { children: ReactNode; test?: boolean }) {
  return (
    <div style={{ minHeight: '100%', background: 'var(--paper)', display: 'flex', justifyContent: 'center', padding: '18px 16px 32px' }}>
      <div style={{ width: '100%', maxWidth: 440, display: 'flex', flexDirection: 'column' }}>
        <div style={{ display: 'flex', justifyContent: 'center', gap: 8 }}>
          <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6, padding: '6px 12px', borderRadius: 999, background: '#EDECE6', fontSize: 12.5, color: 'var(--text-2)' }}><Icon name="lock" size={12} stroke={2.4} />pay.riha.co.mz</span>
          {test && <span style={{ display: 'inline-flex', alignItems: 'center', padding: '6px 12px', borderRadius: 999, background: 'var(--sun-bg)', fontSize: 12.5, fontWeight: 600, color: 'var(--sun-text)' }}>Modo de teste</span>}
        </div>
        {children}
      </div>
    </div>
  )
}

export function MerchantHead({ m, sub, children }: { m: { name: string; initials: string; color: string }; sub?: ReactNode; children?: ReactNode }) {
  return (
    <div className="card" style={{ marginTop: 16, padding: 18, borderRadius: 22 }}>
      <div style={{ display: 'flex', gap: 12, alignItems: 'center' }}>
        <Avatar text={m.initials} color={m.color || '#FFD21F'} size={44} radius={13} font={15} />
        <div style={{ minWidth: 0 }}><div style={{ fontWeight: 700, fontSize: 16 }}>{m.name}</div>{sub && <div style={{ fontSize: 13, color: 'var(--muted)' }}>{sub}</div>}</div>
      </div>
      {children}
    </div>
  )
}

export function Center({ children }: { children: ReactNode }) {
  return <div style={{ minHeight: '100%', display: 'grid', placeItems: 'center', padding: 24, textAlign: 'center', background: 'var(--paper)' }}><div>{children}</div></div>
}
