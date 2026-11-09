import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { Icon, Spinner } from '../../../components/ui'
import { useGateway } from '../../../core/gateway'
import { errMsg } from '../../../core/model'
import { money, StatusPill, SUB_STATUS } from '../../app/subs/common'
import { Center, MerchantHead, PayFrame } from './PayFrame'

// The member's own page, from the link in their SMS: what they're subscribed to,
// what they paid, what failed, a way to pay what's due, and to cancel.

const cents = (c: number) => money(c) + ',' + String(Math.abs(c) % 100).padStart(2, '0')

// What happened to each payment, in the customer's words.
const PAY_STATE: Record<string, { t: string; dot: string }> = {
  paid: { t: 'Pago', dot: 'var(--green)' },
  paid_outside: { t: 'Pago', dot: 'var(--green)' },
  open: { t: 'Por pagar', dot: '#2A4AA8' },
  past_due: { t: 'Em atraso', dot: 'var(--red)' },
  scheduled: { t: 'Agendado', dot: '#C9C7BE' },
  forfeited: { t: 'Não pago', dot: '#8E0609' },
  skipped: { t: 'Sem cobrança', dot: '#C9C7BE' },
}

const CUSTOMER_SUB_STATUS = { ...SUB_STATUS, invited: { ...SUB_STATUS.invited, t: 'Por ativar' } }

