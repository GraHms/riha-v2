import { Fragment, useEffect, useState, type CSSProperties, type ReactNode } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { Avatar, Icon, Logo } from './ui'
import { fmt, initials } from '../lib/format'
import '../styles/ops.css'

// The kit of RIHA Ops, the backoffice and the partners' accounts. Pages say
// what they show; the kit makes it look the same everywhere, on any screen.

// ---- Shell ----

export type NavItem = { to: string; icon: string; label: string; count?: number; hot?: boolean; exact?: boolean }
export type NavGroup = { title?: string; items: NavItem[] }

function Side({ env, org, groups, me, onLogout, onGo }: { env: 'admin' | 'partner'; org?: string; groups: NavGroup[]; me: { name: string; sub: string }; onLogout: () => void; onGo?: () => void }) {
  const nav = useNavigate()
  const path = useLocation().pathname
  const on = (i: NavItem) => (i.exact ? path === i.to : path === i.to || path.startsWith(i.to + '/'))
  return (
    <aside className="o-side">
      <div className="o-brand"><Logo size={24} text={14} /><span className={'o-env' + (env === 'partner' ? ' partner' : '')}>{env === 'admin' ? 'OPS' : 'PARCEIRO'}</span></div>
      {org && <div className="o-org"><small>Conta de</small><b>{org}</b></div>}
      <nav>
        {groups.filter(g => g.items.length).map((g, gi) => (
          <div key={gi} className="o-group">
            {g.title && <div className="o-group-t">{g.title}</div>}
            {g.items.map(i => (
              <button key={i.to} className={'o-nav' + (on(i) ? ' on' : '')} onClick={() => { nav(i.to); onGo?.() }} aria-current={on(i) ? 'page' : undefined}>
                <Icon name={i.icon} size={17} />{i.label}
                {!!i.count && <span className={'o-count' + (i.hot ? ' hot' : '')}>{i.count}</span>}
              </button>
            ))}
          </div>
        ))}
      </nav>
      <div style={{ flex: 1 }} />
      <div className="o-me">
        <Avatar text={initials(me.name || 'R')} color="#DCEFD6" size={32} radius={9} font={12} />
        <div style={{ flex: 1, minWidth: 0 }}><b>{me.name}</b><small>{me.sub}</small></div>
        <button className="btn btn-ghost btn-sm" style={{ padding: 0, width: 30 }} aria-label="Terminar sessão" onClick={onLogout}><Icon name="logout" size={16} /></button>
      </div>
    </aside>
  )
}

export function Shell({ env, org, groups, me, onLogout, top, title, children }:
  { env: 'admin' | 'partner'; org?: string; groups: NavGroup[]; me: { name: string; sub: string }; onLogout: () => void; top?: ReactNode; title: string; children: ReactNode }) {
  const [open, setOpen] = useState(false)
  const path = useLocation().pathname
  useEffect(() => setOpen(false), [path])
  useEffect(() => {
    if (!open) return
    const h = (e: KeyboardEvent) => e.key === 'Escape' && setOpen(false)
    window.addEventListener('keydown', h)
    return () => window.removeEventListener('keydown', h)
  }, [open])
  const hot = groups.flatMap(g => g.items).reduce((a, i) => a + (i.hot ? i.count || 0 : 0), 0)
  return (
    <div className="ops o-shell">
      <Side env={env} org={org} groups={groups} me={me} onLogout={onLogout} />
      <div className="o-main">
        <div className="o-top">{top}<div style={{ flex: 1 }} /><span className="o-live"><i />Produção</span></div>
        <div className="o-mtop">
          <button className="btn btn-outline btn-sm" style={{ width: 36, height: 36, padding: 0, position: 'relative' }} onClick={() => setOpen(true)} aria-label="Abrir menu">
            <Icon name="menu" size={18} />{hot > 0 && <span style={{ position: 'absolute', top: -3, right: -3, width: 10, height: 10, borderRadius: '50%', background: '#FFD21F', border: '2px solid #fff' }} />}
          </button>
          <Logo size={22} text={13} /><span className={'o-env' + (env === 'partner' ? ' partner' : '')} style={{ fontSize: 9.5, fontWeight: 700, letterSpacing: '.08em', padding: '2px 6px', borderRadius: 5, background: env === 'admin' ? 'var(--o-ink)' : 'var(--o-brand-soft)', color: env === 'admin' ? '#fff' : 'var(--o-brand-ink)' }}>{env === 'admin' ? 'OPS' : 'PARCEIRO'}</span>
          <span style={{ flex: 1 }} aria-label={title} />
        </div>
        <main className="o-content">{children}</main>
      </div>
      {open && (
        <div className="ops o-sheet-wrap" role="dialog" aria-label="Menu">
          <div className="o-sheet-bg" onClick={() => setOpen(false)} />
          <div className="o-sheet"><Side env={env} org={org} groups={groups} me={me} onLogout={onLogout} onGo={() => setOpen(false)} /></div>
        </div>
      )}
    </div>
  )
}

