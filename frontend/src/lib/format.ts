const NB = '\u00A0'

export function fmt(c: number | null | undefined, decimals = true): string {
  const v = Math.abs(c || 0)
  const i = Math.floor(v / 100).toString().replace(/\B(?=(\d{3})+(?!\d))/g, NB)
  const s = decimals ? `${i},${String(v % 100).padStart(2, '0')}` : i
  return (c || 0) < 0 ? '−' + s : s
}

export function parseMT(v: string): number {
  const n = parseFloat(String(v).replace(/[\s\u00A0]/g, '').replace(',', '.'))
  return isFinite(n) ? Math.round(n * 100) : 0
}

export const bpsLabel = (bps: number) => (bps % 100 === 0 ? `${bps / 100}%` : `${(bps / 100).toFixed(2).replace(/0+$/, '').replace('.', ',')}%`)

const MONTHS = ['jan', 'fev', 'mar', 'abr', 'mai', 'jun', 'jul', 'ago', 'set', 'out', 'nov', 'dez']
const WEEKDAYS = ['Domingo', 'Segunda-feira', 'Terça-feira', 'Quarta-feira', 'Quinta-feira', 'Sexta-feira', 'Sábado']
const MONTHS_LONG = ['janeiro', 'fevereiro', 'março', 'abril', 'maio', 'junho', 'julho', 'agosto', 'setembro', 'outubro', 'novembro', 'dezembro']

const toDate = (d: string | number | Date) => (typeof d === 'number' ? new Date(d * 1000) : new Date(d))
const hm = (d: Date) => `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`

export const shortDate = (d: string | number | Date) => { const x = toDate(d); return `${x.getDate()} ${MONTHS[x.getMonth()]}` }
export const fullDate = (d: string | number | Date) => { const x = toDate(d); return `${x.getDate()} ${MONTHS[x.getMonth()]} ${x.getFullYear()}, ${hm(x)}` }
export const longToday = () => { const d = new Date(); return `${WEEKDAYS[d.getDay()]}, ${d.getDate()} de ${MONTHS_LONG[d.getMonth()]}` }
export const monthName = (i: number) => MONTHS_LONG[i]
export const weekday = (d: string | number | Date) => WEEKDAYS[toDate(d).getDay()]

export function relDate(d: string | number | Date): string {
  const x = toDate(d)
  const now = new Date()
  const day = (a: Date) => new Date(a.getFullYear(), a.getMonth(), a.getDate()).getTime()
  const diff = (day(now) - day(x)) / 86400000
  if (diff === 0) return `Hoje, ${hm(x)}`
  if (diff === 1) return `Ontem, ${hm(x)}`
  return `${shortDate(x)}, ${hm(x)}`
}

export function ago(d: string | number | Date | null | undefined): string {
  if (!d) return 'nunca'
  const s = (Date.now() - toDate(d).getTime()) / 1000
  if (s < 60) return 'agora'
  if (s < 3600) return `há ${Math.floor(s / 60)} min`
  if (s < 86400) return `há ${Math.floor(s / 3600)} h`
  if (s < 172800) return 'ontem'
  return shortDate(d)
}

export function greeting() {
  const h = new Date().getHours()
  return h < 12 ? 'Bom dia' : h < 19 ? 'Boa tarde' : 'Boa noite'
}

export function initials(name: string) {
  const parts = name.replace(/,.*$/, '').split(/\s+/).filter(w => /^[A-ZÀ-Ú0-9]/.test(w))
  return (parts.slice(0, 2).map(w => w[0]).join('') || name.slice(0, 1)).toUpperCase()
}

export const METHOD_NAME: Record<string, string> = { mpesa: 'M-Pesa', emola: 'e-Mola', card: 'Cartão' }

export function methodLine(p: { method: string; phone_prefix?: string; phone_last4?: string; card_brand?: string; card_last4?: string }) {
  if (p.method === 'card') return `${p.card_brand || 'Cartão'} · •• ${p.card_last4 || ''}`
  return `${METHOD_NAME[p.method]} · ${p.phone_prefix || '8•'} ••• ${p.phone_last4 || ''}`
}

export const STATUS: Record<string, { t: string; bg: string; fg: string }> = {
  succeeded: { t: 'Pago', bg: '#E2F3DE', fg: '#0A6B05' },
  processing: { t: 'Pendente', bg: '#FFF3C4', fg: '#7A5A00' },
  failed: { t: 'Falhado', bg: '#FFE3E1', fg: '#C2080D' },
  refunded: { t: 'Reembolsado', bg: '#EDECE6', fg: '#3E4742' },
  partially_refunded: { t: 'Reemb. parcial', bg: '#EDECE6', fg: '#3E4742' },
}

export const SOURCE: Record<string, { t: string; bg: string; fg: string }> = {
  api_checkout: { t: 'API', bg: '#0B120E', fg: '#FFFFFF' },
  payment_link: { t: 'Link', bg: '#E2F3DE', fg: '#0A6B05' },
  qr: { t: 'QR', bg: '#FFF3C4', fg: '#7A5A00' },
  dashboard: { t: 'Painel', bg: '#EDECE6', fg: '#3E4742' },
}

export const PAYOUT_STATUS: Record<string, { t: string; bg: string; fg: string }> = {
  new: { t: 'Novo', bg: '#EDECE6', fg: '#3E4742' },
  review: { t: 'Em análise', bg: '#FFF3C4', fg: '#7A5A00' },
  sent: { t: 'Enviado ao banco', bg: '#0B120E', fg: '#FFFFFF' },
  paid: { t: 'Transferido', bg: '#E2F3DE', fg: '#0A6B05' },
  rejected: { t: 'Rejeitado', bg: '#FFE3E1', fg: '#C2080D' },
  failed: { t: 'Falhado', bg: '#FFE3E1', fg: '#C2080D' },
}

export const MERCHANT_STATUS: Record<string, { t: string; bg: string; fg: string }> = {
  lead: { t: 'Lead', bg: '#EDECE6', fg: '#3E4742' },
  in_review: { t: 'Em verificação', bg: '#FFF3C4', fg: '#7A5A00' },
  info_requested: { t: 'Docs pedidos', bg: '#EDECE6', fg: '#3E4742' },
  active: { t: 'Ativo', bg: '#E2F3DE', fg: '#0A6B05' },
  rejected: { t: 'Rejeitado', bg: '#FFE3E1', fg: '#C2080D' },
  suspended: { t: 'Suspenso', bg: '#FFE3E1', fg: '#C2080D' },
}

export async function copy(text: string) {
  try { await navigator.clipboard.writeText(text); return true } catch {
    const t = document.createElement('textarea'); t.value = text; document.body.appendChild(t); t.select()
    try { document.execCommand('copy'); return true } finally { t.remove() }
  }
}