export default function MemberPage() {
  const { token } = useParams()
  const g = useGateway()
  const [d, setD] = useState<any>(null)
  const [loadErr, setLoadErr] = useState('')
  const [asking, setAsking] = useState('')
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')
  const [done, setDone] = useState('')

  useEffect(() => {
    g.publicSubs.member(token!).then(setD).catch(e => setLoadErr(errMsg(e)))
  }, [token]) // eslint-disable-line react-hooks/exhaustive-deps

  if (loadErr) return <Center><div style={{ maxWidth: 320 }}><h1 className="display" style={{ fontSize: 24 }}>Link indisponível</h1><p style={{ color: 'var(--muted)' }}>{loadErr}</p></div></Center>
  if (!d) return <Center><Spinner /></Center>

  const cancel = async (s: any) => {
    setBusy(true); setErr('')
    try {
      setD(await g.publicSubs.cancel(token!, s.id))
      setAsking(''); setDone(`${s.plan_name} foi cancelada. Vai receber a confirmação por SMS.`)
    } catch (e) { setErr(errMsg(e)) } finally { setBusy(false) }
  }
  const failed = d.payments.filter((p: any) => p.failure)

  return (
    <PayFrame test={d.test_mode}>
      <MerchantHead m={d.merchant} sub={d.member.name} />

      {d.due > 0 && (
        <div className="card" style={{ marginTop: 12, padding: 18, borderColor: 'var(--sun-bg)', background: '#FFFBEA' }}>
          <div style={{ fontSize: 13, color: 'var(--sun-text)', fontWeight: 600 }}>Por pagar</div>
          <div className="display tnum" style={{ fontSize: 28, marginTop: 2 }}>{cents(d.due)} MT</div>
          {failed.length > 0 && <div style={{ fontSize: 13.5, color: 'var(--red-dark)', marginTop: 6 }}>A última tentativa falhou: {failed[0].failure}</div>}
          {d.pay_url && <Link to={d.pay_url} className="btn btn-green btn-lg btn-block" style={{ marginTop: 14 }}><Icon name="lock" size={16} stroke={2.2} />Pagar agora</Link>}
        </div>
      )}

      {done && <div className="card" role="status" style={{ marginTop: 12, padding: 14, fontSize: 14, display: 'flex', gap: 10, alignItems: 'center' }}><Icon name="check" size={18} color="var(--green-dark)" />{done}</div>}

      <h2 style={{ fontSize: 15, margin: '24px 4px 8px' }}>Inscrições</h2>
      {d.subscriptions.length === 0 && <div className="card" style={{ padding: 16, color: 'var(--muted)', fontSize: 14 }}>Sem inscrições ativas.</div>}
      {d.subscriptions.map((s: any) => (
        <div key={s.id} className="card" style={{ padding: 16, marginBottom: 8 }}>
          <div className="between" style={{ gap: 10, alignItems: 'flex-start' }}>
            <div style={{ minWidth: 0 }}>
              <b style={{ fontSize: 15 }}>{s.plan_name}</b>
              <div style={{ fontSize: 13, color: 'var(--muted)', marginTop: 2 }}><span className="tnum">{money(s.amount)} MT</span>{s.schedule && ` · ${s.schedule}`}</div>
            </div>
            <StatusPill map={CUSTOMER_SUB_STATUS} s={s.status} style={{ flex: 'none' }} />
          </div>
          {s.next_due && <div style={{ fontSize: 13.5, marginTop: 10 }}>Próximo pagamento: <b>{s.next_due}</b></div>}
          {s.new_amount > 0 && <div style={{ fontSize: 13, color: 'var(--muted)', marginTop: 4 }}>Passa a {money(s.new_amount)} MT a partir de {s.new_from}.</div>}
          {s.canceled_at && !s.can_cancel && <div style={{ fontSize: 13, color: 'var(--muted)', marginTop: 8 }}>Cancelada a {s.canceled_at}.</div>}
          {s.can_cancel && asking !== s.id && (
            <button className="btn btn-ghost" style={{ marginTop: 10, padding: '6px 0', fontSize: 13.5, color: 'var(--red-dark)' }} onClick={() => { setAsking(s.id); setErr(''); setDone('') }}>Cancelar inscrição</button>
          )}
          {asking === s.id && (
            <div style={{ marginTop: 12, padding: 14, borderRadius: 14, background: '#FBF3F3' }}>
              <div style={{ fontWeight: 600, fontSize: 14 }}>Cancelar {s.plan_name}?</div>
              <div style={{ fontSize: 13.5, color: 'var(--text-2)', marginTop: 4, lineHeight: 1.5 }}>
                Deixa de receber cobranças e links de pagamento. {s.debt > 0 ? `Os ${cents(s.debt)} MT em atraso continuam por pagar.` : ''} Para voltar, terá de se inscrever de novo.
              </div>
              {err && <div className="form-err" style={{ marginTop: 10 }} role="alert">{err}</div>}
              <div style={{ display: 'flex', gap: 8, marginTop: 12 }}>
                <button className="btn btn-ghost" style={{ flex: 1 }} disabled={busy} onClick={() => setAsking('')}>Manter</button>
                <button className="btn" style={{ flex: 1, background: 'var(--red)', color: '#fff' }} disabled={busy} onClick={() => cancel(s)}>{busy ? <Spinner /> : 'Sim, cancelar'}</button>
              </div>
            </div>
          )}
        </div>
      ))}

      <h2 style={{ fontSize: 15, margin: '24px 4px 8px' }}>Pagamentos</h2>
      <div className="card" style={{ overflow: 'hidden' }}>
        {d.payments.length === 0 && <div style={{ padding: 16, color: 'var(--muted)', fontSize: 14 }}>Ainda sem pagamentos.</div>}
        {d.payments.map((p: any, i: number) => {
          const st = PAY_STATE[p.status] || { t: p.status, dot: '#C9C7BE' }
          const how = p.status === 'paid' ? `${p.paid_at}${p.method ? ' · ' + p.method : ''}`
            : p.status === 'paid_outside' ? `${p.paid_at}${p.outside ? ' · ' + p.outside : ''}`
            : p.status === 'scheduled' || p.status === 'open' ? `vence a ${p.due}`
            : p.status === 'past_due' ? `venceu a ${p.due}` : p.due
          return (
            <div key={p.id} style={{ padding: '12px 16px', borderTop: i ? '1px solid var(--line-3)' : 'none', display: 'flex', gap: 12 }}>
              <span style={{ width: 8, height: 8, borderRadius: '50%', background: st.dot, marginTop: 7, flex: 'none' }} />
              <div style={{ flex: 1, minWidth: 0 }}>
                <div className="between" style={{ gap: 10 }}><b style={{ fontSize: 14 }}>{p.label}</b><span className="tnum" style={{ fontSize: 14 }}>{cents(p.amount)} MT</span></div>
                <div style={{ fontSize: 13, color: 'var(--muted)', marginTop: 2 }}>{st.t} · {how}{d.subscriptions.length > 1 && p.plan_name ? ` · ${p.plan_name}` : ''}</div>
                {p.failure && <div style={{ fontSize: 13, color: 'var(--red-dark)', marginTop: 2 }}>Tentativa falhou: {p.failure}</div>}
              </div>
            </div>
          )
        })}
      </div>

      <div style={{ fontSize: 12.5, color: 'var(--muted)', lineHeight: 1.5, margin: '16px 4px 0' }}>
        Recebe as mensagens por {d.member.channel === 'whatsapp' ? 'WhatsApp' : 'SMS'} no {d.member.phone_masked}. Este link é só seu: não o partilhe. Para pausar ou mudar de plano, fale com {d.merchant.name}.
      </div>
    </PayFrame>
  )
}
