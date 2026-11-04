import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { errMsg } from '../../core/model'
import { useGateway } from '../../core/gateway'
import { useSession } from '../../lib/session'
import { useQuery } from '../../core/useQuery'
import { Icon, Spinner } from '../../components/ui'
import { AuthLayout } from './Auth'

type Doc = { id: string; kind: string; file_name: string; size: number }
const DOCS: [string, string, string][] = [
  ['id_front', 'Bilhete de Identidade · frente', 'Fotografia ou PDF'],
  ['id_back', 'Bilhete de Identidade · verso', 'Fotografia ou PDF'],
  ['registration', 'Alvará ou registo comercial', 'PDF ou fotografia'],
  ['nib_proof', 'Comprovativo de NIB', 'Declaração do banco'],
]
const VOLUMES = ['Até 50 mil MT/mês', '50 – 150 mil MT/mês', '150 – 300 mil MT/mês', '300 – 600 mil MT/mês', 'Mais de 1 M MT/mês']

export const fileSize = (n: number) => (n > 1e6 ? (n / 1e6).toFixed(1).replace('.', ',') + ' MB' : Math.round(n / 1000) + ' KB')

export function DocRow({ kind, title, hint, doc, onUploaded, last }: { kind: string; title: string; hint: string; doc?: Doc; onUploaded: () => void; last?: boolean }) {
  const g = useGateway()
  const ref = useRef<HTMLInputElement>(null)
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')
  const upload = async (f: File) => {
        setBusy(true); setErr('')
    try { await g.merchant.uploadDocument(kind, f); onUploaded() } catch (e) { setErr(errMsg(e)) } finally { setBusy(false) }
  }
  return (
    <>
      <div style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '14px 16px' }}>
        <div style={{ width: 40, height: 40, borderRadius: 12, background: doc ? 'var(--green-bg)' : 'var(--paper)', color: doc ? 'var(--green-dark)' : 'var(--ink)', display: 'grid', placeItems: 'center', flex: 'none' }}>
          <Icon name={doc ? 'check' : 'doc'} size={18} stroke={doc ? 2.4 : 1.9} />
        </div>
        <div style={{ flex: 1, minWidth: 0 }}>
          <div style={{ fontWeight: 600, fontSize: 14.5 }}>{title}</div>
          <div style={{ fontSize: 12.5, color: err ? 'var(--red-dark)' : 'var(--muted)', marginTop: 1, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{err || (doc ? `${doc.file_name} · ${fileSize(doc.size)}` : hint)}</div>
        </div>
        <input ref={ref} type="file" accept=".pdf,.jpg,.jpeg,.png" hidden onChange={e => { const f = e.target.files?.[0]; if (f) upload(f); e.target.value = '' }} />
        {doc
          ? <button type="button" onClick={() => ref.current?.click()} style={{ border: 0, background: 'none', fontSize: 13, color: 'var(--muted)', fontWeight: 600 }}>{busy ? <Spinner dark /> : 'Trocar'}</button>
          : <button type="button" onClick={() => ref.current?.click()} className="btn btn-ink" style={{ height: 34, padding: '0 12px', borderRadius: 10, fontSize: 13 }}>{busy ? <Spinner /> : <><Icon name="upload" size={14} stroke={2.2} />Carregar</>}</button>}
      </div>
      {!last && <div style={{ height: 1, background: '#EEECE5', marginLeft: 68 }} />}
    </>
  )
}