/** titleFor names the page a path is on, for the phone's top bar. */
export function titleFor(groups: NavGroup[], path: string, fallback: string) {
  const all = groups.flatMap(g => g.items).sort((a, b) => b.to.length - a.to.length)
  return all.find(i => (i.exact ? path === i.to : path === i.to || path.startsWith(i.to + '/')))?.label || fallback
}

// ---- Page parts ----

export function Page({ children, style }: { children: ReactNode; style?: CSSProperties }) {
  return <div className="o-page" style={style}>{children}</div>
}

export function PageHead({ title, sub, actions, back }: { title: ReactNode; sub?: ReactNode; actions?: ReactNode; back?: { label: string; onClick: () => void } }) {
  return (
    <div className="o-head">
      <div style={{ minWidth: 0 }}>
        {back && <button className="btn btn-ghost btn-sm" style={{ padding: '0 6px 0 2px', marginBottom: 6, marginLeft: -4 }} onClick={back.onClick}><Icon name="arrowLeft" size={15} stroke={2.2} />{back.label}</button>}
        <h1>{title}</h1>
        {sub && <p>{sub}</p>}
      </div>
      {actions && <div className="o-head-actions">{actions}</div>}
    </div>
  )
}

export function Section({ title, sub, actions }: { title: ReactNode; sub?: ReactNode; actions?: ReactNode }) {
  return (
    <div className="o-section">
      <h2>{title}{sub && <small>{sub}</small>}</h2>
      {actions && <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', alignItems: 'center' }}>{actions}</div>}
    </div>
  )
}

/** Tip holds what a page used to explain in a paragraph: there when wanted. */
export function Tip({ text }: { text: string }) {
  return <button type="button" className="o-tip" data-tip={text} aria-label={text}>i</button>
}

/** Money shows meticais from cents, the cents smaller, so the amount reads first. */
export function Money({ c, unit = true, cents = true }: { c: number | null | undefined; unit?: boolean; cents?: boolean }) {
  const s = fmt(c ?? 0, cents)
  const [whole, dec] = s.split(',')
  return <>{whole}{cents && dec !== undefined && <span className="o-cents">,{dec}</span>}{unit && <small>MT</small>}</>
}

export function Stat({ label, value, hint, tone, tip, onClick, delta, cents = false }:
  { label: ReactNode; value: number | string | null | undefined; hint?: ReactNode; tone?: 'warn' | 'bad' | 'ink'; tip?: string; onClick?: () => void; delta?: number | null; cents?: boolean }) {
  const body = (
    <>
      <span className="o-stat-l">{label}{tip && <Tip text={tip} />}</span>
      <span className="o-stat-v">{typeof value === 'number' ? <Money c={value} cents={cents} /> : value ?? '—'}</span>
      {(hint || delta !== undefined) && (
        <span className="o-stat-h">
          {delta !== undefined && delta !== null && <span className={'o-delta ' + (delta >= 0 ? 'up' : 'down')}>{delta >= 0 ? '↑' : '↓'} {Math.abs(delta)}%</span>}
          {hint}
        </span>
      )}
    </>
  )
  const cls = 'o-stat' + (tone ? ' tone-' + tone : '')
  return onClick ? <button className={cls} onClick={onClick}>{body}</button> : <div className={cls}>{body}</div>
}

export function Panel({ title, sub, action, children, pad = true, foot, style }: { title?: ReactNode; sub?: ReactNode; action?: ReactNode; children: ReactNode; pad?: boolean; foot?: ReactNode; style?: CSSProperties }) {
  return (
    <section className="o-panel" style={style}>
      {(title || action) && <div className="o-panel-h"><div><h3>{title}</h3>{sub && <div className="o-sub">{sub}</div>}</div>{action}</div>}
      {pad ? <div className="o-panel-b">{children}</div> : children}
      {foot && <div className="o-panel-f">{foot}</div>}
    </section>
  )
}

export type Tone = 'ok' | 'warn' | 'bad' | 'info' | 'ink' | undefined
export function Badge({ tone, dot, children, title }: { tone?: Tone; dot?: boolean; children: ReactNode; title?: string }) {
  return <span className={'o-badge' + (tone ? ' ' + tone : '')} title={title}>{dot && <i />}{children}</span>
}

export function Empty({ icon = 'check', title, text, action }: { icon?: string; title: string; text?: string; action?: ReactNode }) {
  return (
    <div className="o-empty">
      <div className="o-empty-i"><Icon name={icon} size={20} /></div>
      <b>{title}</b>{text && <span>{text}</span>}
      {action && <div style={{ marginTop: 14 }}>{action}</div>}
    </div>
  )
}

export function KV({ k, v, total }: { k: ReactNode; v: ReactNode; total?: boolean }) {
  return <div className={'o-kv' + (total ? ' total' : '')}><span>{k}</span><span>{v}</span></div>
}

// ---- Lists ----

export type Col<T> = { head: ReactNode; w: string; cell: (r: T) => ReactNode; right?: boolean }
export type CardView = { lead?: ReactNode; title: ReactNode; sub?: ReactNode; amount?: ReactNode; meta?: ReactNode; foot?: ReactNode }

/** List is a table on wide screens and a stack of cards on phones: each row
 * says how it reads as a card. */
export function List<T>({ cols, rows, rowKey, onRow, card, empty, selected, footer, head = true }:
  { cols: Col<T>[]; rows: T[]; rowKey: (r: T) => string; onRow?: (r: T) => void; card: (r: T) => CardView; empty?: ReactNode; selected?: (r: T) => boolean; footer?: ReactNode; head?: boolean }) {
  const grid = cols.map(c => c.w).join(' ')
  return (
    <div className="o-list">
      {head && rows.length > 0 && <div className="o-list-head" style={{ gridTemplateColumns: grid }}>{cols.map((c, i) => <span key={i} className={c.right ? 'o-cell r' : 'o-cell'}>{c.head}</span>)}</div>}
      {rows.map(r => {
        const k = rowKey(r)
        const sel = selected?.(r)
        const v = card(r)
        return (
          <Fragment key={k}>
            <div className={'o-row' + (onRow ? ' click' : '') + (sel ? ' sel' : '')} style={{ gridTemplateColumns: grid }} onClick={onRow ? () => onRow(r) : undefined}>
              {cols.map((c, i) => <div key={i} className={c.right ? 'o-cell r' : 'o-cell'}>{c.cell(r)}</div>)}
            </div>
            <div className={'o-card-row' + (sel ? ' sel' : '')} onClick={onRow ? () => onRow(r) : undefined} role={onRow ? 'button' : undefined} tabIndex={onRow ? 0 : undefined}
              onKeyDown={onRow ? e => { if (e.key === 'Enter') onRow(r) } : undefined}>
              <div className="o-cr-lead">{v.lead}</div>
              <div className="o-cr-main"><div className="o-t1">{v.title}</div>{v.sub && <div className="o-t2" style={{ whiteSpace: 'normal' }}>{v.sub}</div>}</div>
              <div className="o-cr-amt">{v.amount && <div className="o-amt">{v.amount}</div>}{v.meta && <div style={{ marginTop: 4 }}>{v.meta}</div>}</div>
              {v.foot && <div className="o-cr-foot" onClick={e => e.stopPropagation()}>{v.foot}</div>}
            </div>
          </Fragment>
        )
      })}
      {!rows.length && (empty || <Empty title="Nada por aqui" />)}
      {footer}
    </div>
  )
}

export function Amount({ c, cents = true }: { c: number | null | undefined; cents?: boolean }) {
  return <span className="o-amt">{fmt(c ?? 0, cents)}<small>MT</small></span>
}

// ---- The money map ----

export type Holder = { key: string; label: string; amount: number; color: string; hatch?: boolean }

/** MoneyMap lays where the money is against what is owed to merchants: the
 * difference is RIHA's own, or, if it's short, what is missing. */
export function MoneyMap({ holders, owed, title, action }: { holders: Holder[]; owed: number; title?: ReactNode; action?: ReactNode }) {
  const held = holders.reduce((a, h) => a + Math.max(0, h.amount), 0)
  const own = held - owed
  const scale = Math.max(held, owed, 1)
  const pct = (v: number) => `${Math.max(0, (v / scale) * 100)}%`
  return (
    <section className="o-panel o-map">
      <div className="o-map-top">
        <div>
          <div className="o-stat-l">{title || 'Onde está o dinheiro'}</div>
          <div className="o-stat-v" style={{ marginTop: 4 }}><Money c={held} cents={false} /></div>
        </div>
        <div style={{ display: 'flex', gap: 10, alignItems: 'center', flexWrap: 'wrap' }}>
          <Badge tone={own >= 0 ? 'ok' : 'bad'} dot>{own >= 0 ? 'Cobre o que devemos' : 'Falta cobrir ' + fmt(-own, false) + ' MT'}</Badge>
          {action}
        </div>
      </div>
      <div className="o-map-rail">
        <div className="o-map-line">
          <span>Temos</span>
          <div className="o-map-track">
            {holders.filter(h => h.amount > 0).map(h => (
              <div key={h.key} className={'o-map-seg' + (h.hatch ? ' hatch' : '')} style={{ width: pct(h.amount), background: h.color }} title={`${h.label}: ${fmt(h.amount)} MT`} />
            ))}
          </div>
        </div>
        <div className="o-map-line">
          <span>Devemos</span>
          <div className="o-map-track">
            <div className="o-map-seg" style={{ width: pct(owed), background: 'var(--o-owed)' }} title={`Aos comerciantes: ${fmt(owed)} MT`} />
            {own > 0 && <div className="o-map-seg gap" style={{ width: pct(own) }} title={`Da RIHA: ${fmt(own)} MT`} />}
          </div>
        </div>
      </div>
      <div className="o-map-legend">
        {holders.map(h => <div key={h.key}><i className="o-dot" style={{ background: h.color }} /><span>{h.label}</span><b>{fmt(h.amount, false)}</b></div>)}
        <div><i className="o-dot" style={{ background: 'var(--o-owed)' }} /><span>Aos comerciantes</span><b>{fmt(owed, false)}</b></div>
        <div><i className="o-dot" style={{ background: 'var(--o-brand-soft)', border: '1px dashed #A8D3A0' }} /><span>Da RIHA</span><b style={{ color: own < 0 ? 'var(--o-bad)' : undefined }}>{fmt(own, false)}</b></div>
      </div>
    </section>
  )
}

/** Colors for the partners holding money, in a fixed order by code. */
export const HOLDER_COLORS = ['var(--o-h1)', 'var(--o-h2)', 'var(--o-h3)', 'var(--o-h4)']
