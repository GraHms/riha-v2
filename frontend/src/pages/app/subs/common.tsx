import type { CSSProperties, ReactNode } from 'react'
import { Pill } from '../../../components/ui'
import { fmt } from '../../../lib/format'

export const MONTH_LETTERS = ['J', 'F', 'M', 'A', 'M', 'J', 'J', 'A', 'S', 'O', 'N', 'D']
export const MONTH_SHORT = ['jan', 'fev', 'mar', 'abr', 'mai', 'jun', 'jul', 'ago', 'set', 'out', 'nov', 'dez']
export const ALL_MONTHS = 0xfff

export const SUB_STATUS: Record<string, { t: string; bg: string; fg: string }> = {
  active: { t: 'Ativa', bg: 'var(--green-bg)', fg: 'var(--green-dark)' },
  past_due: { t: 'Em dívida', bg: 'var(--sun-bg)', fg: 'var(--sun-text)' },
  suspended: { t: 'Suspensa', bg: 'var(--red-bg)', fg: 'var(--red-dark)' },
  paused: { t: 'Pausada', bg: '#E6ECFB', fg: '#2A4AA8' },
  invited: { t: 'Convite enviado', bg: '#EDECE6', fg: '#3E4742' },
  incomplete: { t: 'Por concluir', bg: '#EDECE6', fg: '#3E4742' },
  canceled: { t: 'Cancelada', bg: '#EDECE6', fg: '#6B746F' },
  ended: { t: 'Terminou', bg: '#EDECE6', fg: '#3E4742' },
}

export const CHARGE_STATUS: Record<string, { t: string; bg: string; fg: string }> = {
  scheduled: { t: 'Agendada', bg: '#EDECE6', fg: '#3E4742' },
  open: { t: 'Link enviado', bg: '#E6ECFB', fg: '#2A4AA8' },
  past_due: { t: 'Em dívida', bg: 'var(--sun-bg)', fg: 'var(--sun-text)' },
  paid: { t: 'Paga', bg: 'var(--green-bg)', fg: 'var(--green-dark)' },
  paid_outside: { t: 'Paga fora da RIHA', bg: '#EDECE6', fg: '#3E4742' },
  forfeited: { t: 'Perdida', bg: '#F3E9E9', fg: '#8E0609' },
  skipped: { t: 'Sem cobrança', bg: '#F4F3EE', fg: '#6B746F' },
  canceled: { t: 'Cancelada', bg: '#F4F3EE', fg: '#6B746F' },
}

export const PLAN_STATUS: Record<string, { t: string; bg: string; fg: string }> = {
  active: { t: 'Ativo', bg: 'var(--green-bg)', fg: 'var(--green-dark)' },
  draft: { t: 'Rascunho', bg: '#EDECE6', fg: '#3E4742' },
  archived: { t: 'Arquivado', bg: '#EDECE6', fg: '#6B746F' },
}

export const OUTSIDE_METHODS: [string, string][] = [['cash', 'Dinheiro'], ['transfer', 'Transferência'], ['deposit', 'Depósito'], ['other', 'Outro']]
export const VIA: Record<string, string> = { mpesa: 'M-Pesa', emola: 'e-Mola', bim: 'Bim IZI', card: 'Cartão', outside: 'Fora da RIHA' }
export const CHANNEL: Record<string, string> = { sms: 'SMS', whatsapp: 'WhatsApp', email: 'Email' }

export function StatusPill({ map, s, style }: { map: Record<string, { t: string; bg: string; fg: string }>; s: string; style?: CSSProperties }) {
  const v = map[s] || { t: s, bg: '#EDECE6', fg: '#3E4742' }
  return <Pill {...v} style={style} />
}

export const monthOn = (mask: number, i: number) => (mask & (1 << i)) !== 0

