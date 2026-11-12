import { useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { can, errMsg } from '../../core/model'
import { useGateway } from '../../core/gateway'
import { useQuery } from '../../core/useQuery'
import { bpsLabel, fmt, METHOD_NAME, parseMT } from '../../lib/format'
import { useSession } from '../../lib/session'
import { Icon, Modal, Skeleton, Spinner } from '../../components/ui'
import { Amount, Badge, Empty, HOLDER_COLORS, KV, List, Money, MoneyMap, Page, PageHead, Panel, Section, Stat, Tip } from '../../components/ops'

// Where live money is: with each partner whose account collects it, at RIHA,
// and owed to merchants; and, for a period, how each payment's fee divided.

const iso = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
const today = () => iso(new Date())
const PERIODS: [string, string, () => [string, string]][] = [
  ['month', 'Este mês', () => { const n = new Date(); return [iso(new Date(n.getFullYear(), n.getMonth(), 1)), today()] }],
  ['last', 'Mês passado', () => { const n = new Date(); return [iso(new Date(n.getFullYear(), n.getMonth() - 1, 1)), iso(new Date(n.getFullYear(), n.getMonth(), 0))] }],
  ['all', 'Desde sempre', () => ['', '']],
]
const less = (v: number) => (v ? '−' + fmt(v) : fmt(0)) + ' MT'
const day = (s: string) => new Date(s + 'T00:00:00').toLocaleDateString('pt-PT', { day: 'numeric', month: 'short', year: 'numeric' })

function dealChips(i: any, l: any) {
  const ops = Object.entries(i.platform_bps || {}).filter(([, v]) => (v as number) > 0).map(([m, v]) => `${METHOD_NAME[m] || m} ${bpsLabel(v as number)}`)
  const out: string[] = [`${bpsLabel(i.share_bps)} da margem`, ops.length ? 'Operador ' + ops.join(', ') : 'Sem taxa do operador']
  if (l) {
    if (l.live_methods.length) out.push(l.live_methods.map((m: string) => METHOD_NAME[m]).join(' · '))
    if (l.merchants_routed) out.push(`${l.merchants_routed} encaminhado(s)`)
    if (l.merchant_deals) out.push(`${l.merchant_deals} acordo(s) próprio(s)`)
  }
  return out
}

export default function AdminSettlement() {
  const g = useGateway()
  const { toast, me } = useSession()
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
  const color = (code: string) => HOLDER_COLORS[Math.max(0, (data?.partners || []).findIndex((p: any) => p.integration.code === code)) % HOLDER_COLORS.length]
  const partnerName = (code: string) => data?.partners.find((p: any) => p.integration.code === code)?.partner_name || code

  return (
    <Page>
      <PageHead title="Liquidação" sub="Onde está o dinheiro de produção, e quanto devemos."
        actions={<>
          <a className="btn btn-outline" href={g.admin.settlementExportUrl(from || undefined, to || undefined)}><Icon name="download" size={15} />CSV</a>
          <button className="btn btn-green" onClick={() => setRecord('')}><Icon name="plus" size={15} stroke={2.2} />Registar recebimento</button>
        </>} />

      {!data ? <Skeleton h={220} /> : (
        <>
          <MoneyMap owed={pos.owed_to_merchants}
            holders={[
              ...data.partners.map((p: any) => ({ key: p.integration.code, label: p.partner_name, amount: p.held, color: color(p.integration.code) })),
              { key: 'riha', label: 'Conta RIHA', amount: pos.at_riha, color: 'var(--o-riha)' },
            ]} />
          <div className="o-stats" style={{ marginTop: 12 }}>
            <Stat tone="ink" label="Devido aos comerciantes" value={pos.owed_to_merchants} tip={`Saldos ${fmt(pos.merchant_balances)} MT e levantamentos a caminho ${fmt(pos.payouts_in_transit)} MT.`} hint={`${fmt(pos.payouts_in_transit, false)} MT a caminho`} />
            <Stat label="Com os parceiros" value={pos.held_by_partners} tip="Recebido nas contas dos parceiros, menos o que já pagaram à RIHA e aos comerciantes." hint={`${data.partners.length} parceiros`} />
            <Stat label="Na conta da RIHA" value={pos.at_riha} tone={pos.at_riha < 0 ? 'bad' : undefined}
              tip={pos.at_riha < 0 ? 'A RIHA pagou mais em levantamentos do que os recebimentos registados. Faltam recebimentos por registar?' : 'Recebido dos parceiros menos os levantamentos que a RIHA pagou. Calculado: confira com o extrato.'}
              hint={`${fmt(pos.paid_out, false)} MT pagos pela RIHA`} />
            <Stat label="Da RIHA" value={pos.riha_own} tone={pos.riha_own < 0 ? 'bad' : undefined}
              tip={pos.riha_own < 0 ? (pos.at_riha < 0 ? 'Abaixo de zero enquanto faltarem recebimentos por registar. Se não faltarem, falta dinheiro.' : 'Falta dinheiro para cobrir o que devemos.') : 'O que sobra depois de pagar tudo o que devemos: a margem.'}
              hint={pos.riha_own < 0 ? <Badge tone="bad" dot>Rever</Badge> : 'a margem acumulada'} />
          </div>
          {data.unassigned.count > 0 && (
            <div style={{ marginTop: 12 }}><Badge tone="warn" dot>{data.unassigned.count} pagamentos ({fmt(data.unassigned.amount, false)} MT) sem integração, fora destas contas</Badge></div>
          )}

          <Section title="Parceiros" sub={`${data.partners.length}`} />
          <div className="o-grid c2">
            {data.partners.map((p: any) => {
              const i = p.integration
              return (
                <Panel key={i.code} title={<span className="row" style={{ gap: 8 }}><i className="o-dot" style={{ background: color(i.code) }} />{p.partner_name}</span>} sub={`${i.name} · ${i.account}`}
                  action={can(me, 'pricing') && <button className="btn btn-ghost btn-sm" onClick={() => setEdit(i)}><Icon name="settings" size={14} />Acordo</button>}
                  foot={<>
                    <span style={{ flex: 1, fontSize: 12, color: 'var(--o-muted)', alignSelf: 'center' }}>{p.last_settlement ? `Último: ${fmt(p.last_settlement.amount, false)} MT · ${day(p.last_settlement.received_on)}` : 'Sem recebimentos'}</span>
                    <button className="btn btn-outline btn-sm" onClick={() => setRecord(i.code)}><Icon name="plus" size={13} stroke={2.2} />Recebimento</button>
                  </>}>
                  <div className="o-stat-l">Com o parceiro <Tip text="Devido à RIHA: o que entrou na conta deles e ainda não pagaram à RIHA nem aos comerciantes." /></div>
                  <div className="o-stat-v" style={{ marginTop: 2, color: p.held < 0 ? 'var(--o-bad)' : undefined }}><Money c={p.held} cents={false} /></div>
                  {p.reserved > 0 && <div style={{ marginTop: 6 }}><Badge tone="info">{fmt(p.reserved, false)} MT reservados para levantamentos</Badge></div>}
                  <div style={{ marginTop: 12 }}>
                    <KV k={`Recebido de clientes · ${p.all_time.count}`} v={<Amount c={p.all_time.amount} />} />
                    <KV k="Taxa do operador" v={less(p.all_time.platform_fee)} />
                    <KV k="Parte do parceiro" v={less(p.all_time.partner_fee)} />
                    {p.all_time.refunded > 0 && <KV k="Reembolsado" v={less(p.all_time.refunded)} />}
                    <KV k="Pago à RIHA" v={less(p.received)} />
                    <KV k="Pago a comerciantes" v={less(p.paid_out)} />
                  </div>
                  <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap', marginTop: 12 }}>{dealChips(i, live(i.code)).map(c => <span key={c} className="o-chip">{c}</span>)}</div>
                </Panel>
              )
            })}
          </div>

          <Section title="Por comerciante" sub={from ? `${day(from)} – ${day(to || today())}` : 'desde sempre'}
            actions={<>
              <div className="seg">{PERIODS.map(([k, l]) => <button key={k} className={period === k ? 'on' : ''} onClick={() => setSp({ p: k })}>{l}</button>)}</div>
              <input className="input" type="date" value={from} max={to || today()} style={{ width: 148, height: 36 }} aria-label="De" onChange={e => setSp({ p: 'custom', from: e.target.value, to })} />
              <input className="input" type="date" value={to} min={from} max={today()} style={{ width: 148, height: 36 }} aria-label="Até" onChange={e => setSp({ p: 'custom', from, to: e.target.value })} />
            </>} />
          <div className="o-stats" style={{ marginBottom: 12 }}>
            <Stat label="Pagamentos" value={String(data.period.count)} />
            <Stat label="Recebido" value={data.period.amount} />
            <Stat label="Margem da RIHA" value={data.period.margin} tone={data.period.margin < 0 ? 'bad' : undefined} />
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
            {data.partners.map((p: any) => {
              const k = p.integration.code, on = !!open[k]
              return (
                <section key={k} className="o-panel">
                  <button className="o-panel-h" style={{ width: '100%', border: 0, background: 'none', textAlign: 'left', borderBottom: on ? undefined : 0 }} onClick={() => setOpen({ ...open, [k]: !on })} aria-expanded={on}>
                    <span className="row" style={{ gap: 8, fontWeight: 700, fontSize: 14 }}><i className="o-dot" style={{ background: color(k) }} />{p.partner_name}<span className="o-t2" style={{ fontWeight: 500 }}>{p.integration.name}</span></span>
                    <span className="row" style={{ gap: 12, fontSize: 13 }}>
                      <span className="o-t2">{p.period.count} pag.</span><Amount c={p.period.amount} cents={false} />
                      <Badge tone={p.period.margin < 0 ? 'bad' : 'ok'}>margem {fmt(p.period.margin, false)}</Badge>
                      <Icon name={on ? 'chevronDown' : 'chevronRight'} size={16} />
                    </span>
                  </button>
                  {on && (
                    <List rows={p.merchants} rowKey={(m: any) => m.merchant_id}
                      empty={<Empty icon="txns" title="Sem pagamentos neste período" />}
                      cols={[
                        { head: 'Comerciante', w: 'minmax(0,1.6fr)', cell: (m: any) => <div className="o-t1">{m.name}</div> },
                        { head: 'Pag.', w: '56px', right: true, cell: (m: any) => m.count },
                        { head: 'Valor', w: 'minmax(0,1fr)', right: true, cell: (m: any) => <Amount c={m.amount} /> },
                        { head: 'Taxa RIHA', w: 'minmax(0,1fr)', right: true, cell: (m: any) => fmt(m.fee) },
                        { head: 'Operador', w: 'minmax(0,1fr)', right: true, cell: (m: any) => fmt(m.platform_fee) },
                        { head: 'Parceiro', w: 'minmax(0,1fr)', right: true, cell: (m: any) => fmt(m.partner_fee) },
                        { head: 'Margem', w: 'minmax(0,1fr)', right: true, cell: (m: any) => <b style={{ color: m.margin < 0 ? 'var(--o-bad)' : undefined }}>{fmt(m.margin)}</b> },
                        { head: 'Devido à RIHA', w: 'minmax(0,1fr)', right: true, cell: (m: any) => fmt(m.due_to_riha) },
                      ]}
                      card={(m: any) => ({
                        title: m.name, sub: `${m.count} pag. · operador ${fmt(m.platform_fee, false)} · parceiro ${fmt(m.partner_fee, false)}`, amount: <Amount c={m.amount} cents={false} />,
                        meta: <Badge tone={m.margin < 0 ? 'bad' : 'ok'}>margem {fmt(m.margin, false)}</Badge>,
                      })} />
                  )}
                </section>
              )
            })}
          </div>

          <Section title="Recebimentos dos parceiros" />
          <List rows={data.settlements} rowKey={(s: any) => s.id}
            empty={<Empty icon="bank" title="Ainda sem recebimentos" text="Registe aqui cada transferência de um parceiro para a RIHA." action={<button className="btn btn-outline btn-sm" onClick={() => setRecord('')}>Registar recebimento</button>} />}
            cols={[
              { head: 'Data', w: '120px', cell: (s: any) => <span className="o-t2">{day(s.received_on)}</span> },
              { head: 'Parceiro', w: 'minmax(0,1fr)', cell: (s: any) => <span className="row" style={{ gap: 8 }}><i className="o-dot" style={{ background: color(s.integration) }} /><span className="o-t1">{partnerName(s.integration)}</span></span> },
              { head: 'Referência', w: 'minmax(0,1.2fr)', cell: (s: any) => <><div className="o-mono">{s.reference}</div>{s.note && <div className="o-t2">{s.note}</div>}</> },
              { head: 'Valor', w: '150px', right: true, cell: (s: any) => <Amount c={s.amount} /> },
              { head: '', w: '40px', right: true, cell: (s: any) => <button className="btn btn-ghost btn-sm" style={{ width: 30, padding: 0 }} aria-label="Apagar recebimento" onClick={() => del(s)}><Icon name="trash" size={14} /></button> },
            ]}
            card={(s: any) => ({
              title: partnerName(s.integration), sub: <><span className="o-mono">{s.reference}</span> · {day(s.received_on)}</>, amount: <Amount c={s.amount} />,
              foot: <button className="btn btn-ghost btn-sm" onClick={() => del(s)}><Icon name="trash" size={14} />Apagar</button>,
            })} />
        </>
      )}

      {record !== null && data && <RecordModal partners={data.partners} initial={record} onClose={() => setRecord(null)} onDone={() => { setRecord(null); reload() }} />}
      {edit && <DealModal integration={edit} partnerName={partnerName(edit.code)} methods={live(edit.code)?.live_methods || []} onClose={() => setEdit(null)} onDone={() => { setEdit(null); reload(); integrations.reload() }} />}
    </Page>
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
        <select className="input" value={code} onChange={e => setCode(e.target.value)}>{partners.map(x => <option key={x.integration.code} value={x.integration.code}>{x.partner_name} · {x.integration.name}</option>)}</select>
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

function DealModal({ integration: i, partnerName, methods, onClose, onDone }: { integration: any; partnerName?: string; methods: string[]; onClose: () => void; onDone: () => void }) {
  const partners = useQuery(g => g.admin.partners())
  const g = useGateway()
  const { toast } = useSession()
  const pct = (bps: number | undefined) => (bps ? String(bps / 100).replace('.', ',') : '')
  const [f, setF] = useState({ name: i.name, partner_code: i.partner_code, account: i.account, note: i.note || '', share: pct(i.share_bps) || '0' })
  const shown = Array.from(new Set([...methods, ...Object.keys(i.platform_bps || {})]))
  const [ops, setOps] = useState<Record<string, string>>(Object.fromEntries(shown.map(m => [m, pct(i.platform_bps?.[m])])))
  const [busy, setBusy] = useState(false)
  const toBps = (v: string) => Math.round(parseFloat((v || '0').replace(',', '.')) * 100)
  const save = async () => {
    setBusy(true)
    try {
      await g.admin.editIntegration(i.code, { name: f.name, partner_code: f.partner_code, account: f.account, note: f.note, share_bps: toBps(f.share), platform_bps: Object.fromEntries(Object.entries(ops).map(([m, v]) => [m, toBps(v)])) })
      toast('Acordo atualizado: vale a partir do próximo pagamento'); onDone()
    } catch (e) { toast(errMsg(e), true) } finally { setBusy(false) }
  }
  const field = (k: keyof typeof f, label: string) => <label style={{ display: 'block', marginTop: 12 }}><span className="label">{label}</span><input className="input" value={f[k]} onChange={e => setF({ ...f, [k]: e.target.value })} /></label>
  const example = (() => { const fee = 6000, op = toBps(ops.mpesa || ops.emola || '0'), left = Math.max(0, fee - Math.round(100000 * op / 10000)); return Math.round(left * toBps(f.share) / 10000) })()
  return (
    <Modal onClose={onClose} width={520}>
      <div className="display" style={{ fontSize: 22 }}>Acordo com {partnerName || i.partner_code}</div>
      <div style={{ fontSize: 13.5, color: 'var(--muted)', marginTop: 6, lineHeight: 1.5 }}>Integração <span className="mono">{i.code}</span>. Muda a partir do próximo pagamento; os já feitos mantêm a divisão da altura. Um comerciante pode ter uma parte própria na página dele.</div>
      <label style={{ display: 'block', marginTop: 12 }}><span className="label">Parceiro dono da conta</span>
        <select className="input" value={f.partner_code} onChange={e => setF({ ...f, partner_code: e.target.value })}>
          {(partners.data?.data || [{ code: i.partner_code, name: partnerName }]).map((p: any) => <option key={p.code} value={p.code}>{p.name}</option>)}
          <option value="riha">RIHA</option>
        </select>
      </label>
      {field('name', 'Nome da integração')}{field('account', 'Conta onde o dinheiro entra')}
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
