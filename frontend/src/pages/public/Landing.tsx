import { useEffect, useState, type CSSProperties, type ReactNode } from 'react'
import { SUPPORT_PHONE, SUPPORT_TEL } from '../../lib/support'
import { Link } from 'react-router-dom'
import { Icon, QR, stripeBlur } from '../../components/ui'
import { useGateway } from '../../core/gateway'
import { bpsLabel } from '../../lib/format'

const BARS14 = (() => {
  let sd = 5
  const r = () => (sd = (sd * 9301 + 49297) % 233280) / 233280
  const v30 = Array.from({ length: 30 }, (_, i) => 0.3 + r() * 0.55 + (i % 7 === 5 ? 0.15 : 0) + i * 0.006)
  return v30.slice(16).map((v, i) => ({ h: Math.round(v * 68), bg: i === 13 ? '#0A8205' : '#E4E8E1' }))
})()

const DISPLAY = "'Bricolage Grotesque', sans-serif"
const MONO = "'Geist Mono', monospace"

const css = `
.lp { background: #F4F3EE; overflow-x: hidden; min-height: 100%; }
.lp a.lp-plain { color: inherit; }
.lp a.lp-plain:hover { color: #fff; }
.lp-wrap { max-width: 1240px; margin: 0 auto; padding: 0 40px; }
.lp-m { display: none; }
.lp-mi { display: none; }
.lp-eyebrow { font-size: 14px; font-weight: 600; color: #0A7A05; }
.lp-h2 { margin: 10px 0 0; font-family: ${DISPLAY}; font-weight: 700; font-size: 44px; line-height: 1.02; letter-spacing: -0.035em; text-wrap: balance; }
.lp-sec { padding-top: 96px; }
.lp-sec-first { padding-top: 88px; }
.lp-topnav { display: flex; align-items: center; height: 84px; gap: 40px; }
.lp-topnav nav { display: flex; gap: 28px; font-size: 14.5px; }

.lp-hero-grid { display: grid; grid-template-columns: minmax(0,1.05fr) minmax(0,1fr); gap: 40px; align-items: center; padding: 72px 0 96px; }
.lp-h1 { margin: 0; font-family: ${DISPLAY}; font-weight: 700; font-size: 76px; line-height: .98; letter-spacing: -0.04em; text-wrap: balance; }
.lp-lead { margin: 22px 0 0; font-size: 19px; line-height: 1.55; color: #B9C3BD; max-width: 520px; text-wrap: pretty; }
.lp-ctas { display: flex; gap: 12px; margin-top: 32px; }
.lp-cta { height: 56px; padding: 0 26px; border-radius: 16px; font-weight: 600; font-size: 16.5px; display: inline-flex; align-items: center; justify-content: center; gap: 8px; white-space: nowrap; }
.lp-cta-green { background: #0A8205; color: #fff !important; box-shadow: 0 14px 30px -12px rgba(10,130,5,.8); }
.lp-cta-green:hover { background: #097404; }
.lp-cta-ghost { padding: 0 24px; font-size: 16px; border: 1px solid rgba(255,255,255,.22); color: #fff !important; }
.lp-cta-ghost:hover { background: rgba(255,255,255,.06); }
.lp-trust { display: flex; flex-wrap: wrap; gap: 6px 16px; margin-top: 20px; font-size: 14px; color: #93A09A; }
.lp-hero-vis { position: relative; height: 500px; }
.lp-navlink { color: #C9D3CD !important; }
.lp-navlink:hover { color: #fff !important; }
.lp-btn-top { height: 42px; padding: 0 18px; border-radius: 12px; font-weight: 600; font-size: 14.5px; display: inline-flex; align-items: center; }

.lp-methods { display: grid; grid-template-columns: minmax(0,1fr) minmax(0,1.4fr); gap: 48px; align-items: end; }
.lp-methods-grid { display: grid; grid-template-columns: repeat(4, minmax(0,1fr)); gap: 12px; }
.lp-method { background: #fff; border: 1px solid #E8E6DF; border-radius: 20px; padding: 18px; display: flex; flex-direction: column; gap: 18px; }
.lp-method-ico { width: 42px; height: 42px; border-radius: 12px; display: grid; place-items: center; font-weight: 800; }
.lp-method-name { font-weight: 600; font-size: 15.5px; }
.lp-method-sub { font-size: 13px; color: #5E6862; margin-top: 2px; }

.lp-features { display: grid; grid-template-columns: minmax(0,1.25fr) minmax(0,1fr); gap: 16px; }
.lp-dev { grid-row: span 2; }
.lp-pricing { display: grid; grid-template-columns: minmax(0,1fr) minmax(0,1fr); gap: 48px; align-items: center; }
.lp-cta-band { display: flex; align-items: center; justify-content: space-between; gap: 24px; }
.lp-cta-band .lp-cta-btn { margin-right: 340px; }

@media (max-width: 1100px) {
  .lp-hero-grid { grid-template-columns: minmax(0,1fr); padding: 56px 0 72px; gap: 48px; }
  .lp-h1 { font-size: 64px; }
  .lp-hero-vis { width: 100%; max-width: 560px; margin: 0 auto; }
  .lp-methods { grid-template-columns: minmax(0,1fr); gap: 28px; align-items: start; }
  .lp-features { grid-template-columns: minmax(0,1fr); }
  .lp-dev { grid-row: auto; }
  .lp-pricing { grid-template-columns: minmax(0,1fr); gap: 28px; }
  .lp-cta-band { flex-wrap: wrap; }
  .lp-cta-band .lp-cta-btn { margin-right: 0; }
  .lp-topnav { gap: 24px; }
  .lp-topnav nav { gap: 20px; }
}

@media (max-width: 860px) {
  .lp-d { display: none !important; }
  .lp-m { display: block; }
  .lp-mi { display: inline; }
  .lp-wrap { padding: 0 20px; }
  .lp-eyebrow { font-size: 13px; letter-spacing: .02em; }
  .lp-h2 { margin-top: 8px; font-size: 30px; line-height: 1.05; letter-spacing: -0.03em; }
  .lp-sec { padding-top: 48px; }
  .lp-sec-first { padding-top: 44px; }
  .lp-hero-grid { padding: 140px 0 0; gap: 0; }
  .lp-h1 { font-size: 44px; line-height: 1; letter-spacing: -0.035em; }
  .lp-lead { margin-top: 16px; font-size: 16.5px; line-height: 1.5; max-width: none; }
  .lp-ctas { flex-direction: column; gap: 10px; margin-top: 26px; }
  .lp-cta { width: 100%; }
  .lp-trust { justify-content: center; margin-top: 16px; font-size: 13px; }
  .lp-methods { gap: 22px; }
  .lp-methods-grid { grid-template-columns: minmax(0,1fr) minmax(0,1fr); gap: 10px; }
  .lp-method { padding: 16px; gap: 14px; }
  .lp-method-ico { width: 40px; height: 40px; }
  .lp-method-name { font-size: 15px; }
  .lp-method-sub { font-size: 12.5px; }
}
`