export function MonthStrip({ mask, size = 22, paid, highlight, onToggle }: { mask: number; size?: number; paid?: Record<number, string>; highlight?: number; onToggle?: (i: number) => void }) {
  return (
    <div style={{ display: 'flex', gap: size > 30 ? 6 : 4 }}>
      {MONTH_LETTERS.map((l, i) => {
        const on = monthOn(mask, i)
        const st = paid?.[i]
        let bg = on ? 'var(--green-bg)' : '#F4F3EE', fg = on ? 'var(--green-dark)' : '#9AA39E', border = on ? '1px solid #CBE7C4' : '1px solid #E7E5DD'
        if (onToggle && on) { bg = 'var(--green)'; fg = '#fff'; border = '1px solid var(--green)' }
        if (st === 'paid' || st === 'paid_outside') { bg = 'var(--green)'; fg = '#fff'; border = '1px solid var(--green)' }
        if (st === 'past_due' || st === 'open') { bg = 'var(--sun)'; fg = '#3F3200'; border = '1px solid var(--sun)' }
        if (highlight === i) { bg = 'var(--green)'; fg = '#fff'; border = '1px solid var(--green)' }
        const box: CSSProperties = { flex: onToggle ? 1 : 'none', width: onToggle ? undefined : size, height: size, minWidth: 0, borderRadius: size > 30 ? 10 : 5, background: bg, color: fg, border, fontSize: size > 30 ? 14 : 10, fontWeight: 600, display: 'grid', placeItems: 'center', fontFamily: 'var(--mono)', padding: 0 }
        return onToggle
          ? <button key={i} type="button" style={box} onClick={() => onToggle(i)} aria-label={MONTH_SHORT[i]} aria-pressed={on}>{l}</button>
          : <span key={i} style={box} title={MONTH_SHORT[i]}>{l}</span>
      })}
    </div>
  )
}

export const money = (c: number) => fmt(c, false)

export function Stat({ label, value, unit = 'MT', sub, tone, children }: { label: string; value: ReactNode; unit?: string; sub?: ReactNode; tone?: string; children?: ReactNode }) {
  return (
    <div className="card card-pad" style={{ minWidth: 0 }}>
      <div style={{ fontSize: 13, color: 'var(--muted)' }}>{label}</div>
      <div className="display tnum" style={{ fontSize: 30, letterSpacing: '-0.03em', marginTop: 8 }}>{value}{unit && <span style={{ fontSize: 14, color: 'var(--muted)', marginLeft: 4, fontFamily: 'var(--sans)' }}>{unit}</span>}</div>
      {sub && <div style={{ fontSize: 12.5, color: tone || 'var(--muted)', marginTop: 6, fontWeight: tone ? 600 : 400 }}>{sub}</div>}
      {children}
    </div>
  )
}

export function Section({ title, right, children, pad = true }: { title: ReactNode; right?: ReactNode; children: ReactNode; pad?: boolean }) {
  return (
    <div className="card" style={{ overflow: 'hidden', minWidth: 0 }}>
      <div className="between" style={{ padding: '16px 20px', borderBottom: '1px solid var(--line-3)', gap: 12 }}>
        <div style={{ fontWeight: 700, fontSize: 15.5 }}>{title}</div>
        {right && <div style={{ fontSize: 13, color: 'var(--muted)' }}>{right}</div>}
      </div>
      <div style={pad ? { padding: '6px 20px 14px' } : undefined}>{children}</div>
    </div>
  )
}

export function KV({ k, v, last }: { k: ReactNode; v: ReactNode; last?: boolean }) {
  return (
    <div style={{ display: 'grid', gridTemplateColumns: 'minmax(110px, 200px) 1fr', gap: 16, alignItems: 'center', padding: '13px 0', borderBottom: last ? 0 : '1px solid var(--line-3)', fontSize: 14 }}>
      <div style={{ color: 'var(--muted)' }}>{k}</div><div style={{ minWidth: 0 }}>{v}</div>
    </div>
  )
}

export function Radio({ on, title, sub, onClick }: { on: boolean; title: string; sub?: ReactNode; onClick: () => void }) {
  return (
    <button type="button" onClick={onClick} style={{ display: 'flex', gap: 12, alignItems: 'flex-start', textAlign: 'left', padding: '14px 16px', borderRadius: 14, border: on ? '2px solid var(--ink)' : '1px solid #E1DFD7', background: '#fff', margin: on ? 0 : 1, width: '100%' }}>
      <span style={{ width: 20, height: 20, flex: 'none', borderRadius: '50%', border: on ? '6px solid var(--ink)' : '1.5px solid #C9C7BE', marginTop: 1 }} />
      <span><span style={{ display: 'block', fontWeight: 600, fontSize: 14.5 }}>{title}</span>{sub && <span style={{ display: 'block', fontSize: 13, color: 'var(--muted)', marginTop: 3, lineHeight: 1.45 }}>{sub}</span>}</span>
    </button>
  )
}

export function scheduleSentence(p: { anchor: string; billing_day: number; interval: string; months: number; amount: number }) {
  const per = p.interval === 'week' ? 'semana' : p.interval === 'quarter' ? 'trimestre' : p.interval === 'year' ? 'ano' : 'mês'
  const when = p.anchor === 'signup' ? 'no dia da subscrição' : p.interval === 'week' ? 'à ' + ['domingo', 'segunda', 'terça', 'quarta', 'quinta', 'sexta', 'sábado'][p.billing_day % 7] : `no dia ${p.billing_day}`
  return `${money(p.amount)} MT por ${per}, ${when}, ${monthsLabel(p.months)}`
}

