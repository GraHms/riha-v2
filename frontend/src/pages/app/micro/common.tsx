import type { CSSProperties, ReactNode } from 'react'
import { useOutletContext } from 'react-router-dom'
import { asDate, dly, mt, toneColors } from '../../../lib/credit'
import type { ShellCtx } from '../AppShell'

export type MicroCtx = ShellCtx & {
  version: number
  bump: () => void
  products: any[]
  reloadProducts: () => void
  requestUrl: string
  pending: number
  newLoan: () => void
  shareLink: () => void
}
export const useMicro = () => useOutletContext<MicroCtx>()

export function Step({ label, onDec, onInc, canDec = true, canInc = true, style, mono }: { label: ReactNode; onDec: () => void; onInc: () => void; canDec?: boolean; canInc?: boolean; style?: CSSProperties; mono?: boolean }) {
  const b = (on: boolean): CSSProperties => ({ width: 38, height: '100%', border: 0, background: 'none', fontSize: 20, color: on ? 'var(--ink)' : '#C9CDC6', cursor: on ? 'pointer' : 'default', flex: 'none' })
  return (
    <div style={{ display: 'flex', alignItems: 'center', height: 48, borderRadius: 13, border: '1px solid var(--line-2)', background: '#fff', ...style }}>
      <button type="button" style={b(canDec)} disabled={!canDec} onClick={onDec} aria-label="Menos">−</button>
      <span style={{ flex: 1, textAlign: 'center', fontWeight: 600, fontSize: 15, fontFamily: mono ? 'var(--mono)' : undefined, whiteSpace: 'nowrap' }}>{label}</span>
      <button type="button" style={b(canInc)} disabled={!canInc} onClick={onInc} aria-label="Mais">+</button>
    </div>
  )
}

export function Seg<T extends string | boolean>({ value, options, onChange, style }: { value: T; options: [T, string][]; onChange: (v: T) => void; style?: CSSProperties }) {
  return (
    <div className="seg" style={{ display: 'flex', ...style }}>
      {options.map(([k, l]) => <button key={String(k)} type="button" style={{ flex: 1 }} className={value === k ? 'on' : ''} onClick={() => onChange(k)}>{l}</button>)}
    </div>
  )
}

export function Tone({ tone, children, style }: { tone: string; children: ReactNode; style?: CSSProperties }) {
  const [bg, fg] = toneColors[tone] || toneColors.wait
  return <span className="pill" style={{ background: bg, color: fg, ...style }}>{children}</span>
}

export const REQ_TONE: Record<string, string> = { new: 'late', review: 'neu', offered: 'wait', declined: 'bad', canceled: 'wait' }

export const INST_STATUS: Record<string, [string, string]> = {
  paid: ['Paga', 'ok'], paid_outside: ['Paga fora da RIHA', 'wait'], late: ['Em atraso', 'late'], scheduled: ['Agendada', 'wait'],
  replaced: ['Substituída', 'wait'], written_off: ['Perda', 'bad'], canceled: ['Cancelada', 'wait'],
}

const PIP: Record<string, string> = { paid: 'var(--green)', outside: 'repeating-linear-gradient(135deg,#8A938E 0 2px,#D5D3CB 2px 4px)', late: 'var(--sun)', todo: '#E6E4DC' }

export function Pips({ pips, width = 10 }: { pips: string[]; width?: number }) {
  return <span style={{ display: 'inline-flex', gap: 2 }}>{pips.map((p, i) => <span key={i} style={{ width: pips.length > 8 ? 6 : width, height: 6, borderRadius: 2, background: PIP[p] }} />)}</span>
}

export function MT({ v, size = 30, unit = 13, color }: { v: number; size?: number; unit?: number; color?: string }) {
  return <span className="display tnum" style={{ fontSize: size, letterSpacing: '-0.03em', color }}>{mt(v)}<span style={{ fontSize: unit, color: 'var(--muted)', marginLeft: 4, fontFamily: 'var(--sans)', fontWeight: 600 }}>MT</span></span>
}

export function KV({ k, v, strong, color }: { k: ReactNode; v: ReactNode; strong?: boolean; color?: string }) {
  return <div className="between" style={{ padding: '7px 0', fontSize: 14 }}><span style={{ color: 'var(--muted)' }}>{k}</span><span className="tnum" style={{ fontWeight: strong ? 700 : 500, color, textAlign: 'right' }}>{v}</span></div>
}

