import { useEffect, useState, type ReactNode } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { api, errMsg } from '../../lib/api'
import { useData } from '../../lib/useData'
import { ago, copy, fullDate, initials, shortDate } from '../../lib/format'
import { useSession } from '../../lib/session'
import { Avatar, Check, Icon, Modal, Spinner, Stripes, Toggle } from '../../components/ui'
import { DocRow } from '../public/Kyc'
import { ModeSwitch, STATUS_TEXT, useShell } from './AppShell'

const TABS: [string, string][] = [['negocio', 'Negócio'], ['banco', 'Conta bancária'], ['api', 'Chaves API'], ['webhooks', 'Webhooks e URLs'], ['conta', 'Conta e segurança']]

export default function Settings() {
  const { tab } = useParams()
  const nav = useNavigate()
  const { merchant } = useShell()
  const { logout } = useSession()
  const t = tab || 'negocio'
  return (
    <div className="page">
      {/* Mobile menu (2n) when no tab is chosen */}
      {!tab && (
        <div className="only-mobile">
          <h1 className="h1">Definições</h1>
          <div className="card row" style={{ marginTop: 16, padding: 16, borderRadius: 22 }}>
            <Avatar text={initials(merchant.name)} color={merchant.avatar_color} size={52} radius={16} font={17} />
            <div style={{ flex: 1, minWidth: 0 }}><div style={{ fontWeight: 600, fontSize: 16.5 }}>{merchant.legal_name || merchant.name}</div><div style={{ fontSize: 13, color: 'var(--muted)', marginTop: 2 }}>{merchant.nuit ? 'NUIT ' + merchant.nuit.replace(/(\d{3})(\d{3})(\d{3})/, '$1 $2 $3') : 'NUIT por indicar'}</div></div>
            <span className="pill" style={{ background: merchant.status === 'active' ? 'var(--green-bg)' : 'var(--sun-bg)', color: merchant.status === 'active' ? 'var(--green-dark)' : 'var(--sun-text)', fontSize: 12, padding: '5px 9px' }}>{STATUS_TEXT[merchant.status]}</span>
          </div>
          <MenuGroup title="Negócio" items={[['store', 'Perfil do negócio', 'negocio'], ['bank', 'Conta bancária', 'banco'], ['link', 'Links de pagamento', '/painel/links']]} />
          <MenuGroup title="Programadores" dark items={[['key', 'Chaves API', 'api'], ['webhook', 'Webhooks e URLs', 'webhooks'], ['doc', 'Documentação', '/docs']]} />
          <MenuGroup title="Conta" items={[['lock', 'Palavra-passe e segurança', 'conta']]} />
          <button className="btn btn-danger btn-block" style={{ marginTop: 18, height: 50, borderRadius: 15, fontSize: 15 }} onClick={async () => { await logout(); nav('/') }}>Terminar sessão</button>
        </div>
      )}
      <div className={tab ? '' : 'only-desktop'}>
        <div className="row">
          {tab && <button className="icon-btn only-mobile" style={{ width: 42, height: 42, borderRadius: 14 }} onClick={() => nav('/painel/definicoes')} aria-label="Voltar"><Icon name="arrowLeft" size={20} stroke={2} /></button>}
          <h1 className="h1" style={{ flex: 1 }}>Definições</h1>
        </div>
        <div className="tabs" style={{ marginTop: 18 }}>
          {TABS.map(([k, l]) => <button key={k} className={t === k ? 'on' : ''} onClick={() => nav('/painel/definicoes/' + k)}>{l}</button>)}
        </div>
        <div style={{ marginTop: 22 }}>
          {t === 'negocio' && <Business />}
          {t === 'banco' && <Bank />}
          {t === 'api' && <Keys />}
          {t === 'webhooks' && <Hooks />}
          {t === 'conta' && <Account />}
        </div>
      </div>
    </div>
  )
}

function MenuGroup({ title, items, dark }: { title: string; items: [string, string, string][]; dark?: boolean }) {
  const nav = useNavigate()
  return (
    <>
      <div className="eyebrow" style={{ margin: '22px 4px 8px', fontSize: 12.5 }}>{title}</div>
      <div className="card" style={{ borderRadius: 20, overflow: 'hidden' }}>
        {items.map(([ic, l, to], i) => (
          <div key={to}>
            <button onClick={() => nav(to.startsWith('/') ? to : '/painel/definicoes/' + to)} style={{ width: '100%', display: 'flex', alignItems: 'center', gap: 12, padding: '14px 16px', border: 0, background: '#fff', textAlign: 'left' }}>
              <div style={{ width: 38, height: 38, borderRadius: 11, background: dark ? 'var(--ink)' : 'var(--paper)', color: dark ? '#fff' : 'var(--ink)', display: 'grid', placeItems: 'center' }}><Icon name={ic} size={18} /></div>
              <div style={{ flex: 1, fontWeight: 600, fontSize: 15 }}>{l}</div><Icon name="chevronRight" size={16} stroke={2} color="#8A938E" />
            </button>
            {i < items.length - 1 && <div style={{ height: 1, background: '#EEECE5', marginLeft: 66 }} />}
          </div>
        ))}
      </div>
    </>
  )
}