export function monthsLabel(mask: number) {
  if ((mask & ALL_MONTHS) === ALL_MONTHS) return 'todos os meses'
  const parts: string[] = []
  for (let i = 0; i < 12;) {
    if (!monthOn(mask, i)) { i++; continue }
    let j = i
    while (j + 1 < 12 && monthOn(mask, j + 1)) j++
    parts.push(j === i ? MONTH_SHORT[i] : j === i + 1 ? `${MONTH_SHORT[i]}, ${MONTH_SHORT[j]}` : `${MONTH_SHORT[i]} a ${MONTH_SHORT[j]}`)
    i = j + 1
  }
  return parts.join(', ')
}

export const abbrevName = (n: string) => { const p = n.trim().split(/\s+/); return p.length > 1 ? `${p[0]} ${p[p.length - 1][0]}.` : n }

export function downloadSvg(selector: string, name: string) {
  const svg = document.querySelector(selector + ' svg') as SVGSVGElement | null
  if (!svg) return
  const blob = new Blob([new XMLSerializer().serializeToString(svg)], { type: 'image/svg+xml' })
  const a = document.createElement('a'); a.href = URL.createObjectURL(blob); a.download = name; a.click(); URL.revokeObjectURL(a.href)
}

export function nextOnDue(p: { anchor: string; billing_day: number; months: number; interval: string }, from: Date): Date | null {
  if (p.anchor !== 'fixed' || p.interval !== 'month') return null
  for (let k = 0; k < 24; k++) {
    const d = new Date(from.getFullYear(), from.getMonth() + k, p.billing_day)
    if (d >= from && monthOn(p.months, d.getMonth())) return d
  }
  return null
}

export function agreementPreview(merchant: string, p: { anchor: string; billing_day: number; interval: string; months: number; amount: number; ends_at?: string | null }) {
  const per = p.interval === 'week' ? 'semana' : p.interval === 'quarter' ? 'trimestre' : p.interval === 'year' ? 'ano' : 'mês'
  const when = p.anchor === 'signup' ? (p.interval === 'week' ? 'no mesmo dia de cada semana' : `no dia em que subscrevo, em cada ${per}`)
    : p.interval === 'week' ? 'à ' + ['domingo', 'segunda', 'terça', 'quarta', 'quinta', 'sexta', 'sábado'][p.billing_day % 7] + ' de cada semana'
    : p.interval === 'month' ? `no dia ${p.billing_day} de cada mês` : `no dia ${p.billing_day}, em cada ${per}`
  const months = (p.months & ALL_MONTHS) === ALL_MONTHS ? '' : `, nos meses ${monthsLabel(p.months)}`
  return `Aceito pagar ${money(p.amount)} MT à ${merchant} ${when}${months}. Recebo o link de pagamento por SMS antes de cada data.`
}

// The payment schedule, as the server computes it (domain/subscriptions.go), so
// a plan being edited can be previewed as a customer joining today would see it.
const MONTH_NAMES = ['janeiro', 'fevereiro', 'março', 'abril', 'maio', 'junho', 'julho', 'agosto', 'setembro', 'outubro', 'novembro', 'dezembro']
const WEEKDAY_NAMES = ['domingo', 'segunda', 'terça', 'quarta', 'quinta', 'sexta', 'sábado']
export type PlanShape = { interval: string; anchor: string; billing_day: number; months: number; cycles?: number; ends_at?: string | null; amount: number }
const dayIn = (y: number, m: number, d: number) => { const last = new Date(y, m + 1, 0).getDate(); return new Date(y, m, Math.min(d, last)) }
const perMonths = (i: string) => (i === 'quarter' ? 3 : i === 'year' ? 12 : 1)
const dueAt = (p: PlanShape, a: Date, k: number) => (p.interval === 'week' ? new Date(a.getFullYear(), a.getMonth(), a.getDate() + 7 * k) : dayIn(a.getFullYear(), a.getMonth() + k * perMonths(p.interval), a.getDate()))
const endOf = (p: PlanShape) => (p.ends_at ? new Date(p.ends_at.slice(0, 10) + 'T23:59:59') : null)
const inRange = (p: PlanShape, d: Date) => monthOn(p.months, d.getMonth()) && (!endOf(p) || d <= endOf(p)!)

