import { useState } from 'react'
import { errMsg } from '../../core/model'
import { useGateway } from '../../core/gateway'
import { useQuery } from '../../core/useQuery'
import { bpsLabel, METHOD_NAME } from '../../lib/format'
import { useSession } from '../../lib/session'
import { MethodIcon } from '../../components/ui'

// Which integration takes each of a merchant's methods in production, and the
// partner's share of RIHA's fee on its payments.

export const INTEGRATION_NAME: Record<string, string> = { mtoca: 'mtoca', mpesa_direct: 'M-Pesa direto', simulator: 'Simulador' }
const name = (code: string, names: Record<string, string>) => names[code] || INTEGRATION_NAME[code] || code || '—'

export function MerchantRouting({ merchantId }: { merchantId: string }) {
  const g = useGateway()
  const { toast } = useSession()
  const { data, setData } = useQuery(g => g.admin.routing(merchantId), [merchantId])
  const list = useQuery(g => g.admin.integrations())
  const [busy, setBusy] = useState('')
  const [share, setShare] = useState<Record<string, string>>({})
  const names: Record<string, string> = Object.fromEntries((list.data?.data || []).map((i: any) => [i.code, i.name]))
  const run = async (key: string, fn: () => Promise<any>, ok: string) => {
    setBusy(key)
    try { setData(await fn()); toast(ok) } catch (e) { toast(errMsg(e), true) } finally { setBusy('') }
  }
  if (!data) return null
  return (
    <div className="card" style={{ padding: 22 }}>
      <div style={{ fontWeight: 600, fontSize: 16 }}>Integrações</div>
      <div style={{ fontSize: 13, color: 'var(--muted)', marginTop: 4, lineHeight: 1.5 }}>Por onde entra o dinheiro deste comerciante em produção, e quanto o parceiro leva. Vale a partir do próximo pagamento.</div>
      <div style={{ marginTop: 12 }}>
        {data.methods.map((m: any) => (
          <div key={m.method} className="between" style={{ padding: '10px 0', borderTop: '1px solid var(--line-3)', gap: 12 }}>
            <span className="row" style={{ gap: 9, fontSize: 14, fontWeight: 600 }}><MethodIcon method={m.method} size={24} radius={7} />{METHOD_NAME[m.method] || m.method}</span>
            {m.options.length > 1 ? (
              <select className="input" style={{ width: 'auto', minWidth: 200, height: 38 }} value={m.chosen} disabled={busy === m.method} aria-label={`Integração para ${METHOD_NAME[m.method]}`}
                onChange={e => run(m.method, () => g.admin.setRoute(merchantId, m.method, e.target.value), `${METHOD_NAME[m.method]} passa por ${name(e.target.value || m.default, names)}`)}>
                <option value="">Por omissão ({name(m.default, names)})</option>
                {m.options.map((o: string) => <option key={o} value={o}>{name(o, names)}</option>)}
              </select>
            ) : <span style={{ fontSize: 13.5 }}>{name(m.via, names)} <span className="muted">· só esta</span></span>}
          </div>
        ))}
        {!data.methods.length && <div className="empty">Nenhuma integração recebe dinheiro real neste servidor.</div>}
      </div>
      <div style={{ fontWeight: 600, fontSize: 14, marginTop: 16 }}>Parte do parceiro</div>
      <div style={{ fontSize: 12.5, color: 'var(--muted)', marginTop: 2, lineHeight: 1.5 }}>Percentagem do que fica da taxa da RIHA depois da taxa do operador.</div>
      {data.deals.map((d: any) => {
        const own = d.share_bps !== null && d.share_bps !== undefined
        const v = share[d.integration] ?? (own ? String(d.share_bps / 100).replace('.', ',') : '')
        const bps = Math.round(parseFloat(v.replace(',', '.')) * 100)
        return (
          <div key={d.integration} className="between" style={{ padding: '10px 0', borderTop: '1px solid var(--line-3)', gap: 10, flexWrap: 'wrap' }}>
            <span style={{ fontSize: 13.5 }}><b>{d.name}</b> <span className="muted">· {own ? `próprio (omissão ${bpsLabel(d.default_bps)})` : `omissão ${bpsLabel(d.default_bps)}`}</span></span>
            <span className="row" style={{ gap: 6 }}>
              <span style={{ position: 'relative', width: 96 }}><input className="input" inputMode="decimal" placeholder={String(d.default_bps / 100)} value={v} style={{ paddingRight: 26, height: 38, fontWeight: 600 }}
                onChange={e => setShare({ ...share, [d.integration]: e.target.value.replace(/[^\d.,]/g, '') })} aria-label={`Parte de ${d.name}`} /><span style={{ position: 'absolute', right: 10, top: 10, color: 'var(--muted)' }}>%</span></span>
              <button className="btn btn-outline btn-sm" disabled={busy === d.integration || !isFinite(bps) || bps < 0 || bps > 10000}
                onClick={() => run(d.integration, () => g.admin.setDeal(merchantId, d.integration, bps), `${d.name}: ${bpsLabel(bps)} para este comerciante`)}>Guardar</button>
              {own && <button className="btn btn-ghost btn-sm" disabled={busy === d.integration}
                onClick={() => { setShare({ ...share, [d.integration]: '' }); run(d.integration, () => g.admin.setDeal(merchantId, d.integration, null), `${d.name}: volta a ${bpsLabel(d.default_bps)}`) }}>Repor</button>}
            </span>
          </div>
        )
      })}
    </div>
  )
}