export default function Kyc() {
  const g = useGateway()
  const nav = useNavigate()
  const { refresh, toast } = useSession()
  const kyc = useQuery(g => g.merchant.kyc())
  const bank = useQuery(g => g.merchant.bankAccount())
  const [f, setF] = useState({ business_type: 'individual', nuit: '', legal_name: '', city: '', expected_volume: VOLUMES[0] })
  const [b, setB] = useState({ bank_name: 'BCI', holder: '', nib: '' })
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState(false)

  const inited = useRef(false)
  useEffect(() => {
    const k = kyc.data
    if (k && !inited.current) inited.current = true
    else return
    setF(x => ({ ...x, business_type: k.business_type || 'individual', nuit: k.nuit || '', legal_name: k.legal_name || '', city: k.city || '', expected_volume: k.expected_volume || VOLUMES[0] }))
  }, [kyc.data])
  const bankInited = useRef(false)
  useEffect(() => { const a = bank.data?.account; if (a && !bankInited.current) { bankInited.current = true; setB({ bank_name: a.bank_name, holder: a.holder, nib: a.nib }) } }, [bank.data])

  const docs: Doc[] = kyc.data?.documents || []
  const nuitDigits = f.nuit.replace(/\D/g, '')
  const nuitOk = nuitDigits.length === 9
  const locked = kyc.data && ['active', 'suspended'].includes(kyc.data.status)

  const submit = async () => {
    setBusy(true); setErr('')
    try {
      await g.merchant.saveKyc({ ...f, nuit: nuitDigits })
      if (b.nib.replace(/\D/g, '')) await g.merchant.saveBankAccount({ ...b, nib: b.nib.replace(/\D/g, '') })
      await g.merchant.submitKyc()
      await refresh()
      toast('Verificação submetida. Avisamos por SMS.')
      nav('/painel')
    } catch (e) { setErr(errMsg(e)) } finally { setBusy(false) }
  }

  if (locked) return (
    <AuthLayout back={() => nav('/painel')}>
      <h1 style={{ margin: '26px 0 0', fontFamily: 'var(--display)', fontWeight: 700, fontSize: 32, letterSpacing: '-0.03em' }}>Conta verificada</h1>
      <p style={{ color: 'var(--muted)' }}>A verificação do seu negócio já está concluída.</p>
      <button className="btn btn-green btn-lg" onClick={() => nav('/painel')}>Abrir o painel</button>
    </AuthLayout>
  )

  return (
    <AuthLayout step={[2, 2]} back={() => nav('/painel')}>
      <h1 style={{ margin: '26px 0 0', fontFamily: 'var(--display)', fontWeight: 700, fontSize: 32, lineHeight: 1.05, letterSpacing: '-0.03em' }}>Verifique o seu negócio</h1>
      <p style={{ margin: '8px 0 0', fontSize: 15, lineHeight: 1.5, color: 'var(--muted)' }}>A sua conta fica como lead até a equipa RIHA aprovar. Os documentos são opcionais agora; pode enviá-los mais tarde ou quando a equipa pedir.</p>
      {kyc.data?.status === 'info_requested' && <div className="form-err" style={{ marginTop: 14, background: 'var(--sun-soft)', color: '#5B4600' }}>A equipa RIHA pediu mais documentos{kyc.data.review_note ? `: ${kyc.data.review_note}` : '.'}</div>}
      <span className="label" style={{ margin: '24px 0 8px' }}>Tipo de negócio</span>
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 6, padding: 5, borderRadius: 16, background: 'var(--paper-2)' }}>
        {[['individual', 'Em nome individual'], ['company', 'Empresa (Lda, SA)']].map(([k, l]) => (
          <button key={k} type="button" onClick={() => setF({ ...f, business_type: k })} style={{ height: 44, borderRadius: 12, border: 0, background: f.business_type === k ? '#fff' : 'transparent', boxShadow: f.business_type === k ? '0 1px 3px rgba(0,0,0,.12)' : 'none', fontWeight: 600, fontSize: 14 }}>{l}</button>
        ))}
      </div>
      <label style={{ marginTop: 16 }}>
        <span className="label">NUIT</span>
        <div style={{ position: 'relative' }}>
          <input className="input lg mono" inputMode="numeric" value={f.nuit} onChange={e => setF({ ...f, nuit: e.target.value })} placeholder="400 123 456" maxLength={11} />
          {nuitDigits && <span className="pill" style={{ position: 'absolute', right: 12, top: 15, background: nuitOk ? 'var(--green-bg)' : 'var(--sun-bg)', color: nuitOk ? 'var(--green-dark)' : 'var(--sun-text)' }}>{nuitOk ? 'Válido' : '9 dígitos'}</span>}
        </div>
      </label>
      <div className="grid-2" style={{ marginTop: 14, gap: 12 }}>
        <label><span className="label">{f.business_type === 'company' ? 'Razão social' : 'Nome completo'}</span><input className="input" value={f.legal_name} onChange={e => setF({ ...f, legal_name: e.target.value })} placeholder={f.business_type === 'company' ? 'Café Maputo, Lda' : 'Ana Mabota'} /></label>
        <label><span className="label">Cidade</span><input className="input" value={f.city} onChange={e => setF({ ...f, city: e.target.value })} placeholder="Maputo" /></label>
      </div>
      <label style={{ marginTop: 14 }}><span className="label">Volume esperado</span><select className="input" value={f.expected_volume} onChange={e => setF({ ...f, expected_volume: e.target.value })}>{VOLUMES.map(v => <option key={v}>{v}</option>)}</select></label>

      <span className="label" style={{ margin: '20px 0 8px' }}>Documentos <span style={{ color: 'var(--muted)', fontWeight: 400 }}>(opcional)</span></span>
      <div className="card" style={{ borderRadius: 20, overflow: 'hidden' }}>
        {DOCS.map(([k, t, h], i) => <DocRow key={k} kind={k} title={t} hint={h} doc={docs.find(d => d.kind === k)} onUploaded={kyc.reload} last={i === DOCS.length - 1} />)}
      </div>

      <span className="label" style={{ margin: '20px 0 8px' }}>Conta para levantamentos</span>
      <div className="card" style={{ borderRadius: 20, padding: 16, display: 'flex', flexDirection: 'column', gap: 12 }}>
        <div className="grid-2" style={{ gap: 12 }}>
          <label><span className="label">Banco</span><select className="input" value={b.bank_name} onChange={e => setB({ ...b, bank_name: e.target.value })}>{(bank.data?.banks || ['BCI']).map((x: string) => <option key={x}>{x}</option>)}</select></label>
          <label><span className="label">Titular</span><input className="input" value={b.holder} onChange={e => setB({ ...b, holder: e.target.value })} placeholder={f.legal_name || 'Nome do titular'} /></label>
        </div>
        <label><span className="label">NIB (21 dígitos)</span><input className="input mono" inputMode="numeric" value={b.nib} onChange={e => setB({ ...b, nib: e.target.value })} placeholder="0008 0000 4821 1301 0113 5" /></label>
      </div>

      <div style={{ display: 'flex', gap: 10, marginTop: 14, padding: '12px 14px', borderRadius: 14, background: 'var(--sun-soft)', fontSize: 13, lineHeight: 1.45, color: '#5B4600' }}><Icon name="clock" size={18} stroke={2} />A verificação demora até 24 horas. Avisamos por SMS.</div>
      {err && <div className="form-err" style={{ marginTop: 14 }}>{err}</div>}
      <div style={{ flex: 1, minHeight: 24 }} />
      <button className="btn btn-green btn-lg" onClick={submit} disabled={busy || !nuitOk}>{busy ? <Spinner /> : 'Submeter e abrir o painel'}</button>
      <button className="btn btn-ghost" style={{ height: 44, marginTop: 8 }} onClick={() => nav('/painel')}>Fazer mais tarde</button>
    </AuthLayout>
  )
}
