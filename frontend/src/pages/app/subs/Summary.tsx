import { useNavigate } from 'react-router-dom'
import { Icon, Skeleton, Stripes } from '../../../components/ui'
import { useGateway } from '../../../core/gateway'
import { errMsg } from '../../../core/model'
import { useQuery } from '../../../core/useQuery'
import { useSession } from '../../../lib/session'
import { money, Stat } from './common'
import { useSubs } from './SubsShell'

export default function Summary() {
  const g = useGateway()
  const nav = useNavigate()
  const { toast } = useSession()
  const sx = useSubs()
  const { data, reload } = useQuery(g => g.subscriptions.overview(), [sx.version])
  if (!data) return <div className="grid-4" style={{ gap: 14 }}>{[0, 1, 2, 3].map(i => <Skeleton key={i} h={130} r={22} />)}</div>
  const o = data.overview
  const mo = o.month
  const pct = (v: number) => (mo.expected ? Math.max(0, (v / mo.expected) * 100) : 0)
  const resendAll = async () => { try { const r = await g.subscriptions.resendInvites(); toast(`${r.sent} convites reenviados`); reload() } catch (e) { toast(errMsg(e), true) } }
  const empty = data.plans.length === 0
  return (
    <div>
      <style>{`
        .sum-top{display:grid;grid-template-columns:1.3fr 1fr 1fr 1fr;gap:14px}
        .sum-mid{display:grid;grid-template-columns:minmax(0,1.6fr) minmax(0,1fr);gap:14px;margin-top:14px;align-items:start}
        .sum-quick{display:none}
        @media (max-width:1100px){.sum-top{grid-template-columns:1fr 1fr}.sum-mid{grid-template-columns:1fr}}
        @media (max-width:860px){.sum-top{grid-template-columns:1fr}.sum-top .only-wide{display:none}.sum-quick{display:grid;grid-template-columns:repeat(3,1fr);gap:8px;margin-top:10px}}
      `}</style>
      {empty && (
        <div className="card card-pad" style={{ marginBottom: 14, display: 'flex', gap: 16, alignItems: 'center', flexWrap: 'wrap' }}>
          <div style={{ flex: 1, minWidth: 220 }}>
            <div style={{ fontWeight: 700, fontSize: 16 }}>Comece por criar um plano</div>
            <div style={{ fontSize: 14, color: 'var(--muted)', marginTop: 4, lineHeight: 1.5 }}>Propinas, mensalidades ou sessões. A Riha envia um link de pagamento a cada cliente em cada período; nada é cobrado sem a confirmação dele.</div>
          </div>
          <button className="btn btn-green" onClick={() => nav('/painel/assinaturas/planos/novo')}><Icon name="plus" size={16} stroke={2.4} />Novo plano</button>
        </div>
      )}
      <div className="sum-top">
        <div className="dark-card" style={{ position: 'relative', overflow: 'hidden', padding: '20px 22px', borderRadius: 22, background: 'var(--ink)', color: '#fff', minHeight: 130 }}>
          <Stripes top={-60} right={-10} w={34} h={300} gap={10} />
          <div style={{ position: 'relative' }}>
            <div style={{ fontSize: 13, color: '#B9C3BD' }}>A receber por mês</div>
            <div className="display tnum" style={{ fontSize: 36, letterSpacing: '-0.03em', marginTop: 8 }}>{money(o.monthly)} <span style={{ fontSize: 15, color: '#93A09A', fontFamily: 'var(--sans)' }}>MT</span></div>
            <div style={{ fontSize: 13, color: '#B9C3BD', marginTop: 8 }}>{o.members} membros em {o.plans} {o.plans === 1 ? 'plano' : 'planos'}</div>
          </div>
        </div>
        <div className="only-wide"><Stat label="Membros ativos" value={o.members} unit="" sub={o.invites ? `${o.invites} convites por aceitar` : 'Sem convites pendentes'} /></div>
        <div className="only-wide"><Stat label="Em dívida" value={money(o.debt)} sub={`${o.debt_members} ${o.debt_members === 1 ? 'membro' : 'membros'}`} /></div>
        <div className="only-wide"><Stat label={`Recebido pela Riha em ${mo.label.toLowerCase()}`} value={money(mo.received_in_riha)} sub="No saldo" tone="var(--green-text)" /></div>
      </div>
      <div className="sum-quick">
        {[['plus', 'Novo plano', () => nav('/painel/assinaturas/planos/novo')], ['userPlus', 'Adicionar membro', () => sx.openAdd('one')], ['grid', 'Partilhar catálogo', () => nav('/painel/assinaturas/catalogo')]].map(([ic, l, f]) => (
          <button key={l as string} className="card" onClick={f as () => void} style={{ padding: '14px 8px', display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 8, fontWeight: 600, fontSize: 13, textAlign: 'center', border: '1px solid var(--line)' }}>
            <span style={{ width: 36, height: 36, borderRadius: 11, display: 'grid', placeItems: 'center', background: ic === 'plus' ? 'var(--green-bg)' : 'var(--paper-2)', color: ic === 'plus' ? 'var(--green)' : 'var(--ink)' }}><Icon name={ic as string} size={18} /></span>{l as string}
          </button>
        ))}
      </div>
      {o.debt_members > 0 && (
        <button className="only-mobile" onClick={() => nav('/painel/assinaturas/membros?status=past_due')} style={{ width: '100%', marginTop: 10, display: 'flex', alignItems: 'center', gap: 10, padding: '14px 16px', borderRadius: 16, border: 0, background: 'var(--sun-soft)', color: '#5B4600', textAlign: 'left' }}>
          <span style={{ width: 8, height: 8, borderRadius: '50%', background: '#E0A800' }} />
          <span style={{ flex: 1 }}><b style={{ display: 'block', fontSize: 14 }}>{o.debt_members} membros em dívida · {money(o.debt)} MT</b>{o.max_debt_count > 1 && <span style={{ fontSize: 12.5 }}>Há quem tenha {o.max_debt_count} meses acumulados.</span>}</span>
          <Icon name="chevronRight" size={16} />
        </button>
      )}
      <div className="sum-mid">
        <div style={{ display: 'flex', flexDirection: 'column', gap: 14, minWidth: 0 }}>
          <div className="card card-pad">
            <div className="between"><div className="display" style={{ fontSize: 20 }}>{mo.label}</div><div style={{ fontSize: 13, color: 'var(--muted)' }}>{money(mo.expected)} MT esperados</div></div>
            <div style={{ display: 'flex', height: 12, borderRadius: 6, overflow: 'hidden', marginTop: 16, background: '#EEECE5', gap: 2 }}>
              <div style={{ width: pct(mo.riha) + '%', background: 'var(--green)' }} />
              <div style={{ width: pct(mo.outside) + '%', background: 'repeating-linear-gradient(135deg,#9AA39E 0 3px,#D7D5CD 3px 6px)' }} />
              <div style={{ width: pct(mo.debt + mo.open) + '%', background: 'var(--sun)' }} />
            </div>
            <div className="grid-3" style={{ gap: 14, marginTop: 16 }}>
              <div><div style={{ fontSize: 13, fontWeight: 600, display: 'flex', gap: 6, alignItems: 'center' }}><span style={{ width: 9, height: 9, borderRadius: 2, background: 'var(--green)' }} />Recebido pela Riha</div><div className="display tnum" style={{ fontSize: 22, marginTop: 4 }}>{money(mo.riha)}</div><div style={{ fontSize: 12.5, color: 'var(--muted)', marginTop: 2 }}>{mo.riha_count} cobranças. No saldo e nos levantamentos.</div></div>
              <div><div style={{ fontSize: 13, fontWeight: 600, display: 'flex', gap: 6, alignItems: 'center' }}><span style={{ width: 9, height: 9, borderRadius: 2, background: 'repeating-linear-gradient(135deg,#9AA39E 0 2px,#D7D5CD 2px 4px)' }} />Pago fora da Riha</div><div className="display tnum" style={{ fontSize: 22, marginTop: 4, color: 'var(--text-2)' }}>{money(mo.outside)}</div><div style={{ fontSize: 12.5, color: 'var(--muted)', marginTop: 2 }}>{mo.outside_count} cobranças marcadas. Não entra no saldo Riha.</div></div>
              <div><div style={{ fontSize: 13, fontWeight: 600, display: 'flex', gap: 6, alignItems: 'center' }}><span style={{ width: 9, height: 9, borderRadius: 2, background: 'var(--sun)' }} />Em dívida</div><div className="display tnum" style={{ fontSize: 22, marginTop: 4 }}>{money(mo.debt + mo.open)}</div><div style={{ fontSize: 12.5, color: 'var(--muted)', marginTop: 2 }}>{mo.debt_count + mo.open_count} cobranças. Lembretes a correr.</div></div>
            </div>
          </div>
          <div className="table">
            <div style={{ padding: '16px 20px', fontWeight: 700, fontSize: 15.5, borderBottom: '1px solid var(--line-3)' }}>Próximas cobranças</div>
            {o.upcoming.length === 0 && <div className="empty" style={{ padding: 24, fontSize: 14, color: 'var(--muted)' }}>Sem cobranças nas próximas semanas.</div>}
            {o.upcoming.map((u: any, i: number) => (
              <div key={i} className="trow" style={{ gridTemplateColumns: '110px 1fr 1fr auto', color: u.empty ? 'var(--muted)' : undefined }}>
                <span className="mono" style={{ fontSize: 12.5 }}>{u.when}</span>
                <span style={{ fontWeight: 600, fontSize: 14 }}>{u.label}</span>
                <span style={{ fontSize: 13.5, color: 'var(--muted)' }} className="only-desktop">{u.detail}</span>
                <span className="tnum" style={{ textAlign: 'right', fontSize: 14 }}>{u.empty ? '—' : money(u.total) + ' MT'}</span>
              </div>
            ))}
          </div>
        </div>
        <div className="card" style={{ overflow: 'hidden' }}>
          <div style={{ padding: '16px 20px', fontWeight: 700, fontSize: 15.5, borderBottom: '1px solid var(--line-3)' }}>Precisa de atenção</div>
          {o.attention.length === 0 && <div style={{ padding: 20, fontSize: 14, color: 'var(--muted)' }}>Nada a fazer por agora. Os lembretes correm sozinhos.</div>}
          {o.attention.map((a: any, i: number) => (
            <div key={i} style={{ display: 'flex', gap: 12, padding: '14px 20px', borderBottom: i < o.attention.length - 1 ? '1px solid var(--line-3)' : 0, cursor: a.member_id ? 'pointer' : 'default' }} onClick={() => a.member_id && nav('/painel/assinaturas/membros/' + a.member_id)}>
              <span style={{ width: 8, height: 8, borderRadius: '50%', marginTop: 6, flex: 'none', background: a.tone === 'red' ? 'var(--red)' : a.tone === 'sun' ? '#E0A800' : '#B9BEB8' }} />
              <div style={{ minWidth: 0 }}>
                <div style={{ fontWeight: 600, fontSize: 14 }}>{a.title}</div>
                {a.detail === 'resend' ? <button className="btn-ghost" onClick={resendAll} style={{ border: 0, background: 'none', padding: 0, color: 'var(--green-text)', fontSize: 13, textDecoration: 'underline', marginTop: 3 }}>Reenviar todos</button>
                  : <div style={{ fontSize: 13, color: 'var(--muted)', marginTop: 3 }}>{a.detail}</div>}
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