function usePricing() {
  const g = useGateway()
  const [bps, setBps] = useState(600)
  // Until the server answers, promise only what is certain.
  const [soon, setSoon] = useState<string[]>(['emola', 'bim'])
  useEffect(() => {
    let alive = true
    g.checkout.pricing()
      .then(d => {
        if (!alive) return
        if (typeof d?.default_fee_bps === 'number') setBps(d.default_fee_bps)
        setSoon(d?.coming_soon ?? [])
      })
      .catch(() => {})
    return () => { alive = false }
  }, [g])
  return { fee: bpsLabel(bps), soon }
}

const stripe = (s: CSSProperties, red: boolean): CSSProperties => ({ position: 'absolute', transform: 'skewX(22deg)', background: red ? '#FF0F15' : '#0A8205', pointerEvents: 'none', opacity: 0.7, filter: stripeBlur(Number(s.width)), ...s })

function Brand({ dark, size, text }: { dark?: boolean; size: number; text: number }) {
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: size > 30 ? 10 : 9 }}>
      <img src={dark ? '/assets/riha-icon-light.png' : '/assets/riha-icon-dark.png'} alt="RIHA" style={{ width: size, height: size, objectFit: 'contain' }} />
      <span style={{ fontFamily: DISPLAY, fontWeight: 800, fontSize: text, letterSpacing: '.2em', color: dark ? '#fff' : '#0B120E' }}>RIHA</span>
    </div>
  )
}

function CheckLine({ children, size = 15 }: { children: ReactNode; size?: number }) {
  return (
    <div style={{ display: 'flex', gap: 10, alignItems: 'center', fontSize: size }}>
      <Icon name="check" size={18} stroke={2.4} color="#0A8205" />
      {children}
    </div>
  )
}

function Arrow({ size = 18 }: { size?: number }) {
  return <Icon name="arrowRight" size={size} stroke={2.2} color="#93A09A" />
}

function PayCard({ mobile }: { mobile?: boolean }) {
  return (
    <>
      {mobile ? (
        <>
          <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
            <div style={{ width: 34, height: 34, borderRadius: 10, background: '#FFD21F', display: 'grid', placeItems: 'center', fontWeight: 700, fontSize: 13 }}>CM</div>
            <div><div style={{ fontWeight: 600, fontSize: 14 }}>Café Maputo</div><div style={{ fontSize: 11.5, color: '#5E6862' }}>rihagateway.com</div></div>
          </div>
          <div style={{ marginTop: 14, fontSize: 11.5, color: '#5E6862' }}>Total a pagar</div>
          <div style={{ fontFamily: DISPLAY, fontWeight: 700, fontSize: 30, letterSpacing: '-0.03em' }}>1 250,00 <span style={{ fontSize: 15, color: '#5E6862' }}>MT</span></div>
        </>
      ) : (
        <>
          <div style={{ fontSize: 11.5, color: '#5E6862' }}>rihagateway.com</div>
          <div style={{ fontFamily: DISPLAY, fontWeight: 700, fontSize: 28, letterSpacing: '-0.03em', marginTop: 4 }}>1 250,00 <span style={{ fontSize: 14, color: '#5E6862' }}>MT</span></div>
        </>
      )}
      <div style={{ display: 'flex', flexDirection: 'column', gap: 6, marginTop: 12 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 9, height: 40, padding: '0 10px', borderRadius: 12, border: '2px solid #0A8205', background: '#F3FAF1', fontSize: 13, fontWeight: 600 }}>
          <span style={{ width: 22, height: 22, borderRadius: 7, background: '#FDE7E6', color: '#C8102E', display: 'grid', placeItems: 'center', fontSize: 11, fontWeight: 800 }}>M</span>M-Pesa
          <span style={{ marginLeft: 'auto', width: 16, height: 16, borderRadius: '50%', background: '#0A8205' }} />
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 9, height: 40, padding: '0 10px', borderRadius: 12, border: '1px solid #E4E2DA', fontSize: 13, fontWeight: 500 }}>
          <span style={{ width: 22, height: 22, borderRadius: 7, background: '#FFF0D6', color: '#B45F00', display: 'grid', placeItems: 'center', fontSize: 12, fontWeight: 800 }}>e</span>e-Mola<span style={{ marginLeft: 'auto', fontSize: 11, color: '#6B7570' }}>Em breve</span>
        </div>
      </div>
      <div style={{ marginTop: 12, height: 42, borderRadius: 12, background: '#0A8205', color: '#fff', display: 'grid', placeItems: 'center', fontWeight: 600, fontSize: 13.5 }}>Pagar 1 250,00 MT</div>
    </>
  )
}