function Card({ children, style }: { children: ReactNode; style?: React.CSSProperties }) {
  return <div className="card" style={{ padding: 22, ...style }}>{children}</div>
}

// ------------------------------------------------------------------ business
function Business() {
  const { me, refresh, toast } = useSession()
  const m = me!.merchant!
  const kyc = useData<any>('/merchant/kyc')
  const [f, setF] = useState({ name: m.name, legal_name: m.legal_name, phone: m.phone, city: m.city, statement_descriptor: m.statement_descriptor, sms_notifications: m.sms_notifications })
  const [busy, setBusy] = useState(false)
  const save = async () => {
    setBusy(true)
    try { await api('/merchant', { method: 'PATCH', body: f }); await refresh(); toast('Perfil guardado') } catch (e) { toast(errMsg(e), true) } finally { setBusy(false) }
  }
  const docs: any[] = kyc.data?.documents || []
  const has = (k: string) => docs.some(d => d.kind === k)
  const active = m.status === 'active'
  return (
    <div className="set-grid">
      <style>{`.set-grid{display:grid;grid-template-columns:minmax(0,1fr) 360px;gap:16px;align-items:start}@media(max-width:1100px){.set-grid{grid-template-columns:1fr}}`}</style>
      <Card>
        <div style={{ fontWeight: 600, fontSize: 16 }}>Perfil do negócio</div>
        <div className="grid-2" style={{ gap: 14, marginTop: 18 }}>
          <label><span className="label">Nome comercial</span><input className="input" value={f.name} onChange={e => setF({ ...f, name: e.target.value })} /></label>
          <label><span className="label">Razão social</span><input className="input" value={f.legal_name} onChange={e => setF({ ...f, legal_name: e.target.value })} /></label>
          <label><span className="label">NUIT</span><input className="input mono" value={m.nuit ? m.nuit.replace(/(\d{3})(\d{3})(\d{3})/, '$1 $2 $3') : ''} disabled placeholder="Indique na verificação" style={{ background: '#FAFAF7' }} /></label>
          <label><span className="label">Telemóvel</span><input className="input" value={f.phone} onChange={e => setF({ ...f, phone: e.target.value })} /></label>
          <label><span className="label">Cidade</span><input className="input" value={f.city} onChange={e => setF({ ...f, city: e.target.value })} /></label>
          <label><span className="label">Nome no extrato do cliente</span><input className="input mono" maxLength={22} value={f.statement_descriptor} onChange={e => setF({ ...f, statement_descriptor: e.target.value.toUpperCase() })} /></label>
        </div>
        <div className="row" style={{ marginTop: 16, padding: '12px 0', borderTop: '1px solid var(--line-3)' }}><div style={{ flex: 1 }}><div style={{ fontWeight: 600, fontSize: 14 }}>Notificações por SMS</div><div style={{ fontSize: 12.5, color: 'var(--muted)' }}>Avisos de pagamentos e levantamentos</div></div><Toggle on={f.sms_notifications} onChange={v => setF({ ...f, sms_notifications: v })} label="SMS" /></div>
        <div className="between" style={{ marginTop: 10, flexWrap: 'wrap' }}>
          <span style={{ fontSize: 13, color: 'var(--muted)' }}>Taxa por transação: <b style={{ color: 'var(--ink)' }}>{m.fee_label}</b>{m.fee_negotiated ? ' (negociada)' : ''}</span>
          <button className="btn btn-green" onClick={save} disabled={busy}>{busy ? <Spinner /> : 'Guardar'}</button>
        </div>
      </Card>
      <Card>
        <div className="between"><div style={{ fontWeight: 600, fontSize: 16 }}>Verificação</div><span className="pill" style={{ fontSize: 12, padding: '4px 9px', background: active ? 'var(--green-bg)' : 'var(--sun-bg)', color: active ? 'var(--green-dark)' : 'var(--sun-text)' }}>{active ? 'Aprovada' : STATUS_TEXT[m.status]}</span></div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 12, marginTop: 16, fontSize: 14 }}>
          <div className="row" style={{ gap: 10 }}><Check ok={!!m.nuit} />NUIT {m.nuit ? 'indicado' : 'em falta'}</div>
          <div className="row" style={{ gap: 10 }}><Check ok={has('id_front')} />Bilhete de Identidade</div>
          <div className="row" style={{ gap: 10 }}><Check ok={has('registration')} />Certidão de registo comercial</div>
          <div className="row" style={{ gap: 10 }}><Check ok={has('nib_proof')} />Comprovativo de NIB</div>
        </div>
        <div style={{ fontSize: 12.5, color: 'var(--muted)', marginTop: 16 }}>{m.approved_at ? `Aprovada a ${shortDate(m.approved_at)} ${new Date(m.approved_at).getFullYear()}` : m.kyc_submitted_at ? `Submetida a ${shortDate(m.kyc_submitted_at)}` : 'Ainda não submetida'}</div>
        {!active && <a href="/verificacao" className="btn btn-outline btn-block" style={{ marginTop: 14 }}>Completar verificação</a>}
      </Card>
    </div>
  )
}

