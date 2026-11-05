import { Fragment } from 'react'
import { useFlow } from './useFlow'

// The store-to-RIHA-and-back story on the landing page. Frames, not real calls.
const FRAMES: { sc: string; d: number; x?: number; y?: number; move?: boolean; press?: boolean; hide?: boolean; riha?: boolean; mp?: boolean; load?: boolean; pin?: number; back?: boolean }[] = [
  { sc: 'cart', x: 760, y: 160, d: 1100 },
  { sc: 'cart', x: 700, y: 382, d: 900, move: true },
  { sc: 'cart', x: 700, y: 382, d: 260, press: true },
  { sc: 'storeCheckout', x: 700, y: 382, d: 700 },
  { sc: 'storeCheckout', x: 240, y: 196, d: 850, move: true },
  { sc: 'storeCheckout', x: 240, y: 196, d: 450, press: true, riha: true },
  { sc: 'storeCheckout', x: 700, y: 382, d: 850, move: true, riha: true },
  { sc: 'storeCheckout', x: 700, y: 382, d: 260, press: true, riha: true },
  { sc: 'redirect', x: 700, y: 382, d: 900, hide: true, load: true },
  { sc: 'riha', x: 560, y: 120, d: 700 },
  { sc: 'riha', x: 380, y: 196, d: 800, move: true },
  { sc: 'riha', x: 380, y: 196, d: 450, press: true, mp: true },
  { sc: 'riha', x: 440, y: 438, d: 850, move: true, mp: true },
  { sc: 'riha', x: 440, y: 438, d: 260, press: true, mp: true },
  { sc: 'pin', d: 700, hide: true, pin: 0 },
  { sc: 'pin', d: 280, hide: true, pin: 1 },
  { sc: 'pin', d: 280, hide: true, pin: 2 },
  { sc: 'pin', d: 280, hide: true, pin: 3 },
  { sc: 'pin', d: 500, hide: true, pin: 4 },
  { sc: 'pin', d: 600, hide: true, pin: 5 },
  { sc: 'rihaOk', d: 200, hide: true },
  { sc: 'rihaOk', d: 1400, hide: true, back: true },
  { sc: 'redirectBack', d: 600, hide: true, load: true },
  { sc: 'done', d: 3200, hide: true },
]
const PAY_HOST = 'pay.rihagateway.com'
const URLS: Record<string, [string, string]> = {
  cart: ['loja.co.mz', '/carrinho'], storeCheckout: ['loja.co.mz', '/checkout'],
  redirect: [PAY_HOST, '/c/cs_live_8Hq2…'], riha: [PAY_HOST, '/c/cs_live_8Hq2…'], pin: [PAY_HOST, '/c/cs_live_8Hq2…'], rihaOk: [PAY_HOST, '/c/cs_live_8Hq2…'],
  redirectBack: ['loja.co.mz', '/encomenda/4471?riha=ok'], done: ['loja.co.mz', '/encomenda/4471?riha=ok'],
}
const STARTS = [0, 8, 14, 22]
// On a phone, the camera follows the cursor in the store, frames the RIHA card, then
// goes from the waiting message to the phone that asks for the PIN.
const focusOf = (k: number) => {
  const f = FRAMES[k]
  if (f.sc === 'pin') return k === 14 ? 240 : 640
  if (f.sc !== 'cart' && f.sc !== 'storeCheckout') return 440
  return f.x ?? 440
}

