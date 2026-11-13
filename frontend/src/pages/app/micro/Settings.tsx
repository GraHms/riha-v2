import { useEffect, useState } from 'react'
import { Skeleton, Spinner, Toggle } from '../../../components/ui'
import { useGateway } from '../../../core/gateway'
import { errMsg } from '../../../core/model'
import { digitsMT, days, mt, parseMT } from '../../../lib/credit'
import { useSession } from '../../../lib/session'
import { useMicro } from './common'

const STEPS: Record<string, [string, string, boolean]> = {
  remind: ['Lembrete', 'Valor e data, para ter saldo', false],
  link: ['Link de pagamento', 'Sempre enviado', true],
  evening: ['Lembrete', 'Só se não abriu o link', false],
  resend1: ['Reenvio', 'Passa a Em atraso. A mora começa a contar.', false],
  resend2: ['Reenvio', 'Segundo aviso. A instituição vê-o em Precisa de atenção.', false],
  proposal: ['Proposta de acordo', 'O cliente pode pedir um novo acordo no link', false],
  final: ['Ação final', 'Ver abaixo', true],
}

export default function Settings() {
  const g = useGateway()
  const mc = useMicro()
  const { toast } = useSession()
  const [d, setD] = useState<any>(null)
  const [s, setS] = useState<any>(null)
  const [limit, setLimit] = useState('')
  const [two, setTwo] = useState('')
  const [busy, setBusy] = useState(false)
  useEffect(() => { g.microcredit.settings().then(x => { setD(x); setS(x.settings); setLimit(digitsMT(String(x.settings.delivery_limit / 100))); setTwo(digitsMT(String(x.settings.two_above / 100))) }) }, [mc.version]) // eslint-disable-line react-hooks/exhaustive-deps
  if (!s) return <Skeleton h={400} r={22} />
  const steps: any[] = s.steps
  const at = (k: string) => steps.find(x => x.key === k)
  const setStep = (k: string, o: any) => setS({ ...s, steps: steps.map(x => (x.key === k ? { ...x, ...o } : x)) })
  const bounds: Record<string, [number, number]> = {
    remind: [1, 10], resend1: [1, at('resend2').days - 1], resend2: [at('resend1').days + 1, at('proposal').days - 1],
    proposal: [at('resend2').days + 1, at('final').days - 1], final: [at('proposal').days + 1, 90],
  }
  const label = (k: string, v: number) => (k === 'remind' ? `${days(v)} antes` : k === 'link' ? 'No dia · 08:00' : k === 'evening' ? 'No dia · 18:00' : `+${days(v)}`)
  const save = async () => {
    setBusy(true)
    try { const r = await g.microcredit.saveSettings({ ...s, delivery_limit: parseMT(limit), two_above: parseMT(two) }); setS(r.settings); toast('Definições guardadas') } catch (e) { toast(errMsg(e), true) } finally { setBusy(false) }
  }
  const btn = (on: boolean) => ({ width: 30, height: 30, borderRadius: 9, border: '1px solid var(--line-2)', background: '#fff', color: on ? 'var(--ink)' : '#C9CDC6', flex: 'none' as const })
  return (
    <div>
      <div className="grid-2" style={{ alignItems: 'start' }}>
        <div className="card" style={{ overflow: 'hidden' }}>
          <div style={{ padding: '18px 20px 10px' }}><b style={{ fontSize: 16 }}>Cobrança das prestações</b><div className="mc-sub" style={{ marginTop: 4 }}>A RIHA envia o link em cada data. O cliente confirma com o PIN. Ajuste os dias com − e +.</div></div>
          {steps.map(x => {
            const [t, n, locked] = STEPS[x.key]
            const adj = !!bounds[x.key]
            const [lo, hi] = bounds[x.key] || [0, 0]
            return (
              <div key={x.key} style={{ display: 'grid', gridTemplateColumns: '170px 1fr auto', gap: 12, alignItems: 'center', padding: '10px 20px' }}>
                <span style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <button style={{ ...btn(adj && x.days > lo), visibility: adj ? 'visible' : 'hidden' }} onClick={() => adj && x.days > lo && setStep(x.key, { days: x.days - 1 })}>−</button>
                  <span className="mono" style={{ fontSize: 12.5, flex: 1, textAlign: 'center', whiteSpace: 'nowrap' }}>{label(x.key, x.days)}</span>
                  <button style={{ ...btn(adj && x.days < hi), visibility: adj ? 'visible' : 'hidden' }} onClick={() => adj && x.days < hi && setStep(x.key, { days: x.days + 1 })}>+</button>
                </span>
                <span style={{ opacity: x.on ? 1 : 0.55 }}><b style={{ fontSize: 14 }}>{t}</b><div className="mc-sub">{n}</div></span>
                <span style={{ opacity: locked ? 0.5 : 1, pointerEvents: locked ? 'none' : 'auto' }}><Toggle on={x.on} onChange={v => setStep(x.key, { on: v })} /></span>
              </div>
            )
          })}
          <div className="grid-2" style={{ gap: 12, padding: '14px 20px 18px', borderTop: '1px solid var(--line-3)', marginTop: 6 }}>
            <div><span className="label">Tolerância padrão</span>
              <div style={{ display: 'flex', alignItems: 'center', height: 46, borderRadius: 13, border: '1px solid var(--line-2)', background: '#fff' }}>
                <button style={{ width: 40, border: 0, background: 'none', fontSize: 18 }} onClick={() => s.grace > 0 && setS({ ...s, grace: s.grace - 1 })}>−</button>
                <b style={{ flex: 1, textAlign: 'center', fontSize: 14 }}>{s.grace ? `${days(s.grace)} sem mora` : 'Sem tolerância'}</b>
                <button style={{ width: 40, border: 0, background: 'none', fontSize: 18 }} onClick={() => s.grace < 7 && setS({ ...s, grace: s.grace + 1 })}>+</button>
              </div>
              <div className="mc-sub">Para produtos novos.</div>
            </div>
            <label><span className="label">Ao fim de {at('final').days} dias</span>
              <select className="input" style={{ height: 46 }} value={s.final} onChange={e => setS({ ...s, final: e.target.value })}>
                <option value="manual">Cobrança manual</option>
                <option value="continue">Continuar os avisos</option>
              </select>
            </label>
          </div>
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
          <div className="card" style={{ overflow: 'hidden' }}>
            <div style={{ padding: '16px 20px', borderBottom: '1px solid var(--line-3)' }}><b style={{ fontSize: 16 }}>Entrega do dinheiro</b><div className="mc-sub">Só depois de o cliente aceitar o acordo.</div></div>
            <Row t="Pelo saldo RIHA" n={d.can_disburse ? 'Para o M-Pesa ou e-Mola do cliente' : 'Ainda não disponível nesta conta'}><Toggle on={s.via_riha} onChange={v => setS({ ...s, via_riha: v })} /></Row>
            <Row t="Fora da RIHA" n="Dinheiro ou transferência, marcado à mão"><Toggle on={s.outside} onChange={v => setS({ ...s, outside: v })} /></Row>
            <Row t="Limite por entrega"><div className="phone-input" style={{ height: 40, width: 150 }}><input inputMode="numeric" value={limit} onChange={e => setLimit(digitsMT(e.target.value))} style={{ textAlign: 'right' }} /><span style={{ padding: '0 10px', color: 'var(--muted)' }}>MT</span></div></Row>
          </div>
          <div className="card" style={{ overflow: 'hidden' }}>
            <div style={{ padding: '16px 20px', borderBottom: '1px solid var(--line-3)' }}><b style={{ fontSize: 16 }}>Aprovação</b></div>
            <Row t="Quem pode aprovar" n={`Qualquer pessoa da conta (${d.people})`} />
            <Row t={`Duas pessoas acima de ${mt(parseMT(two))} MT`} n={d.people < 2 ? 'Precisa de pelo menos duas pessoas na conta' : 'A segunda pessoa aprova no empréstimo'}>
              <span style={{ display: 'flex', gap: 8, alignItems: 'center' }}><div className="phone-input" style={{ height: 40, width: 120 }}><input inputMode="numeric" value={two} onChange={e => setTwo(digitsMT(e.target.value))} style={{ textAlign: 'right' }} /></div><Toggle on={s.two_approvers} onChange={v => setS({ ...s, two_approvers: v })} /></span>
            </Row>
          </div>
          <div className="card" style={{ overflow: 'hidden' }}>
            <div style={{ padding: '16px 20px', borderBottom: '1px solid var(--line-3)' }}><b style={{ fontSize: 16 }}>Mensagens</b></div>
            <Row t="Remetente do SMS" n="Até 11 letras, sem espaços"><input className="input mono" style={{ height: 40, width: 150, textAlign: 'right' }} maxLength={11} value={s.sender_id} placeholder="RIHA" onChange={e => setS({ ...s, sender_id: e.target.value.replace(/\s/g, '') })} /></Row>
            <Row t="Horário" n="Lembretes e reenvios só neste horário"><span className="tnum" style={{ fontSize: 14 }}>{String(s.quiet_start).padStart(2, '0')}:00 a {s.quiet_end}:00{s.no_sundays ? ' · sem domingos' : ''}</span></Row>
            <Row t="Sem domingos"><Toggle on={s.no_sundays} onChange={v => setS({ ...s, no_sundays: v })} /></Row>
          </div>
        </div>
      </div>
      <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: 16 }}><button className="btn btn-green" disabled={busy} onClick={save}>{busy ? <Spinner /> : 'Guardar definições'}</button></div>
    </div>
  )
}

function Row({ t, n, children }: { t: string; n?: string; children?: React.ReactNode }) {
  return <div className="between" style={{ padding: '12px 20px', borderBottom: '1px solid var(--line-3)' }}><span><b style={{ fontSize: 14, fontWeight: 600 }}>{t}</b>{n && <div className="mc-sub">{n}</div>}</span>{children}</div>
}
