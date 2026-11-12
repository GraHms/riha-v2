import { useState, type InputHTMLAttributes } from 'react'
import { errMsg } from '../../core/model'
import { useGateway } from '../../core/gateway'
import { useQuery } from '../../core/useQuery'
import { bpsLabel, METHOD_NAME } from '../../lib/format'
import { useSession } from '../../lib/session'
import { MethodIcon } from '../../components/ui'
import { Badge, Empty, Panel, Tip } from '../../components/ops'

// Which integration takes each of a merchant's methods in production, and the
// partner's share of RIHA's fee on its payments.

export const INTEGRATION_NAME: Record<string, string> = { mtoca: 'mtoca', mpesa_direct: 'M-Pesa direto', simulator: 'Simulador' }
const name = (code: string, names: Record<string, string>) => names[code] || INTEGRATION_NAME[code] || code || '—'

/** Suffixed is an input with its unit (%, MT) written inside, on the right. */
export function Suffixed({ suffix, width, ...p }: { suffix: string; width?: number | string } & InputHTMLAttributes<HTMLInputElement>) {
  return (
    <span style={{ position: 'relative', display: 'block', width, flex: 'none', maxWidth: '100%' }}>
      <input className="input" {...p} style={{ width: '100%', paddingRight: 14 + suffix.length * 9, fontWeight: 600, fontVariantNumeric: 'tabular-nums', ...p.style }} />
      <span style={{ position: 'absolute', right: 12, top: '50%', transform: 'translateY(-50%)', fontSize: 12.5, fontWeight: 600, color: 'var(--o-muted)', pointerEvents: 'none' }}>{suffix}</span>
    </span>
  )
}

const CSS = `
.rt-h{display:inline-flex;align-items:center;gap:6px}
.rt-row{display:flex;align-items:center;justify-content:space-between;gap:8px 12px;flex-wrap:wrap;padding:11px 18px;border-top:1px solid var(--o-line-2)}
.rt-row:first-child{border-top:0}
.rt-m{display:flex;align-items:center;gap:10px;min-width:0;flex:1 1 120px}
.rt-m .o-t1{font-size:13.5px}
.rt-row .o-select{flex:1 1 190px;min-width:0;max-width:100%;height:36px}
.rt-sub{display:flex;align-items:center;gap:6px;padding:10px 18px;background:var(--o-surface-2);border-top:1px solid var(--o-line-2);border-bottom:1px solid var(--o-line-2);font-size:12px;font-weight:700;color:var(--o-muted);letter-spacing:.02em}
.rt-sub+.rt-row{border-top:0}
.rt-do{display:flex;align-items:center;gap:6px;flex:none}
.rt-do .input{height:34px}
`

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
    <Panel pad={false} title={<span className="rt-h">Integrações<Tip text="Por onde entra o dinheiro deste comerciante em produção, e quanto o parceiro leva. Vale a partir do próximo pagamento." /></span>}>
      <style>{CSS}</style>
      {data.methods.map((m: any) => (
        <div key={m.method} className="rt-row">
          <span className="rt-m"><MethodIcon method={m.method} size={26} radius={8} /><span className="o-t1">{METHOD_NAME[m.method] || m.method}</span></span>
          {m.options.length > 1 ? (
            <select className="o-select" value={m.chosen} disabled={busy === m.method} aria-label={`Integração para ${METHOD_NAME[m.method]}`}
              onChange={e => run(m.method, () => g.admin.setRoute(merchantId, m.method, e.target.value), `${METHOD_NAME[m.method]} passa por ${name(e.target.value || m.default, names)}`)}>
              <option value="">Por omissão ({name(m.default, names)})</option>
              {m.options.map((o: string) => <option key={o} value={o}>{name(o, names)}</option>)}
            </select>
          ) : <span className="o-t2" style={{ marginTop: 0 }}><b style={{ color: 'var(--o-ink)', fontWeight: 600 }}>{name(m.via, names)}</b> · só esta</span>}
        </div>
      ))}
      {!data.methods.length && <Empty icon="link" title="Sem integrações" text="Nenhuma recebe dinheiro real neste servidor." />}
      {data.deals.length > 0 && <div className="rt-sub">Parte do parceiro<Tip text="Percentagem do que fica da taxa da RIHA depois da taxa do operador." /></div>}
      {data.deals.map((d: any) => {
        const own = d.share_bps !== null && d.share_bps !== undefined
        const v = share[d.integration] ?? (own ? String(d.share_bps / 100).replace('.', ',') : '')
        const bps = Math.round(parseFloat(v.replace(',', '.')) * 100)
        return (
          <div key={d.integration} className="rt-row">
            <div style={{ minWidth: 0, flex: '1 1 120px' }}>
              <div className="o-t1" style={{ fontSize: 13.5 }}>{d.name}</div>
              <div style={{ marginTop: 3 }}>{own ? <Badge tone="info">Própria · omissão {bpsLabel(d.default_bps)}</Badge> : <Badge>Omissão {bpsLabel(d.default_bps)}</Badge>}</div>
            </div>
            <span className="rt-do">
              <Suffixed suffix="%" width={86} inputMode="decimal" placeholder={String(d.default_bps / 100)} value={v}
                onChange={e => setShare({ ...share, [d.integration]: e.target.value.replace(/[^\d.,]/g, '') })} aria-label={`Parte de ${d.name}`} />
              <button className="btn btn-outline btn-sm" disabled={busy === d.integration || !isFinite(bps) || bps < 0 || bps > 10000}
                onClick={() => run(d.integration, () => g.admin.setDeal(merchantId, d.integration, bps), `${d.name}: ${bpsLabel(bps)} para este comerciante`)}>Guardar</button>
              {own && <button className="btn btn-ghost btn-sm" disabled={busy === d.integration}
                onClick={() => { setShare({ ...share, [d.integration]: '' }); run(d.integration, () => g.admin.setDeal(merchantId, d.integration, null), `${d.name}: volta a ${bpsLabel(d.default_bps)}`) }}>Repor</button>}
            </span>
          </div>
        )
      })}
    </Panel>
  )
}
