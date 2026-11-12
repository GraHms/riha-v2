import { useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { errMsg } from '../../core/model'
import { useGateway } from '../../core/gateway'
import { useQuery } from '../../core/useQuery'
import { bpsLabel, fmt, METHOD_NAME, parseMT } from '../../lib/format'
import { useSession } from '../../lib/session'
import { Icon, Modal, Skeleton, Spinner } from '../../components/ui'

// Where live money is: with each partner whose account collects it, at RIHA,
// and owed to merchants; and, for a period, how each payment's fee divided.

const iso = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
const today = () => iso(new Date())
const PERIODS: [string, string, () => [string, string]][] = [
  ['month', 'Este mês', () => { const n = new Date(); return [iso(new Date(n.getFullYear(), n.getMonth(), 1)), today()] }],
  ['last', 'Mês passado', () => { const n = new Date(); return [iso(new Date(n.getFullYear(), n.getMonth() - 1, 1)), iso(new Date(n.getFullYear(), n.getMonth(), 0))] }],
  ['all', 'Desde sempre', () => ['', '']],
]
const less = (v: number) => (v ? '−' + fmt(v) : fmt(0))
const day = (s: string) => new Date(s + 'T00:00:00').toLocaleDateString('pt-PT', { day: 'numeric', month: 'short', year: 'numeric' })

function dealLine(i: any) {
  const ops = Object.entries(i.platform_bps || {}).filter(([, v]) => (v as number) > 0).map(([m, v]) => `${METHOD_NAME[m] || m} ${bpsLabel(v as number)}`)
  return `${bpsLabel(i.share_bps)} da margem ao parceiro · ${ops.length ? 'operador ' + ops.join(', ') : 'sem taxa do operador'}`
}

function Tile({ label, value, sub, dark, warn }: { label: string; value: number; sub: string; dark?: boolean; warn?: boolean }) {
  return (
    <div className={dark ? 'dark-card' : 'card'} style={{ padding: 18, borderRadius: 20 }}>
      <div style={{ fontSize: 13, color: dark ? '#A9B5AE' : 'var(--muted)' }}>{label}</div>
      <div className="display tnum" style={{ fontSize: 26, marginTop: 4, color: warn ? 'var(--red)' : undefined }}>{fmt(value)} MT</div>
      <div style={{ fontSize: 12.5, color: dark ? '#A9B5AE' : 'var(--muted)', marginTop: 4, lineHeight: 1.45 }}>{sub}</div>
    </div>
  )
}

export default function AdminSettlement() {
  const g = useGateway()
  const { toast } = useSession()
  const [sp, setSp] = useSearchParams()
  const period = sp.get('p') || 'month'
  const [from, to] = period === 'custom' ? [sp.get('from') || '', sp.get('to') || ''] : (PERIODS.find(p => p[0] === period) || PERIODS[0])[2]()
  const { data, reload } = useQuery(g => g.admin.settlement(from || undefined, to || undefined), [from, to])
  const integrations = useQuery(g => g.admin.integrations())
  const [record, setRecord] = useState<string | null>(null)
  const [edit, setEdit] = useState<any | null>(null)
  const [open, setOpen] = useState<Record<string, boolean>>({})

  const pos = data?.position
  const del = async (s: any) => {
    if (!confirm(`Apagar o recebimento ${s.reference} de ${fmt(s.amount)} MT?`)) return
    try { await g.admin.deleteSettlement(s.id); toast('Recebimento apagado'); reload() } catch (e) { toast(errMsg(e), true) }
  }
  const live = (code: string) => (integrations.data?.data || []).find((i: any) => i.code === code)

  return (
    <div className="page">
      <style>{`.st-row{display:grid;grid-template-columns:minmax(0,1.6fr) 70px repeat(6,minmax(0,1fr));gap:10px;padding:10px 20px;align-items:center;font-size:13.5px}
        .st-row>span:not(:first-child){text-align:right}
        .st-head{font-size:11.5px;font-weight:700;color:var(--muted);text-transform:uppercase;letter-spacing:.05em}
        .st-tiles{margin-top:22px}
        @media(max-width:600px){.st-tiles{grid-template-columns:1fr!important}}
        .st-partners{display:grid;grid-template-columns:repeat(auto-fit,minmax(320px,1fr));gap:16px;margin-top:16px}
        @media(max-width:900px){.st-row{grid-template-columns:1fr 1fr;gap:4px 12px}.st-row>span:not(:first-child){text-align:left}.st-head{display:none!important}}`}</style>
      <div className="between" style={{ gap: 12, flexWrap: 'wrap' }}>
        <div>
          <h1 className="h1">Liquidação</h1>
          <div style={{ fontSize: 14.5, color: 'var(--muted)', marginTop: 4, maxWidth: 700, lineHeight: 1.5 }}>Onde está o dinheiro de produção: nas contas dos parceiros que o recebem, na conta da RIHA, e quanto devemos aos comerciantes.</div>
        </div>
        <button className="btn btn-green" onClick={() => setRecord('')}><Icon name="plus" size={16} stroke={2.2} />Registar recebimento</button>
      </div>

      {!data ? <div style={{ marginTop: 22 }}><Skeleton h={120} /></div> : (
        <>
          <div className="grid-4 st-tiles">
            <Tile dark label="Devido aos comerciantes" value={pos.owed_to_merchants} sub={`Saldos ${fmt(pos.merchant_balances)} MT + levantamentos a caminho ${fmt(pos.payouts_in_transit)} MT`} />
            <Tile label="Com os parceiros" value={pos.held_by_partners} sub="Recebido nas contas deles, menos o que já nos pagaram" />
            <Tile label="Na conta da RIHA" value={pos.at_riha} sub={pos.at_riha < 0 ? `Pagámos ${fmt(pos.paid_out)} MT em levantamentos, mais do que os recebimentos registados. Faltam recebimentos dos parceiros por registar?` : `Recebido dos parceiros menos ${fmt(pos.paid_out)} MT de levantamentos pagos. Calculado: confira com o extrato.`} warn={pos.at_riha < 0} />
            <Tile label="Da RIHA" value={pos.riha_own} sub={pos.riha_own < 0 ? (pos.at_riha < 0 ? 'Abaixo de zero enquanto faltarem recebimentos por registar. Se não faltarem, falta dinheiro: ver já.' : 'Falta dinheiro para cobrir o que devemos. Ver já.') : 'O que sobra depois de pagar tudo o que devemos: a margem.'} warn={pos.riha_own < 0} />
          </div>
          {data.unassigned.count > 0 && (
            <div className="card" style={{ marginTop: 14, padding: '12px 18px', fontSize: 13.5, background: '#FFF6E5', borderColor: '#F2D9A6' }}>
              {data.unassigned.count} pagamentos pagos ({fmt(data.unassigned.amount)} MT) não têm integração: ficam fora destas contas.
            </div>
          )}

          <div className="st-partners">
            {data.partners.map((p: any) => {
              const i = p.integration
              const l = live(i.code)
              return (
                <div key={i.code} className="card" style={{ padding: 22, display: 'flex', flexDirection: 'column', gap: 12 }}>
                  <div className="between" style={{ alignItems: 'flex-start', gap: 10 }}>
                    <div>
                      <div style={{ fontWeight: 700, fontSize: 17 }}>{i.partner}</div>
                      <div style={{ fontSize: 13, color: 'var(--muted)', marginTop: 2 }}>{i.name} · {i.account}</div>
                    </div>
                    <button className="btn btn-ghost btn-sm" onClick={() => setEdit(i)}>Editar acordo</button>
                  </div>
                  <div>
                    <div style={{ fontSize: 12.5, color: 'var(--muted)' }}>Com o parceiro, devido à RIHA</div>
                    <div className="display tnum" style={{ fontSize: 30, letterSpacing: '-0.03em', color: p.held < 0 ? 'var(--red)' : undefined }}>{fmt(p.held)} MT</div>
                  </div>
                  <div style={{ fontSize: 13, lineHeight: 1.7 }}>
                    <div className="between"><span className="muted">Recebido de clientes</span><span className="tnum">{fmt(p.all_time.amount)} MT · {p.all_time.count}</span></div>
                    <div className="between"><span className="muted">Taxa do operador</span><span className="tnum">{less(p.all_time.platform_fee)} MT</span></div>
                    <div className="between"><span className="muted">Parte do parceiro</span><span className="tnum">{less(p.all_time.partner_fee)} MT</span></div>
                    {p.all_time.refunded > 0 && <div className="between"><span className="muted">Reembolsado</span><span className="tnum">{less(p.all_time.refunded)} MT</span></div>}
                    <div className="between"><span className="muted">Já pago à RIHA</span><span className="tnum">{less(p.received)} MT</span></div>
                  </div>
                  <div style={{ fontSize: 12.5, color: 'var(--muted)', lineHeight: 1.5, borderTop: '1px solid var(--line-3)', paddingTop: 10 }}>
                    {dealLine(i)}
                    {l && <div>{l.live_methods.length ? `Recebe ${l.live_methods.map((m: string) => METHOD_NAME[m]).join(', ')}` : 'Não está ligada neste servidor'}{l.default_for.length ? ` · por omissão para ${l.default_for.map((m: string) => METHOD_NAME[m]).join(', ')}` : ''}{l.merchants_routed ? ` · ${l.merchants_routed} comerciante(s) encaminhado(s)` : ''}{l.merchant_deals ? ` · ${l.merchant_deals} acordo(s) próprio(s)` : ''}</div>}
                    {p.last_settlement && <div>Último recebimento: {fmt(p.last_settlement.amount)} MT a {day(p.last_settlement.received_on)}</div>}
                  </div>
                  <button className="btn btn-outline" onClick={() => setRecord(i.code)}>Registar recebimento de {i.partner}</button>
                </div>
              )
            })}
          </div>

          <div className="between" style={{ marginTop: 30, gap: 10, flexWrap: 'wrap' }}>
            <div className="h2">Por comerciante</div>
            <div className="row" style={{ gap: 8, flexWrap: 'wrap' }}>
              <div className="seg">{PERIODS.map(([k, l]) => <button key={k} className={period === k ? 'on' : ''} onClick={() => setSp({ p: k })}>{l}</button>)}</div>
              <input className="input" type="date" value={from} max={to || today()} style={{ width: 150, height: 38 }} aria-label="De" onChange={e => setSp({ p: 'custom', from: e.target.value, to })} />
              <input className="input" type="date" value={to} min={from} max={today()} style={{ width: 150, height: 38 }} aria-label="Até" onChange={e => setSp({ p: 'custom', from, to: e.target.value })} />
              <a className="btn btn-outline btn-sm" style={{ height: 38 }} href={g.admin.settlementExportUrl(from || undefined, to || undefined)}><Icon name="download" size={15} />CSV</a>
            </div>
          </div>
          <div style={{ fontSize: 13, color: 'var(--muted)', marginTop: 4 }}>Pagamentos pagos {from ? `de ${day(from)} a ${day(to || today())}` : 'desde sempre'}: {data.period.count}, {fmt(data.period.amount)} MT, margem da RIHA {fmt(data.period.margin)} MT.</div>
          {data.partners.map((p: any) => (
            <div key={p.integration.code} className="table" style={{ marginTop: 14, overflowX: 'auto' }}>
              <button className="between" style={{ width: '100%', padding: '14px 20px', border: 0, background: 'none', textAlign: 'left' }} onClick={() => setOpen({ ...open, [p.integration.code]: !open[p.integration.code] })} aria-expanded={!!open[p.integration.code]}>
                <span style={{ fontWeight: 700, fontSize: 14.5 }}>{p.integration.partner} <span className="muted" style={{ fontWeight: 500 }}>· {p.integration.name}</span></span>
                <span className="row tnum" style={{ gap: 14, fontSize: 13.5 }}><span>{p.period.count} pag.</span><span>{fmt(p.period.amount)} MT</span><span className="muted">margem {fmt(p.period.margin)} MT</span><Icon name={open[p.integration.code] ? 'chevronDown' : 'chevronRight'} size={16} /></span>
              </button>
              {open[p.integration.code] && (
                <>
                  <div className="st-row st-head" style={{ borderTop: '1px solid var(--line-3)' }}><span>Comerciante</span><span>Pag.</span><span>Valor</span><span>Taxa RIHA</span><span>Operador</span><span>Parceiro</span><span>Margem RIHA</span><span>Devido à RIHA</span></div>
                  {p.merchants.map((m: any) => (
                    <div key={m.merchant_id} className="st-row tnum" style={{ borderTop: '1px solid var(--line-3)' }}>
                      <span style={{ fontWeight: 600, minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{m.name}</span>
                      <span>{m.count}</span><span>{fmt(m.amount)}</span><span>{fmt(m.fee)}</span><span>{fmt(m.platform_fee)}</span><span>{fmt(m.partner_fee)}</span>
                      <span style={{ fontWeight: 600, color: m.margin < 0 ? 'var(--red)' : undefined }}>{fmt(m.margin)}</span><span>{fmt(m.due_to_riha)}</span>
                    </div>
                  ))}
                  {!p.merchants.length && <div className="empty">Sem pagamentos neste período.</div>}
                </>
              )}
            </div>
          ))}

          <div className="h2" style={{ marginTop: 30 }}>Recebimentos dos parceiros</div>
          <div className="table" style={{ marginTop: 12 }}>
            {data.settlements.map((s: any) => (
              <div key={s.id} className="row" style={{ padding: '11px 20px', borderTop: '1px solid var(--line-3)', gap: 12, fontSize: 13.5 }}>
                <span style={{ width: 110, color: 'var(--muted)' }}>{day(s.received_on)}</span>
                <span style={{ flex: 1, minWidth: 0 }}><b>{data.partners.find((p: any) => p.integration.code === s.integration)?.integration.partner || s.integration}</b> · <span className="mono">{s.reference}</span>{s.note && <span className="muted"> · {s.note}</span>}</span>
                <span className="tnum" style={{ fontWeight: 600 }}>{fmt(s.amount)} MT</span>
                <button className="btn btn-ghost btn-sm" aria-label="Apagar recebimento" onClick={() => del(s)}><Icon name="trash" size={15} /></button>
              </div>
            ))}
            {!data.settlements.length && <div className="empty">Ainda nenhum recebimento registado. Quando um parceiro transferir para a RIHA, registe-o aqui.</div>}
          </div>
        </>
      )}

      {record !== null && data && <RecordModal partners={data.partners} initial={record} onClose={() => setRecord(null)} onDone={() => { setRecord(null); reload() }} />}
      {edit && <DealModal integration={edit} methods={live(edit.code)?.live_methods || []} onClose={() => setEdit(null)} onDone={() => { setEdit(null); reload(); integrations.reload() }} />}
    </div>
  )
}

function RecordModal({ partners, initial, onClose, onDone }: { partners: any[]; initial: string; onClose: () => void; onDone: () => void }) {
  const g = useGateway()
  const { toast } = useSession()
  const [code, setCode] = useState(initial || partners[0]?.integration.code || '')
  const [amount, setAmount] = useState('')
  const [on, setOn] = useState(today())
  const [ref, setRef] = useState('')
  const [note, setNote] = useState('')
  const [busy, setBusy] = useState(false)
  const p = partners.find(x => x.integration.code === code)
  const save = async () => {
    setBusy(true)
    try { await g.admin.recordSettlement({ integration: code, amount: parseMT(amount), reference: ref, note, received_on: on }); toast(`Recebimento de ${fmt(parseMT(amount))} MT registado`); onDone() }
    catch (e) { toast(errMsg(e), true) } finally { setBusy(false) }
  }
  return (
    <Modal onClose={onClose}>
      <div className="display" style={{ fontSize: 22 }}>Registar recebimento</div>
      <div style={{ fontSize: 14, color: 'var(--muted)', marginTop: 6, lineHeight: 1.5 }}>Dinheiro que um parceiro transferiu para a conta da RIHA. Para corrigir um registo, apague-o ou registe um valor negativo.</div>
      <label style={{ display: 'block', marginTop: 14 }}><span className="label">Parceiro</span>
        <select className="input" value={code} onChange={e => setCode(e.target.value)}>{partners.map(x => <option key={x.integration.code} value={x.integration.code}>{x.integration.partner} · {x.integration.name}</option>)}</select>
      </label>
      {p && <div style={{ fontSize: 12.5, color: 'var(--muted)', marginTop: 6 }}>Ainda com o parceiro: {fmt(p.held)} MT</div>}
      <div className="grid-2" style={{ marginTop: 12 }}>
        <label><span className="label">Valor</span><div style={{ position: 'relative' }}><input className="input" inputMode="decimal" value={amount} onChange={e => setAmount(e.target.value.replace(/[^\d.,\s-]/g, ''))} style={{ paddingRight: 36, fontWeight: 600 }} /><span style={{ position: 'absolute', right: 12, top: 14, color: 'var(--muted)' }}>MT</span></div></label>
        <label><span className="label">Chegou a</span><input className="input" type="date" value={on} max={today()} onChange={e => setOn(e.target.value)} /></label>
      </div>
      <label style={{ display: 'block', marginTop: 12 }}><span className="label">Referência da transferência</span><input className="input" value={ref} onChange={e => setRef(e.target.value)} placeholder="Como aparece no extrato" /></label>
      <label style={{ display: 'block', marginTop: 12 }}><span className="label">Nota (opcional)</span><input className="input" value={note} onChange={e => setNote(e.target.value)} /></label>
      <div style={{ display: 'flex', gap: 10, marginTop: 18 }}>
        <button className="btn btn-outline" style={{ flex: 1 }} onClick={onClose}>Cancelar</button>
        <button className="btn btn-green" style={{ flex: 1 }} disabled={busy || !amount || !ref.trim()} onClick={save}>{busy ? <Spinner /> : 'Registar'}</button>
      </div>
    </Modal>
  )
}

function DealModal({ integration: i, methods, onClose, onDone }: { integration: any; methods: string[]; onClose: () => void; onDone: () => void }) {
  const g = useGateway()
  const { toast } = useSession()
  const pct = (bps: number | undefined) => (bps ? String(bps / 100).replace('.', ',') : '')
  const [f, setF] = useState({ name: i.name, partner: i.partner, account: i.account, note: i.note || '', share: pct(i.share_bps) || '0' })
  const shown = Array.from(new Set([...methods, ...Object.keys(i.platform_bps || {})]))
  const [ops, setOps] = useState<Record<string, string>>(Object.fromEntries(shown.map(m => [m, pct(i.platform_bps?.[m])])))
  const [busy, setBusy] = useState(false)
  const toBps = (v: string) => Math.round(parseFloat((v || '0').replace(',', '.')) * 100)
  const save = async () => {
    setBusy(true)
    try {
      await g.admin.editIntegration(i.code, { name: f.name, partner: f.partner, account: f.account, note: f.note, share_bps: toBps(f.share), platform_bps: Object.fromEntries(Object.entries(ops).map(([m, v]) => [m, toBps(v)])) })
      toast('Acordo atualizado: vale a partir do próximo pagamento'); onDone()
    } catch (e) { toast(errMsg(e), true) } finally { setBusy(false) }
  }
  const field = (k: keyof typeof f, label: string) => <label style={{ display: 'block', marginTop: 12 }}><span className="label">{label}</span><input className="input" value={f[k]} onChange={e => setF({ ...f, [k]: e.target.value })} /></label>
  const example = (() => { const fee = 6000, op = toBps(ops.mpesa || ops.emola || '0'), left = Math.max(0, fee - Math.round(100000 * op / 10000)); return Math.round(left * toBps(f.share) / 10000) })()
  return (
    <Modal onClose={onClose} width={520}>
      <div className="display" style={{ fontSize: 22 }}>Acordo com {i.partner}</div>
      <div style={{ fontSize: 13.5, color: 'var(--muted)', marginTop: 6, lineHeight: 1.5 }}>Integração <span className="mono">{i.code}</span>. Muda a partir do próximo pagamento; os já feitos mantêm a divisão da altura. Um comerciante pode ter uma parte própria na página dele.</div>
      {field('partner', 'Parceiro')}{field('name', 'Nome da integração')}{field('account', 'Conta onde o dinheiro entra')}
      <div className="grid-2" style={{ marginTop: 12 }}>
        <label><span className="label">Parte do parceiro</span><div style={{ position: 'relative' }}><input className="input" inputMode="decimal" value={f.share} onChange={e => setF({ ...f, share: e.target.value.replace(/[^\d.,]/g, '') })} style={{ paddingRight: 30, fontWeight: 600 }} /><span style={{ position: 'absolute', right: 12, top: 14, color: 'var(--muted)' }}>%</span></div></label>
        <div style={{ fontSize: 12.5, color: 'var(--muted)', alignSelf: 'end', lineHeight: 1.45, paddingBottom: 6 }}>Da taxa da RIHA que sobra depois da taxa do operador.</div>
      </div>
      {shown.length > 0 && <div className="label" style={{ marginTop: 14 }}>Taxa do operador, por método</div>}
      <div className="grid-2">
        {shown.map(m => (
          <label key={m}><span style={{ fontSize: 12.5, color: 'var(--muted)' }}>{METHOD_NAME[m] || m}</span><div style={{ position: 'relative' }}><input className="input" inputMode="decimal" placeholder="0" value={ops[m] || ''} onChange={e => setOps({ ...ops, [m]: e.target.value.replace(/[^\d.,]/g, '') })} style={{ paddingRight: 30, fontWeight: 600 }} /><span style={{ position: 'absolute', right: 12, top: 14, color: 'var(--muted)' }}>%</span></div></label>
        ))}
      </div>
      <div style={{ fontSize: 12.5, color: 'var(--muted)', marginTop: 10 }}>Exemplo: 1 000,00 MT a 6% → 60,00 MT de taxa; o parceiro leva {fmt(example)} MT.</div>
      {field('note', 'Nota (opcional)')}
      <div style={{ display: 'flex', gap: 10, marginTop: 18 }}>
        <button className="btn btn-outline" style={{ flex: 1 }} onClick={onClose}>Cancelar</button>
        <button className="btn btn-green" style={{ flex: 1 }} disabled={busy} onClick={save}>{busy ? <Spinner /> : 'Guardar acordo'}</button>
      </div>
    </Modal>
  )
}