// ------------------------------------------------------------------ bank
function Bank() {
  const { me, toast } = useSession()
  const m = me!.merchant!
  const { data, setData } = useData<any>('/bank-account')
  const kyc = useData<any>('/merchant/kyc')
  const [f, setF] = useState({ bank_name: 'BCI', holder: m.legal_name || m.name, nib: '' })
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')
  useEffect(() => { const a = data?.account; if (a) setF({ bank_name: a.bank_name, holder: a.holder, nib: a.nib.replace(/(\d{4})(?=\d)/g, '$1 ') }) }, [data])
  const a = data?.account
  const nib = f.nib.replace(/\D/g, '')
  const save = async () => {
    setBusy(true); setErr('')
    try { const r = await api<any>('/bank-account', { method: 'PUT', body: { ...f, nib } }); setData(r); toast('Conta bancária guardada') } catch (e) { setErr(errMsg(e)) } finally { setBusy(false) }
  }
  const proof = (kyc.data?.documents || []).find((d: any) => d.kind === 'nib_proof')
  return (
    <div className="bank-grid">
      <style>{`.bank-grid{display:grid;grid-template-columns:380px minmax(0,1fr);gap:16px;align-items:start}@media(max-width:1100px){.bank-grid{grid-template-columns:1fr}}`}</style>
      <div className="dark-card" style={{ height: 220, borderRadius: 24, padding: 24, display: 'flex', flexDirection: 'column' }}>
        <div className="stripe g" style={{ top: -30, left: 170, width: 34, height: 320 }} /><div className="stripe r" style={{ top: -30, left: 216, width: 34, height: 320 }} />
        <div style={{ position: 'absolute', inset: 0, background: 'linear-gradient(180deg,rgba(11,18,14,.2),rgba(11,18,14,.88) 70%)' }} />
        <div className="between" style={{ position: 'relative' }}><span className="display" style={{ fontWeight: 800, fontSize: 24 }}>{a?.bank_name || f.bank_name}</span>{a && <span className="pill" style={{ fontSize: 12, padding: '5px 10px', background: a.verified ? 'rgba(142,224,127,.16)' : 'rgba(255,210,31,.16)', color: a.verified ? 'var(--lime)' : 'var(--sun)' }}>{a.verified ? 'Verificada' : 'Por verificar'}</span>}</div>
        <div style={{ flex: 1 }} />
        <div className="mono" style={{ position: 'relative', fontSize: 16, letterSpacing: '.08em' }}>{a ? `${a.nib.slice(0, 4)} •••• •••• •••• ${a.nib.slice(16, 20)} ${a.nib.slice(20)}` : 'NIB por indicar'}</div>
        <div className="between" style={{ position: 'relative', marginTop: 8, fontSize: 13, color: '#A9B5AE' }}><span>{a?.holder || f.holder}</span><span>Conta principal</span></div>
      </div>
      <Card>
        <div style={{ fontWeight: 600, fontSize: 16 }}>Dados da conta para levantamentos</div>
        <div className="grid-2" style={{ gap: 14, marginTop: 18 }}>
          <label><span className="label">Banco</span><select className="input" value={f.bank_name} onChange={e => setF({ ...f, bank_name: e.target.value })}>{(data?.banks || []).map((b: string) => <option key={b}>{b}</option>)}</select></label>
          <label><span className="label">Titular</span><input className="input" value={f.holder} onChange={e => setF({ ...f, holder: e.target.value })} /></label>
          <label style={{ gridColumn: '1 / -1' }}><span className="label">NIB (21 dígitos)</span><input className={'input mono' + (nib && nib.length !== 21 ? ' err' : '')} inputMode="numeric" value={f.nib} onChange={e => setF({ ...f, nib: e.target.value.replace(/[^\d\s]/g, '').slice(0, 27) })} placeholder="0008 0000 4821 1301 0113 5" />{nib && nib.length !== 21 && <div className="field-err">{nib.length}/21 dígitos</div>}</label>
        </div>
        <div style={{ marginTop: 14, borderRadius: 14, border: '1.5px dashed #D6D4CB', background: '#FAFAF7' }}>
          <DocRow kind="nib_proof" title="Comprovativo de NIB" hint="Declaração do banco, PDF ou fotografia" doc={proof} onUploaded={kyc.reload} last />
        </div>
        {err && <div className="form-err" style={{ marginTop: 12 }}>{err}</div>}
        <div className="between" style={{ marginTop: 18, flexWrap: 'wrap' }}><span style={{ fontSize: 13, color: 'var(--muted)', flex: 1, minWidth: 220 }}>O titular tem de ter o mesmo nome ou NUIT registado na Riha. Alterações ficam registadas.</span><button className="btn btn-green" onClick={save} disabled={busy || nib.length !== 21}>{busy ? <Spinner /> : 'Guardar'}</button></div>
      </Card>
    </div>
  )
}