function Received({ style }: { style: CSSProperties }) {
  return (
    <div style={{ position: 'absolute', display: 'flex', alignItems: 'center', gap: 10, borderRadius: 18, background: 'rgba(28,40,33,.9)', border: '1px solid rgba(255,255,255,.14)', backdropFilter: 'blur(14px)', WebkitBackdropFilter: 'blur(14px)', boxShadow: '0 24px 50px -20px rgba(0,0,0,.9)', ...style }}>
      <div style={{ width: 34, height: 34, flex: 'none', borderRadius: '50%', background: '#0A8205', display: 'grid', placeItems: 'center' }}><Icon name="check" size={18} stroke={2.6} color="#fff" /></div>
      <div><div style={{ fontSize: 12, color: '#AFC0B6' }}>Pagamento recebido</div><div style={{ fontWeight: 600, fontSize: 14.5, whiteSpace: 'nowrap' }}>+1 250,00 MT · M-Pesa</div></div>
    </div>
  )
}

const METHODS = [
  { name: 'M-Pesa', method: 'mpesa', sub: 'Vodacom', extra: ' · 84, 85', ico: <img className="lp-method-ico" src="/assets/methods/mpesa.png" alt="" /> },
  { name: 'e-Mola', method: 'emola', sub: 'Movitel', extra: ' · 86, 87', ico: <img className="lp-method-ico" src="/assets/methods/emola.png" alt="" /> },
  { name: 'Bim IZI', method: 'bim', sub: 'Millennium bim', extra: '', ico: <img className="lp-method-ico" src="/assets/methods/bim.png" alt="" /> },
  { name: 'Visa', sub: 'Em breve', extra: '', ico: <div className="lp-method-ico" style={{ background: '#0B120E', color: '#fff' }}><Icon name="card" size={20} /></div> },
  { name: 'Mastercard', sub: 'Em breve', extra: '', ico: <div className="lp-method-ico" style={{ background: '#0B120E', color: '#fff' }}><Icon name="card" size={20} /></div> },
]

const STEPS = [
  { n: '01', t: 'Crie a conta', d: 'Registe o negócio com o NUIT e o BI. Comece a receber no mesmo dia.' },
  { n: '02', t: 'Crie um link ou ligue a API', d: 'Partilhe o link por WhatsApp ou SMS, ou envie os clientes da sua loja online para o checkout RIHA.' },
  { n: '03', t: 'Receba e levante', d: 'Veja cada pagamento em tempo real e peça o levantamento para o seu banco. Chega em até 72 horas.' },
]

const FOOTER: { h: string; items: { t: string; to?: string; href?: string }[] }[] = [
  { h: 'Produto', items: [{ t: 'Links', href: '#produto' }, { t: 'Checkout', href: '#programadores' }, { t: 'API', to: '/docs' }] },
  { h: 'Empresa', items: [{ t: 'Sobre' }, { t: 'Contacto' }, { t: 'Ajuda', to: '/docs' }] },
  { h: 'Legal', items: [{ t: 'Termos' }, { t: 'Privacidade' }] },
]

function FooterCols({ gap, size }: { gap: number; size: number }) {
  return (
    <>
      {FOOTER.map(col => (
        <div key={col.h} style={{ display: 'flex', flexDirection: 'column', gap, fontSize: size }}>
          <div style={{ fontWeight: 600 }}>{col.h}</div>
          {col.items.map(it => it.to
            ? <Link key={it.t} to={it.to} style={{ color: '#5E6862' }}>{it.t}</Link>
            : it.href
              ? <a key={it.t} href={it.href} style={{ color: '#5E6862' }}>{it.t}</a>
              : <span key={it.t} style={{ color: '#5E6862' }}>{it.t}</span>)}
        </div>
      ))}
    </>
  )
}

