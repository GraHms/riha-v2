import { Fragment, useEffect, useRef, useState } from 'react'
import { useFlow } from './useFlow'

// A gym member subscribing and paying again a month later. Frames, not real calls.
const FRAMES: { base: string; d: number; tap?: [number, number]; sel?: boolean; pin?: number; marta?: number; ev?: number; jump?: boolean; flip?: boolean; notif?: boolean }[] = [
  { base: 'chat', d: 1300 },
  { base: 'chat', tap: [115, 215], d: 450 },
  { base: 'plans', d: 750 },
  { base: 'plans', tap: [115, 155], sel: true, d: 650 },
  { base: 'plans', tap: [115, 442], sel: true, d: 450 },
  { base: 'form', d: 1100 },
  { base: 'form', tap: [115, 442], d: 450 },
  { base: 'form', pin: 0, d: 550 }, { base: 'form', pin: 1, d: 200 }, { base: 'form', pin: 2, d: 200 }, { base: 'form', pin: 3, d: 200 }, { base: 'form', pin: 4, d: 500 },
  { base: 'welcome', marta: 1, ev: 2, d: 2400 },
  { base: 'welcome', marta: 1, ev: 2, jump: true, d: 800 },
  { base: 'sms', marta: 2, ev: 2, jump: true, flip: true, d: 1300 },
  { base: 'sms', marta: 2, ev: 2, notif: true, d: 1000 },
  { base: 'sms', marta: 2, ev: 2, notif: true, tap: [115, 180], d: 450 },
  { base: 'pay', marta: 2, ev: 2, d: 1000 },
  { base: 'pay', marta: 2, ev: 2, tap: [115, 442], d: 450 },
  { base: 'pay', marta: 2, ev: 2, pin: 0, d: 450 }, { base: 'pay', marta: 2, ev: 2, pin: 4, d: 600 },
  { base: 'paid', marta: 3, ev: 3, d: 3000 },
]
const ORDER = ['chat', 'plans', 'form', 'welcome', 'sms', 'pay', 'paid']
const EVENTS = [
  ['18 nov 10:42', 'subscription.created', 'Marta Cossa · Mensal', '#8EE07F'],
  ['18 nov 10:42', 'payment.succeeded', 'nov · 1 800 MT · M-Pesa', '#8EE07F'],
  ['18 dez 09:03', 'payment.succeeded', 'dez · 1 800 MT · M-Pesa', '#8EE07F'],
]
const ROWS = [
  ['João Mabote', 'Mensal · dia 22', 'M-Pesa · 22 out', 'Ativa', '#E2F3DE', '#0A6B05'],
  ['Lúcia Tembe', 'Trimestral', 'e-Mola · 2 out', 'Ativa', '#E2F3DE', '#0A6B05'],
  ['Hélio Bila', 'Mensal · dia 9', '1 reenvio', 'Em dívida', '#FFF1C2', '#6B5200'],
  ['Sara Langa', 'Estudante', 'até jan', 'Pausada', '#E6EEFB', '#2F4E86'],
].map(([n, p, l, s, bg, fg]) => ({ n, p, l, s, bg, fg }))
// On a phone, the camera stays on Marta's phone and visits the dashboard when it changes.
const focusOf = (k: number) => (k === 12 || k === 21 ? 555 : 150)
const fmt = (n: number) => n.toString().replace(/\B(?=(\d{3})+(?!\d))/g, ' ')

