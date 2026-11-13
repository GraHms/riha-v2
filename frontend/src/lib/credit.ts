// The same arithmetic as the server's domain/microcredit.go, so screens can
// show a plan while the person changes the numbers. The server always recomputes.

export type Freq = 'semanal' | 'quinzenal' | 'mensal'

const MONTHS = ['jan', 'fev', 'mar', 'abr', 'mai', 'jun', 'jul', 'ago', 'set', 'out', 'nov', 'dez']
const UNITS: Record<Freq, [string, string, string, string, string]> = {
  semanal: ['semana', 'semanas', 'por semana', 'semanais', 'sem'],
  quinzenal: ['quinzena', 'quinzenas', 'por quinzena', 'quinzenais', 'quinz'],
  mensal: ['mês', 'meses', 'ao mês', 'mensais', 'mês'],
}
export const TERM_MAX: Record<Freq, number> = { semanal: 52, quinzenal: 26, mensal: 36 }
export const FREQ_DEFAULTS: Record<Freq, [number, number]> = { semanal: [4, 12], quinzenal: [2, 12], mensal: [1, 12] }
export const FREQ_LABEL: Record<Freq, string> = { semanal: 'Semanal', quinzenal: 'Quinzenal', mensal: 'Mensal' }

export const mt = (cents: number) => Math.round(cents / 100).toString().replace(/\B(?=(\d{3})+(?!\d))/g, ' ')
export const unit = (f: Freq, n: number) => `${n} ${n === 1 ? UNITS[f][0] : UNITS[f][1]}`
export const perLabel = (f: Freq) => UNITS[f][2]
export const perShort = (f: Freq) => UNITS[f][4]
export const adjPlural = (f: Freq, n: number) => (n === 1 ? (f === 'mensal' ? 'mensal' : f === 'semanal' ? 'semanal' : 'quinzenal') : UNITS[f][3])
export const prest = (n: number) => `${n} ${n === 1 ? 'prestação' : 'prestações'}`
export const days = (n: number) => `${n} ${n === 1 ? 'dia' : 'dias'}`
export const pct = (bp: number) => (bp / 100).toString().replace('.', ',')
export const rateLabel = (bp: number, f: Freq) => `${pct(bp)}% ${perLabel(f)}`
export const dl = (d: Date) => `${d.getDate()} ${MONTHS[d.getMonth()]}`
export const dly = (d: Date) => `${dl(d)} ${d.getFullYear()}`

// Dates are shown in Maputo time, which is the browser's for nearly everyone using RIHA.
export const asDate = (iso: string) => new Date(iso)

export function instalment(amount: number, n: number, rateBP: number) {
  if (n <= 0) return 0
  const mtv = Math.floor(amount / 100)
  const num = mtv * (10000 + rateBP * n)
  const den = 10000 * n
  return Math.ceil(num / den) * 100
}

const PPM: Record<Freq, number> = { semanal: 52 / 12, quinzenal: 26 / 12, mensal: 1 }
export const effort = (inst: number, f: Freq, income: number) => (income > 0 ? Math.round((inst * PPM[f]) / income * 100) : 0)

const clampDay = (d: number) => Math.max(1, Math.min(28, d))

export function dueDates(f: Freq, day: number, n: number, start = new Date()) {
  const base = new Date(start.getFullYear(), start.getMonth(), start.getDate(), 9)
  const out: Date[] = []
  if (f !== 'mensal') {
    const step = f === 'semanal' ? 7 : 14
    for (let i = 0; i < n; i++) out.push(new Date(base.getFullYear(), base.getMonth(), base.getDate() + step * (i + 1), 9))
    return out
  }
  const d = day >= 1 && day <= 28 ? day : clampDay(start.getDate())
  let first = new Date(start.getFullYear(), start.getMonth() + 1, d, 9)
  if (first.getTime() - base.getTime() < 7 * 864e5) first = new Date(first.getFullYear(), first.getMonth() + 1, d, 9)
  for (let i = 0; i < n; i++) out.push(new Date(first.getFullYear(), first.getMonth() + i, d, 9))
  return out
}