export function Plan({ rows, head, foot }: { rows: { i: string; d: string; v: string }[]; head?: ReactNode; foot?: ReactNode }) {
  return (
    <div className="card" style={{ overflow: 'hidden', borderRadius: 16 }}>
      {head && <div style={{ padding: '12px 16px', borderBottom: '1px solid var(--line-3)' }}>{head}</div>}
      <div style={{ padding: '6px 16px' }}>
        {rows.map((r, i) => <div key={i} style={{ display: 'grid', gridTemplateColumns: '22px 1fr auto', gap: 8, padding: '6px 0', fontSize: 13.5 }}><span className="muted">{r.i}</span><span>{r.d}</span><span className="tnum">{r.v}</span></div>)}
      </div>
      {foot && <div style={{ padding: '10px 16px', borderTop: '1px solid var(--line-3)', fontSize: 13 }}>{foot}</div>}
    </div>
  )
}

export function Radio({ on, title, note, onClick }: { on: boolean; title: string; note?: ReactNode; onClick: () => void }) {
  return (
    <button type="button" onClick={onClick} className="card" style={{ display: 'flex', gap: 12, alignItems: 'flex-start', padding: '14px 16px', textAlign: 'left', width: '100%', border: on ? '2px solid var(--ink)' : '1px solid var(--line)', margin: on ? 0 : 1, borderRadius: 16 }}>
      <span style={{ width: 18, height: 18, borderRadius: '50%', border: on ? '5px solid var(--ink)' : '1.5px solid #C9CDC6', flex: 'none', marginTop: 1 }} />
      <span><b style={{ display: 'block', fontSize: 14 }}>{title}</b>{note && <span style={{ display: 'block', fontSize: 12.5, color: 'var(--muted)', marginTop: 3, lineHeight: 1.45 }}>{note}</span>}</span>
    </button>
  )
}

export const shortWhen = (iso: string) => {
  const d = asDate(iso), now = new Date()
  const same = d.toDateString() === now.toDateString()
  const hm = d.toLocaleTimeString('pt-PT', { hour: '2-digit', minute: '2-digit' })
  return same ? `hoje ${hm}` : `${d.getDate()} ${['jan', 'fev', 'mar', 'abr', 'mai', 'jun', 'jul', 'ago', 'set', 'out', 'nov', 'dez'][d.getMonth()]}${d.getFullYear() !== now.getFullYear() ? ' ' + d.getFullYear() : ''} ${hm}`
}
export const longDay = (iso: string) => dly(asDate(iso))

export function Back({ to, label, nav }: { to: string; label: string; nav: (p: string) => void }) {
  return <button className="link-btn" style={{ color: 'var(--muted)', fontSize: 13, fontWeight: 500 }} onClick={() => nav(to)}>‹ {label}</button>
}

export const CSS = `
.mc-head{display:flex;align-items:flex-end;gap:12px;flex-wrap:wrap}
.mc-actions{display:flex;gap:8px;flex-wrap:wrap}
.mc-row{display:grid;align-items:center;gap:12px;padding:12px 20px;border-bottom:1px solid var(--line-3);font-size:14px}
.mc-row:last-child{border-bottom:0}
.mc-row.click{cursor:pointer}.mc-row.click:hover{background:#FBFBF8}
.mc-th{display:grid;gap:12px;padding:12px 20px;font-size:11.5px;font-weight:600;color:var(--muted);text-transform:uppercase;letter-spacing:.06em;border-bottom:1px solid var(--line-3)}
.mc-chips{display:flex;gap:8px;flex-wrap:wrap;align-items:center}
.mc-chip{height:34px;padding:0 14px;border-radius:999px;border:1px solid var(--line);background:#fff;font-weight:600;font-size:13px;white-space:nowrap}
.mc-chip.on{background:var(--ink);color:#fff;border-color:var(--ink)}
.mc-cols{display:grid;grid-template-columns:minmax(0,1.6fr) minmax(0,1fr);gap:14px;align-items:start}
.mc-card-list{display:none}
.mc-sub{font-size:12.5px;color:var(--muted);margin-top:2px}
@media (max-width:1100px){.mc-cols{grid-template-columns:1fr}}
@media (max-width:860px){
  .mc-title{font-size:26px!important}
  .mc-actions .btn{height:38px;padding:0 12px;font-size:13.5px}
  .mc-actions .secondary{display:none}
  .mc-table{display:none}
  .mc-card-list{display:flex;flex-direction:column;gap:10px}
}`
