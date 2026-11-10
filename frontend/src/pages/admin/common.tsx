import { Pill } from '../../components/ui'

export const RISK: Record<string, [string, string]> = { Baixo: ['#E2F3DE', '#0A6B05'], 'Médio': ['#FFF3C4', '#7A5A00'], Alto: ['#FFE3E1', '#C2080D'] }
export const RiskPill = ({ risk }: { risk: string }) => <Pill t={'Risco ' + risk.toLowerCase()} bg={RISK[risk][0]} fg={RISK[risk][1]} />

export function sla(p: { status: string; due_at_iso: string; requested_at_iso: string }) {
  const total = 72 * 3600e3
  const elapsed = Date.now() - new Date(p.requested_at_iso).getTime()
  const leftH = Math.floor((new Date(p.due_at_iso).getTime() - Date.now()) / 3600e3)
  const done = ['paid', 'sent', 'rejected', 'failed'].includes(p.status)
  const c = done ? '#8A938E' : leftH <= 8 ? '#FF0F15' : leftH <= 24 ? '#E0A800' : '#0A8205'
  const fg = done ? '#5E6862' : leftH <= 8 ? '#C2080D' : leftH <= 24 ? '#7A5A00' : '#0A6B05'
  const label = done ? (p.status === 'paid' ? 'Concluído' : p.status === 'sent' ? 'No banco' : '—') : leftH < 0 ? `Atrasado ${-leftH} h` : `${leftH} h restantes`
  return { label, pct: Math.min(100, Math.max(0, Math.round((elapsed / total) * 100))) + '%', c, fg }
}

export function SlaBar({ p }: { p: any }) {
  const s = sla(p)
  return <div><div style={{ fontSize: 12, fontWeight: 600, color: s.fg }}>{s.label}</div><div style={{ height: 5, borderRadius: 3, background: '#EEECE5', marginTop: 5, overflow: 'hidden' }}><div style={{ width: s.pct, height: '100%', background: s.c }} /></div></div>
}

export const submittedAgo = (iso: string | null) => {
  if (!iso) return '—'
  const h = (Date.now() - new Date(iso).getTime()) / 3600e3
  if (h < 1) return 'agora'
  if (h < 24) return `há ${Math.floor(h)} h`
  const d = Math.floor(h / 24)
  return d === 1 ? 'há 1 dia' : `há ${d} dias`
}

// Marks an account brought over from the first RIHA.
export const V1Tag = ({ big }: { big?: boolean }) => (
  <span title="Conta trazida da primeira versão da RIHA" style={{ marginLeft: 8, verticalAlign: 'middle', fontFamily: 'var(--font-sans, inherit)', fontSize: big ? 12 : 10.5, fontWeight: 600, letterSpacing: 0.2, padding: big ? '3px 8px' : '1px 6px', borderRadius: 999, background: '#EDECE6', color: '#5B5A55' }}>v1</span>
)