export default function Landing() {
  const { fee, soon } = usePricing()

  return (
    <div className="lp">
      <style>{css}</style>

      <div style={{ background: '#0B120E', color: '#fff', position: 'relative', overflow: 'hidden' }}>
        <div className="lp-d" style={{ position: 'absolute', inset: 0 }}>
          <div style={stripe({ top: -120, right: 330, width: 120, height: 1000 }, false)} />
          <div style={stripe({ top: -120, right: 180, width: 120, height: 1000 }, true)} />
          <div style={{ position: 'absolute', inset: 0, background: 'linear-gradient(90deg,#0B120E 38%,rgba(11,18,14,.55) 70%,rgba(11,18,14,.2))' }} />
        </div>
        <div className="lp-m" style={{ position: 'absolute', inset: 0 }}>
          <div style={stripe({ top: -80, right: 62, width: 58, height: 560 }, false)} />
          <div style={stripe({ top: -80, right: -8, width: 58, height: 560 }, true)} />
          <div style={{ position: 'absolute', top: -120, right: -140, width: 420, height: 420, borderRadius: '50%', background: 'radial-gradient(circle,rgba(10,130,5,.35),rgba(11,18,14,0) 65%)' }} />
          <div style={{ position: 'absolute', inset: 0, background: 'linear-gradient(180deg,rgba(11,18,14,0) 0%,rgba(11,18,14,.55) 38%,#0B120E 70%)' }} />
        </div>

        <div className="lp-wrap" style={{ position: 'relative' }}>
          <div className="lp-d lp-topnav">
            <Brand dark size={34} text={19} />
            <nav>
              <a className="lp-navlink" href="#produto">Produto</a>
              <a className="lp-navlink" href="#precos">Preços</a>
              <Link className="lp-navlink" to="/docs">Programadores</Link>
              <Link className="lp-navlink" to="/docs">Ajuda</Link>
            </nav>
            <div style={{ flex: 1 }} />
            <Link to="/entrar" className="lp-btn-top lp-plain" style={{ color: '#fff' }}>Entrar</Link>
            <Link to="/registo" className="lp-btn-top" style={{ background: '#fff', color: '#0B120E' }}>Criar conta</Link>
          </div>
          <div className="lp-m">
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', paddingTop: 'max(14px, env(safe-area-inset-top))' }}>
              <Brand dark size={30} text={17} />
              <Link to="/entrar" style={{ height: 38, padding: '0 16px', borderRadius: 999, background: 'rgba(255,255,255,.1)', border: '1px solid rgba(255,255,255,.18)', color: '#fff', fontWeight: 600, fontSize: 14, display: 'inline-flex', alignItems: 'center', backdropFilter: 'blur(8px)', WebkitBackdropFilter: 'blur(8px)' }}>Entrar</Link>
            </div>
          </div>

          <div className="lp-hero-grid">
            <div>
              <h1 className="lp-h1">Receba de todo <span style={{ color: '#8EE07F' }}>Moçambique</span>. Num só lugar.</h1>
              <p className="lp-lead">Crie links de pagamento em segundos ou integre a nossa API. O dinheiro entra no seu saldo RIHA e segue para o seu banco em até 72 horas.</p>
              <div className="lp-ctas">
                <Link to="/registo" className="lp-cta lp-cta-green">Criar conta grátis</Link>
                <Link to="/docs" className="lp-cta lp-cta-ghost">Ver a API<Icon name="arrowRight" size={18} stroke={2} /></Link>
              </div>
              <div className="lp-trust">
                <span>Sem mensalidade</span><span>·</span><span>Levantamentos em 72h</span>
                <span className="lp-d">·</span><span className="lp-d">Taxa única de {fee}</span>
              </div>
            </div>

            <div className="lp-d lp-hero-vis">
              <div style={{ position: 'absolute', right: 0, top: 20, width: 470, maxWidth: '100%', background: '#F4F3EE', color: '#0B120E', borderRadius: 24, padding: 20, boxShadow: '0 50px 100px -30px rgba(0,0,0,.8)' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                  <div style={{ width: 34, height: 34, borderRadius: 10, background: '#FFD21F', display: 'grid', placeItems: 'center', fontWeight: 700, fontSize: 13 }}>CM</div>
                  <div style={{ fontWeight: 600, fontSize: 14 }}>Café Maputo</div>
                  <div style={{ flex: 1 }} />
                  <span style={{ fontSize: 11.5, fontWeight: 600, color: '#0A6B05', background: '#E2F3DE', padding: '4px 9px', borderRadius: 999 }}>Produção</span>
                </div>
                <div style={{ marginTop: 14, background: '#0B120E', color: '#fff', borderRadius: 18, padding: 18, position: 'relative', overflow: 'hidden' }}>
                  <div style={stripe({ top: -20, right: 40, width: 24, height: 160 }, false)} />
                  <div style={stripe({ top: -20, right: 6, width: 24, height: 160 }, true)} />
                  <div style={{ position: 'relative', fontSize: 12, color: '#A9B5AE' }}>Saldo disponível</div>
                  <div style={{ position: 'relative', fontFamily: DISPLAY, fontWeight: 700, fontSize: 34, letterSpacing: '-0.04em', marginTop: 4 }}>48 320,50 <span style={{ fontSize: 15, color: '#A9B5AE' }}>MT</span></div>
                </div>
                <div style={{ display: 'flex', alignItems: 'flex-end', gap: 5, height: 92, marginTop: 14, padding: 12, background: '#fff', borderRadius: 16 }}>
                  {BARS14.map((b, i) => <div key={i} style={{ flex: 1, height: b.h, borderRadius: 5, background: b.bg }} />)}
                </div>
              </div>
              <div style={{ position: 'absolute', left: 0, top: 190, width: 268, background: '#fff', color: '#0B120E', borderRadius: 24, padding: 18, transform: 'rotate(-4deg)', boxShadow: '0 40px 80px -24px rgba(0,0,0,.75)' }}>
                <PayCard />
              </div>
              <Received style={{ right: 30, bottom: 6, padding: '12px 16px' }} />
            </div>
          </div>
        </div>

        <div className="lp-m" style={{ position: 'relative', paddingBottom: 36 }}>
          <div style={{ position: 'relative', height: 330, maxWidth: 480, margin: '34px auto 0' }}>
            <div style={{ position: 'absolute', left: 22, top: 18, width: 252, background: '#fff', color: '#0B120E', borderRadius: 24, padding: 18, transform: 'rotate(-4deg)', boxShadow: '0 40px 80px -30px rgba(0,0,0,.8)' }}>
              <PayCard mobile />
            </div>
            <Received style={{ right: 16, top: 196, width: 212, padding: '12px 14px', background: 'rgba(28,40,33,.86)' }} />
            <div style={{ position: 'absolute', right: 24, top: 28, display: 'flex', alignItems: 'center', gap: 8, padding: '9px 12px', borderRadius: 999, background: '#fff', color: '#0B120E', fontSize: 12, fontWeight: 600, transform: 'rotate(3deg)', boxShadow: '0 16px 30px -14px rgba(0,0,0,.7)', whiteSpace: 'nowrap' }}>
              <span style={{ width: 8, height: 8, borderRadius: '50%', background: '#FFD21F' }} />Levantamento a caminho
            </div>
          </div>
        </div>
      </div>

      <div id="produto" className="lp-wrap lp-sec lp-sec-first">
        <div className="lp-methods">
          <div>
            <div className="lp-eyebrow">Métodos de pagamento</div>
            <h2 className="lp-h2">Receba como os seus clientes já pagam: pelo telemóvel.</h2>
          </div>
          <div className="lp-methods-grid">
            {METHODS.map(m => (
              <div key={m.name} className="lp-method">
                {m.ico}
                <div>
                  <div className="lp-method-name">{m.name}</div>
                  <div className="lp-method-sub">{m.method && soon.includes(m.method) ? 'Em breve' : <>{m.sub}{m.extra && <span className="lp-mi">{m.extra}</span>}</>}</div>
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>

      <div className="lp-wrap lp-sec">
        <div className="lp-eyebrow">Como funciona</div>
        <h2 className="lp-h2">Três passos. Zero papelada.</h2>
        <div className="lp-d" style={{ display: 'grid', gridTemplateColumns: 'repeat(3, minmax(0,1fr))', gap: 16, marginTop: 32 }}>
          {STEPS.map((s, i) => {
            const last = i === STEPS.length - 1
            return (
              <div key={s.n} style={last ? { background: '#0A8205', color: '#fff', borderRadius: 24, padding: 26 } : { background: '#fff', border: '1px solid #E8E6DF', borderRadius: 24, padding: 26 }}>
                <div style={{ fontFamily: MONO, fontSize: 13, color: last ? '#CDEBC7' : '#5E6862' }}>{s.n}</div>
                <div style={{ fontFamily: DISPLAY, fontWeight: 700, fontSize: 24, letterSpacing: '-0.02em', marginTop: 28 }}>{s.t}</div>
                <div style={{ fontSize: 15, lineHeight: 1.55, color: last ? '#E3F4DF' : '#5E6862', marginTop: 8 }}>{s.d}</div>
              </div>
            )
          })}
        </div>
        <div className="lp-m" style={{ marginTop: 22 }}>
          {STEPS.map((s, i) => {
            const last = i === STEPS.length - 1
            return (
              <div key={s.n} style={{ display: 'flex', gap: 16 }}>
                <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
                  <div style={{ width: 36, height: 36, borderRadius: '50%', background: last ? '#0A8205' : '#0B120E', color: '#fff', display: 'grid', placeItems: 'center', fontFamily: MONO, fontSize: 13, flex: 'none' }}>{s.n}</div>
                  {!last && <div style={{ flex: 1, width: 2, background: '#E1DFD7', margin: '6px 0' }} />}
                </div>
                <div style={{ padding: last ? '6px 0 0' : '6px 0 26px', minWidth: 0 }}>
                  <div style={{ fontWeight: 600, fontSize: 17 }}>{s.t}</div>
                  <div style={{ fontSize: 14.5, lineHeight: 1.5, color: '#5E6862', marginTop: 4 }}>{s.d}</div>
                </div>
              </div>
            )
          })}
        </div>
      </div>

      <div id="programadores" className="lp-wrap lp-sec">
        <div className="lp-d lp-features">
          <div className="lp-dev" style={{ background: '#0B120E', color: '#fff', borderRadius: 28, padding: 34, position: 'relative', overflow: 'hidden', minWidth: 0 }}>
            <div style={stripe({ top: -40, right: 60, width: 40, height: 300 }, false)} />
            <div style={stripe({ top: -40, right: 8, width: 40, height: 300 }, true)} />
            <Link to="/docs" className="lp-plain" style={{ position: 'relative', fontSize: 14, fontWeight: 600, color: '#8EE07F' }}>Programadores</Link>
            <div style={{ position: 'relative', fontFamily: DISPLAY, fontWeight: 700, fontSize: 36, letterSpacing: '-0.03em', marginTop: 10, maxWidth: 420, lineHeight: 1.05 }}>Uma chamada à API. O resto fica connosco.</div>
            <div style={{ position: 'relative', fontSize: 15.5, lineHeight: 1.55, color: '#B9C3BD', marginTop: 12, maxWidth: 440 }}>Crie um checkout, envie o cliente para a RIHA e receba-o de volta com o resultado. Webhooks avisam o seu servidor.</div>
            <div style={{ position: 'relative', display: 'flex', alignItems: 'center', gap: 10, marginTop: 26 }}>
              <div style={{ flex: 1, minWidth: 0, padding: 14, borderRadius: 14, background: 'rgba(255,255,255,.07)', border: '1px solid rgba(255,255,255,.12)' }}><div style={{ fontSize: 14, fontWeight: 600 }}>A sua loja</div><div style={{ fontSize: 12.5, color: '#93A09A', marginTop: 3 }}>cria o checkout</div></div>
              <Arrow />
              <div style={{ flex: 1, minWidth: 0, padding: 14, borderRadius: 14, background: '#0A8205' }}><div style={{ fontSize: 14, fontWeight: 600 }}>rihagateway.com</div><div style={{ fontSize: 12.5, color: '#D4F0CE', marginTop: 3 }}>o cliente paga</div></div>
              <Arrow />
              <div style={{ flex: 1, minWidth: 0, padding: 14, borderRadius: 14, background: 'rgba(255,255,255,.07)', border: '1px solid rgba(255,255,255,.12)' }}><div style={{ fontSize: 14, fontWeight: 600 }}>success_url</div><div style={{ fontSize: 12.5, color: '#93A09A', marginTop: 3 }}>volta à loja</div></div>
            </div>
            <div style={{ position: 'relative', marginTop: 16, padding: 20, borderRadius: 16, background: '#151E19', border: '1px solid rgba(255,255,255,.08)', fontFamily: MONO, fontSize: 13.5, lineHeight: 1.75, color: '#C9D3CD', whiteSpace: 'pre', overflowX: 'auto' }}>
              {'curl https://rihagateway.com/v1/checkout \\\n  -u '}<span style={{ color: '#FFD21F' }}>sk_live_…</span>{': \\\n  -d amount=125000 \\\n  -d currency=MZN \\\n  -d success_url='}<span style={{ color: '#8EE07F' }}>https://loja.co.mz/ok</span>
            </div>
            <Link to="/docs" className="lp-plain" style={{ position: 'relative', display: 'inline-flex', alignItems: 'center', gap: 8, marginTop: 18, fontSize: 14.5, fontWeight: 600, color: '#fff' }}>Ler a documentação<Icon name="arrowRight" size={16} stroke={2} /></Link>
          </div>
          <div style={{ background: '#fff', border: '1px solid #E8E6DF', borderRadius: 28, padding: 30, display: 'flex', gap: 22, alignItems: 'center' }}>
            <div style={{ flex: 1, minWidth: 0 }}>
              <div style={{ fontFamily: DISPLAY, fontWeight: 700, fontSize: 26, letterSpacing: '-0.025em' }}>Links de pagamento</div>
              <div style={{ fontSize: 15, lineHeight: 1.55, color: '#5E6862', marginTop: 8 }}>Sem site, sem código. Crie no telemóvel e partilhe por WhatsApp, SMS ou QR no balcão.</div>
            </div>
            <div style={{ width: 120, height: 120, flex: 'none', borderRadius: 20, background: '#F4F3EE', display: 'grid', placeItems: 'center' }}>
              <QR value="https://pay.rihagateway.com/p/cafe-maputo/bolo" size={92} logo={false} />
            </div>
          </div>
          <div style={{ display: 'grid', gridTemplateColumns: 'minmax(0,1fr) minmax(0,1fr)', gap: 16 }}>
            <div style={{ background: '#fff', border: '1px solid #E8E6DF', borderRadius: 28, padding: 26 }}>
              <div style={{ width: 42, height: 42, borderRadius: 12, background: '#E2F3DE', color: '#0A6B05', display: 'grid', placeItems: 'center' }}><Icon name="shield" size={20} stroke={2} /></div>
              <div style={{ fontWeight: 600, fontSize: 17, marginTop: 18 }}>Checkout seguro</div>
              <div style={{ fontSize: 14, lineHeight: 1.5, color: '#5E6862', marginTop: 6 }}>O cliente confirma com o PIN no próprio telemóvel.</div>
            </div>
            <div style={{ background: '#FFD21F', borderRadius: 28, padding: 26 }}>
              <div style={{ width: 42, height: 42, borderRadius: 12, background: '#0B120E', color: '#FFD21F', display: 'grid', placeItems: 'center' }}><Icon name="clock" size={20} stroke={2} /></div>
              <div style={{ fontWeight: 600, fontSize: 17, marginTop: 18 }}>72h no banco</div>
              <div style={{ fontSize: 14, lineHeight: 1.5, color: '#3B3410', marginTop: 6 }}>Peça o levantamento quando quiser, para qualquer banco.</div>
            </div>
          </div>
        </div>

        <div className="lp-m">
          <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
            <div style={{ background: '#fff', border: '1px solid #E8E6DF', borderRadius: 26, padding: 22, overflow: 'hidden' }}>
              <div style={{ fontFamily: DISPLAY, fontWeight: 700, fontSize: 23, letterSpacing: '-0.025em' }}>Links de pagamento</div>
              <div style={{ fontSize: 14.5, lineHeight: 1.5, color: '#5E6862', marginTop: 6 }}>Sem site, sem código. Crie no telemóvel e partilhe onde os seus clientes estão.</div>
              <div style={{ marginTop: 18, display: 'flex', alignItems: 'center', gap: 10, padding: 12, borderRadius: 16, background: '#F4F3EE' }}>
                <div style={{ width: 36, height: 36, flex: 'none', borderRadius: 10, background: '#E2F3DE', color: '#0A6B05', display: 'grid', placeItems: 'center' }}><Icon name="link" size={18} stroke={2} /></div>
                <div style={{ minWidth: 0, flex: 1 }}>
                  <div style={{ fontWeight: 600, fontSize: 13.5 }}>Bolo de chocolate · 1 kg</div>
                  <div style={{ fontFamily: MONO, fontSize: 11.5, color: '#5E6862', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>pay.rihagateway.com/p/cafe-maputo/bolo</div>
                </div>
              </div>
              <div style={{ display: 'flex', gap: 8, marginTop: 10 }}>
                <div style={{ flex: 1, height: 42, borderRadius: 12, background: '#0B120E', color: '#fff', display: 'grid', placeItems: 'center', fontWeight: 600, fontSize: 13.5 }}>Enviar por WhatsApp</div>
                <div style={{ width: 42, height: 42, flex: 'none', borderRadius: 12, border: '1px solid #E4E2DA', display: 'grid', placeItems: 'center' }}><Icon name="qr" size={18} stroke={2} /></div>
              </div>
            </div>
            <div style={{ background: '#0B120E', color: '#fff', borderRadius: 26, padding: 22, position: 'relative', overflow: 'hidden' }}>
              <div style={stripe({ top: -30, right: 20, width: 26, height: 180 }, false)} />
              <div style={stripe({ top: -30, right: -12, width: 26, height: 180 }, true)} />
              <div style={{ position: 'relative', fontFamily: DISPLAY, fontWeight: 700, fontSize: 23, letterSpacing: '-0.025em' }}>API para a sua loja</div>
              <div style={{ position: 'relative', fontSize: 14.5, lineHeight: 1.5, color: '#B9C3BD', marginTop: 6, maxWidth: 250 }}>Crie um checkout, envie o cliente para a RIHA e receba-o de volta com o resultado.</div>
              <div style={{ position: 'relative', display: 'flex', alignItems: 'center', gap: 6, marginTop: 18 }}>
                <div style={{ flex: 1, minWidth: 0, textAlign: 'center', padding: '10px 4px', borderRadius: 12, background: 'rgba(255,255,255,.07)', border: '1px solid rgba(255,255,255,.12)' }}><div style={{ fontSize: 12.5, fontWeight: 600 }}>A sua loja</div><div style={{ fontSize: 10.5, color: '#93A09A', marginTop: 2 }}>cria checkout</div></div>
                <Arrow size={14} />
                <div style={{ flex: 1, minWidth: 0, textAlign: 'center', padding: '10px 4px', borderRadius: 12, background: '#0A8205' }}><div style={{ fontSize: 12.5, fontWeight: 600 }}>RIHA</div><div style={{ fontSize: 10.5, color: '#D4F0CE', marginTop: 2 }}>cliente paga</div></div>
                <Arrow size={14} />
                <div style={{ flex: 1, minWidth: 0, textAlign: 'center', padding: '10px 4px', borderRadius: 12, background: 'rgba(255,255,255,.07)', border: '1px solid rgba(255,255,255,.12)' }}><div style={{ fontSize: 12.5, fontWeight: 600 }}>A sua loja</div><div style={{ fontSize: 10.5, color: '#93A09A', marginTop: 2 }}>recebe resultado</div></div>
              </div>
              <div style={{ position: 'relative', marginTop: 14, padding: 14, borderRadius: 14, background: '#151E19', border: '1px solid rgba(255,255,255,.08)', fontFamily: MONO, fontSize: 11.5, lineHeight: 1.7, color: '#C9D3CD', whiteSpace: 'pre', overflowX: 'auto' }}>
                <span style={{ color: '#8EE07F' }}>POST</span>{' /v1/checkout\n{ '}<span style={{ color: '#FFD21F' }}>"amount"</span>{': 125000,\n  '}<span style={{ color: '#FFD21F' }}>"currency"</span>{': "MZN",\n  '}<span style={{ color: '#FFD21F' }}>"success_url"</span>{': "https://loja.co.mz/ok" }'}
              </div>
              <Link to="/docs" className="lp-plain" style={{ position: 'relative', display: 'inline-flex', alignItems: 'center', gap: 8, marginTop: 16, fontSize: 14, fontWeight: 600, color: '#fff' }}>Documentação<Icon name="arrowRight" size={16} stroke={2} /></Link>
            </div>
            <div style={{ display: 'grid', gridTemplateColumns: 'minmax(0,1fr) minmax(0,1fr)', gap: 12 }}>
              <div style={{ background: '#fff', border: '1px solid #E8E6DF', borderRadius: 22, padding: 18 }}>
                <div style={{ width: 38, height: 38, borderRadius: 12, background: '#E2F3DE', color: '#0A6B05', display: 'grid', placeItems: 'center' }}><Icon name="shield" size={19} stroke={2} /></div>
                <div style={{ fontWeight: 600, fontSize: 15, marginTop: 14 }}>Checkout seguro</div>
                <div style={{ fontSize: 13, lineHeight: 1.45, color: '#5E6862', marginTop: 4 }}>O cliente confirma com o PIN no próprio telemóvel.</div>
              </div>
              <div style={{ background: '#FFD21F', borderRadius: 22, padding: 18 }}>
                <div style={{ width: 38, height: 38, borderRadius: 12, background: '#0B120E', color: '#FFD21F', display: 'grid', placeItems: 'center' }}><Icon name="clock" size={19} stroke={2} /></div>
                <div style={{ fontWeight: 600, fontSize: 15, marginTop: 14 }}>72h no banco</div>
                <div style={{ fontSize: 13, lineHeight: 1.45, color: '#3B3410', marginTop: 4 }}>Peça o levantamento quando quiser, para qualquer banco.</div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div id="precos" className="lp-wrap lp-sec">
        <div className="lp-d lp-pricing">
          <div>
            <div className="lp-eyebrow">Preço</div>
            <h2 className="lp-h2">Uma taxa. Sem surpresas.</h2>
            <p style={{ margin: '14px 0 0', fontSize: 17, lineHeight: 1.55, color: '#5E6862', maxWidth: 440 }}>Paga apenas quando recebe. A mesma taxa em todos os métodos, para não ter de fazer contas.</p>
          </div>
          <div style={{ background: '#fff', border: '1px solid #E8E6DF', borderRadius: 28, padding: 32, display: 'flex', gap: 32, alignItems: 'center' }}>
            <div>
              <div style={{ fontFamily: DISPLAY, fontWeight: 800, fontSize: 96, lineHeight: 1, letterSpacing: '-0.05em' }}>{fee}</div>
              <div style={{ fontSize: 15, color: '#5E6862', marginTop: 6 }}>por transação concluída</div>
            </div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 12, paddingLeft: 28, borderLeft: '1px solid #EEECE5' }}>
              <CheckLine>Carteiras móveis, sem integrações à parte</CheckLine>
              <CheckLine>Sem mensalidade</CheckLine>
              <CheckLine>Sem taxa de adesão</CheckLine>
            </div>
          </div>
        </div>
        <div className="lp-m">
          <div style={{ background: '#fff', border: '1px solid #E8E6DF', borderRadius: 28, padding: '26px 22px', textAlign: 'center' }}>
            <div style={{ fontSize: 13, fontWeight: 600, color: '#0A7A05' }}>Preço</div>
            <div style={{ fontFamily: DISPLAY, fontWeight: 800, fontSize: 84, lineHeight: 1, letterSpacing: '-0.05em', marginTop: 10 }}>{fee}</div>
            <div style={{ fontSize: 15, color: '#5E6862', marginTop: 6 }}>por transação concluída</div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 10, textAlign: 'left', margin: '22px 0 0', padding: '18px 0 0', borderTop: '1px solid #EEECE5' }}>
              <CheckLine size={14.5}>A mesma taxa para todos os métodos</CheckLine>
              <CheckLine size={14.5}>Sem mensalidade nem taxa de adesão</CheckLine>
              <CheckLine size={14.5}>Só paga quando recebe</CheckLine>
            </div>
          </div>
        </div>
      </div>

      <div className="lp-wrap lp-sec">
        <div className="lp-d">
          <div className="lp-cta-band" style={{ background: '#0B120E', color: '#fff', borderRadius: 32, padding: 56, position: 'relative', overflow: 'hidden' }}>
            <div style={stripe({ top: -60, right: 220, width: 70, height: 420 }, false)} />
            <div style={stripe({ top: -60, right: 130, width: 70, height: 420 }, true)} />
            <div style={{ position: 'relative' }}>
              <div style={{ fontFamily: DISPLAY, fontWeight: 700, fontSize: 48, letterSpacing: '-0.035em', lineHeight: 1 }}>Comece a receber hoje.</div>
              <div style={{ fontSize: 16, color: '#B9C3BD', marginTop: 12 }}>Conta grátis. O primeiro link fica pronto em 2 minutos.</div>
            </div>
            <Link to="/registo" className="lp-cta lp-cta-btn" style={{ position: 'relative', background: '#fff', color: '#0B120E', fontSize: 16 }}>Criar conta grátis</Link>
          </div>
        </div>
        <div className="lp-m">
          <div style={{ background: '#0B120E', color: '#fff', borderRadius: 28, padding: '30px 22px', position: 'relative', overflow: 'hidden' }}>
            <div style={stripe({ bottom: -40, right: 44, width: 40, height: 240 }, false)} />
            <div style={stripe({ bottom: -40, right: -6, width: 40, height: 240 }, true)} />
            <div style={{ position: 'relative', fontFamily: DISPLAY, fontWeight: 700, fontSize: 32, lineHeight: 1.02, letterSpacing: '-0.03em', maxWidth: 230 }}>Comece a receber hoje.</div>
            <div style={{ position: 'relative', fontSize: 14.5, color: '#B9C3BD', marginTop: 10, maxWidth: 220, lineHeight: 1.5 }}>Conta grátis. O primeiro link fica pronto em 2 minutos.</div>
            <Link to="/registo" style={{ position: 'relative', marginTop: 22, height: 52, padding: '0 22px', borderRadius: 14, background: '#fff', color: '#0B120E', fontWeight: 600, fontSize: 15.5, display: 'inline-flex', alignItems: 'center' }}>Criar conta grátis</Link>
          </div>
        </div>
      </div>

      <div className="lp-wrap">
        <div className="lp-d" style={{ padding: '56px 0 40px', display: 'flex', alignItems: 'flex-start', gap: 64 }}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 14, flex: 1 }}>
            <Brand size={30} text={17} />
            <div style={{ fontSize: 13.5, color: '#5E6862' }}>© 2026 RIHA · Maputo, Moçambique · Ajuda: <a href={SUPPORT_TEL} style={{ color: 'inherit' }}>{SUPPORT_PHONE}</a></div>
          </div>
          <FooterCols gap={10} size={14} />
        </div>
        <div className="lp-m" style={{ padding: '36px 0 max(28px, env(safe-area-inset-bottom))' }}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 22 }}>
            <Brand size={28} text={16} />
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, minmax(0,1fr))', gap: 12 }}>
              <FooterCols gap={9} size={13.5} />
            </div>
            <div style={{ fontSize: 12.5, color: '#5E6862', paddingTop: 18, borderTop: '1px solid #E4E2DA' }}>© 2026 RIHA · Maputo, Moçambique · Ajuda: <a href={SUPPORT_TEL} style={{ color: 'inherit' }}>{SUPPORT_PHONE}</a></div>
          </div>
        </div>
      </div>
    </div>
  )
}
