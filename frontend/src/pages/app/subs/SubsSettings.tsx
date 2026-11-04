import { useEffect, useState } from 'react'
import { Skeleton, Spinner, Toggle } from '../../../components/ui'
import { useGateway } from '../../../core/gateway'
import { errMsg } from '../../../core/model'
import { useQuery } from '../../../core/useQuery'
import { useSession } from '../../../lib/session'
import { useSubs } from './SubsShell'

const FINAL: [string, string][] = [['suspend', 'Suspender'], ['cancel', 'Cancelar'], ['keep', 'Manter ativo, fica em dívida']]
const when = (d: number, h: number) => (d === 0 ? `No dia · ${String(h).padStart(2, '0')}:00` : d < 0 ? `${-d} dias antes` : `+${d} dias`)

export default function SubsSettings() {
  const g = useGateway()
  const { toast } = useSession()
  const sx = useSubs()
  const { data, reload } = useQuery(g => g.subscriptions.settings(), [sx.version])
  const [s, setS] = useState<any>(null)
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')
  useEffect(() => { if (data) setS({ ...data, disabled: data.steps.filter((x: any) => !x.on).map((x: any) => x.key) }) }, [data])
  if (!s) return <Skeleton h={400} r={22} />
  const steps = (data.steps as any[]).map(st => {
    if (st.key === 'last') return { ...st, days: Number(s.tolerance_days) - 3 }
    if (st.key === 'final') return { ...st, days: Number(s.tolerance_days) }
    return st
  })
  const dirty = JSON.stringify([s.disabled.slice().sort(), s.channels, s.sender_id, Number(s.tolerance_days), s.final_action, s.require_proof, s.receipt_sms]) !== JSON.stringify([data.steps.filter((x: any) => !x.on).map((x: any) => x.key).sort(), data.channels, data.sender_id, data.tolerance_days, data.final_action, data.require_proof, data.receipt_sms])
  const save = async () => {
    setBusy(true); setErr('')
    try { await g.subscriptions.saveSettings({ disabled_steps: s.disabled, channels: s.channels, sender_id: s.sender_id, tolerance_days: Number(s.tolerance_days), final_action: s.final_action, require_proof: s.require_proof, receipt_sms: s.receipt_sms }); toast('Definições guardadas'); reload() }
    catch (e) { setErr(errMsg(e)) } finally { setBusy(false) }
  }
  const toggleCh = (c: string) => setS({ ...s, channels: s.channels.includes(c) ? s.channels.filter((x: string) => x !== c) : [...s.channels, c] })
  return (
    <div>
      <div style={{ display: 'grid', gridTemplateColumns: 'minmax(0,1.2fr) minmax(0,1fr)', gap: 14, alignItems: 'start' }} className="ss-grid">
        <style>{`@media (max-width:1000px){.ss-grid{grid-template-columns:1fr!important}}`}</style>
        <div className="card" style={{ overflow: 'hidden' }}>
          <div style={{ padding: '18px 20px 6px' }}><b style={{ fontSize: 15.5 }}>Lembretes e atrasos</b><div style={{ fontSize: 13, color: 'var(--muted)', marginTop: 3 }}>Padrão para todos os planos. Todas as mensagens levam o mesmo link. Paramos assim que o cliente paga.</div></div>
          {steps.map(st => (
            <div key={st.key} style={{ display: 'grid', gridTemplateColumns: '100px 1fr auto', gap: 12, alignItems: 'center', padding: '12px 20px' }}>
              <span className="mono" style={{ fontSize: 12.5 }}>{when(st.days, st.hour)}</span>
              <div><div style={{ fontWeight: 600, fontSize: 14 }}>{st.title}</div><div style={{ fontSize: 12.5, color: 'var(--muted)' }}>{st.key === 'final' ? 'Ver abaixo' : st.hint}</div></div>
              <span style={{ opacity: st.locked ? 0.5 : 1 }}><Toggle on={st.locked || !s.disabled.includes(st.key)} label={st.title} onChange={v => !st.locked && setS({ ...s, disabled: v ? s.disabled.filter((k: string) => k !== st.key) : [...s.disabled, st.key] })} /></span>
            </div>
          ))}
          <div className="grid-2" style={{ gap: 14, padding: '16px 20px', borderTop: '1px solid var(--line-3)', background: '#FBFBF8' }}>
            <label><span style={{ fontSize: 12.5, color: 'var(--muted)' }}>Tolerância</span><div style={{ display: 'flex', alignItems: 'center', gap: 8, marginTop: 4 }}><input className="input" type="number" min={6} max={30} value={s.tolerance_days} onChange={e => setS({ ...s, tolerance_days: e.target.value })} style={{ width: 80, height: 38 }} /><span style={{ fontSize: 14 }}>dias com acesso</span></div></label>
            <label><span style={{ fontSize: 12.5, color: 'var(--muted)' }}>Se não pagar até ao fim da tolerância</span><select className="input" value={s.final_action} onChange={e => setS({ ...s, final_action: e.target.value })} style={{ height: 38, marginTop: 4 }}>{FINAL.map(([k, l]) => <option key={k} value={k}>{l}</option>)}</select></label>
          </div>
          <div style={{ padding: '0 20px 16px', background: '#FBFBF8', fontSize: 12.5, color: 'var(--muted)', lineHeight: 1.5 }}>A cobrança passa a Em dívida 2 dias depois do vencimento. O último aviso sai 3 dias antes do fim da tolerância e a ação final corre no último dia. Nos planos com “Perde esse mês”, o mês fecha sem dívida em vez da ação final.</div>
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
          <div className="card" style={{ overflow: 'hidden' }}>
            <div style={{ padding: '16px 20px', borderBottom: '1px solid var(--line-3)', fontWeight: 700 }}>Mensagens</div>
            <div className="between" style={{ padding: '12px 20px', borderBottom: '1px solid var(--line-3)', gap: 12 }}><span style={{ color: 'var(--muted)', fontSize: 14 }}>Remetente do SMS</span><input className="input mono" value={s.sender_id} maxLength={11} onChange={e => setS({ ...s, sender_id: e.target.value.replace(/[^A-Za-z0-9]/g, '') })} style={{ width: 150, height: 34, textAlign: 'right' }} /></div>
            <div className="between" style={{ padding: '12px 20px', borderBottom: '1px solid var(--line-3)', gap: 12 }}><span style={{ color: 'var(--muted)', fontSize: 14 }}>Canais</span><span style={{ display: 'flex', gap: 6 }}>{[['sms', 'SMS'], ['whatsapp', 'WhatsApp'], ['email', 'Email']].map(([k, l]) => <button key={k} className={'btn btn-sm ' + (s.channels.includes(k) ? 'btn-ink' : 'btn-outline')} style={{ borderRadius: 999 }} onClick={() => toggleCh(k)}>{l}</button>)}</span></div>
            <div className="between" style={{ padding: '12px 20px', gap: 12 }}><span style={{ color: 'var(--muted)', fontSize: 14 }}>Horário</span><span style={{ fontSize: 14 }}>08:00 a 20:00 · sem domingos</span></div>
          </div>
          <div className="card" style={{ overflow: 'hidden' }}>
            <div style={{ padding: '16px 20px', borderBottom: '1px solid var(--line-3)' }}><b>Pagamentos fora da RIHA</b><div style={{ fontSize: 12.5, color: 'var(--muted)', marginTop: 3 }}>Fecham a cobrança. Não entram no saldo, nos levantamentos nem nas taxas.</div></div>
            <div className="between" style={{ padding: '12px 20px', borderBottom: '1px solid var(--line-3)' }}><span style={{ fontSize: 14 }}>Exigir comprovativo</span><Toggle on={s.require_proof} label="Exigir comprovativo" onChange={v => setS({ ...s, require_proof: v })} /></div>
            <div className="between" style={{ padding: '12px 20px', borderBottom: '1px solid var(--line-3)' }}><span style={{ fontSize: 14 }}>Enviar recibo ao cliente por SMS</span><Toggle on={s.receipt_sms} label="Enviar recibo" onChange={v => setS({ ...s, receipt_sms: v })} /></div>
            <div className="between" style={{ padding: '12px 20px' }}><span style={{ fontSize: 14 }}>Quem pode marcar</span><span style={{ fontSize: 14, fontWeight: 600 }}>Dono da conta</span></div>
          </div>
        </div>
      </div>
      {err && <div className="form-err" style={{ marginTop: 14 }}>{err}</div>}
      {dirty && <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: 14 }}><button className="btn btn-green" disabled={busy} onClick={save}>{busy ? <Spinner /> : 'Guardar definições'}</button></div>}
    </div>
  )
}
