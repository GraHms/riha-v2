import { useEffect, useMemo, useState } from 'react'
import { Icon, Modal, Spinner } from '../../../components/ui'
import { useGateway } from '../../../core/gateway'
import { errMsg, type ApiError } from '../../../core/model'
import { useSession } from '../../../lib/session'
import { money } from './common'

export default function AddMembers({ initial, planId, plans, onClose, onDone }: { initial: 'one' | 'import'; planId?: string; plans: any[]; onClose: () => void; onDone: () => void }) {
  const [tab, setTab] = useState(initial)
  return (
    <Modal onClose={onClose} width={720}>
      <div className="between" style={{ gap: 12, flexWrap: 'wrap' }}>
        <div className="display" style={{ fontSize: 22 }}>Adicionar membros</div>
        <div className="seg"><button className={tab === 'one' ? 'on' : ''} onClick={() => setTab('one')}>Um membro</button><button className={tab === 'import' ? 'on' : ''} onClick={() => setTab('import')}>Importar lista</button></div>
      </div>
      {plans.length === 0 ? <div style={{ padding: '24px 0', color: 'var(--muted)' }}>Crie e ative um plano primeiro.</div>
        : tab === 'one' ? <OneMember plans={plans} planId={planId} onDone={onDone} onClose={onClose} /> : <ImportList plans={plans} planId={planId} onDone={onDone} />}
    </Modal>
  )
}

function OneMember({ plans, planId, onDone, onClose }: { plans: any[]; planId?: string; onDone: () => void; onClose: () => void }) {
  const g = useGateway()
  const { toast } = useSession()
  const [ids, setIds] = useState<string[]>([planId || plans[0].id])
  const [phone, setPhone] = useState('')
  const [payer, setPayer] = useState('')
  const [fields, setFields] = useState<Record<string, string>>({})
  const [first, setFirst] = useState('')
  const [periods, setPeriods] = useState<any[]>([])
  const [channel, setChannel] = useState('sms')
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState<ApiError | null>(null)
  const chosen = ids.map(id => plans.find(p => p.id === id)).filter(Boolean)
  const allFields = useMemo(() => { const s: string[] = []; chosen.forEach((p: any) => p.fields.forEach((f: string) => !s.includes(f) && s.push(f))); return s }, [ids])
  useEffect(() => { g.subscriptions.plan(ids[0]).then(p => { setPeriods(p.first_periods); setFirst(p.first_periods[0]?.key || '') }).catch(() => {}) }, [ids[0]])
  const total = chosen.reduce((a: number, p: any) => a + p.amount, 0)
  const send = async () => {
    setBusy(true); setErr(null)
    try {
      await g.subscriptions.invite({ plan_ids: ids, phone, payer_name: payer, fields, first_period: first, channel, name: allFields.length ? '' : payer })
      toast('Convite enviado'); onDone()
    } catch (e) { setErr(e as ApiError) } finally { setBusy(false) }
  }
  return (
    <div style={{ marginTop: 18 }}>
      <span className="label">Plano</span>
      {ids.map((id, i) => (
        <div key={i} style={{ display: 'flex', gap: 8, marginBottom: 8 }}>
          <select className="input" value={id} onChange={e => setIds(ids.map((x, j) => (j === i ? e.target.value : x)))} style={{ flex: 1 }}>
            {plans.map(p => <option key={p.id} value={p.id} disabled={ids.includes(p.id) && p.id !== id}>{p.name} · {money(p.amount)} MT · {p.schedule_label}</option>)}
          </select>
          {i > 0 && <button className="icon-btn" onClick={() => setIds(ids.filter((_, j) => j !== i))} aria-label="Remover plano"><Icon name="close" size={14} /></button>}
        </div>
      ))}
      {ids.length < Math.min(plans.length, 5) && <button className="link-btn" style={{ fontSize: 13 }} onClick={() => setIds([...ids, plans.find(p => !ids.includes(p.id)).id])}>+ Juntar outro plano, por exemplo transporte</button>}
      <div className="grid-2" style={{ gap: 12, marginTop: 16 }}>
        <label><span className="label">Número de quem paga</span><div className={'phone-input' + (err?.param === 'phone' ? ' err' : '')}><span className="cc">+258</span><input inputMode="tel" value={phone} onChange={e => setPhone(e.target.value)} placeholder="84 123 4567" /></div></label>
        <label><span className="label">Quem paga <span style={{ fontWeight: 400, color: 'var(--muted)' }}>{allFields.length ? '(opcional)' : ''}</span></span><input className="input" value={payer} onChange={e => setPayer(e.target.value)} placeholder="Tânia Mabunda" /></label>
        {allFields.map(f => <label key={f}><span className="label">{f}</span><input className={'input' + (err?.param === 'fields.' + f ? ' err' : '')} value={fields[f] || ''} onChange={e => setFields({ ...fields, [f]: e.target.value })} /></label>)}
        <label><span className="label">Primeiro período</span><select className="input" value={first} onChange={e => setFirst(e.target.value)}>{periods.map(p => <option key={p.key} value={p.key}>{p.label} · {p.due}</option>)}</select></label>
        <label><span className="label">Enviar por</span><div className="seg"><button type="button" className={channel === 'sms' ? 'on' : ''} onClick={() => setChannel('sms')}>SMS</button><button type="button" className={channel === 'whatsapp' ? 'on' : ''} onClick={() => setChannel('whatsapp')}>WhatsApp</button></div></label>
      </div>
      {err && <div className="form-err" style={{ marginTop: 14 }}>{err.message}</div>}
      <div className="between" style={{ marginTop: 20, gap: 12, flexWrap: 'wrap' }}>
        <div style={{ fontSize: 13, color: 'var(--muted)', maxWidth: 380, lineHeight: 1.5 }}>Enviamos um {channel === 'whatsapp' ? 'WhatsApp' : 'SMS'} para aceitar e pagar o primeiro período ({money(total)} MT). Só fica ativo depois disso.</div>
        <div style={{ display: 'flex', gap: 8 }}><button className="btn btn-outline" onClick={onClose}>Cancelar</button><button className="btn btn-green" disabled={busy} onClick={send}>{busy ? <Spinner /> : 'Enviar convite'}</button></div>
      </div>
    </div>
  )
}