export function firstDue(p: PlanShape, today: Date): Date | null {
  const t = new Date(today.getFullYear(), today.getMonth(), today.getDate())
  let start = t
  if (p.anchor !== 'signup') {
    if (p.interval === 'week') { const wd = t.getDay() || 7; start = new Date(t.getFullYear(), t.getMonth(), t.getDate() - ((wd - p.billing_day + 7) % 7)) }
    else start = dayIn(t.getFullYear(), t.getMonth(), p.billing_day)
  }
  for (let k = 0; k < 120; k++) {
    const d = dueAt(p, start, k)
    if (endOf(p) && d > endOf(p)!) return null
    if (inRange(p, d)) return d
  }
  return null
}

export function schedule(p: PlanShape, first: Date, n: number): Date[] {
  const out: Date[] = []
  let count = 0
  for (let k = 0; k < 600 && out.length < n; k++) {
    const d = dueAt(p, first, k)
    if (endOf(p) && d > endOf(p)!) break
    if (!inRange(p, d)) continue
    count++
    if (p.cycles && count > p.cycles) break
    out.push(d)
  }
  return out
}

export function customerWhen(p: PlanShape, join: Date) {
  const day = p.anchor === 'signup' ? join.getDate() : p.billing_day
  const wd = p.anchor === 'signup' ? join.getDay() : p.billing_day % 7
  if (p.interval === 'week') return 'Todas as semanas, à ' + WEEKDAY_NAMES[wd]
  if (p.interval === 'quarter') return `Dia ${day}, de 3 em 3 meses`
  if (p.interval === 'year') return p.anchor === 'signup' ? `Todos os anos, a ${day} de ${MONTH_NAMES[join.getMonth()]}` : `Todos os anos, no dia ${day}`
  return `Todos os meses, no dia ${day}`
}

export function durationLabel(p: { cycles?: number; ends_at?: string | null }) {
  if (p.cycles === 1) return '1 pagamento'
  if (p.cycles && p.cycles > 1) return `${p.cycles} pagamentos`
  if (p.ends_at) { const d = new Date(p.ends_at.slice(0, 10) + 'T12:00:00'); return `Até ${d.getDate()} de ${MONTH_NAMES[d.getMonth()]} de ${d.getFullYear()}` }
  return ''
}

export const shortDate = (d: Date) => `${d.getDate()} ${MONTH_SHORT[d.getMonth()]}`

function periodLabel(p: PlanShape, d: Date) {
  if (p.interval === 'week') return 'semana de ' + shortDate(d)
  if (p.interval === 'quarter') { const e = new Date(d.getFullYear(), d.getMonth() + 2, 1); return `${MONTH_SHORT[d.getMonth()]} ${d.getFullYear()} a ${MONTH_SHORT[e.getMonth()]} ${e.getFullYear()}` }
  if (p.interval === 'year') return `ano de ${MONTH_SHORT[d.getMonth()]} ${d.getFullYear()}`
  return `${MONTH_NAMES[d.getMonth()]} ${d.getFullYear()}`
}

// customerView is the same shape the server sends to the public pages.
export function customerView(p: PlanShape, today = new Date()) {
  const first = firstDue(p, today)
  const base = { when_text: '', dates: [] as { label: string; period: string }[], duration_label: durationLabel(p), first_today: true, first_period: '', months_off: (p.months & ALL_MONTHS) === ALL_MONTHS ? '' : monthsLabel(ALL_MONTHS & ~p.months) }
  if (!first) return base
  const t = new Date(today.getFullYear(), today.getMonth(), today.getDate())
  const n = p.cycles && p.cycles <= 6 ? p.cycles : 4
  return { ...base, when_text: customerWhen(p, first), first_today: first <= t, first_period: periodLabel(p, first),
    dates: schedule(p, first, n).map((d, i) => ({ label: i === 0 && d <= t ? 'Hoje' : shortDate(d), period: periodLabel(p, d) })) }
}

// How a plan charges, in the merchant's words: a plan that follows each
// member's join day has no single date.
export function merchantWhen(p: { anchor: string; interval: string; billing_day: number; months: number }) {
  const when = p.anchor === 'signup' ? `No dia em que cada um entra${p.interval === 'month' ? '' : `, por ${perWord(p.interval)}`}` : customerWhen({ ...p, amount: 0 }, new Date())
  return (p.months & ALL_MONTHS) === ALL_MONTHS ? when : `${when} · ${monthsLabel(p.months)}`
}


const perWord = (i: string) => (i === 'week' ? 'semana' : i === 'quarter' ? 'trimestre' : i === 'year' ? 'ano' : 'mês')
