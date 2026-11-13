import { useNavigate } from 'react-router-dom'
import { Icon, Skeleton, Stripes } from '../../../components/ui'
import { useGateway } from '../../../core/gateway'
import { errMsg } from '../../../core/model'
import { useQuery } from '../../../core/useQuery'
import { asDate, dl, mt } from '../../../lib/credit'
import { useSession } from '../../../lib/session'
import { useMicro } from './common'

const DOT: Record<string, string> = { sun: '#E0A800', red: 'var(--red)', grey: '#B9BEB8' }

function hoursSince(iso: string) {
  const h = Math.max(0, Math.round((Date.now() - new Date(iso).getTime()) / 3600e3))
  return h < 48 ? `${h} h` : `${Math.round(h / 24)} dias`
}

export default function Summary() {
  const g = useGateway()
  const nav = useNavigate()
  const { toast } = useSession()
  const mc = useMicro()
  const { data: o, reload } = useQuery(g => g.microcredit.overview(), [mc.version])
  if (!o) return <div className="grid-4">{[0, 1, 2, 3].map(i => <Skeleton key={i} h={130} r={22} />)}</div>
  const m = o.month
  const pc = (v: number) => (m.expected ? (v / m.expected) * 100 : 0)
  const agingTotal = o.aging.reduce((a: number, b: number) => a + b, 0)
  const empty = mc.products.length === 0
  const act = async (a: any) => {
    try {
      if (a.action === 'retry') { await g.microcredit.loanAction(a.loan_id, 'retry'); toast('Entrega enviada de novo') }
      else if (a.action === 'resend_offers') { const r = await g.microcredit.resendOffers(); toast(`${r.sent} SMS reenviados`) }
      reload(); mc.bump()
    } catch (e) { toast(errMsg(e), true) }
  }
  const open = (a: any) => a.loan_id ? nav('/painel/microcredito/emprestimos/' + a.loan_id) : a.request_id ? nav('/painel/microcredito/pedidos') : undefined
  return (
    <div>
      <style>{`
        .ms-top{display:grid;grid-template-columns:1.35fr 1fr 1fr 1fr;gap:14px}
        .ms-mid{display:grid;grid-template-columns:minmax(0,1.6fr) minmax(0,1fr);gap:14px;margin-top:14px;align-items:start}
        @media (max-width:1100px){.ms-top{grid-template-columns:1fr 1fr}.ms-mid{grid-template-columns:1fr}}
        @media (max-width:860px){.ms-top{grid-template-columns:1fr 1fr;gap:10px}.ms-top .ms-dark{grid-column:1/-1}.ms-top .ms-rec{display:none}}
      `}</style>
      {empty && (
        <div className="card card-pad" style={{ marginBottom: 14, display: 'flex', gap: 16, alignItems: 'center', flexWrap: 'wrap' }}>
          <div style={{ flex: 1, minWidth: 220 }}>
            <div style={{ fontWeight: 700, fontSize: 16 }}>Comece por criar um produto</div>
            <div style={{ fontSize: 14, color: 'var(--muted)', marginTop: 4, lineHeight: 1.5 }}>Montantes, prazos, taxa e mora. Depois partilhe o link de pedido: o cliente simula, envia os dados e a sua instituição decide.</div>
          </div>
          <button className="btn btn-green" onClick={() => nav('/painel/microcredito/produtos/novo')}><Icon name="plus" size={16} stroke={2.4} />Novo produto</button>
        </div>
      )}
      <div className="ms-top">
        <div className="ms-dark" style={{ position: 'relative', overflow: 'hidden', padding: '20px 22px', borderRadius: 22, background: 'var(--ink)', color: '#fff', minHeight: 128 }}>
          <Stripes top={-60} right={-10} w={34} h={300} gap={10} />
          <div style={{ position: 'relative' }}>
            <div style={{ fontSize: 13, color: '#B9C3BD' }}>Carteira ativa</div>
            <div className="display tnum" style={{ fontSize: 36, letterSpacing: '-0.03em', marginTop: 8 }}>{mt(o.book)} <span style={{ fontSize: 15, color: '#93A09A', fontFamily: 'var(--sans)' }}>MT</span></div>
            <div style={{ fontSize: 13, color: '#B9C3BD', marginTop: 8 }}>{o.book_loans} {o.book_loans === 1 ? 'empréstimo' : 'empréstimos'} em {o.book_products} {o.book_products === 1 ? 'produto' : 'produtos'}</div>
          </div>
        </div>
        <button className="card card-pad" style={{ textAlign: 'left', border: '1px solid var(--line)' }} onClick={() => nav('/painel/microcredito/pedidos')}>
          <div style={{ fontSize: 13, color: 'var(--muted)' }}>Pedidos por decidir</div>
          <div className="display tnum" style={{ fontSize: 30, marginTop: 8 }}>{o.pending}</div>
          <div style={{ fontSize: 12.5, marginTop: 6, fontWeight: 600, color: o.pending ? 'var(--sun-text)' : 'var(--muted)' }}>{o.oldest_wait ? `O mais antigo espera há ${hoursSince(o.oldest_wait)}` : 'Nenhum à espera'}</div>
        </button>
        <button className="card card-pad" style={{ textAlign: 'left', border: '1px solid var(--line)' }} onClick={() => nav('/painel/microcredito/emprestimos?tab=atraso')}>
          <div style={{ fontSize: 13, color: 'var(--muted)' }}>Em atraso</div>
          <div className="display tnum" style={{ fontSize: 30, marginTop: 8 }}>{mt(o.overdue)}<span style={{ fontSize: 13, color: 'var(--muted)', marginLeft: 4, fontFamily: 'var(--sans)' }}>MT</span></div>
          <div style={{ fontSize: 12.5, color: 'var(--muted)', marginTop: 6 }}>{o.overdue_loans} {o.overdue_loans === 1 ? 'empréstimo' : 'empréstimos'}</div>
        </button>
        <div className="card card-pad ms-rec">
          <div style={{ fontSize: 13, color: 'var(--muted)' }}>Recebido pela RIHA em {m.label.toLowerCase()}</div>
          <div className="display tnum" style={{ fontSize: 30, marginTop: 8 }}>{mt(o.received_riha)}<span style={{ fontSize: 13, color: 'var(--muted)', marginLeft: 4, fontFamily: 'var(--sans)' }}>MT</span></div>
          <div style={{ fontSize: 12.5, marginTop: 6, fontWeight: 600, color: 'var(--green-text)' }}>No saldo</div>
        </div>
      </div>
      <div className="ms-mid">
        <div style={{ display: 'flex', flexDirection: 'column', gap: 14, minWidth: 0 }}>
          <div className="card card-pad">
            <div className="between"><div className="display" style={{ fontSize: 20 }}>{m.label}</div><div style={{ fontSize: 13, color: 'var(--muted)' }}>{mt(m.expected)} MT em prestações</div></div>
            <div style={{ display: 'flex', height: 12, borderRadius: 6, overflow: 'hidden', marginTop: 16, background: '#EEECE5', gap: 2 }}>
              <div style={{ width: pc(m.riha) + '%', background: 'var(--green)' }} />
              <div style={{ width: pc(m.outside) + '%', background: 'repeating-linear-gradient(135deg,#9AA39E 0 3px,#D7D5CD 3px 6px)' }} />
              <div style={{ width: pc(m.late) + '%', background: 'var(--sun)' }} />
            </div>
            <div className="grid-4" style={{ gap: 14, marginTop: 16 }}>
              {[['var(--green)', 'Recebido pela RIHA', m.riha, `${m.riha_n} prestações. No saldo.`],
                ['repeating-linear-gradient(135deg,#9AA39E 0 2px,#D7D5CD 2px 4px)', 'Pago fora da RIHA', m.outside, `${m.outside_n} marcadas. Não entra no saldo.`],
                ['var(--sun)', 'Em atraso', m.late, `${m.late_n} prestações. Reenvios a correr.`],
                ['#D7D5CD', 'Por vencer', m.to_come, `${m.to_come_n} links até ao fim do mês.`]].map(([c, l, v, s]) => (
                <div key={l as string}>
                  <div style={{ fontSize: 13, fontWeight: 600, display: 'flex', gap: 6, alignItems: 'center' }}><span style={{ width: 9, height: 9, borderRadius: 2, background: c as string }} />{l as string}</div>
                  <div className="display tnum" style={{ fontSize: 22, marginTop: 4 }}>{mt(v as number)}</div>
                  <div style={{ fontSize: 12.5, color: 'var(--muted)', marginTop: 2 }}>{s as string}</div>
                </div>
              ))}
            </div>
          </div>
          <div className="table">
            <div style={{ padding: '16px 20px', fontWeight: 700, fontSize: 15.5, borderBottom: '1px solid var(--line-3)' }}>Próximas prestações</div>
            {o.upcoming.length === 0 && <div className="empty" style={{ padding: 24 }}>Sem prestações nas próximas semanas.</div>}
            {o.upcoming.map((u: any, i: number) => (
              <div key={i} className="trow" style={{ gridTemplateColumns: '80px 90px 1fr auto' }}>
                <span className="mono" style={{ fontSize: 12.5 }}>{dl(asDate(u.date))}</span>
                <b style={{ fontSize: 14 }}>{u.links} {u.links === 1 ? 'link' : 'links'}</b>
                <span style={{ fontSize: 13.5, color: 'var(--muted)' }}>{u.product}</span>
                <span className="tnum" style={{ fontSize: 14 }}>{mt(u.total)} MT</span>
              </div>
            ))}
          </div>
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 14, minWidth: 0 }}>
          <div className="card" style={{ overflow: 'hidden' }}>
            <div style={{ padding: '16px 20px', fontWeight: 700, fontSize: 15.5, borderBottom: '1px solid var(--line-3)' }}>Precisa de atenção</div>
            {o.attention.length === 0 && <div style={{ padding: 20, fontSize: 14, color: 'var(--muted)' }}>Nada a fazer por agora. As cobranças correm sozinhas.</div>}
            {o.attention.map((a: any, i: number) => (
              <div key={i} style={{ display: 'flex', gap: 12, padding: '14px 20px', borderBottom: i < o.attention.length - 1 ? '1px solid var(--line-3)' : 0, cursor: a.loan_id || a.request_id ? 'pointer' : 'default' }} onClick={() => open(a)}>
                <span style={{ width: 8, height: 8, borderRadius: '50%', marginTop: 6, flex: 'none', background: DOT[a.tone] || DOT.grey }} />
                <div style={{ minWidth: 0 }}>
                  <div style={{ fontWeight: 600, fontSize: 14 }}>{a.title}</div>
                  {a.detail && <div style={{ fontSize: 13, color: 'var(--muted)', marginTop: 3 }}>{a.detail}</div>}
                  {a.action && <button className="link-btn" style={{ fontSize: 13, marginTop: 3 }} onClick={e => { e.stopPropagation(); act(a) }}>{a.action === 'retry' ? 'Tentar de novo' : 'Reenviar SMS'}</button>}
                </div>
              </div>
            ))}
          </div>
          <div className="card card-pad">
            <div className="between"><b style={{ fontSize: 15.5 }}>Atraso por idade</b><span className="muted" style={{ fontSize: 13 }}>{mt(agingTotal)} MT</span></div>
            <div style={{ marginTop: 12 }}>
              {['1 a 7 dias', '8 a 30 dias', '31 a 60 dias', 'Mais de 60'].map((l, i) => (
                <div key={l} style={{ display: 'grid', gridTemplateColumns: '92px 1fr 70px', alignItems: 'center', gap: 10, padding: '5px 0', fontSize: 13.5 }}>
                  <span>{l}</span>
                  <span style={{ height: 6, borderRadius: 3, background: '#EEECE5', overflow: 'hidden' }}><span style={{ display: 'block', height: '100%', width: (agingTotal ? (o.aging[i] / agingTotal) * 100 : 0) + '%', background: i < 2 ? 'var(--sun)' : 'var(--red)' }} /></span>
                  <span className="tnum" style={{ textAlign: 'right' }}>{mt(o.aging[i])}</span>
                </div>
              ))}
            </div>
            <div className="between" style={{ borderTop: '1px solid var(--line-3)', marginTop: 10, paddingTop: 10, fontSize: 13.5 }}><span className="muted">Carteira em risco a mais de 30 dias</span><b>{o.par30.toFixed(1).replace('.', ',')}%</b></div>
          </div>
        </div>
      </div>
    </div>
  )
}