export default function CheckoutFlow() {
  const { ref, k, boxH, scaleT, clip, pan, jump } = useFlow(FRAMES, FRAMES.length - 1, focusOf)
  const f = FRAMES[k], sc = f.sc
  let last = f
  for (let i = k; i >= 0; i--) { if (FRAMES[i].x != null) { last = FRAMES[i]; break } }
  const onStore = sc === 'cart' || sc === 'storeCheckout'
  const url = URLS[sc]
  const isRiha = url[0].startsWith('pay')
  const pinN = f.pin ?? 0
  const stepIdx = onStore ? 0 : (sc === 'redirect' || sc === 'riha') ? 1 : (sc === 'pin' || sc === 'rihaOk') ? 2 : 3
  const v = {
    wrapRef: ref, boxH, scaleT, clip, pan,
    s: { store: onStore, cart: sc === 'cart', storeCheckout: sc === 'storeCheckout', redirect: sc === 'redirect' || sc === 'redirectBack', riha: sc === 'riha', pin: sc === 'pin', rihaOk: sc === 'rihaOk', done: sc === 'done' },
    urlHost: url[0], urlPath: url[1], urlW: isRiha ? 600 : 400, urlC: isRiha ? '#0A6B05' : '#3E4742',
    loadW: f.load ? '100%' : '0%', loadT: f.load ? 'width .8s ease' : 'none',
    storeTitle: sc === 'cart' ? 'O seu carrinho' : 'Pagamento',
    storeBtn: sc === 'cart' ? 'Finalizar compra' : 'Pagar 3 450 MT',
    storeBtnT: f.press && onStore && !(sc === 'storeCheckout' && k === 5) ? 'scale(.96)' : 'scale(1)',
    rihaBd: f.riha ? '2px solid #7A2E12' : '1px solid #EFE6D8', rihaRc: f.riha ? '#7A2E12' : '#D8CBB8', rihaDot: f.riha ? '#7A2E12' : 'transparent',
    mpBd: f.mp ? '2px solid #0B120E' : '1px solid #E4E2DA', mpBg: f.mp ? '#FAFAF7' : '#fff', mpRc: f.mp ? '#0B120E' : '#C9CDC6', mpDot: f.mp ? '#0B120E' : 'transparent',
    numTxt: f.mp ? '+258 84 123 4567' : 'Número de telemóvel', numC: f.mp ? '#0B120E' : '#8A938E',
    payBg: f.mp ? '#0A8205' : '#B8BFBA', payBtnT: f.press && sc === 'riha' && k === 13 ? 'scale(.96)' : 'scale(1)',
    pinDots: [0, 1, 2, 3].map(i => ({ bg: i < pinN ? '#0B120E' : 'transparent' })),
    okBg: pinN >= 5 ? '#0A8205' : '#3E4742', pinBar: Math.min(100, pinN * 22) + '%',
    backBar: f.back ? '100%' : '0%',
    curT: `translate(${last.x ?? 700}px, ${last.y ?? 380}px)`, curTr: f.move ? 'transform .8s cubic-bezier(.45,0,.2,1), opacity .3s' : 'opacity .3s',
    curO: f.hide ? 0 : 1, ringS: f.press ? 1.4 : 0,
    showSteps: true,
    steps: ['Na loja', 'RIHA', 'Confirma com PIN', 'De volta à loja'].map((l, i) => ({ l, bar: i <= stepIdx ? '#0A8205' : 'rgba(11,18,14,.15)', fg: i === stepIdx ? '#0B120E' : '#8A938E', go: () => jump(STARTS[i]) })),
  }
  return (
<div ref={v.wrapRef} style={{ width: '100%', fontFamily: 'var(--sans)', color: '#0B120E' }}>
  <div aria-hidden="true" style={{ width: '100%', height: v.boxH, position: 'relative', overflow: v.clip, borderRadius: '16px' }}>
    <div style={{ position: 'absolute', left: '0', top: '0', width: '880px', height: '540px', transformOrigin: '0 0', transform: v.scaleT, transition: v.pan, borderRadius: '16px', overflow: 'hidden', background: '#fff', boxShadow: '0 0 0 1px rgba(11,18,14,.1),0 40px 80px -30px rgba(0,0,0,.45)' }}>
      <div style={{ height: '44px', display: 'flex', alignItems: 'center', gap: '14px', padding: '0 14px', background: '#EEEDE8', borderBottom: '1px solid #E1DFD7' }}>
        <div style={{ display: 'flex', gap: '7px' }}><span style={{ width: '11px', height: '11px', borderRadius: '50%', background: '#D5D3CB' }}></span><span style={{ width: '11px', height: '11px', borderRadius: '50%', background: '#D5D3CB' }}></span><span style={{ width: '11px', height: '11px', borderRadius: '50%', background: '#D5D3CB' }}></span></div>
        <div style={{ flex: '1', maxWidth: '520px', margin: '0 auto', height: '28px', borderRadius: '8px', background: '#fff', display: 'flex', alignItems: 'center', gap: '8px', padding: '0 12px', fontSize: '12.5px', color: '#3E4742', position: 'relative', overflow: 'hidden' }}>
          <svg width="11" height="11" viewBox="0 0 24 24" style={{ fill: 'none', stroke: 'currentColor', strokeWidth: '2.4', strokeLinecap: 'round', strokeLinejoin: 'round', flex: 'none' }}><rect x="4" y="11" width="16" height="10" rx="2"></rect><path d="M8 11V7a4 4 0 0 1 8 0v4"></path></svg>
          <span style={{ fontWeight: v.urlW, color: v.urlC }}>{v.urlHost}</span><span style={{ color: '#8A938E', marginLeft: '-8px' }}>{v.urlPath}</span>
          <span style={{ position: 'absolute', left: '0', bottom: '0', height: '2px', background: '#0A8205', width: v.loadW, transition: v.loadT }}></span>
        </div>
        <div style={{ width: '54px' }}></div>
      </div>
      <div style={{ position: 'relative', height: '496px', overflow: 'hidden' }}>

        {v.s.store && (<>
          <div style={{ position: 'absolute', inset: '0', background: '#FBF7F1' }}>
            <div style={{ height: '56px', display: 'flex', alignItems: 'center', gap: '28px', padding: '0 40px', borderBottom: '1px solid #EFE6D8' }}><span style={{ fontFamily: 'var(--display)', fontWeight: '800', fontSize: '19px', letterSpacing: '-0.02em', color: '#7A2E12' }}>Capulana<span style={{ color: '#D9822B' }}>.</span></span><span style={{ fontSize: '13px', color: '#6E5B4C' }}>Tecidos</span><span style={{ fontSize: '13px', color: '#6E5B4C' }}>Bolsas</span><span style={{ fontSize: '13px', color: '#6E5B4C' }}>Novidades</span><span style={{ flex: '1' }}></span><span style={{ fontSize: '13px', fontWeight: '600', color: '#7A2E12' }}>Carrinho (3)</span></div>
            <div style={{ position: 'absolute', left: '40px', top: '84px', width: '480px' }}>
              <div style={{ fontFamily: 'var(--display)', fontWeight: '700', fontSize: '26px', letterSpacing: '-0.02em', color: '#2A1A10' }}>{v.storeTitle}</div>
              {v.s.cart && (<>
                <div style={{ display: 'flex', flexDirection: 'column', gap: '10px', marginTop: '18px' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '14px', padding: '12px', borderRadius: '14px', background: '#fff', border: '1px solid #EFE6D8' }}><span style={{ width: '56px', height: '56px', borderRadius: '10px', background: 'repeating-linear-gradient(45deg,#D9822B 0 8px,#7A2E12 8px 16px)' }}></span><span style={{ flex: '1' }}><span style={{ display: 'block', fontWeight: '600', fontSize: '14px' }}>Capulana tradicional</span><span style={{ display: 'block', fontSize: '12.5px', color: '#6E5B4C' }}>2 × 900 MT</span></span><span style={{ fontWeight: '600', fontSize: '14px' }}>1 800 MT</span></div>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '14px', padding: '12px', borderRadius: '14px', background: '#fff', border: '1px solid #EFE6D8' }}><span style={{ width: '56px', height: '56px', borderRadius: '10px', background: 'repeating-linear-gradient(-45deg,#2F6B4F 0 7px,#E8C468 7px 14px)' }}></span><span style={{ flex: '1' }}><span style={{ display: 'block', fontWeight: '600', fontSize: '14px' }}>Bolsa de capulana</span><span style={{ display: 'block', fontSize: '12.5px', color: '#6E5B4C' }}>1 × 1 650 MT</span></span><span style={{ fontWeight: '600', fontSize: '14px' }}>1 650 MT</span></div>
                </div>
              </>)}
              {v.s.storeCheckout && (<>
                <div style={{ fontSize: '13.5px', color: '#6E5B4C', marginTop: '6px' }}>Como quer pagar?</div>
                <div style={{ position: 'absolute', left: '0', top: '76px', width: '480px', height: '72px', boxSizing: 'border-box', display: 'flex', alignItems: 'center', gap: '14px', padding: '0 16px', borderRadius: '14px', background: '#fff', border: v.rihaBd }}>
                  <span style={{ width: '20px', height: '20px', flex: 'none', borderRadius: '50%', border: `2px solid ${v.rihaRc}`, display: 'grid', placeItems: 'center', boxSizing: 'border-box' }}><span style={{ width: '10px', height: '10px', borderRadius: '50%', background: v.rihaDot }}></span></span>
                  <span style={{ flex: '1' }}><span style={{ display: 'flex', alignItems: 'center', gap: '8px', fontWeight: '600', fontSize: '14.5px' }}><img src="/assets/riha-icon-dark.png" alt="" style={{ width: '18px', height: '18px', objectFit: 'contain' }} />Pagar com RIHA</span><span style={{ display: 'block', fontSize: '12.5px', color: '#6E5B4C', marginTop: '2px' }}>M-Pesa, e-Mola ou Bim IZI</span></span>
                  <span style={{ display: 'flex', gap: '4px' }}><img src="/assets/methods/mpesa.png" alt="" style={{ width: '24px', height: '24px', borderRadius: '7px', background: '#FDE7E6', color: '#C8102E', display: 'grid', placeItems: 'center', fontWeight: '800', fontSize: '11px', objectFit: 'cover' }} /><img src="/assets/methods/emola.png" alt="" style={{ width: '24px', height: '24px', borderRadius: '7px', background: '#FFF0D6', color: '#B45F00', display: 'grid', placeItems: 'center', fontWeight: '800', fontSize: '12px', objectFit: 'cover' }} /><img src="/assets/methods/bim.png" alt="" style={{ width: '24px', height: '24px', borderRadius: '7px', background: '#0B120E', color: '#fff', display: 'grid', placeItems: 'center', fontWeight: '700', fontSize: '7px', objectFit: 'cover' }} /></span>
                </div>
                <div style={{ position: 'absolute', left: '0', top: '160px', width: '480px', height: '60px', boxSizing: 'border-box', display: 'flex', alignItems: 'center', gap: '14px', padding: '0 16px', borderRadius: '14px', background: '#fff', border: '1px solid #EFE6D8', fontSize: '14px', color: '#6E5B4C' }}><span style={{ width: '20px', height: '20px', borderRadius: '50%', border: '2px solid #D8CBB8', boxSizing: 'border-box' }}></span>Pagamento na entrega</div>
              </>)}
            </div>
            <div style={{ position: 'absolute', left: '560px', top: '84px', width: '280px', height: '340px', boxSizing: 'border-box', padding: '20px', borderRadius: '18px', background: '#fff', border: '1px solid #EFE6D8', fontSize: '13.5px' }}>
              <div style={{ fontWeight: '600', fontSize: '15px' }}>Resumo</div>
              <div style={{ display: 'flex', justifyContent: 'space-between', marginTop: '16px', color: '#6E5B4C' }}><span>Subtotal</span><span>3 450 MT</span></div>
              <div style={{ display: 'flex', justifyContent: 'space-between', marginTop: '8px', color: '#6E5B4C' }}><span>Entrega · Maputo</span><span>Grátis</span></div>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', marginTop: '16px', paddingTop: '14px', borderTop: '1px solid #EFE6D8' }}><span style={{ fontWeight: '600' }}>Total</span><span style={{ fontFamily: 'var(--display)', fontWeight: '700', fontSize: '24px', letterSpacing: '-0.02em' }}>3 450 MT</span></div>
            </div>
            <div style={{ position: 'absolute', left: '580px', top: '356px', width: '240px', height: '52px', borderRadius: '14px', background: '#7A2E12', color: '#fff', display: 'grid', placeItems: 'center', fontWeight: '600', fontSize: '15px', transform: v.storeBtnT, transition: 'transform .15s' }}>{v.storeBtn}</div>
          </div>
        </>)}

        {v.s.redirect && (<>
          <div style={{ position: 'absolute', inset: '0', background: '#F4F3EE', display: 'grid', placeItems: 'center' }}>
            <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '14px' }}><img src="/assets/riha-icon-dark.png" alt="" style={{ width: '44px', height: '44px', objectFit: 'contain' }} /><div style={{ fontSize: '14px', color: '#3E4742' }}>A abrir pagamento seguro…</div></div>
          </div>
        </>)}

        {v.s.riha && (<>
          <div style={{ position: 'absolute', inset: '0', background: '#F4F3EE' }}>
            <div style={{ position: 'absolute', left: '250px', top: '8px', width: '380px', height: '480px', boxSizing: 'border-box', padding: '20px', borderRadius: '22px', background: '#fff', border: '1px solid #E8E6DF' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}><span style={{ width: '34px', height: '34px', borderRadius: '10px', background: '#7A2E12', color: '#fff', display: 'grid', placeItems: 'center', fontFamily: 'var(--display)', fontWeight: '800', fontSize: '15px' }}>C</span><span style={{ flex: '1' }}><span style={{ display: 'block', fontWeight: '600', fontSize: '14px' }}>Capulana Store</span><span style={{ display: 'block', fontSize: '11.5px', color: '#5E6862' }}>Encomenda #4471</span></span><span style={{ display: 'flex', alignItems: 'center', gap: '5px', fontSize: '11px', fontWeight: '600', color: '#5E6862' }}><img src="/assets/riha-icon-dark.png" alt="" style={{ width: '14px', height: '14px', objectFit: 'contain' }} />RIHA</span></div>
              <div style={{ fontFamily: 'var(--display)', fontWeight: '700', fontSize: '36px', letterSpacing: '-0.04em', marginTop: '18px' }}>3 450,00 <span style={{ fontSize: '15px', color: '#5E6862' }}>MT</span></div>
              <div style={{ position: 'absolute', left: '20px', right: '20px', top: '162px', height: '52px', boxSizing: 'border-box', display: 'flex', alignItems: 'center', gap: '10px', padding: '0 14px', borderRadius: '14px', border: v.mpBd, background: v.mpBg, fontSize: '14px', fontWeight: '600' }}><img src="/assets/methods/mpesa.png" alt="" style={{ width: '28px', height: '28px', borderRadius: '9px', background: '#FDE7E6', color: '#C8102E', display: 'grid', placeItems: 'center', fontWeight: '800', fontSize: '13px', objectFit: 'cover' }} />M-Pesa<span style={{ marginLeft: 'auto', width: '18px', height: '18px', borderRadius: '50%', border: `2px solid ${v.mpRc}`, display: 'grid', placeItems: 'center', boxSizing: 'border-box' }}><span style={{ width: '9px', height: '9px', borderRadius: '50%', background: v.mpDot }}></span></span></div>
              <div style={{ position: 'absolute', left: '20px', right: '20px', top: '222px', height: '52px', boxSizing: 'border-box', display: 'flex', alignItems: 'center', gap: '10px', padding: '0 14px', borderRadius: '14px', border: '1px solid #E4E2DA', fontSize: '14px', fontWeight: '500' }}><img src="/assets/methods/emola.png" alt="" style={{ width: '28px', height: '28px', borderRadius: '9px', background: '#FFF0D6', color: '#B45F00', display: 'grid', placeItems: 'center', fontWeight: '800', fontSize: '14px', objectFit: 'cover' }} />e-Mola</div>
              <div style={{ position: 'absolute', left: '20px', right: '20px', top: '282px', height: '52px', boxSizing: 'border-box', display: 'flex', alignItems: 'center', gap: '10px', padding: '0 14px', borderRadius: '14px', border: '1px solid #E4E2DA', fontSize: '14px', fontWeight: '500' }}><img src="/assets/methods/bim.png" alt="" style={{ width: '28px', height: '28px', borderRadius: '9px', background: '#0B120E', color: '#fff', display: 'grid', placeItems: 'center', fontWeight: '700', fontSize: '8px', objectFit: 'cover' }} />Bim IZI</div>
              <div style={{ position: 'absolute', left: '20px', right: '20px', top: '344px', height: '44px', boxSizing: 'border-box', display: 'flex', alignItems: 'center', padding: '0 14px', borderRadius: '12px', border: '1px solid #E2E0D8', fontSize: '14px', color: v.numC }}>{v.numTxt}</div>
              <div style={{ position: 'absolute', left: '20px', right: '20px', top: '404px', height: '52px', borderRadius: '14px', background: v.payBg, color: '#fff', display: 'grid', placeItems: 'center', fontWeight: '600', fontSize: '15px', transform: v.payBtnT, transition: 'transform .15s,background .3s' }}>Pagar 3 450,00 MT</div>
            </div>
          </div>
        </>)}

        {v.s.pin && (<>
          <div style={{ position: 'absolute', inset: '0', background: '#F4F3EE', display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '56px' }}>
            <div style={{ width: '320px' }}>
              <div style={{ width: '52px', height: '52px', borderRadius: '16px', background: '#FFF1C2', display: 'grid', placeItems: 'center' }}><svg width="24" height="24" viewBox="0 0 24 24" style={{ fill: 'none', stroke: '#6B5200', strokeWidth: '2', strokeLinecap: 'round', strokeLinejoin: 'round' }}><rect x="7" y="2" width="10" height="20" rx="2.5"></rect><path d="M11 18h2"></path></svg></div>
              <div style={{ fontFamily: 'var(--display)', fontWeight: '700', fontSize: '28px', letterSpacing: '-0.03em', marginTop: '16px', lineHeight: '1.1' }}>Confirme no telemóvel</div>
              <div style={{ fontSize: '14px', lineHeight: '1.55', color: '#3E4742', marginTop: '8px' }}>Enviámos um pedido M-Pesa para 84 123 4567. Insira o PIN para concluir.</div>
              <div style={{ height: '6px', borderRadius: '3px', background: '#E4E2DA', marginTop: '20px', overflow: 'hidden' }}><div style={{ height: '100%', background: '#0A8205', width: v.pinBar, transition: 'width .4s' }}></div></div>
            </div>
            <div style={{ width: '210px', height: '400px', borderRadius: '32px', background: '#0B120E', padding: '10px', boxSizing: 'border-box', boxShadow: '0 30px 60px -20px rgba(0,0,0,.5)' }}>
              <div style={{ width: '100%', height: '100%', borderRadius: '24px', background: '#1C2621', display: 'flex', alignItems: 'center', justifyContent: 'center', padding: '14px', boxSizing: 'border-box' }}>
                <div style={{ width: '100%', background: '#F4F3EE', borderRadius: '14px', padding: '14px', boxSizing: 'border-box', fontSize: '12px', lineHeight: '1.45' }}>
                  <div style={{ fontWeight: '700', color: '#C8102E' }}>M-Pesa</div>
                  <div style={{ marginTop: '6px' }}>Confirme o pagamento de 3 450,00 MT. Insira o PIN:</div>
                  <div style={{ display: 'flex', gap: '8px', justifyContent: 'center', marginTop: '12px' }}>{v.pinDots.map((d, i) => (<Fragment key={i}><span style={{ width: '12px', height: '12px', borderRadius: '50%', border: '2px solid #0B120E', boxSizing: 'border-box', background: d.bg }}></span></Fragment>))}</div>
                  <div style={{ marginTop: '12px', height: '30px', borderRadius: '8px', background: v.okBg, color: '#fff', display: 'grid', placeItems: 'center', fontWeight: '600', transition: 'background .2s' }}>OK</div>
                </div>
              </div>
            </div>
          </div>
        </>)}

        {v.s.rihaOk && (<>
          <div style={{ position: 'absolute', inset: '0', background: '#F4F3EE', display: 'grid', placeItems: 'center' }}>
            <div style={{ width: '380px', textAlign: 'center', display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
              <div style={{ width: '64px', height: '64px', borderRadius: '22px', background: '#0A8205', display: 'grid', placeItems: 'center' }}><svg width="30" height="30" viewBox="0 0 24 24" style={{ fill: 'none', stroke: '#fff', strokeWidth: '2.8', strokeLinecap: 'round', strokeLinejoin: 'round' }}><path d="M20 6 9 17l-5-5"></path></svg></div>
              <div style={{ fontFamily: 'var(--display)', fontWeight: '700', fontSize: '30px', letterSpacing: '-0.03em', marginTop: '16px' }}>Pagamento confirmado</div>
              <div style={{ fontSize: '14px', color: '#3E4742', marginTop: '6px' }}>3 450,00 MT · M-Pesa</div>
              <div style={{ fontSize: '13px', color: '#5E6862', marginTop: '22px' }}>A voltar para Capulana Store…</div>
              <div style={{ width: '220px', height: '5px', borderRadius: '3px', background: '#E4E2DA', marginTop: '10px', overflow: 'hidden' }}><div style={{ height: '100%', background: '#0B120E', width: v.backBar, transition: 'width 1.3s linear' }}></div></div>
            </div>
          </div>
        </>)}

        {v.s.done && (<>
          <div style={{ position: 'absolute', inset: '0', background: '#FBF7F1' }}>
            <div style={{ height: '56px', display: 'flex', alignItems: 'center', gap: '28px', padding: '0 40px', borderBottom: '1px solid #EFE6D8' }}><span style={{ fontFamily: 'var(--display)', fontWeight: '800', fontSize: '19px', letterSpacing: '-0.02em', color: '#7A2E12' }}>Capulana<span style={{ color: '#D9822B' }}>.</span></span><span style={{ fontSize: '13px', color: '#6E5B4C' }}>Tecidos</span><span style={{ fontSize: '13px', color: '#6E5B4C' }}>Bolsas</span><span style={{ fontSize: '13px', color: '#6E5B4C' }}>Novidades</span></div>
            <div style={{ maxWidth: '520px', margin: '56px auto 0', textAlign: 'center', display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
              <div style={{ width: '60px', height: '60px', borderRadius: '50%', background: '#2F6B4F', display: 'grid', placeItems: 'center' }}><svg width="28" height="28" viewBox="0 0 24 24" style={{ fill: 'none', stroke: '#fff', strokeWidth: '2.8', strokeLinecap: 'round', strokeLinejoin: 'round' }}><path d="M20 6 9 17l-5-5"></path></svg></div>
              <div style={{ fontFamily: 'var(--display)', fontWeight: '700', fontSize: '32px', letterSpacing: '-0.03em', marginTop: '16px', color: '#2A1A10' }}>Obrigado, Carlos!</div>
              <div style={{ fontSize: '15px', color: '#6E5B4C', marginTop: '6px' }}>A encomenda #4471 está paga. Entregamos amanhã em Maputo.</div>
              <div style={{ width: '100%', marginTop: '24px', padding: '16px 18px', borderRadius: '16px', background: '#fff', border: '1px solid #EFE6D8', fontSize: '13.5px', textAlign: 'left', display: 'flex', flexDirection: 'column', gap: '8px' }}>
                <div style={{ display: 'flex', justifyContent: 'space-between' }}><span style={{ color: '#6E5B4C' }}>Total pago</span><span style={{ fontWeight: '600' }}>3 450 MT</span></div>
                <div style={{ display: 'flex', justifyContent: 'space-between' }}><span style={{ color: '#6E5B4C' }}>Método</span><span style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>M-Pesa via <img src="/assets/riha-icon-dark.png" alt="" style={{ width: '14px', height: '14px', objectFit: 'contain' }} /><b style={{ fontWeight: '600' }}>RIHA</b></span></div>
                <div style={{ display: 'flex', justifyContent: 'space-between' }}><span style={{ color: '#6E5B4C' }}>Referência</span><span style={{ fontFamily: 'var(--mono)', fontSize: '12.5px' }}>pay_9Xk2Lm4Q</span></div>
              </div>
            </div>
          </div>
        </>)}

        <div style={{ position: 'absolute', left: '0', top: '0', width: '22px', height: '22px', pointerEvents: 'none', transform: v.curT, transition: v.curTr, opacity: v.curO }}>
          <span style={{ position: 'absolute', left: '-14px', top: '-14px', width: '28px', height: '28px', borderRadius: '50%', background: 'rgba(10,130,5,.25)', transform: `scale(${v.ringS})`, transition: 'transform .25s' }}></span>
          <svg width="22" height="22" viewBox="0 0 24 24" style={{ position: 'absolute', left: '-3px', top: '-2px', filter: 'drop-shadow(0 2px 3px rgba(0,0,0,.35))' }}><path d="M4 2l15 9.5-6.6 1.3 3.9 7.4-2.8 1.5-3.9-7.5L4 19z" style={{ fill: '#0B120E', stroke: '#fff', strokeWidth: '1.4', strokeLinejoin: 'round' }}></path></svg>
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
