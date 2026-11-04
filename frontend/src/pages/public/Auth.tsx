import { useState, type FormEvent, type ReactNode } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { api, errMsg, type ApiError } from '../../lib/api'
import { useSession, type Me } from '../../lib/session'
import { Icon, Logo, Spinner, Stripes } from '../../components/ui'

/** Split layout: brand panel on desktop, single column on phones. */
export function AuthLayout({ children, step, back }: { children: ReactNode; step?: [number, number]; back?: () => void }) {
  return (
    <div className="auth">
      <style>{`
        .auth{min-height:100%;display:grid;grid-template-columns:minmax(0,1fr) minmax(0,1fr)}
        .auth-side{background:var(--ink);color:#fff;position:relative;overflow:hidden;padding:40px 56px;display:flex;flex-direction:column}
        .auth-main{display:flex;justify-content:center;padding:40px 24px;overflow:auto}
        .auth-col{width:100%;max-width:420px;display:flex;flex-direction:column;min-height:100%}
        @media (max-width:860px){.auth{grid-template-columns:1fr}.auth-side{display:none}.auth-main{padding:max(14px,env(safe-area-inset-top)) 20px 28px}}
      `}</style>
      <div className="auth-side">
        <Stripes top={-120} right={80} w={90} h={1200} gap={40} fade="linear-gradient(90deg,#0B120E 30%,rgba(11,18,14,.4))" />
        <Link to="/" style={{ position: 'relative' }}><Logo dark size={34} text={19} /></Link>
        <div style={{ flex: 1 }} />
        <div style={{ position: 'relative', fontFamily: 'var(--display)', fontWeight: 700, fontSize: 52, lineHeight: 1, letterSpacing: '-0.04em', maxWidth: 460 }}>Receba de todo <span style={{ color: 'var(--lime)' }}>Moçambique</span>.</div>
        <div style={{ position: 'relative', fontSize: 16, lineHeight: 1.55, color: '#B9C3BD', marginTop: 16, maxWidth: 420 }}>M-Pesa, e-Mola e cartões num só painel. Links de pagamento em segundos, API para a sua loja e levantamentos para o banco em até 72 horas.</div>
        <div style={{ position: 'relative', display: 'flex', gap: 16, marginTop: 28, fontSize: 13.5, color: '#93A09A' }}><span>Sem mensalidade</span><span>·</span><span>Levantamentos em 72h</span></div>
      </div>
      <div className="auth-main">
        <div className="auth-col">
          {(back || step) && (
            <div className="between">
              {back ? <button onClick={back} className="icon-btn" style={{ width: 42, height: 42, borderRadius: 14 }} aria-label="Voltar"><Icon name="arrowLeft" size={20} stroke={2} /></button> : <span />}
              {step && <span style={{ fontSize: 13, fontWeight: 500, color: 'var(--muted)' }}>Passo {step[0]} de {step[1]}</span>}
            </div>
          )}
          {step && <div style={{ display: 'flex', gap: 6, marginTop: 18 }}>{Array.from({ length: step[1] }, (_, i) => <div key={i} style={{ flex: 1, height: 4, borderRadius: 2, background: i < step[0] ? 'var(--green)' : '#E1DFD7' }} />)}</div>}
          {children}
        </div>
      </div>
    </div>
  )
}

function strength(p: string) {
  let s = 0
  if (p.length >= 8) s++
  if (p.length >= 12) s++
  if (/[A-Z]/.test(p) && /[a-z]/.test(p)) s++
  if (/\d/.test(p) && /[^A-Za-z0-9]/.test(p)) s++
  return s
}

