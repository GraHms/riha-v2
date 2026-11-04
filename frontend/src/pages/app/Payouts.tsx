import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, errMsg } from '../../lib/api'
import { useData } from '../../lib/useData'
import { fmt, monthName, parseMT, PAYOUT_STATUS, relDate, shortDate, weekday } from '../../lib/format'
import { useSession } from '../../lib/session'
import { Icon, Pill, Spinner, Stripes } from '../../components/ui'
import { useShell } from './AppShell'

const GRID = '100px minmax(0,1fr) 130px 130px 120px'

export default function Payouts() {
  const { version, bump, merchant, live } = useShell()
  const { toast } = useSession()
  const nav = useNavigate()
  const bal = useData<any>('/balance', [version], 10000)
  const list = useData<any>('/payouts', [version], 10000)
  const bank = useData<any>('/bank-account', [version])
  const [w, setW] = useState('')
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')
  const available = bal.data?.available ?? 0
  useEffect(() => { if (bal.data && w === '') setW(fmt(Math.max(0, bal.data.available)).replace(/\u00A0/g, ' ')) }, [bal.data]) // eslint-disable-line react-hooks/exhaustive-deps
  const amt = parseMT(w)
  const over = amt > available
  const acct = bank.data?.account
  const due = new Date(Date.now() + 72 * 3600 * 1000)
  const blocked = live && merchant.status !== 'active'
  const disabled = busy || !amt || over || amt < 10000 || !acct || blocked

  const request = async () => {
    setBusy(true); setErr('')
    try { const p = await api<any>('/payouts', { body: { amount: amt } }); toast(`Levantamento ${p.reference} pedido`); setW(''); bump() }
    catch (e) { setErr(errMsg(e)) } finally { setBusy(false) }
  }
  const now = new Date()
  const prevMonth = monthName((now.getMonth() + 11) % 12)
  const rows: any[] = list.data?.data || []

  return (
    <div className="page">
      <h1 className="h1">Saldo e levantamentos</h1>
      <div className="grid-3" style={{ marginTop: 22 }}>
        <div className="dark-card" style={{ borderRadius: 22, padding: 22 }}>
          <Stripes top={-30} right={4} w={28} h={200} gap={8} />
          <div style={{ position: 'relative', fontSize: 13.5, color: '#A9B5AE' }}>Disponível</div>
          <div className="display tnum" style={{ position: 'relative', fontSize: 34, letterSpacing: '-0.04em', marginTop: 6 }}>{fmt(available)} <span style={{ fontSize: 15, color: '#A9B5AE' }}>MT</span></div>
        </div>
        <div style={{ background: 'var(--sun-soft)', borderRadius: 22, padding: 22 }}>
          <div style={{ fontSize: 13.5, color: '#5B4600' }}>A caminho do banco</div>
          <div className="display tnum" style={{ fontSize: 34, letterSpacing: '-0.04em', marginTop: 6 }}>{fmt(bal.data?.in_transit)} <span style={{ fontSize: 15, color: '#7A6A2A' }}>MT</span></div>
          <div style={{ fontSize: 13, color: '#5B4600', marginTop: 4 }}>{bal.data?.in_transit_due ? `Chega até ${weekday(bal.data.in_transit_due).split('-')[0].toLowerCase()}, ${shortDate(bal.data.in_transit_due)}` : 'Nada em curso'}</div>
        </div>
        <div className="card" style={{ padding: 22 }}>
          <div style={{ fontSize: 13.5, color: 'var(--muted)' }}>Levantado em {prevMonth}</div>
          <div className="display tnum" style={{ fontSize: 34, letterSpacing: '-0.04em', marginTop: 6 }}>{fmt(bal.data?.paid_previous_month?.total)} <span style={{ fontSize: 15, color: 'var(--muted)' }}>MT</span></div>
          <div style={{ fontSize: 13, color: 'var(--muted)', marginTop: 4 }}>{bal.data?.paid_previous_month?.count ?? 0} levantamentos</div>
        </div>
      </div>

      <div className="payout-grid" style={{ marginTop: 16 }}>
        <style>{`.payout-grid{display:grid;grid-template-columns:minmax(0,1fr) 380px;gap:16px;align-items:start}@media(max-width:1100px){.payout-grid{grid-template-columns:1fr}.payout-grid>.req{order:-1}}`}</style>
        <div className="table">
          <div className="h2" style={{ padding: '18px 20px 12px' }}>Histórico</div>
          <div className="thead only-desktop" style={{ gridTemplateColumns: GRID, borderTop: '1px solid #EEECE5' }}><span>Ref.</span><span>Conta</span><span>Pedido</span><span>Estado</span><span style={{ textAlign: 'right' }}>Valor</span></div>
          {rows.map(p => (
            <div key={p.id}>
              <div className="trow only-desktop" style={{ gridTemplateColumns: GRID, fontSize: 13.5, padding: '13px 20px' }}>
                <span className="mono" style={{ fontSize: 12.5 }}>{p.reference}</span>
                <span style={{ color: 'var(--text-2)' }}>{p.bank_account.bank} · NIB {p.bank_account.nib}</span>
                <span style={{ color: 'var(--text-2)' }}>{relDate(p.requested_at_iso)}</span>
                <span><Pill {...PAYOUT_STATUS[p.status]} /></span>
                <span className="tnum" style={{ textAlign: 'right', fontWeight: 600 }}>{fmt(p.amount)} MT</span>
              </div>
              <div className="only-mobile" style={{ padding: '12px 16px', borderTop: '1px solid var(--line-3)' }}>
                <div className="between"><span className="mono" style={{ fontSize: 12.5 }}>{p.reference}</span><span className="tnum" style={{ fontWeight: 600 }}>{fmt(p.amount)} MT</span></div>
                <div className="between" style={{ marginTop: 4 }}><span style={{ fontSize: 12.5, color: 'var(--muted)' }}>{relDate(p.requested_at_iso)}</span><Pill {...PAYOUT_STATUS[p.status]} /></div>
                {p.failure_reason && <div style={{ fontSize: 12.5, color: 'var(--red-dark)', marginTop: 4 }}>{p.failure_reason}</div>}
              </div>
            </div>
          ))}
          {list.data && !rows.length && <div className="empty">Ainda não fez levantamentos.</div>}
        </div>

        <div className="card req" style={{ padding: 22 }}>
          <div className="h2">Pedir levantamento</div>
          <div style={{ marginTop: 16, padding: '14px 16px', borderRadius: 16, border: '2px solid var(--ink)' }}>
            <div style={{ fontSize: 12.5, color: 'var(--muted)' }}>Valor</div>
            <div style={{ display: 'flex', alignItems: 'baseline', gap: 6 }}>
              <input aria-label="Valor a levantar" value={w} onChange={e => setW(e.target.value.replace(/[^\d.,\s]/g, '').slice(0, 12))} inputMode="decimal" className="display" style={{ flex: 1, minWidth: 0, border: 0, background: 'transparent', fontSize: 34, letterSpacing: '-0.04em', padding: 0, color: over ? 'var(--red-dark)' : 'var(--ink)' }} />
              <span className="display" style={{ fontSize: 17, color: 'var(--muted-2)' }}>MT</span>
            </div>
          </div>
          <div className="between" style={{ marginTop: 8 }}><span style={{ fontSize: 12.5, color: over ? 'var(--red-dark)' : 'var(--muted)' }}>{over ? 'Acima do saldo disponível' : `Disponível: ${fmt(available)} MT`}</span><button className="btn" style={{ height: 30, padding: '0 11px', borderRadius: 999, background: 'var(--paper-2)', fontSize: 12.5 }} onClick={() => setW(fmt(available).replace(/\u00A0/g, ' '))}>Máximo</button></div>
          {acct ? (
            <div className="row" style={{ marginTop: 16, padding: '12px 14px', borderRadius: 14, background: 'var(--paper)' }}>
              <div style={{ width: 38, height: 38, flex: 'none', borderRadius: 11, background: '#fff', display: 'grid', placeItems: 'center' }}><Icon name="bank" size={18} /></div>
              <div style={{ flex: 1, minWidth: 0 }}><div style={{ fontWeight: 600, fontSize: 14 }}>{acct.bank_name} · {acct.holder}</div><div className="mono" style={{ fontSize: 12, color: 'var(--muted)' }}>NIB •••• {acct.nib_last5.slice(0, 4)} {acct.nib_last5.slice(4)}</div></div>
              <a href="#" onClick={e => { e.preventDefault(); nav('/painel/definicoes/banco') }} style={{ fontSize: 13, fontWeight: 600 }}>Alterar</a>
            </div>
          ) : (
            <button className="btn btn-outline btn-block" style={{ marginTop: 16 }} onClick={() => nav('/painel/definicoes/banco')}><Icon name="bank" size={17} />Adicionar conta bancária</button>
          )}
          <div style={{ display: 'flex', gap: 10, marginTop: 14, fontSize: 13, lineHeight: 1.45, color: 'var(--text-2)' }}><Icon name="clock" size={17} stroke={2} style={{ marginTop: 1 }} />Processado em até 72 horas. Se pedir hoje, chega até {weekday(due).toLowerCase()}, {shortDate(due)}.</div>
          {blocked && <div className="form-err" style={{ marginTop: 12, background: 'var(--sun-soft)', color: '#5B4600' }}>Os levantamentos em produção ficam disponíveis depois da verificação da conta.</div>}
          {err && <div className="form-err" style={{ marginTop: 12 }}>{err}</div>}
          <button className="btn btn-green btn-block" style={{ marginTop: 16, height: 52, borderRadius: 14, fontSize: 15.5 }} disabled={disabled} onClick={request}>{busy ? <Spinner /> : `Levantar ${fmt(amt)} MT`}</button>
          {!live && <div style={{ fontSize: 12, color: 'var(--muted)', marginTop: 10, textAlign: 'center' }}>Em modo de teste os levantamentos são concluídos automaticamente em segundos.</div>}
        </div>
      </div>
    </div>
  )
}