export function nextDates(day: number, n: number, from = new Date()) {
  let first = new Date(from.getFullYear(), from.getMonth(), clampDay(day), 9)
  while (first.getTime() - from.getTime() < 48 * 3600e3) first = new Date(first.getFullYear(), first.getMonth() + 1, clampDay(day), 9)
  return Array.from({ length: n }, (_, i) => new Date(first.getFullYear(), first.getMonth() + i, clampDay(day), 9))
}

export function evenSplit(total: number, n: number) {
  if (n <= 0) return []
  const m = Math.floor(total / 100)
  const a = Math.ceil(m / n)
  return Array.from({ length: n }, (_, i) => (i === n - 1 ? (m - a * (n - 1)) * 100 : a * 100))
}

export type Product = {
  id: string; name: string; min: number; max: number; freq: Freq; term_min: number; term_max: number; rate_bp: number; day_mode: string
  client_day: boolean; mora: boolean; mora_bp: number; grace: number; mora_max_weeks: number; effort_pct: number; fields: string[]; status?: string
  term_label?: string; rate_label?: string; loans?: number; book?: number
}

export function quote(p: Pick<Product, 'freq' | 'rate_bp'>, amount: number, n: number, day: number, start = new Date()) {
  const inst = instalment(amount, n, p.rate_bp)
  const dates = dueDates(p.freq, day, n, start)
  return { inst, total: inst * n, interest: inst * n - amount, dates }
}

export const whenText = (f: Freq, day: number) => (f === 'mensal' ? `no dia ${day} de cada mês` : f === 'semanal' ? 'uma por semana' : 'uma a cada 2 semanas')

export function spanText(dates: Date[]) {
  if (!dates.length) return ''
  return dates.length === 1 ? `a ${dly(dates[0])}` : `de ${dl(dates[0])} a ${dly(dates[dates.length - 1])}`
}

export function moraText(p: { mora_bp: number; grace: number; mora_max_weeks?: number; mora_max?: number }) {
  const max = p.mora_max_weeks ?? p.mora_max ?? 0
  if (!p.mora_bp) return ''
  return `${p.grace ? `Depois de ${days(p.grace)}, m` : 'M'}ora de ${pct(p.mora_bp)}% por semana sobre a prestação em atraso, até ${unit('semanal', max)}.`
}

export function consent(merchant: string, p: Product, amount: number, n: number, day: number) {
  const q = quote(p, amount, n, day)
  const mora = p.mora ? ` Se atrasar${p.grace ? ' mais de ' + days(p.grace) : ''}, pago mora de ${pct(p.mora_bp)}% por semana sobre a prestação em atraso, até ${unit('semanal', p.mora_max_weeks)}.` : ''
  return `Aceito pagar ${prest(n)} de ${mt(q.inst)} MT à ${merchant}, ${whenText(p.freq, q.dates[0]?.getDate() ?? day)}, ${spanText(q.dates)}. Recebo o link de pagamento por SMS antes de cada data.${mora}`
}

// Rows of a plan: all of them up to six, otherwise the first three, a summary, and the last.
export function planRows<T>(rows: T[], summary: (hidden: number) => T) {
  return rows.length <= 6 ? rows : [...rows.slice(0, 3), summary(rows.length - 4), rows[rows.length - 1]]
}

export const FIELD_LABEL: Record<string, string> = { id_doc: 'BI, frente e verso', activity: 'Atividade', income: 'Rendimento mensal', purpose: 'Para quê' }

export const toneColors: Record<string, [string, string]> = {
  ok: ['#E2F3DE', '#0A6B05'], late: ['#FFF1C2', '#6B5200'], bad: ['#FDE7E6', '#C2080D'], neu: ['#E6EEFB', '#2F4E86'], wait: ['#EEEDE8', '#3E4742'],
}

export const digitsMT = (v: string) => {
  const d = v.replace(/\D/g, '').replace(/^0+(?=\d)/, '').slice(0, 7)
  return d ? Number(d).toString().replace(/\B(?=(\d{3})+(?!\d))/g, ' ') : ''
}
export const parseMT = (v: string) => Number(String(v).replace(/\D/g, '')) * 100