export default function GymFlow() {
  const { ref, k, boxH, scaleT, clip, pan, jump } = useFlow(FRAMES, FRAMES.length - 1, focusOf)
  const f = FRAMES[k]
  const [mrr, setMrr] = useState(115200)
  const mrrRef = useRef(mrr)
  mrrRef.current = mrr
  useEffect(() => {
    if (k === 0) { setMrr(115200); return }
    if (k !== 12) return
    const start = mrrRef.current, t0 = Date.now()
    const id = setInterval(() => {
      const p = Math.min(1, (Date.now() - t0) / 900), e = 1 - Math.pow(1 - p, 3)
      setMrr(Math.round((start + (117000 - start) * e) / 10) * 10)
      if (p >= 1) clearInterval(id)
    }, 30)
    return () => clearInterval(id)
  }, [k]) // the count runs when its frame arrives, never again until the loop restarts
  const go = (to: number) => { setMrr(to >= 12 ? 117000 : 115200); jump(to) }
  const bi = ORDER.indexOf(f.base)
  const sc: Record<string, { t: string; o: number }> = {}
  ORDER.forEach((n, i) => { sc[n] = { t: i === bi ? 'translateX(0)' : i < bi ? 'translateX(-40px)' : 'translateX(40px)', o: i === bi ? 1 : 0 } })
  const pinOn = f.pin != null, pinN = f.pin ?? 0, m = f.marta || 0, ev = f.ev || 0
  const stepIdx = k < 2 ? 0 : k < 13 ? 1 : k < 17 ? 2 : 3
  const v = {
    wrapRef: ref, boxH, scaleT, clip, pan, sc,
    clock: k >= 14 ? '09:00' : '10:41', sbC: (f.base === 'sms' || f.base === 'chat') && !f.jump ? '#fff' : '#0B120E',
    tapLinkT: f.tap && f.base === 'chat' ? 'scale(.97)' : 'scale(1)',
    plans: [['Mensal', 'Livre-trânsito, todos os dias', '1 800'], ['Trimestral', '3 meses de uma vez', '4 900'], ['Estudante', 'Com cartão de estudante', '1 200']].map(([n, s, a], i) => ({ n, s, v: a, bd: f.sel && i === 0 ? '2px solid #0B120E' : '1px solid #E4E2DA', t: k === 3 && i === 0 ? 'scale(.97)' : 'scale(1)' })),
    contBg: f.sel ? '#0B120E' : '#B8BFBA', contT: k === 4 ? 'scale(.96)' : 'scale(1)',
    subT: k === 6 ? 'scale(.96)' : 'scale(1)', payT: k === 18 ? 'scale(.96)' : 'scale(1)',
    popT: f.base === 'welcome' ? 'scale(1)' : 'scale(.4)', pop2T: f.base === 'paid' ? 'scale(1)' : 'scale(.4)',
    notifT: f.notif ? 'translateY(0)' : 'translateY(-30px)', notifO: f.notif ? 1 : 0,
    pinO: pinOn ? 1 : 0, pinT: pinOn ? 'translateY(0)' : 'translateY(100%)',
    pinDots: [0, 1, 2, 3].map(i => ({ bg: i < pinN ? '#0B120E' : 'transparent', t: i === pinN - 1 ? 'scale(1.25)' : 'scale(1)' })),
    jumpO: f.jump ? 1 : 0, monA: f.flip ? 'translateY(-30px)' : 'translateY(0)', monB: f.flip ? 'translateY(0)' : 'translateY(30px)',
    tapT: f.tap ? `translate(${f.tap[0]}px, ${f.tap[1]}px)` : 'translate(115px, 300px)', tapO: f.tap ? 1 : 0,
    members: m ? 65 : 64, mrr: fmt(mrr),
    mRowH: m ? '48px' : '0px', mRowBg: m === 1 && k === 12 ? '#FFF6D1' : '#fff',
    mPill: m === 2 ? 'Link enviado' : 'Ativa', mPillBg: m === 2 ? '#FFF1C2' : '#E2F3DE', mPillFg: m === 2 ? '#6B5200' : '#0A6B05',
    mLast: m === 3 ? 'M-Pesa · 18 dez' : m === 2 ? 'vence hoje' : 'M-Pesa · 18 nov',
    rows: ROWS,
    events: EVENTS.map(([time, n, d, c], i) => ({ time, n, d, c, h: i < ev ? '20px' : '0px', o: i < ev ? 1 : 0, t: i < ev ? 'translateY(0)' : 'translateY(8px)' })),
    showSteps: true,
    steps: ['Recebe o link', 'Subscreve e paga', '30 dias depois', 'Paga num toque'].map((l, i) => ({ l, bar: i <= stepIdx ? '#0A8205' : 'rgba(11,18,14,.15)', fg: i === stepIdx ? '#0B120E' : '#8A938E', go: () => go([0, 2, 13, 15][i]) })),
  }
  return (
<div ref={v.wrapRef} style={{ width: '100%', fontFamily: 'var(--sans)', color: '#0B120E' }}>
  <div aria-hidden="true" style={{ width: '100%', height: v.boxH, position: 'relative', overflow: v.clip, borderRadius: '16px' }}>
    <div style={{ position: 'absolute', left: '0', top: '0', width: '880px', height: '540px', transformOrigin: '0 0', transform: v.scaleT, transition: v.pan, borderRadius: '20px', overflow: 'hidden', background: '#EAE8E1' }}>

      <div style={{ position: 'absolute', left: '40px', top: '20px', width: '250px', height: '500px', borderRadius: '38px', background: '#0B120E', padding: '10px', boxSizing: 'border-box', boxShadow: '0 40px 70px -30px rgba(0,0,0,.55)' }}>
        <div style={{ position: 'relative', width: '230px', height: '480px', borderRadius: '30px', overflow: 'hidden', background: '#F4F3EE' }}>
          <div style={{ position: 'absolute', left: '0', right: '0', top: '0', height: '30px', display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '0 20px', fontSize: '11px', fontWeight: '600', zIndex: '5', color: v.sbC, transition: 'color .4s' }}><span>{v.clock}</span><span style={{ width: '18px', height: '9px', borderRadius: '2px', border: '1.3px solid currentColor', boxSizing: 'border-box' }}></span></div>

          <div style={{ position: 'absolute', inset: '0', background: '#EFEAE2', transition: 'transform .55s cubic-bezier(.3,0,.2,1),opacity .45s', transform: v.sc.chat.t, opacity: v.sc.chat.o }}>
            <div style={{ height: '78px', background: '#1F2C26', color: '#fff', display: 'flex', alignItems: 'flex-end', gap: '10px', padding: '0 14px 12px', boxSizing: 'border-box' }}><span style={{ width: '30px', height: '30px', borderRadius: '50%', background: '#FF5A1F', display: 'grid', placeItems: 'center', fontWeight: '800', fontSize: '11px' }}>FG</span><span><span style={{ display: 'block', fontWeight: '600', fontSize: '13px' }}>Força Gym</span><span style={{ display: 'block', fontSize: '10.5px', color: '#A9B5AE' }}>online</span></span></div>
            <div style={{ padding: '14px 12px', display: 'flex', flexDirection: 'column', gap: '8px' }}>
              <div style={{ alignSelf: 'flex-start', maxWidth: '190px', padding: '9px 11px', borderRadius: '14px 14px 14px 4px', background: '#fff', fontSize: '12px', lineHeight: '1.4' }}>Olá Marta! As inscrições de novembro estão abertas. Escolha o seu plano aqui:</div>
              <div style={{ alignSelf: 'flex-start', width: '190px', borderRadius: '14px', background: '#fff', overflow: 'hidden', fontSize: '11.5px', transform: v.tapLinkT, transition: 'transform .15s' }}>
                <div style={{ height: '54px', background: '#0B120E', display: 'flex', alignItems: 'center', padding: '0 12px', gap: '8px' }}><span style={{ width: '26px', height: '26px', borderRadius: '8px', background: '#FF5A1F', color: '#fff', display: 'grid', placeItems: 'center', fontWeight: '800', fontSize: '10px' }}>FG</span><span style={{ color: '#fff', fontWeight: '600', fontSize: '12px' }}>Planos Força Gym</span></div>
                <div style={{ padding: '8px 11px' }}><div style={{ color: '#5E6862', fontSize: '10.5px' }}>pay.rihagateway.com</div><div style={{ color: '#0A6B05', textDecoration: 'underline', marginTop: '2px' }}>pay.rihagateway.com/a/forca-gym</div></div>
              </div>
            </div>
          </div>

          <div style={{ position: 'absolute', inset: '0', padding: '44px 14px 0', boxSizing: 'border-box', transition: 'transform .55s cubic-bezier(.3,0,.2,1),opacity .45s', transform: v.sc.plans.t, opacity: v.sc.plans.o }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}><span style={{ width: '28px', height: '28px', borderRadius: '9px', background: '#FF5A1F', color: '#fff', display: 'grid', placeItems: 'center', fontWeight: '800', fontSize: '10px' }}>FG</span><span style={{ fontWeight: '600', fontSize: '13px' }}>Força Gym</span></div>
            <div style={{ fontFamily: 'var(--display)', fontWeight: '700', fontSize: '21px', letterSpacing: '-0.03em', marginTop: '14px' }}>Escolha o plano</div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '8px', marginTop: '12px' }}>
              {v.plans.map((p, i) => (<Fragment key={i}>
                <div style={{ height: '62px', boxSizing: 'border-box', display: 'flex', alignItems: 'center', gap: '10px', padding: '0 12px', borderRadius: '14px', background: '#fff', border: p.bd, transition: 'border-color .25s,transform .15s', transform: p.t }}><span style={{ flex: '1' }}><span style={{ display: 'block', fontWeight: '600', fontSize: '13px' }}>{p.n}</span><span style={{ display: 'block', fontSize: '10.5px', color: '#5E6862', marginTop: '1px' }}>{p.s}</span></span><span style={{ fontFamily: 'var(--display)', fontWeight: '700', fontSize: '15px' }}>{p.v}</span></div>
              </Fragment>))}
            </div>
            <div style={{ position: 'absolute', left: '14px', right: '14px', top: '420px', height: '44px', borderRadius: '13px', background: v.contBg, color: '#fff', display: 'grid', placeItems: 'center', fontWeight: '600', fontSize: '13px', transition: 'background .3s,transform .15s', transform: v.contT }}>Continuar</div>
          </div>

          <div style={{ position: 'absolute', inset: '0', padding: '44px 14px 0', boxSizing: 'border-box', transition: 'transform .55s cubic-bezier(.3,0,.2,1),opacity .45s', transform: v.sc.form.t, opacity: v.sc.form.o }}>
            <div style={{ fontSize: '11px', color: '#5E6862' }}>Força Gym · Mensal</div>
            <div style={{ fontFamily: 'var(--display)', fontWeight: '700', fontSize: '21px', letterSpacing: '-0.03em', marginTop: '4px' }}>1 800 MT por mês</div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '8px', marginTop: '14px', fontSize: '12.5px' }}>
              <div style={{ height: '40px', borderRadius: '11px', border: '1px solid #E2E0D8', background: '#fff', display: 'flex', alignItems: 'center', padding: '0 12px' }}>Marta Cossa</div>
              <div style={{ height: '40px', borderRadius: '11px', border: '1px solid #E2E0D8', background: '#fff', display: 'flex', alignItems: 'center', padding: '0 12px' }}>+258 84 552 1907</div>
              <div style={{ height: '44px', borderRadius: '11px', border: '2px solid #0B120E', background: '#fff', display: 'flex', alignItems: 'center', gap: '8px', padding: '0 10px', boxSizing: 'border-box', fontWeight: '600' }}><img src="/assets/methods/mpesa.png" alt="" style={{ width: '22px', height: '22px', borderRadius: '7px', background: '#FDE7E6', color: '#C8102E', display: 'grid', placeItems: 'center', fontWeight: '800', fontSize: '10px', objectFit: 'cover' }} />M-Pesa</div>
            </div>
            <div style={{ marginTop: '12px', padding: '10px 11px', borderRadius: '11px', background: '#fff', border: '1px solid #E8E6DF', fontSize: '10.5px', lineHeight: '1.45', color: '#3E4742' }}>Aceito pagar 1 800 MT ao Força Gym no dia 18 de cada mês. Recebo o link por SMS antes de cada data.</div>
            <div style={{ position: 'absolute', left: '14px', right: '14px', top: '420px', height: '44px', borderRadius: '13px', background: '#0A8205', color: '#fff', display: 'grid', placeItems: 'center', fontWeight: '600', fontSize: '13px', transition: 'transform .15s', transform: v.subT }}>Subscrever · 1 800 MT</div>
          </div>

          <div style={{ position: 'absolute', inset: '0', padding: '60px 18px 0', boxSizing: 'border-box', textAlign: 'center', display: 'flex', flexDirection: 'column', alignItems: 'center', transition: 'transform .55s cubic-bezier(.3,0,.2,1),opacity .45s', transform: v.sc.welcome.t, opacity: v.sc.welcome.o }}>
            <div style={{ width: '54px', height: '54px', borderRadius: '18px', background: '#0A8205', display: 'grid', placeItems: 'center', transform: v.popT, transition: 'transform .5s cubic-bezier(.3,1.6,.5,1)' }}><svg width="26" height="26" viewBox="0 0 24 24" style={{ fill: 'none', stroke: '#fff', strokeWidth: '2.8', strokeLinecap: 'round', strokeLinejoin: 'round' }}><path d="M20 6 9 17l-5-5"></path></svg></div>
            <div style={{ fontFamily: 'var(--display)', fontWeight: '700', fontSize: '22px', letterSpacing: '-0.03em', marginTop: '14px', lineHeight: '1.1' }}>Bem-vinda ao Força Gym</div>
            <div style={{ fontSize: '12px', color: '#3E4742', marginTop: '6px' }}>Novembro pago · 1 800 MT</div>
            <div style={{ width: '100%', marginTop: '18px', padding: '12px', borderRadius: '14px', background: '#fff', border: '1px solid #E8E6DF', textAlign: 'left', boxSizing: 'border-box' }}><div style={{ fontSize: '10.5px', color: '#5E6862' }}>Próximo pagamento</div><div style={{ fontWeight: '600', fontSize: '14px', marginTop: '2px' }}>18 de dezembro</div><div style={{ fontSize: '10.5px', color: '#5E6862', marginTop: '6px' }}>Recebe o link por SMS. Nada é cobrado sem o seu PIN.</div></div>
          </div>

          <div style={{ position: 'absolute', inset: '0', background: '#0B120E', color: '#fff', transition: 'transform .55s cubic-bezier(.3,0,.2,1),opacity .45s', transform: v.sc.sms.t, opacity: v.sc.sms.o }}>
            <div style={{ textAlign: 'center', marginTop: '52px', fontFamily: 'var(--display)', fontWeight: '600', fontSize: '48px', letterSpacing: '-0.03em' }}>09:00</div>
            <div style={{ textAlign: 'center', fontSize: '11.5px', color: '#A9B5AE' }}>quinta-feira, 18 de dezembro</div>
            <div style={{ margin: '22px 10px 0', padding: '11px 12px', borderRadius: '16px', background: 'rgba(255,255,255,.14)', fontSize: '11.5px', lineHeight: '1.4', transform: v.notifT, opacity: v.notifO, transition: 'transform .5s cubic-bezier(.3,1.3,.5,1),opacity .3s' }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '10.5px', color: '#C9D3CD' }}><b style={{ fontWeight: '600', color: '#fff' }}>ForcaGym</b><span>agora</span></div>
              <div style={{ marginTop: '4px' }}>Mensalidade de dezembro, 1 800 MT. Pague aqui: <span style={{ textDecoration: 'underline' }}>pay.rihagateway.com/i/9TQ4</span></div>
            </div>
          </div>

          <div style={{ position: 'absolute', inset: '0', padding: '44px 14px 0', boxSizing: 'border-box', transition: 'transform .55s cubic-bezier(.3,0,.2,1),opacity .45s', transform: v.sc.pay.t, opacity: v.sc.pay.o }}>
            <div style={{ padding: '14px', borderRadius: '16px', background: '#fff', border: '1px solid #E8E6DF' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}><span style={{ width: '28px', height: '28px', borderRadius: '9px', background: '#FF5A1F', color: '#fff', display: 'grid', placeItems: 'center', fontWeight: '800', fontSize: '10px' }}>FG</span><span><span style={{ display: 'block', fontWeight: '600', fontSize: '12.5px' }}>Força Gym</span><span style={{ display: 'block', fontSize: '10.5px', color: '#5E6862' }}>Marta Cossa · Mensal</span></span></div>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-end', marginTop: '12px', paddingTop: '10px', borderTop: '1px dashed #E1DFD7' }}><span style={{ fontSize: '11px', color: '#5E6862' }}>Dezembro</span><span style={{ fontFamily: 'var(--display)', fontWeight: '700', fontSize: '22px', letterSpacing: '-0.03em' }}>1 800,00</span></div>
            </div>
            <div style={{ fontSize: '12px', fontWeight: '600', margin: '16px 2px 8px' }}>Pagar com</div>
            <div style={{ height: '52px', borderRadius: '13px', border: '2px solid #0B120E', background: '#fff', display: 'flex', alignItems: 'center', gap: '8px', padding: '0 10px', boxSizing: 'border-box' }}><img src="/assets/methods/mpesa.png" alt="" style={{ width: '26px', height: '26px', borderRadius: '8px', background: '#FDE7E6', color: '#C8102E', display: 'grid', placeItems: 'center', fontWeight: '800', fontSize: '11px', objectFit: 'cover' }} /><span><span style={{ display: 'block', fontWeight: '600', fontSize: '12.5px' }}>M-Pesa</span><span style={{ display: 'block', fontSize: '10px', color: '#5E6862' }}>84 552 1907 · usado da última vez</span></span></div>
            <div style={{ position: 'absolute', left: '14px', right: '14px', top: '420px', height: '44px', borderRadius: '13px', background: '#0A8205', color: '#fff', display: 'grid', placeItems: 'center', fontWeight: '600', fontSize: '13px', transition: 'transform .15s', transform: v.payT }}>Pagar 1 800,00 MT</div>
          </div>

          <div style={{ position: 'absolute', inset: '0', padding: '60px 18px 0', boxSizing: 'border-box', textAlign: 'center', display: 'flex', flexDirection: 'column', alignItems: 'center', transition: 'transform .55s cubic-bezier(.3,0,.2,1),opacity .45s', transform: v.sc.paid.t, opacity: v.sc.paid.o }}>
            <div style={{ width: '54px', height: '54px', borderRadius: '18px', background: '#0A8205', display: 'grid', placeItems: 'center', transform: v.pop2T, transition: 'transform .5s cubic-bezier(.3,1.6,.5,1)' }}><svg width="26" height="26" viewBox="0 0 24 24" style={{ fill: 'none', stroke: '#fff', strokeWidth: '2.8', strokeLinecap: 'round', strokeLinejoin: 'round' }}><path d="M20 6 9 17l-5-5"></path></svg></div>
            <div style={{ fontFamily: 'var(--display)', fontWeight: '700', fontSize: '22px', letterSpacing: '-0.03em', marginTop: '14px' }}>Dezembro pago</div>
            <div style={{ fontSize: '12px', color: '#3E4742', marginTop: '6px' }}>Próximo: 18 de janeiro</div>
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(6,1fr)', gap: '3px', width: '100%', marginTop: '18px', fontFamily: 'var(--mono)', fontSize: '10px' }}>
              <span style={{ padding: '6px 0', borderRadius: '5px', background: '#0A8205', color: '#fff' }}>N</span><span style={{ padding: '6px 0', borderRadius: '5px', background: '#0A8205', color: '#fff' }}>D</span><span style={{ padding: '6px 0', borderRadius: '5px', border: '1px solid #C9CDC6', boxSizing: 'border-box' }}>J</span><span style={{ padding: '6px 0', borderRadius: '5px', border: '1px solid #E2E0D8', color: '#8A938E', boxSizing: 'border-box' }}>F</span><span style={{ padding: '6px 0', borderRadius: '5px', border: '1px solid #E2E0D8', color: '#8A938E', boxSizing: 'border-box' }}>M</span><span style={{ padding: '6px 0', borderRadius: '5px', border: '1px solid #E2E0D8', color: '#8A938E', boxSizing: 'border-box' }}>A</span>
            </div>
          </div>

          <div style={{ position: 'absolute', inset: '0', background: 'rgba(11,18,14,.55)', display: 'flex', alignItems: 'flex-end', opacity: v.pinO, transition: 'opacity .3s', zIndex: '6' }}>
            <div style={{ width: '100%', padding: '16px 16px 26px', boxSizing: 'border-box', borderRadius: '20px 20px 0 0', background: '#fff', transform: v.pinT, transition: 'transform .4s cubic-bezier(.3,0,.2,1)' }}>
              <div style={{ fontWeight: '700', color: '#C8102E', fontSize: '12px' }}>M-Pesa</div>
              <div style={{ fontSize: '11.5px', lineHeight: '1.45', marginTop: '6px' }}>Confirme o pagamento de 1 800,00 MT. Insira o PIN:</div>
              <div style={{ display: 'flex', gap: '10px', justifyContent: 'center', marginTop: '14px' }}>{v.pinDots.map((d, i) => (<Fragment key={i}><span style={{ width: '12px', height: '12px', borderRadius: '50%', border: '2px solid #0B120E', boxSizing: 'border-box', background: d.bg, transform: d.t, transition: 'background .15s,transform .2s' }}></span></Fragment>))}</div>
            </div>
          </div>

          <div style={{ position: 'absolute', inset: '0', background: '#0A8205', color: '#fff', display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: '14px', opacity: v.jumpO, transition: 'opacity .4s', zIndex: '7' }}>
            <div style={{ width: '120px', height: '132px', borderRadius: '20px', background: '#fff', color: '#0B120E', overflow: 'hidden', boxShadow: '0 20px 40px -16px rgba(0,0,0,.5)' }}>
              <div style={{ height: '34px', background: '#0B120E', color: '#fff', display: 'grid', placeItems: 'center', fontSize: '11px', fontWeight: '700', letterSpacing: '.12em', overflow: 'hidden', position: 'relative' }}><span style={{ position: 'absolute', transition: 'transform .6s cubic-bezier(.6,0,.2,1)', transform: v.monA }}>NOV</span><span style={{ position: 'absolute', transition: 'transform .6s cubic-bezier(.6,0,.2,1)', transform: v.monB }}>DEZ</span></div>
              <div style={{ height: '98px', display: 'grid', placeItems: 'center', fontFamily: 'var(--display)', fontWeight: '700', fontSize: '54px', letterSpacing: '-0.04em' }}>18</div>
            </div>
            <div style={{ fontWeight: '600', fontSize: '14px' }}>30 dias depois</div>
          </div>

          <div style={{ position: 'absolute', left: '0', top: '0', width: '44px', height: '44px', margin: '-22px 0 0 -22px', borderRadius: '50%', background: 'rgba(11,18,14,.22)', border: '2px solid rgba(255,255,255,.9)', boxSizing: 'border-box', pointerEvents: 'none', zIndex: '9', transform: v.tapT, opacity: v.tapO, transition: 'opacity .2s' }}></div>
        </div>
      </div>

      <div style={{ position: 'absolute', left: '330px', top: '20px', width: '510px', height: '500px', borderRadius: '22px', background: '#fff', boxShadow: '0 30px 60px -36px rgba(0,0,0,.4)', overflow: 'hidden', display: 'flex', flexDirection: 'column' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '10px', padding: '16px 20px', borderBottom: '1px solid #EEECE5' }}><span style={{ width: '30px', height: '30px', borderRadius: '9px', background: '#FF5A1F', color: '#fff', display: 'grid', placeItems: 'center', fontWeight: '800', fontSize: '11px' }}>FG</span><span style={{ flex: '1' }}><span style={{ display: 'block', fontSize: '11px', color: '#5E6862' }}>Força Gym · RIHA</span><span style={{ display: 'block', fontWeight: '600', fontSize: '14.5px' }}>Assinaturas</span></span><span style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '11px', fontWeight: '600', color: '#0A6B05' }}><span style={{ width: '7px', height: '7px', borderRadius: '50%', background: '#0A8205' }}></span>ao vivo</span></div>
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '10px', padding: '14px 20px 0' }}>
          <div style={{ padding: '12px 14px', borderRadius: '14px', background: '#F4F3EE' }}><div style={{ fontSize: '11px', color: '#5E6862' }}>Membros ativos</div><div style={{ fontFamily: 'var(--display)', fontWeight: '700', fontSize: '26px', letterSpacing: '-0.03em', marginTop: '2px' }}>{v.members}</div></div>
          <div style={{ padding: '12px 14px', borderRadius: '14px', background: '#0B120E', color: '#fff' }}><div style={{ fontSize: '11px', color: '#A9B5AE' }}>A receber por mês</div><div style={{ fontFamily: 'var(--display)', fontWeight: '700', fontSize: '26px', letterSpacing: '-0.03em', marginTop: '2px' }}>{v.mrr} <span style={{ fontSize: '12px', color: '#A9B5AE' }}>MT</span></div></div>
        </div>
        <div style={{ margin: '12px 20px 0', border: '1px solid #EEECE5', borderRadius: '14px', overflow: 'hidden', fontSize: '12.5px' }}>
          <div style={{ overflow: 'hidden', maxHeight: v.mRowH, transition: 'max-height .5s cubic-bezier(.3,0,.2,1)' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '10px', height: '48px', padding: '0 14px', borderBottom: '1px solid #F1EFE9', background: v.mRowBg, transition: 'background 1.2s' }}><span style={{ flex: '1', minWidth: '0' }}><span style={{ display: 'block', fontWeight: '600' }}>Marta Cossa</span><span style={{ display: 'block', fontSize: '10.5px', color: '#5E6862' }}>Mensal · dia 18</span></span><span style={{ fontSize: '11px', color: '#5E6862' }}>{v.mLast}</span><span style={{ fontSize: '10.5px', fontWeight: '600', padding: '2px 8px', borderRadius: '999px', background: v.mPillBg, color: v.mPillFg, transition: 'background .3s' }}>{v.mPill}</span></div>
          </div>
          {v.rows.map((r, i) => (<Fragment key={i}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '10px', height: '48px', padding: '0 14px', borderBottom: '1px solid #F1EFE9' }}><span style={{ flex: '1', minWidth: '0' }}><span style={{ display: 'block', fontWeight: '600' }}>{r.n}</span><span style={{ display: 'block', fontSize: '10.5px', color: '#5E6862' }}>{r.p}</span></span><span style={{ fontSize: '11px', color: '#5E6862' }}>{r.l}</span><span style={{ fontSize: '10.5px', fontWeight: '600', padding: '2px 8px', borderRadius: '999px', background: r.bg, color: r.fg }}>{r.s}</span></div>
          </Fragment>))}
        </div>
        <div style={{ flex: '1' }}></div>
        <div style={{ margin: '0 20px 16px', padding: '12px 14px', borderRadius: '14px', background: '#0B120E', fontFamily: 'var(--mono)', fontSize: '11px', lineHeight: '1.5', color: '#C9D3CD', height: '104px', boxSizing: 'border-box', display: 'flex', flexDirection: 'column', justifyContent: 'flex-end', overflow: 'hidden' }}>
          {v.events.map((e, i) => (<Fragment key={i}>
            <div style={{ display: 'flex', gap: '10px', maxHeight: e.h, opacity: e.o, transform: e.t, transition: 'max-height .4s,opacity .4s,transform .4s', overflow: 'hidden', whiteSpace: 'nowrap' }}><span style={{ color: '#6F7D75' }}>{e.time}</span><span style={{ color: e.c }}>{e.n}</span><span style={{ color: '#6F7D75', overflow: 'hidden', textOverflow: 'ellipsis' }}>{e.d}</span></div>
          </Fragment>))}
        </div>
      </div>
    </div>
  </div>
  {v.showSteps && (<>
    <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4,minmax(0,1fr))', gap: '8px', marginTop: '16px' }}>
      {v.steps.map((st, i) => (<Fragment key={i}>
        <button onClick={st.go} style={{ display: 'flex', flexDirection: 'column', gap: '8px', padding: '0', border: '0', background: 'none', textAlign: 'left', cursor: 'pointer' }}>
          <span style={{ width: '100%', height: '3px', borderRadius: '2px', background: st.bar, transition: 'background .3s' }}></span>
          <span style={{ fontSize: '13px', fontWeight: '600', color: st.fg, transition: 'color .3s' }}>{st.l}</span>
        </button>
      </Fragment>))}
    </div>
  </>)}
</div>
  )
}