// ------------------------------------------------------------------ API keys
function Keys() {
  const { live, toast } = useSession()
  const { merchant } = useShell()
  const { data, reload } = useData<any>('/keys', [live])
  const logs = useData<any>('/api-logs', [live], 10000)
  const [fresh, setFresh] = useState<string | null>(null)
  const [reveal, setReveal] = useState(false)
  const [copied, setCopied] = useState('')
  const [confirm, setConfirm] = useState(false)
  const [busy, setBusy] = useState(false)
  useEffect(() => { setFresh(null); setReveal(false) }, [live])
  const doCopy = async (k: string, v: string) => { await copy(v); setCopied(k); setTimeout(() => setCopied(''), 1500) }
  const roll = async () => {
    setBusy(true)
    try { const r = await api<any>('/keys/roll', { method: 'POST' }); setFresh(r.secret_key); setReveal(true); setConfirm(false); reload(); toast('Nova chave gerada. Copie-a agora.') }
    catch (e) { toast(errMsg(e), true); setConfirm(false) } finally { setBusy(false) }
  }
  const sk = data?.secret
  const prefix = live ? 'sk_live_' : 'sk_test_'
  const skText = fresh ? (reveal ? fresh : prefix + '••••••••••••' + fresh.slice(-4)) : sk ? prefix + '••••••••••••' + sk.last4 : 'Nenhuma chave secreta'
  const skShort = fresh ? prefix + '…' + fresh.slice(-4) : sk ? prefix + '…' + sk.last4 : prefix + '…'
  const base = window.location.origin
  return (
    <div className="grid-2" style={{ alignItems: 'start' }}>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
        <Card>
          <div className="between"><div style={{ fontWeight: 600, fontSize: 16 }}>Chaves API</div><span className="pill" style={{ fontSize: 12, padding: '4px 9px', background: live ? 'var(--green-bg)' : 'var(--sun-bg)', color: live ? 'var(--green-dark)' : 'var(--sun-text)' }}>{live ? 'Produção' : 'Teste'}</span></div>
          <div className="row" style={{ fontSize: 13, color: 'var(--muted)', marginTop: 4, justifyContent: 'space-between' }}><span>Use o interruptor para alternar entre teste e produção.</span><span className="only-mobile"><ModeSwitch /></span></div>
          <span className="label" style={{ margin: '18px 0 6px' }}>Chave pública</span>
          <div className="row" style={{ gap: 8, padding: '6px 6px 6px 14px', borderRadius: 12, background: 'var(--paper)' }}><span className="mono" style={{ flex: 1, fontSize: 13, minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{data?.publishable || '…'}</span><button className="btn btn-outline btn-sm" style={{ height: 34 }} onClick={() => doCopy('pk', data.publishable)}>{copied === 'pk' ? 'Copiado' : 'Copiar'}</button></div>
          <div className="between" style={{ margin: '16px 0 6px' }}><span className="label" style={{ margin: 0 }}>Chave secreta</span><span style={{ fontSize: 12, fontWeight: 600, color: 'var(--red-dark)' }}>Só no servidor</span></div>
          <div className="row" style={{ gap: 6, padding: '6px 6px 6px 14px', borderRadius: 12, background: 'var(--ink)', color: '#fff' }}>
            <span className="mono" style={{ flex: 1, fontSize: 13, minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{skText}</span>
            {fresh && <button className="btn btn-sm" style={{ height: 34, background: 'rgba(255,255,255,.1)', color: '#fff' }} onClick={() => setReveal(!reveal)}>{reveal ? 'Ocultar' : 'Mostrar'}</button>}
            {fresh && <button className="btn btn-white btn-sm" style={{ height: 34 }} onClick={() => doCopy('sk', fresh)}>{copied === 'sk' ? 'Copiado' : 'Copiar'}</button>}
          </div>
          {fresh && <div style={{ marginTop: 10, padding: '10px 12px', borderRadius: 12, background: 'var(--sun-soft)', fontSize: 12.5, color: '#5B4600', lineHeight: 1.45 }}>Guarde esta chave num local seguro. Por segurança só a mostramos agora.</div>}
          <div className="between" style={{ marginTop: 14, flexWrap: 'wrap' }}>
            <span style={{ fontSize: 12.5, color: 'var(--muted)' }}>{sk ? `Criada a ${shortDate(sk.created_at)} · Último uso ${ago(sk.last_used_at)}` : 'Gere uma chave secreta para usar a API.'}</span>
            <button className="btn btn-outline" style={{ height: 36, fontSize: 13 }} onClick={() => (sk ? setConfirm(true) : roll())} disabled={busy || (live && merchant.status !== 'active')}>{busy ? <Spinner dark /> : sk ? 'Gerar nova chave' : 'Gerar chave secreta'}</button>
          </div>
          {live && merchant.status !== 'active' && <div style={{ fontSize: 12.5, color: 'var(--sun-text)', marginTop: 10 }}>As chaves de produção ficam disponíveis depois da verificação.</div>}
        </Card>
        <div className="card" style={{ padding: '8px 0' }}>
          <div style={{ padding: '14px 22px 8px', fontWeight: 600, fontSize: 16 }}>Pedidos recentes</div>
          {(logs.data?.data || []).map((r: any, i: number) => (
            <div key={i} className="row mono" style={{ padding: '10px 22px', borderTop: '1px solid var(--line-3)', fontSize: 12.5 }}>
              <span style={{ padding: '2px 7px', borderRadius: 6, background: r.status < 300 ? 'var(--green-bg)' : 'var(--red-bg)', color: r.status < 300 ? 'var(--green-dark)' : 'var(--red-dark)' }}>{r.status}</span>
              <span style={{ fontWeight: 500, width: 44 }}>{r.method}</span><span style={{ flex: 1, minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis' }}>{r.path}</span><span style={{ fontFamily: 'Geist', color: 'var(--muted)' }}>{ago(r.created_at)}</span>
            </div>
          ))}
          {logs.data && !logs.data.data.length && <div className="empty" style={{ padding: 22 }}>Ainda sem pedidos neste modo.</div>}
        </div>
      </div>
      <div style={{ background: 'var(--ink)', color: '#fff', borderRadius: 22, padding: 22, overflow: 'hidden' }}>
        <div className="between"><div style={{ fontWeight: 600, fontSize: 16 }}>Criar um checkout</div><span className="mono" style={{ fontSize: 11.5, padding: '4px 9px', borderRadius: 7, background: 'rgba(255,255,255,.14)' }}>cURL</span></div>
        <pre className="mono" style={{ margin: '16px 0 0', fontSize: 13, lineHeight: 1.8, color: '#C9D3CD', whiteSpace: 'pre', overflowX: 'auto' }}>{`curl ${base}/v1/checkout/sessions \\
  -u `}<span style={{ color: 'var(--sun)' }}>{skShort}</span>{`: \\
  -H "Idempotency-Key: pedido-2094" \\
  -d amount=125000 \\
  -d currency=MZN \\
  -d description="Encomenda #2094" \\
  -d success_url=`}<span style={{ color: 'var(--lime)' }}>https://loja.co.mz/ok</span>{` \\
  -d cancel_url=`}<span style={{ color: 'var(--lime)' }}>https://loja.co.mz/carrinho</span></pre>
        <pre className="mono" style={{ margin: '14px 0 0', paddingTop: 14, borderTop: '1px solid rgba(255,255,255,.1)', fontSize: 13, lineHeight: 1.8, color: '#C9D3CD', whiteSpace: 'pre', overflowX: 'auto' }}><span style={{ color: '#93A09A' }}># Resposta: redirecione o cliente para url</span>{`
{
  "id": "${live ? 'cs_live' : 'cs_test'}_8K2D4410",
  "status": "open",
  "url": "`}<span style={{ color: 'var(--lime)' }}>{base}/c/{live ? 'cs_live' : 'cs_test'}_8K2D4410</span>{`"
}`}</pre>
        <a href="/docs" style={{ display: 'inline-flex', alignItems: 'center', gap: 6, marginTop: 16, fontSize: 14, fontWeight: 600, color: 'var(--lime)' }}>Documentação completa<Icon name="arrowUpRight" size={15} stroke={2.2} /></a>
      </div>
      {confirm && (
        <Modal onClose={() => setConfirm(false)}>
          <div className="display" style={{ fontSize: 22 }}>Gerar nova chave secreta?</div>
          <div style={{ fontSize: 14, color: 'var(--muted)', marginTop: 8, lineHeight: 1.5 }}>A chave atual (<span className="mono">{prefix}…{sk?.last4}</span>) deixa de funcionar imediatamente. Atualize o seu servidor com a nova chave.</div>
          <div style={{ display: 'flex', gap: 10, marginTop: 18 }}><button className="btn btn-outline" style={{ flex: 1 }} onClick={() => setConfirm(false)}>Cancelar</button><button className="btn btn-ink" style={{ flex: 1 }} onClick={roll} disabled={busy}>{busy ? <Spinner /> : 'Gerar nova chave'}</button></div>
        </Modal>
      )}
    </div>
  )
}

// ------------------------------------------------------------------ webhooks
function Hooks() {
  const { live, toast } = useSession()
  const { data, setData } = useData<any>('/developers/settings', [live])
  const dels = useData<any>('/developers/deliveries', [live], 5000)
  const [redir, setRedir] = useState({ success_url: '', cancel_url: '' })
  const [domain, setDomain] = useState('')
  const [addDomain, setAddDomain] = useState(false)
  const [ep, setEp] = useState<{ url: string; events: string[] } | null>(null)
  const [secret, setSecret] = useState('')
  const [copied, setCopied] = useState(false)
  const [err, setErr] = useState('')
  useEffect(() => { if (data) setRedir({ success_url: data.success_url, cancel_url: data.cancel_url }) }, [data?.success_url, data?.cancel_url]) // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => setSecret(''), [live])
  const call = async (path: string, method: string, body?: unknown, msg?: string) => {
    setErr('')
    try { const r = await api<any>(path, { method, body }); setData(r); if (msg) toast(msg); return true } catch (e) { setErr(errMsg(e)); toast(errMsg(e), true); return false }
  }
  const revealSecret = async () => { const r = await api<any>('/developers/secret'); setSecret(r.secret) }
  const copySecret = async () => { const s = secret || (await api<any>('/developers/secret')).secret; await copy(s); setCopied(true); setTimeout(() => setCopied(false), 1500) }
  if (!data) return <Spinner dark />
  const mono = { fontFamily: 'var(--mono)', fontSize: 13 }
  return (
    <div className="grid-2" style={{ alignItems: 'start' }}>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
        <Card>
          <div style={{ fontWeight: 600, fontSize: 16 }}>URLs de redirecionamento</div>
          <div style={{ fontSize: 13, color: 'var(--muted)', marginTop: 4 }}>Para onde enviamos o cliente depois do checkout dos links. Na API, envie success_url e cancel_url em cada sessão.</div>
          <span className="label row" style={{ margin: '16px 0 6px', gap: 6 }}><span style={{ width: 7, height: 7, borderRadius: '50%', background: 'var(--green)' }} />Sucesso</span>
          <input className="input" style={mono} value={redir.success_url} onChange={e => setRedir({ ...redir, success_url: e.target.value })} placeholder="https://a-sua-loja.co.mz/obrigado" />
          <span className="label row" style={{ margin: '12px 0 6px', gap: 6 }}><span style={{ width: 7, height: 7, borderRadius: '50%', background: 'var(--red)' }} />Cancelado ou falhado</span>
          <input className="input" style={mono} value={redir.cancel_url} onChange={e => setRedir({ ...redir, cancel_url: e.target.value })} placeholder="https://a-sua-loja.co.mz/carrinho" />
          <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: 12 }}><button className="btn btn-outline" style={{ height: 38 }} onClick={() => call('/developers/redirects', 'PUT', redir, 'URLs guardados')}>Guardar</button></div>
        </Card>
        <Card>
          <div className="between"><div style={{ fontWeight: 600, fontSize: 16 }}>Callback por transação</div><span className="tag" style={{ background: 'var(--ink)', color: '#fff', fontSize: 11.5, padding: '3px 8px' }}>DINÂMICO</span></div>
          <div style={{ fontSize: 13, lineHeight: 1.5, color: 'var(--muted)', marginTop: 4 }}>Envie <span className="mono" style={{ color: 'var(--ink)' }}>callback_url</span> ao criar a sessão. A Riha envia o resultado dessa transação para esse endereço, além dos endpoints abaixo.</div>
          <div className="mono" style={{ marginTop: 12, padding: '10px 14px', borderRadius: 12, background: 'var(--ink)', color: '#C9D3CD', fontSize: 12.5, overflowX: 'auto', whiteSpace: 'nowrap' }}><span style={{ color: 'var(--sun)' }}>"callback_url"</span>: <span style={{ color: 'var(--lime)' }}>"https://{data.domains[0]?.domain || 'loja.co.mz'}/pedidos/2094/riha"</span></div>
          <span className="label" style={{ margin: '14px 0 8px' }}>Domínios autorizados</span>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
            {data.domains.map((d: any) => <span key={d.id} className="chip" style={{ background: 'var(--green-bg)', color: 'var(--green-dark)' }}><Icon name="check" size={12} stroke={3} />{d.domain}<button aria-label="Remover" onClick={() => call('/developers/domains/' + d.id, 'DELETE', undefined, 'Domínio removido')} style={{ border: 0, background: 'none', padding: 0, color: 'inherit', display: 'grid' }}><Icon name="close" size={12} stroke={2.4} /></button></span>)}
            {addDomain ? (
              <form onSubmit={async e => { e.preventDefault(); if (await call('/developers/domains', 'POST', { domain }, 'Domínio verificado')) { setDomain(''); setAddDomain(false) } }} className="row" style={{ gap: 6 }}>
                <input autoFocus className="input mono" style={{ height: 30, width: 200, fontSize: 12, borderRadius: 9 }} value={domain} onChange={e => setDomain(e.target.value)} placeholder="loja.co.mz" />
                <button className="btn btn-ink btn-sm" style={{ height: 30 }}>Adicionar</button>
              </form>
            ) : <button className="chip" style={{ border: '1px dashed #D6D4CB', color: 'var(--muted)', background: 'none' }} onClick={() => setAddDomain(true)}>+ verificar domínio</button>}
          </div>
          <div style={{ fontSize: 12.5, color: 'var(--muted)', marginTop: 10 }}>Pedidos com callback_url fora destes domínios são recusados com o erro <span className="mono">callback_url_not_allowed</span>. {!live && 'Em teste, localhost também é aceite.'}</div>
        </Card>
        <Card>
          <div className="between"><div style={{ fontWeight: 600, fontSize: 16 }}>Endpoints de webhook</div><button className="btn btn-outline btn-sm" onClick={() => setEp({ url: '', events: ['payment.succeeded', 'payment.failed', 'payout.paid'] })}>+ Endpoint</button></div>
          <div style={{ fontSize: 13, color: 'var(--muted)', marginTop: 4 }}>Fixos. Recebem os eventos escolhidos de toda a conta.</div>
          {data.endpoints.map((e: any) => (
            <div key={e.id} style={{ marginTop: 14, paddingTop: 14, borderTop: '1px solid var(--line-3)' }}>
              <div className="row" style={{ gap: 8 }}>
                <div className="mono" style={{ flex: 1, minWidth: 0, height: 46, borderRadius: 12, background: 'var(--paper)', display: 'flex', alignItems: 'center', padding: '0 14px', fontSize: 13, overflow: 'hidden', whiteSpace: 'nowrap', textOverflow: 'ellipsis' }}>{e.url}</div>
                <Toggle on={e.status === 'enabled'} onChange={v => call('/developers/endpoints/' + e.id, 'PATCH', { active: v }, v ? 'Endpoint ativado' : 'Endpoint desativado')} label="Ativo" />
                <button className="icon-btn" aria-label="Apagar" onClick={() => call('/developers/endpoints/' + e.id, 'DELETE', undefined, 'Endpoint apagado')}><Icon name="trash" size={15} /></button>
              </div>
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6, marginTop: 10 }}>{e.enabled_events.map((x: string) => <span key={x} className="chip" style={{ background: 'var(--ink)', color: '#fff' }}>{x}</span>)}</div>
            </div>
          ))}
          {!data.endpoints.length && <div style={{ marginTop: 14, fontSize: 13.5, color: 'var(--muted)' }}>Nenhum endpoint neste modo.</div>}
          <div className="row" style={{ gap: 8, marginTop: 14, padding: '6px 6px 6px 14px', borderRadius: 12, background: 'var(--paper)' }}>
            <span style={{ fontSize: 12.5, color: 'var(--muted)' }}>Segredo</span>
            <span className="mono" style={{ flex: 1, fontSize: 13, minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{secret || data.secret_hint}</span>
            {!secret && <button className="btn btn-outline btn-sm" onClick={revealSecret}>Mostrar</button>}
            <button className="btn btn-outline btn-sm" onClick={copySecret}>{copied ? 'Copiado' : 'Copiar'}</button>
          </div>
          <div style={{ fontSize: 12, color: 'var(--muted)', marginTop: 8 }}>Usado para assinar endpoints e callback_url: <span className="mono">Riha-Signature: t=…,v1=HMAC-SHA256(t + "." + corpo)</span></div>
        </Card>
      </div>
      <div className="card" style={{ padding: '8px 0' }}>
        <div className="between" style={{ padding: '14px 22px 8px' }}><div style={{ fontWeight: 600, fontSize: 16 }}>Entregas recentes</div><button className="btn btn-outline" style={{ height: 34, fontSize: 13 }} onClick={async () => { try { await api('/developers/test-event', { method: 'POST' }); toast('Evento de teste enviado'); dels.reload() } catch (e) { toast(errMsg(e), true) } }}>Enviar evento de teste</button></div>
        {(dels.data?.data || []).map((d: any) => {
          const ok = d.status === 'succeeded', pend = d.status === 'pending'
          return (
            <div key={d.id} className="row" style={{ padding: '12px 22px', borderTop: '1px solid var(--line-3)' }}>
              <span className="mono" style={{ fontSize: 12, padding: '2px 7px', borderRadius: 6, background: ok ? 'var(--green-bg)' : pend ? 'var(--sun-bg)' : 'var(--red-bg)', color: ok ? 'var(--green-dark)' : pend ? 'var(--sun-text)' : 'var(--red-dark)', minWidth: 34, textAlign: 'center' }}>{d.last_status_code || (pend ? '···' : 'ERR')}</span>
              <div style={{ flex: 1, minWidth: 0 }}>
                <div className="mono" style={{ fontSize: 13 }}>{d.event_type}</div>
                <div style={{ fontSize: 11.5, color: 'var(--muted)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }} title={d.last_error}>{d.target === 'callback_url' ? 'callback_url · ' : ''}{d.url.replace(/^https?:\/\//, '')}{d.attempts > 1 ? ` · ${d.attempts} tentativas` : ''}{pend && d.next_attempt_at && d.attempts > 0 ? ` · próxima ${fullDate(d.next_attempt_at)}` : ''}</div>
              </div>
              {!ok ? <button className="btn btn-ghost btn-sm" style={{ color: 'var(--green-text)' }} onClick={async () => { await api(`/developers/deliveries/${d.id}/resend`, { method: 'POST' }); toast('Reenvio agendado'); dels.reload() }}>Reenviar</button> : <span style={{ fontSize: 12.5, color: 'var(--muted)' }}>{ago(d.updated_at)}</span>}
            </div>
          )
        })}
        {dels.data && !dels.data.data.length && <div className="empty">Sem entregas neste modo.</div>}
      </div>
      {ep && (
        <Modal onClose={() => setEp(null)} width={500}>
          <div className="display" style={{ fontSize: 22 }}>Novo endpoint</div>
          <label style={{ display: 'block', marginTop: 16 }}><span className="label">URL</span><input autoFocus className="input mono" value={ep.url} onChange={e => setEp({ ...ep, url: e.target.value })} placeholder="https://a-sua-loja.co.mz/api/riha" /></label>
          <span className="label" style={{ marginTop: 14 }}>Eventos</span>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
            {data.event_types.map((x: string) => { const on = ep.events.includes(x); return <button key={x} className="chip" onClick={() => setEp({ ...ep, events: on ? ep.events.filter(y => y !== x) : [...ep.events, x] })} style={{ border: on ? 0 : '1px solid var(--line-2)', background: on ? 'var(--ink)' : '#fff', color: on ? '#fff' : 'var(--ink)' }}>{x}</button> })}
          </div>
          {err && <div className="form-err" style={{ marginTop: 12 }}>{err}</div>}
          <div style={{ display: 'flex', gap: 10, marginTop: 18 }}><button className="btn btn-outline" style={{ flex: 1 }} onClick={() => setEp(null)}>Cancelar</button><button className="btn btn-green" style={{ flex: 1 }} onClick={async () => { if (await call('/developers/endpoints', 'POST', { url: ep.url, enabled_events: ep.events }, 'Endpoint criado')) setEp(null) }}>Criar endpoint</button></div>
        </Modal>
      )}
    </div>
  )
}

// ------------------------------------------------------------------ account
function Account() {
  const { me, logout, toast } = useSession()
  const nav = useNavigate()
  const [f, setF] = useState({ current_password: '', new_password: '' })
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState(false)
  const save = async () => {
    setBusy(true); setErr('')
    try { await api('/auth/password', { body: f }); setF({ current_password: '', new_password: '' }); toast('Palavra-passe alterada') } catch (e) { setErr(errMsg(e)) } finally { setBusy(false) }
  }
  return (
    <div className="grid-2" style={{ alignItems: 'start' }}>
      <Card>
        <div style={{ fontWeight: 600, fontSize: 16 }}>Utilizador</div>
        <div className="row" style={{ marginTop: 14 }}><Avatar text={initials(me!.user.name || me!.user.email)} color="var(--lime)" /><div><div style={{ fontWeight: 600 }}>{me!.user.name || '—'}</div><div style={{ fontSize: 13, color: 'var(--muted)' }}>{me!.user.email} · Dono</div></div></div>
        <button className="btn btn-danger" style={{ marginTop: 18 }} onClick={async () => { await logout(); nav('/') }}><Icon name="logout" size={16} />Terminar sessão</button>
      </Card>
      <Card>
        <div style={{ fontWeight: 600, fontSize: 16 }}>Alterar palavra-passe</div>
        <label style={{ display: 'block', marginTop: 14 }}><span className="label">Palavra-passe atual</span><input className="input" type="password" autoComplete="current-password" value={f.current_password} onChange={e => setF({ ...f, current_password: e.target.value })} /></label>
        <label style={{ display: 'block', marginTop: 12 }}><span className="label">Nova palavra-passe</span><input className="input" type="password" autoComplete="new-password" value={f.new_password} onChange={e => setF({ ...f, new_password: e.target.value })} /></label>
        {err && <div className="form-err" style={{ marginTop: 12 }}>{err}</div>}
        <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: 14 }}><button className="btn btn-green" onClick={save} disabled={busy || f.new_password.length < 8 || !f.current_password}>{busy ? <Spinner /> : 'Alterar'}</button></div>
      </Card>
    </div>
  )
}

void Stripes