function ImportList({ plans, planId, onDone }: { plans: any[]; planId?: string; onDone: () => void }) {
  const g = useGateway()
  const { toast } = useSession()
  const [file, setFile] = useState<File | null>(null)
  const [plan, setPlan] = useState(planId || (plans.length === 1 ? plans[0].id : ''))
  const [pv, setPv] = useState<any>(null)
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')
  useEffect(() => {
    if (!file) return
    setBusy(true); setErr('')
    g.subscriptions.previewImport(file, plan).then(setPv).catch(e => { setPv(null); setErr(errMsg(e)) }).finally(() => setBusy(false))
  }, [file, plan])
  const go = async () => {
    if (!file) return
    setBusy(true); setErr('')
    try { const r = await g.subscriptions.importMembers(file, plan); toast(`${r.invited} convites enviados${r.skipped ? `, ${r.skipped} ignorados` : ''}`); onDone() }
    catch (e) { setErr(errMsg(e)) } finally { setBusy(false) }
  }
  const cols: string[] = pv ? Array.from(new Set(pv.rows.flatMap((r: any) => Object.keys(r.fields)))) as string[] : []
  return (
    <div style={{ marginTop: 18 }}>
      <label style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '12px 14px', borderRadius: 14, border: file ? '1px solid var(--line)' : '1.5px dashed #D6D4CB', background: file ? '#fff' : '#FAFAF7', cursor: 'pointer' }}>
        <span style={{ width: 38, height: 38, borderRadius: 10, background: 'var(--green-bg)', color: 'var(--green-dark)', fontSize: 11, fontWeight: 800, display: 'grid', placeItems: 'center' }}>{file ? (file.name.toLowerCase().endsWith('.csv') ? 'CSV' : 'XLS') : <Icon name="upload" size={17} />}</span>
        <span style={{ flex: 1, minWidth: 0 }}>{file ? <><b style={{ display: 'block' }}>{file.name}</b><span style={{ fontSize: 12.5, color: 'var(--muted)' }}>{pv ? `${pv.rows.length} linhas · colunas reconhecidas: ${pv.columns.join(', ')}` : 'A ler…'}</span></> : <><b style={{ display: 'block' }}>Escolher folha .xlsx ou .csv</b><span style={{ fontSize: 12.5, color: 'var(--muted)' }}>Colunas: nome, número de telefone, plano e os campos do plano (por exemplo Turma).</span></>}</span>
        {file && <span className="link-btn" style={{ fontSize: 13 }}>Trocar</span>}
        <input type="file" accept=".xlsx,.csv" hidden onChange={e => setFile(e.target.files?.[0] || null)} />
      </label>
      <label style={{ display: 'flex', alignItems: 'center', gap: 10, marginTop: 10, fontSize: 13.5 }}>Plano quando a folha não diz
        <select className="input" style={{ height: 36, width: 'auto', flex: 1 }} value={plan} onChange={e => setPlan(e.target.value)}><option value="">Nenhum</option>{plans.map(p => <option key={p.id} value={p.id}>{p.name}</option>)}</select>
      </label>
      {busy && !pv && <div style={{ padding: 20, textAlign: 'center' }}><Spinner dark /></div>}
      {pv && (
        <>
          <div className="table" style={{ marginTop: 12, maxHeight: 280, overflow: 'auto', borderRadius: 14 }}>
            <div className="thead" style={{ gridTemplateColumns: `1.3fr ${cols.slice(1).map(() => '80px').join(' ')} 120px 1.3fr`, position: 'sticky', top: 0 }}><span>Nome</span>{cols.slice(1).map(c => <span key={c}>{c}</span>)}<span>Número</span><span>Plano</span></div>
            {pv.rows.map((r: any) => (
              <div key={r.line} className="trow" style={{ gridTemplateColumns: `1.3fr ${cols.slice(1).map(() => '80px').join(' ')} 120px 1.3fr`, background: r.error ? '#FFF1F0' : undefined, fontSize: 13.5 }} title={r.error || ''}>
                <span>{r.name || '—'}</span>{cols.slice(1).map(c => <span key={c}>{r.fields[c] || '—'}</span>)}
                <span style={{ color: r.error === 'Número inválido' ? 'var(--red-dark)' : undefined, fontWeight: r.error === 'Número inválido' ? 700 : 400 }}>{r.phone}</span>
                <span style={{ color: r.error && r.error !== 'Número inválido' ? 'var(--red-dark)' : undefined }}>{r.error && r.error !== 'Número inválido' ? r.error : r.plan_label}</span>
              </div>
            ))}
          </div>
          <div style={{ fontSize: 13, marginTop: 10 }}><b style={{ color: 'var(--green-text)' }}>{pv.ready} prontos</b>{pv.invalid > 0 && <b style={{ color: 'var(--red-dark)', marginLeft: 12 }}>{pv.invalid} com erros · ficam de fora</b>}</div>
        </>
      )}
      {err && <div className="form-err" style={{ marginTop: 12 }}>{err}</div>}
      <div className="between" style={{ marginTop: 18, gap: 12, flexWrap: 'wrap' }}>
        <div style={{ fontSize: 13, color: 'var(--muted)', maxWidth: 360, lineHeight: 1.5 }}>Cada pessoa recebe um SMS para aceitar e pagar o primeiro período.</div>
        <button className="btn btn-green" disabled={busy || !pv || pv.ready === 0} onClick={go}>{busy && pv ? <Spinner /> : `Enviar ${pv?.ready || 0} convites`}</button>
      </div>
    </div>
  )
}