export function Signup() {
  const nav = useNavigate()
  const { setMe } = useSession()
  const [f, setF] = useState({ business_name: '', name: '', email: '', phone: '', password: '' })
  const [show, setShow] = useState(false)
  const [terms, setTerms] = useState(false)
  const [err, setErr] = useState<ApiError | null>(null)
  const [busy, setBusy] = useState(false)
  const st = strength(f.password)
  const set = (k: keyof typeof f) => (e: { target: { value: string } }) => setF({ ...f, [k]: e.target.value })
  const submit = async (e: FormEvent) => {
    e.preventDefault()
    if (!terms) { setErr({ code: 'terms', message: 'Aceite os termos para continuar.', status: 400 }); return }
    setBusy(true); setErr(null)
    try { const me = await api<Me>('/auth/signup', { body: f }); setMe(me); nav('/verificacao') }
    catch (e) { setErr(e as ApiError) } finally { setBusy(false) }
  }
  const fe = (p: string) => (err?.param === p ? err.message : undefined)
  return (
    <AuthLayout step={[1, 2]} back={() => nav('/')}>
      <form onSubmit={submit} style={{ display: 'flex', flexDirection: 'column', flex: 1 }}>
        <h1 style={{ margin: '26px 0 0', fontFamily: 'var(--display)', fontWeight: 700, fontSize: 32, lineHeight: 1.05, letterSpacing: '-0.03em' }}>Crie a sua conta Riha</h1>
        <p style={{ margin: '8px 0 0', fontSize: 15, color: 'var(--muted)' }}>Leva menos de 3 minutos.</p>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16, marginTop: 26 }}>
          <label><span className="label">Nome do negócio</span><input className={'input lg' + (fe('business_name') ? ' err' : '')} value={f.business_name} onChange={set('business_name')} autoComplete="organization" required placeholder="Café Maputo" />{fe('business_name') && <div className="field-err">{fe('business_name')}</div>}</label>
          <label><span className="label">O seu nome</span><input className="input lg" value={f.name} onChange={set('name')} autoComplete="name" placeholder="Ana Mabota" /></label>
          <label><span className="label">Email</span><input className={'input lg' + (fe('email') ? ' err' : '')} type="email" value={f.email} onChange={set('email')} autoComplete="email" required placeholder="ola@cafemaputo.co.mz" />{fe('email') && <div className="field-err">{fe('email')}</div>}</label>
          <label><span className="label">Telemóvel</span><div className="phone-input"><span className="cc">+258</span><input inputMode="tel" value={f.phone} onChange={set('phone')} autoComplete="tel-national" required placeholder="84 123 4567" /></div>{fe('phone') && <div className="field-err">{fe('phone')}</div>}</label>
          <label>
            <span className="label">Palavra-passe</span>
            <div style={{ position: 'relative' }}>
              <input className="input lg" type={show ? 'text' : 'password'} value={f.password} onChange={set('password')} autoComplete="new-password" required minLength={8} style={{ paddingRight: 48 }} />
              <button type="button" onClick={() => setShow(!show)} aria-label="Mostrar palavra-passe" style={{ position: 'absolute', right: 8, top: 8, width: 38, height: 38, border: 0, background: 'none', display: 'grid', placeItems: 'center' }}><Icon name="eye" size={20} stroke={1.8} color="#8A938E" /></button>
            </div>
            {f.password && <div style={{ fontSize: 12.5, color: st >= 2 ? 'var(--green-text)' : st === 1 ? 'var(--sun-text)' : 'var(--red-dark)', marginTop: 7, display: 'flex', gap: 6, alignItems: 'center' }}><span style={{ width: 6, height: 6, borderRadius: '50%', background: 'currentColor' }} />{st >= 3 ? 'Palavra-passe forte' : st === 2 ? 'Palavra-passe razoável' : st === 1 ? 'Palavra-passe fraca' : 'Mínimo 8 caracteres'}</div>}
            {fe('password') && <div className="field-err">{fe('password')}</div>}
          </label>
        </div>
        <label style={{ display: 'flex', gap: 10, alignItems: 'flex-start', marginTop: 20, fontSize: 13.5, lineHeight: 1.45, color: 'var(--text-2)', cursor: 'pointer' }}>
          <input type="checkbox" checked={terms} onChange={e => setTerms(e.target.checked)} style={{ position: 'absolute', opacity: 0 }} />
          <span style={{ width: 20, height: 20, flex: 'none', borderRadius: 6, background: terms ? 'var(--ink)' : '#fff', border: terms ? 0 : '1.5px solid #CFCDC4', display: 'grid', placeItems: 'center', marginTop: 1 }}>{terms && <Icon name="check" size={13} stroke={3} color="#fff" />}</span>
          <span>Aceito os <a href="#">Termos de serviço</a> e a <a href="#">Política de privacidade</a> da Riha.</span>
        </label>
        {err && !err.param && <div className="form-err" style={{ marginTop: 16 }}>{err.message}</div>}
        <div style={{ flex: 1, minHeight: 24 }} />
        <button className="btn btn-green btn-lg" disabled={busy}>{busy ? <Spinner /> : 'Continuar'}</button>
        <div style={{ textAlign: 'center', fontSize: 14, color: 'var(--muted)', marginTop: 16 }}>Já tem conta? <Link to="/entrar" style={{ fontWeight: 600 }}>Entrar</Link></div>
      </form>
    </AuthLayout>
  )
}

export function Login() {
  const nav = useNavigate()
  const [sp] = useSearchParams()
  const { setMe } = useSession()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState(false)
  const submit = async (e: FormEvent) => {
    e.preventDefault(); setBusy(true); setErr('')
    try {
      const me = await api<Me>('/auth/login', { body: { email, password } })
      setMe(me)
      const next = sp.get('next')
      if (me.user.role === 'admin') nav(next?.startsWith('/admin') ? next : '/admin')
      else if (me.merchant?.status === 'onboarding') nav('/verificacao')
      else nav(next && next.startsWith('/painel') ? next : '/painel')
    } catch (e) { setErr(errMsg(e)) } finally { setBusy(false) }
  }
  return (
    <AuthLayout back={() => nav('/')}>
      <form onSubmit={submit} style={{ display: 'flex', flexDirection: 'column', flex: 1 }}>
        <div className="only-mobile" style={{ marginTop: 22 }}><Logo size={30} text={17} /></div>
        <h1 style={{ margin: '26px 0 0', fontFamily: 'var(--display)', fontWeight: 700, fontSize: 32, lineHeight: 1.05, letterSpacing: '-0.03em' }}>Entrar na Riha</h1>
        <p style={{ margin: '8px 0 0', fontSize: 15, color: 'var(--muted)' }}>Bem-vindo de volta.</p>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16, marginTop: 26 }}>
          <label><span className="label">Email</span><input className="input lg" type="email" value={email} onChange={e => setEmail(e.target.value)} autoComplete="email" required autoFocus /></label>
          <label><span className="label">Palavra-passe</span><input className="input lg" type="password" value={password} onChange={e => setPassword(e.target.value)} autoComplete="current-password" required /></label>
        </div>
        {err && <div className="form-err" style={{ marginTop: 16 }}>{err}</div>}
        <div style={{ flex: 1, minHeight: 24 }} />
        <button className="btn btn-green btn-lg" disabled={busy} style={{ marginTop: 24 }}>{busy ? <Spinner /> : 'Entrar'}</button>
        <div style={{ textAlign: 'center', fontSize: 14, color: 'var(--muted)', marginTop: 16 }}>Ainda não tem conta? <Link to="/registo" style={{ fontWeight: 600 }}>Criar conta</Link></div>
      </form>
    </AuthLayout>
  )
}
