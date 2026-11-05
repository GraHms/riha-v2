import { Fragment, useEffect, useState, type ChangeEvent } from 'react'
import { Link } from 'react-router-dom'
import { Icon, QR, stripeBlur } from '../../components/ui'
import { useGateway } from '../../core/gateway'
import { bpsLabel } from '../../lib/format'
import { SUPPORT_EMAIL, SUPPORT_MAILTO } from '../../lib/support'
import CheckoutFlow from './landing/CheckoutFlow'
import GymFlow from './landing/GymFlow'

const css = `
.lp a { text-decoration: none; }
.lp a:hover { color: inherit; }
@media (max-width: 720px) {
  .lp-nav, .lp-foot-gap { display: none !important; }
  .lp-hero { height: 640px !important; }
  .lp-hero-dash { display: none !important; }
  .lp-hero-phone { left: 50% !important; right: auto !important; top: 26px !important; transform: translateX(-50%); }
  .lp-hero-toast { left: 50% !important; bottom: 16px !important; transform: translateX(-50%); white-space: nowrap; }
  .lp-share { min-height: 550px !important; }
  .lp-share-qr { top: 240px !important; }
}
`

const ORIGIN: Record<string, [string, string]> = {
  Assinatura: ['#E2F3DE', '#0A6B05'], API: ['#0B120E', '#fff'], Link: ['#EEEDE8', '#3E4742'], QR: ['#FFF1C2', '#6B5200'],
}
const STATE: Record<string, string> = { Paga: '#0A6B05', Pendente: '#8A6A00', Falhada: '#C2080D' }
const TXS = [
  ['Tânia Mabunda', 'M-Pesa', 'Assinatura', '2 500,00', 'Paga'],
  ['Pedido #4471 · loja.co.mz', 'M-Pesa', 'API', '8 900,00', 'Paga'],
  ['Carlos Sitoe', 'e-Mola', 'Link', '1 250,00', 'Paga'],
  ['Balcão Sommerschield', 'M-Pesa', 'QR', '350,00', 'Paga'],
  ['Sara Nhantumbo', 'Bim IZI', 'Assinatura', '3 000,00', 'Pendente'],
  ['Pedido #4470 · loja.co.mz', 'e-Mola', 'API', '4 200,00', 'Falhada'],
  ['Fátima Mondlane', 'e-Mola', 'Link', '780,00', 'Paga'],
].map(([who, m, o, v, s]) => ({ who, m, o, v, s, obg: ORIGIN[o][0], ofg: ORIGIN[o][1], sc: STATE[s] }))
const SECTORS = [
  ['Escolas', 'Assinatura', 'Propinas mensais com meses sem cobrança e catálogo de planos.', 'Propina mensal', '2 500 MT'],
  ['Ginásios', 'Assinatura', 'Mensalidade no dia de inscrição. Quem não paga perde esse mês, ou fica em dívida.', 'Livre-trânsito', '1 800 MT'],
  ['Lojas online', 'API', 'Checkout na sua loja, com referência de encomenda e eventos para o seu servidor.', 'Encomenda', '8 900 MT'],
  ['Balcão e mercados', 'QR', 'Um QR impresso. O cliente paga com o telemóvel e o valor aparece no painel.', 'Venda ao balcão', '350 MT'],
  ['Turismo e atividades', 'Link', 'Envie o link da reserva por WhatsApp. O cliente paga e o valor aparece no painel.', 'Jet ski · 2 pessoas', '7 000 MT'],
  ['Terapeutas e explicadores', 'Assinatura', 'Sessões semanais ou mensais, com pausas e saltos para cada cliente.', 'Sessão semanal', '1 200 MT'],
].map(([t, o, d, ex, v]) => ({ t, o, d, ex, v, obg: ORIGIN[o][0], ofg: ORIGIN[o][1] }))
const METHODS: { name: string; method?: string; sub: string; logo?: string }[] = [
  { name: 'M-Pesa', method: 'mpesa', sub: 'Vodacom · confirma com PIN', logo: '/assets/methods/mpesa.png' },
  { name: 'e-Mola', method: 'emola', sub: 'Movitel · confirma com PIN', logo: '/assets/methods/emola.png' },
  { name: 'Bim IZI', method: 'bim', sub: 'Millennium bim · confirma no *181#', logo: '/assets/methods/bim.png' },
  { name: 'Visa e Mastercard', sub: 'Em breve' },
]
const fmt = (n: number) => Math.round(n).toString().replace(/\B(?=(\d{3})+(?!\d))/g, ' ')

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
  return { bps, soon }
}

export default function Landing() {
  const { bps, soon } = usePricing()
  const [vol, setVol] = useState(250000)
  const v = {
    txs: TXS, sectorCards: SECTORS, vol, fee: bpsLabel(bps),
    volLabel: fmt(vol), feeLabel: fmt(vol * bps / 10000), netLabel: fmt(vol * (1 - bps / 10000)),
    setVol: (e: ChangeEvent<HTMLInputElement>) => setVol(+e.target.value),
  }
  return (
    <div className="lp">
      <style>{css}</style>
<div style={{ containerType: 'inline-size', background: '#F4F3EE', color: '#0B120E', fontFamily: 'var(--sans)', overflow: 'hidden' }}>

  <div style={{ maxWidth: '1280px', margin: '0 auto', padding: '0 clamp(20px,4cqi,48px)', display: 'flex', alignItems: 'center', gap: '28px', height: '76px' }}>
    <div style={{ display: 'flex', alignItems: 'center', gap: '10px', flex: 'none' }}><img src="/assets/riha-icon-dark.png" alt="RIHA" style={{ width: '30px', height: '30px', objectFit: 'contain' }} /><span style={{ fontFamily: 'var(--display)', fontWeight: '800', fontSize: '17px', letterSpacing: '.2em' }}>RIHA</span></div>
    <div className="lp-nav" style={{ flex: '1', minWidth: '0', height: '22px', overflow: 'hidden', display: 'flex', flexWrap: 'wrap', gap: '0 26px', fontSize: '14.5px', fontWeight: '500', color: '#3E4742' }}><a href="#pagamentos" style={{ color: 'inherit', textDecoration: 'none' }}>Pagamentos</a><a href="#assinaturas" style={{ color: 'inherit', textDecoration: 'none' }}>Assinaturas</a><a href="#programadores" style={{ color: 'inherit', textDecoration: 'none' }}>Programadores</a><a href="#preco" style={{ color: 'inherit', textDecoration: 'none' }}>Preço</a><a href="#seguranca" style={{ color: 'inherit', textDecoration: 'none' }}>Segurança</a></div>
    <Link to="/entrar" style={{ fontSize: '14.5px', fontWeight: '600', flex: 'none', color: '#0B120E', textDecoration: 'none', whiteSpace: 'nowrap' }}>Entrar</Link>
    <Link to="/registo" style={{ height: '40px', padding: '0 16px', borderRadius: '12px', background: '#0B120E', color: '#fff', fontWeight: '600', fontSize: '14px', display: 'flex', alignItems: 'center', flex: 'none', textDecoration: 'none', whiteSpace: 'nowrap' }}>Criar conta</Link>
  </div>

  <div style={{ maxWidth: '1280px', margin: '0 auto', padding: 'clamp(40px,6cqi,80px) clamp(20px,4cqi,48px) 0', textAlign: 'center', display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
    <div style={{ display: 'inline-flex', alignItems: 'center', gap: '8px', height: '32px', padding: '0 12px 0 6px', borderRadius: '999px', background: '#fff', border: '1px solid #E4E2DA', fontSize: '13px', fontWeight: '600' }}><span style={{ fontSize: '11px', padding: '3px 8px', borderRadius: '999px', background: '#0A8205', color: '#fff' }}>Novo</span>Assinaturas</div>
    <h1 style={{ margin: '22px 0 0', fontFamily: 'var(--display)', fontWeight: '700', fontSize: 'clamp(44px,7.4cqi,100px)', lineHeight: '.94', letterSpacing: '-0.045em', maxWidth: '1000px', textWrap: 'balance' }}>Todos os pagamentos do seu negócio. Num só painel.</h1>
    <p style={{ margin: '24px 0 0', fontSize: 'clamp(16px,1.5cqi,19px)', lineHeight: '1.55', color: '#3E4742', maxWidth: '620px', textWrap: 'pretty' }}>M-Pesa, e-Mola e Bim IZI por link, QR ou API. Assinaturas para quem cobra todos os meses. Cada pagamento mostra de onde veio.</p>
    <div style={{ display: 'flex', flexWrap: 'wrap', justifyContent: 'center', gap: '12px', marginTop: '30px', width: '100%' }}>
      <Link to="/registo" style={{ height: '56px', padding: '0 26px', borderRadius: '16px', background: '#0A8205', color: '#fff', fontWeight: '600', fontSize: '16.5px', display: 'flex', alignItems: 'center', boxShadow: '0 14px 30px -12px rgba(10,130,5,.7)', textDecoration: 'none', whiteSpace: 'nowrap' }}>Criar conta grátis</Link>
      <Link to="/docs" style={{ height: '56px', padding: '0 24px', borderRadius: '16px', background: '#fff', border: '1px solid #E4E2DA', fontWeight: '600', fontSize: '16px', display: 'flex', alignItems: 'center', color: '#0B120E', textDecoration: 'none', whiteSpace: 'nowrap' }}>Ver a API</Link>
    </div>
  </div>

  <div style={{ maxWidth: '1280px', margin: '0 auto', padding: 'clamp(40px,5cqi,64px) clamp(12px,3cqi,48px) 0' }}>
    <div className="lp-hero" style={{ position: 'relative', height: 'clamp(500px,52cqi,690px)', borderRadius: 'clamp(22px,3cqi,36px)', background: '#0B120E', overflow: 'hidden' }}>
      <div style={{ position: 'absolute', top: '-100px', right: '22%', width: 'clamp(60px,9cqi,130px)', height: '1200px', background: '#0A8205', transform: 'skewX(22deg)', opacity: '.22', filter: stripeBlur(100) }}></div>
      <div style={{ position: 'absolute', top: '-100px', right: '8%', width: 'clamp(60px,9cqi,130px)', height: '1200px', background: '#FF0F15', transform: 'skewX(22deg)', opacity: '.16', filter: stripeBlur(100) }}></div>
      <div className="lp-hero-dash" style={{ position: 'absolute', left: 'clamp(16px,5cqi,72px)', top: 'clamp(28px,4cqi,56px)', width: '1000px', height: '600px', borderRadius: '18px', background: '#F4F3EE', boxShadow: '0 50px 100px -30px rgba(0,0,0,.8)', display: 'flex', overflow: 'hidden', fontSize: '12.5px' }}>
        <div style={{ width: '190px', flex: 'none', background: '#fff', borderRight: '1px solid #E8E6DF', padding: '16px 12px', boxSizing: 'border-box', display: 'flex', flexDirection: 'column', gap: '2px', fontWeight: '600', color: '#3E4742' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '0 6px 14px' }}><img src="/assets/riha-icon-dark.png" alt="" style={{ width: '22px', height: '22px', objectFit: 'contain' }} /><span style={{ fontFamily: 'var(--display)', fontWeight: '800', fontSize: '13px', letterSpacing: '.2em', color: '#0B120E' }}>RIHA</span></div>
          <span style={{ height: '34px', padding: '0 10px', borderRadius: '10px', background: '#F4F3EE', color: '#0B120E', display: 'flex', alignItems: 'center' }}>Visão geral</span>
          <span style={{ height: '34px', padding: '0 10px', display: 'flex', alignItems: 'center' }}>Transações</span>
          <span style={{ height: '34px', padding: '0 10px', display: 'flex', alignItems: 'center' }}>Links de pagamento</span>
          <span style={{ height: '34px', padding: '0 10px', display: 'flex', alignItems: 'center' }}>Saldo</span>
          <span style={{ fontSize: '10px', color: '#8A938E', textTransform: 'uppercase', letterSpacing: '.08em', padding: '14px 10px 6px' }}>Serviços</span>
          <span style={{ height: '34px', padding: '0 10px', display: 'flex', alignItems: 'center' }}>Assinaturas</span>
        </div>
        <div style={{ flex: '1', padding: '18px 22px', display: 'flex', flexDirection: 'column', gap: '12px', minWidth: '0' }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}><span style={{ fontFamily: 'var(--display)', fontWeight: '700', fontSize: '20px', letterSpacing: '-0.02em' }}>Visão geral</span><span style={{ display: 'flex', gap: '2px', padding: '2px', borderRadius: '9px', background: '#EAE8E1', fontSize: '11px', fontWeight: '600' }}><span style={{ padding: '4px 9px', color: '#5E6862' }}>Teste</span><span style={{ padding: '4px 9px', borderRadius: '7px', background: '#fff' }}>Produção</span></span></div>
          <div style={{ display: 'grid', gridTemplateColumns: '1.3fr 1fr 1fr', gap: '10px' }}>
            <div style={{ background: '#0B120E', color: '#fff', borderRadius: '14px', padding: '14px', position: 'relative', overflow: 'hidden' }}><div style={{ position: 'absolute', top: '-20px', right: '36px', width: '22px', height: '140px', background: '#0A8205', transform: 'skewX(22deg)', opacity: '.22', filter: stripeBlur(100) }}></div><div style={{ position: 'absolute', top: '-20px', right: '6px', width: '22px', height: '140px', background: '#FF0F15', transform: 'skewX(22deg)', opacity: '.16', filter: stripeBlur(100) }}></div><div style={{ position: 'relative', color: '#A9B5AE', fontSize: '11.5px' }}>Saldo disponível</div><div style={{ position: 'relative', fontFamily: 'var(--display)', fontWeight: '700', fontSize: '26px', letterSpacing: '-0.035em', marginTop: '4px' }}>48 320,50 <span style={{ fontSize: '12px', color: '#A9B5AE' }}>MT</span></div></div>
            <div style={{ background: '#fff', border: '1px solid #E8E6DF', borderRadius: '14px', padding: '14px' }}><div style={{ color: '#5E6862', fontSize: '11.5px' }}>Recebido hoje</div><div style={{ fontFamily: 'var(--display)', fontWeight: '700', fontSize: '22px', letterSpacing: '-0.03em', marginTop: '4px' }}>16 200 MT</div><div style={{ color: '#0A6B05', fontWeight: '600', fontSize: '11px', marginTop: '2px' }}>23 pagamentos</div></div>
            <div style={{ background: '#fff', border: '1px solid #E8E6DF', borderRadius: '14px', padding: '14px' }}><div style={{ color: '#5E6862', fontSize: '11.5px' }}>Assinaturas ativas</div><div style={{ fontFamily: 'var(--display)', fontWeight: '700', fontSize: '22px', letterSpacing: '-0.03em', marginTop: '4px' }}>165</div><div style={{ color: '#6B5200', fontWeight: '600', fontSize: '11px', marginTop: '2px' }}>3 em dívida</div></div>
          </div>
          <div style={{ background: '#fff', border: '1px solid #E8E6DF', borderRadius: '14px', overflow: 'hidden', flex: '1' }}>
            <div style={{ display: 'grid', gridTemplateColumns: 'minmax(0,1.6fr) 90px 100px 100px 80px', gap: '12px', padding: '9px 14px', borderBottom: '1px solid #EEECE5', fontSize: '10.5px', fontWeight: '600', color: '#5E6862', textTransform: 'uppercase', letterSpacing: '.05em' }}><span>Cliente</span><span>Método</span><span>Origem</span><span style={{ textAlign: 'right' }}>Valor</span><span>Estado</span></div>
            {v.txs.map((t, i) => (<Fragment key={i}>
              <div style={{ display: 'grid', gridTemplateColumns: 'minmax(0,1.6fr) 90px 100px 100px 80px', gap: '12px', padding: '9px 14px', borderBottom: '1px solid #F1EFE9', alignItems: 'center' }}><span style={{ minWidth: '0' }}><span style={{ display: 'block', fontWeight: '600', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{t.who}</span></span><span style={{ color: '#3E4742' }}>{t.m}</span><span><span style={{ fontSize: '10.5px', fontWeight: '600', padding: '2px 7px', borderRadius: '6px', background: t.obg, color: t.ofg }}>{t.o}</span></span><span style={{ textAlign: 'right', fontVariantNumeric: 'tabular-nums', fontWeight: '600' }}>{t.v}</span><span style={{ fontSize: '11px', fontWeight: '600', color: t.sc }}>{t.s}</span></div>
            </Fragment>))}
          </div>
        </div>
      </div>
      <div className="lp-hero-phone" style={{ position: 'absolute', right: 'clamp(10px,5cqi,72px)', top: 'clamp(150px,9cqi,120px)', width: '260px', height: '520px', borderRadius: '38px', background: '#F4F3EE', boxShadow: '0 0 0 8px #1C2621,0 50px 100px -20px rgba(0,0,0,.9)', overflow: 'hidden', display: 'flex', flexDirection: 'column' }}>
        <div style={{ height: '38px', display: 'flex', alignItems: 'flex-end', justifyContent: 'center', paddingBottom: '6px' }}><span style={{ fontSize: '11px', color: '#3E4742', padding: '4px 10px', borderRadius: '8px', background: '#EAE8E1' }}>pay.rihagateway.com</span></div>
        <div style={{ padding: '12px 16px', display: 'flex', flexDirection: 'column', gap: '10px', flex: '1' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '9px' }}><div style={{ width: '32px', height: '32px', borderRadius: '10px', background: '#FFD21F', display: 'grid', placeItems: 'center', fontWeight: '700', fontSize: '12px' }}>PX</div><div><div style={{ fontWeight: '600', fontSize: '13px' }}>Pastelaria Xiluva</div><div style={{ fontSize: '11px', color: '#5E6862' }}>Bolo de aniversário</div></div></div>
          <div style={{ fontFamily: 'var(--display)', fontWeight: '700', fontSize: '32px', letterSpacing: '-0.035em', marginTop: '6px' }}>1 250,00 <span style={{ fontSize: '14px', color: '#5E6862' }}>MT</span></div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '9px', height: '46px', padding: '0 12px', borderRadius: '13px', border: '2px solid #0B120E', background: '#fff', fontSize: '13px', fontWeight: '600' }}><img src="/assets/methods/mpesa.png" alt="" style={{ width: '26px', height: '26px', borderRadius: '8px', background: '#FDE7E6', color: '#C8102E', display: 'grid', placeItems: 'center', fontSize: '12px', fontWeight: '800', objectFit: 'cover' }} />M-Pesa<span style={{ marginLeft: 'auto', width: '16px', height: '16px', borderRadius: '50%', border: '2px solid #0B120E', display: 'grid', placeItems: 'center', boxSizing: 'border-box' }}><span style={{ width: '8px', height: '8px', borderRadius: '50%', background: '#0B120E' }}></span></span></div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '9px', height: '46px', padding: '0 12px', borderRadius: '13px', border: '1px solid #E4E2DA', background: '#fff', fontSize: '13px', fontWeight: '500' }}><img src="/assets/methods/emola.png" alt="" style={{ width: '26px', height: '26px', borderRadius: '8px', background: '#FFF0D6', color: '#B45F00', display: 'grid', placeItems: 'center', fontSize: '13px', fontWeight: '800', objectFit: 'cover' }} />e-Mola</div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '9px', height: '46px', padding: '0 12px', borderRadius: '13px', border: '1px solid #E4E2DA', background: '#fff', fontSize: '13px', fontWeight: '500' }}><img src="/assets/methods/bim.png" alt="" style={{ width: '26px', height: '26px', borderRadius: '8px', background: '#0B120E', color: '#fff', display: 'grid', placeItems: 'center', fontSize: '10px', fontWeight: '700', objectFit: 'cover' }} />Bim IZI</div>
          <div style={{ height: '46px', borderRadius: '13px', border: '1px solid #E2E0D8', background: '#fff', display: 'flex', alignItems: 'center', padding: '0 12px', fontSize: '13px' }}>+258 84 123 4567</div>
          <div style={{ flex: '1' }}></div>
          <div style={{ height: '48px', borderRadius: '14px', background: '#0A8205', color: '#fff', display: 'grid', placeItems: 'center', fontWeight: '600', fontSize: '14px', marginBottom: '10px' }}>Pagar 1 250,00 MT</div>
        </div>
      </div>
      <div className="lp-hero-toast" style={{ position: 'absolute', left: 'clamp(16px,30cqi,420px)', bottom: 'clamp(20px,3cqi,40px)', display: 'flex', alignItems: 'center', gap: '10px', padding: '12px 16px', borderRadius: '18px', background: 'rgba(28,40,33,.94)', color: '#fff', border: '1px solid rgba(255,255,255,.14)', boxShadow: '0 24px 50px -20px rgba(0,0,0,.9)' }}>
        <div style={{ width: '32px', height: '32px', flex: 'none', borderRadius: '50%', background: '#0A8205', display: 'grid', placeItems: 'center' }}><svg width="16" height="16" viewBox="0 0 24 24" style={{ fill: 'none', stroke: '#fff', strokeWidth: '2.8', strokeLinecap: 'round', strokeLinejoin: 'round' }}><path d="M20 6 9 17l-5-5"></path></svg></div>
        <div><div style={{ fontSize: '12px', color: '#AFC0B6' }}>Pagamento recebido · Link</div><div style={{ fontWeight: '600', fontSize: '14px' }}>+1 250,00 MT · M-Pesa</div></div>
      </div>
    </div>
    <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit,minmax(min(100%,220px),1fr))', gap: '10px', marginTop: '12px' }}>
      {METHODS.map(m => (
        <div key={m.name} style={{ display: 'flex', alignItems: 'center', gap: '12px', padding: '16px', borderRadius: '18px', background: '#fff', border: '1px solid #E8E6DF' }}>
          {m.logo
            ? <img src={m.logo} alt="" style={{ width: '40px', height: '40px', flex: 'none', borderRadius: '12px', objectFit: 'cover' }} />
            : <span style={{ width: '40px', height: '40px', flex: 'none', borderRadius: '12px', background: '#0B120E', color: '#fff', display: 'grid', placeItems: 'center' }}><Icon name="card" size={19} /></span>}
          <span><span style={{ display: 'block', fontWeight: '600', fontSize: '15px' }}>{m.name}</span><span style={{ display: 'block', fontSize: '12.5px', color: '#5E6862' }}>{!m.method || soon.includes(m.method) ? 'Em breve' : m.sub}</span></span>
        </div>
      ))}
    </div>
  </div>

  <div style={{ maxWidth: '1280px', margin: '0 auto', padding: 'clamp(72px,9cqi,128px) clamp(20px,4cqi,48px) 0', display: 'flex', flexWrap: 'wrap', gap: 'clamp(32px,5cqi,64px)', alignItems: 'center' }}>
    <div style={{ flex: '1 1 300px', minWidth: '0' }}>
      <div id="pagamentos" style={{ scrollMarginTop: '24px', fontSize: '14px', fontWeight: '600', color: '#0A7A05' }}>Na sua loja online</div>
      <h2 style={{ margin: '10px 0 0', fontFamily: 'var(--display)', fontWeight: '700', fontSize: 'clamp(32px,4cqi,52px)', lineHeight: '1', letterSpacing: '-0.035em', textWrap: 'balance' }}>O cliente paga na RIHA e volta à sua loja.</h2>
      <p style={{ margin: '18px 0 0', fontSize: '16.5px', lineHeight: '1.6', color: '#3E4742' }}>Adicione "Pagar com RIHA" ao seu checkout. O cliente escolhe M-Pesa, e-Mola ou Bim IZI numa página segura da RIHA, confirma, e volta à sua loja com a encomenda paga.</p>
      <div style={{ display: 'flex', flexDirection: 'column', gap: '10px', marginTop: '20px', fontSize: '14.5px', color: '#3E4742' }}>
        <div style={{ display: 'flex', gap: '10px' }}><span style={{ fontFamily: 'var(--mono)', fontSize: '12.5px', color: '#8A938E', width: '22px', flex: 'none', paddingTop: '2px' }}>01</span>A sua loja cria o checkout por API</div>
        <div style={{ display: 'flex', gap: '10px' }}><span style={{ fontFamily: 'var(--mono)', fontSize: '12.5px', color: '#8A938E', width: '22px', flex: 'none', paddingTop: '2px' }}>02</span>O cliente paga em pay.rihagateway.com</div>
        <div style={{ display: 'flex', gap: '10px' }}><span style={{ fontFamily: 'var(--mono)', fontSize: '12.5px', color: '#8A938E', width: '22px', flex: 'none', paddingTop: '2px' }}>03</span>Volta ao seu success_url, e o seu servidor recebe o evento</div>
      </div>
    </div>
    <div style={{ flex: '2.2 1 560px', minWidth: '0' }}><CheckoutFlow /></div>
  </div>

  <div style={{ maxWidth: '1280px', margin: '0 auto', padding: 'clamp(72px,9cqi,128px) clamp(20px,4cqi,48px) 0' }}>
    <div style={{ fontSize: '14px', fontWeight: '600', color: '#0A7A05' }}>Como funciona</div>
    <h2 style={{ margin: '10px 0 0', fontFamily: 'var(--display)', fontWeight: '700', fontSize: 'clamp(32px,4.4cqi,56px)', lineHeight: '1', letterSpacing: '-0.035em' }}>Conta hoje. Primeiro pagamento hoje.</h2>
    <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit,minmax(min(100%,280px),1fr))', gap: '12px', marginTop: '32px' }}>
      <div style={{ padding: '26px', borderRadius: '24px', background: '#fff', border: '1px solid #E8E6DF' }}><div style={{ fontFamily: 'var(--display)', fontWeight: '800', fontSize: '56px', lineHeight: '1', letterSpacing: '-0.04em', color: '#D5D3CB' }}>1</div><div style={{ fontWeight: '600', fontSize: '19px', marginTop: '18px' }}>Crie a conta</div><div style={{ fontSize: '15px', lineHeight: '1.55', color: '#5E6862', marginTop: '6px' }}>Registe o negócio com o NUIT e o BI. Verificamos e ativamos a conta.</div></div>
      <div style={{ padding: '26px', borderRadius: '24px', background: '#fff', border: '1px solid #E8E6DF' }}><div style={{ fontFamily: 'var(--display)', fontWeight: '800', fontSize: '56px', lineHeight: '1', letterSpacing: '-0.04em', color: '#D5D3CB' }}>2</div><div style={{ fontWeight: '600', fontSize: '19px', marginTop: '18px' }}>Escolha como cobra</div><div style={{ fontSize: '15px', lineHeight: '1.55', color: '#5E6862', marginTop: '6px' }}>Um link no WhatsApp, um QR no balcão, um plano mensal ou a API.</div></div>
      <div style={{ padding: '26px', borderRadius: '24px', background: '#0A8205', color: '#fff' }}><div style={{ fontFamily: 'var(--display)', fontWeight: '800', fontSize: '56px', lineHeight: '1', letterSpacing: '-0.04em', color: '#5FB858' }}>3</div><div style={{ fontWeight: '600', fontSize: '19px', marginTop: '18px' }}>Receba e levante</div><div style={{ fontSize: '15px', lineHeight: '1.55', color: '#E3F4DF', marginTop: '6px' }}>Cada pagamento entra no saldo em tempo real. Levante para o seu banco, em até 72 horas.</div></div>
    </div>
  </div>

  <div style={{ maxWidth: '1280px', margin: '0 auto', padding: 'clamp(72px,9cqi,128px) clamp(20px,4cqi,48px) 0', display: 'grid', gridTemplateColumns: 'repeat(auto-fit,minmax(min(100%,440px),1fr))', gap: 'clamp(32px,5cqi,64px)', alignItems: 'center' }}>
    <div>
      <div style={{ fontSize: '14px', fontWeight: '600', color: '#0A7A05' }}>Links e QR</div>
      <h2 style={{ margin: '10px 0 0', fontFamily: 'var(--display)', fontWeight: '700', fontSize: 'clamp(32px,4.4cqi,56px)', lineHeight: '1', letterSpacing: '-0.035em', textWrap: 'balance' }}>Sem site. Sem código. Só partilhar.</h2>
      <p style={{ margin: '18px 0 0', fontSize: '17px', lineHeight: '1.6', color: '#3E4742', maxWidth: '480px' }}>Crie um link no telemóvel em segundos e envie por WhatsApp ou SMS. No balcão, imprima o QR e o cliente paga com o próprio telemóvel.</p>
    </div>
    <div className="lp-share" style={{ position: 'relative', minHeight: '400px' }}>
      <div style={{ position: 'absolute', left: '0', top: '0', width: 'min(100%,340px)', background: '#fff', border: '1px solid #E8E6DF', borderRadius: '26px', padding: '20px', boxSizing: 'border-box', boxShadow: '0 30px 60px -40px rgba(0,0,0,.4)' }}>
        <div style={{ fontSize: '12.5px', color: '#5E6862' }}>Novo link</div>
        <div style={{ fontFamily: 'var(--display)', fontWeight: '700', fontSize: '38px', letterSpacing: '-0.04em', marginTop: '6px' }}>1 250,00 <span style={{ fontSize: '16px', color: '#5E6862' }}>MT</span></div>
        <div style={{ height: '46px', borderRadius: '13px', border: '1px solid #E2E0D8', display: 'flex', alignItems: 'center', padding: '0 14px', fontSize: '14.5px', marginTop: '12px' }}>Bolo de aniversário</div>
        <div style={{ display: 'flex', gap: '6px', marginTop: '12px' }}><span style={{ flex: '1', height: '42px', borderRadius: '12px', background: '#0B120E', color: '#fff', fontWeight: '600', fontSize: '13.5px', display: 'grid', placeItems: 'center' }}>WhatsApp</span><span style={{ flex: '1', height: '42px', borderRadius: '12px', border: '1px solid #E4E2DA', fontWeight: '600', fontSize: '13.5px', display: 'grid', placeItems: 'center' }}>SMS</span><span style={{ flex: '1', height: '42px', borderRadius: '12px', border: '1px solid #E4E2DA', fontWeight: '600', fontSize: '13.5px', display: 'grid', placeItems: 'center' }}>Copiar</span></div>
        <div style={{ fontFamily: 'var(--mono)', fontSize: '12px', color: '#3E4742', marginTop: '12px' }}>pay.rihagateway.com/p/cafe-maputo/bolo</div>
      </div>
      <div className="lp-share-qr" style={{ position: 'absolute', right: '0', top: '130px', width: '230px', background: '#FFD21F', borderRadius: '24px', padding: '18px', boxSizing: 'border-box', transform: 'rotate(3deg)', boxShadow: '0 30px 60px -30px rgba(0,0,0,.5)', textAlign: 'center' }}>
        <div style={{ fontFamily: 'var(--display)', fontWeight: '700', fontSize: '20px', letterSpacing: '-0.02em', lineHeight: '1.1' }}>Pague aqui</div>
        <div style={{ fontSize: '12px', color: '#3B3410', marginTop: '3px' }}>M-Pesa · e-Mola · Bim IZI</div>
        <div style={{ width: '166px', margin: '14px auto 0', padding: '8px', boxSizing: 'border-box', borderRadius: '14px', background: '#fff' }}><QR value="https://pay.rihagateway.com/p/cafe-maputo/bolo" size={150} logo={false} /></div>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '6px', marginTop: '12px' }}><img src="/assets/riha-icon-dark.png" alt="" style={{ width: '16px', height: '16px', objectFit: 'contain' }} /><span style={{ fontFamily: 'var(--display)', fontWeight: '800', fontSize: '11px', letterSpacing: '.2em' }}>RIHA</span></div>
      </div>
    </div>
  </div>

  <div style={{ maxWidth: '1280px', margin: '0 auto', padding: 'clamp(72px,9cqi,128px) clamp(20px,4cqi,48px) 0' }}>
    <div style={{ background: '#fff', border: '1px solid #E8E6DF', borderRadius: 'clamp(24px,3cqi,36px)', padding: 'clamp(24px,4cqi,48px)', display: 'flex', flexWrap: 'wrap', gap: 'clamp(28px,4cqi,56px)', alignItems: 'center' }}>
      <div style={{ flex: '1 1 320px', minWidth: '0' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}><span id="assinaturas" style={{ scrollMarginTop: '24px', fontSize: '14px', fontWeight: '600', color: '#0A7A05' }}>Assinaturas</span><span style={{ fontSize: '11px', fontWeight: '600', padding: '3px 8px', borderRadius: '999px', background: '#0A8205', color: '#fff' }}>Novo serviço</span></div>
        <h2 style={{ margin: '10px 0 0', fontFamily: 'var(--display)', fontWeight: '700', fontSize: 'clamp(32px,4.4cqi,56px)', lineHeight: '1', letterSpacing: '-0.035em', textWrap: 'balance' }}>Cobranças mensais, membro a membro.</h2>
        <p style={{ margin: '18px 0 0', fontSize: '17px', lineHeight: '1.6', color: '#3E4742', maxWidth: '500px' }}>Crie o plano e partilhe o link ou o catálogo. A RIHA abre a cobrança de cada membro em cada período e mostra quem pagou, quem deve e quem está em pausa.</p>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '10px', marginTop: '22px', fontSize: '15px' }}>
          <div style={{ display: 'flex', gap: '10px', alignItems: 'flex-start' }}><svg width="18" height="18" viewBox="0 0 24 24" style={{ fill: 'none', stroke: '#0A8205', strokeWidth: '2.4', strokeLinecap: 'round', strokeLinejoin: 'round', flex: 'none', marginTop: '2px' }}><path d="M20 6 9 17l-5-5"></path></svg>Dia fixo ou no dia em que cada cliente subscreve</div>
          <div style={{ display: 'flex', gap: '10px', alignItems: 'flex-start' }}><svg width="18" height="18" viewBox="0 0 24 24" style={{ fill: 'none', stroke: '#0A8205', strokeWidth: '2.4', strokeLinecap: 'round', strokeLinejoin: 'round', flex: 'none', marginTop: '2px' }}><path d="M20 6 9 17l-5-5"></path></svg>Meses sem cobrança, pausas e saltos por membro</div>
          <div style={{ display: 'flex', gap: '10px', alignItems: 'flex-start' }}><svg width="18" height="18" viewBox="0 0 24 24" style={{ fill: 'none', stroke: '#0A8205', strokeWidth: '2.4', strokeLinecap: 'round', strokeLinejoin: 'round', flex: 'none', marginTop: '2px' }}><path d="M20 6 9 17l-5-5"></path></svg>Mês em falta fica em dívida, ou perde-se, à sua escolha</div>
          <div style={{ display: 'flex', gap: '10px', alignItems: 'flex-start' }}><svg width="18" height="18" viewBox="0 0 24 24" style={{ fill: 'none', stroke: '#0A8205', strokeWidth: '2.4', strokeLinecap: 'round', strokeLinejoin: 'round', flex: 'none', marginTop: '2px' }}><path d="M20 6 9 17l-5-5"></path></svg>Pagou em dinheiro? Marque como pago fora da RIHA</div>
        </div>
      </div>
      <div style={{ flex: '1.8 1 560px', minWidth: '0' }}><div style={{ fontSize: '12px', fontWeight: '600', color: '#5E6862', textTransform: 'uppercase', letterSpacing: '.06em', marginBottom: '10px' }}>Exemplo · Força Gym · envio por SMS e WhatsApp em breve</div><GymFlow /></div>
    </div>
  </div>

  <div style={{ maxWidth: '1280px', margin: '0 auto', padding: 'clamp(72px,9cqi,128px) clamp(20px,4cqi,48px) 0' }}>
    <div style={{ fontSize: '14px', fontWeight: '600', color: '#0A7A05' }}>Quem usa</div>
    <h2 style={{ margin: '10px 0 0', fontFamily: 'var(--display)', fontWeight: '700', fontSize: 'clamp(32px,4.4cqi,56px)', lineHeight: '1', letterSpacing: '-0.035em', maxWidth: '720px', textWrap: 'balance' }}>Do balcão à loja online, da escola ao ginásio.</h2>
    <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit,minmax(min(100%,280px),1fr))', gap: '12px', marginTop: '32px' }}>
      {v.sectorCards.map((c, i) => (<Fragment key={i}>
        <div style={{ padding: '22px', borderRadius: '22px', background: '#fff', border: '1px solid #E8E6DF', display: 'flex', flexDirection: 'column', gap: '12px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}><span style={{ fontWeight: '600', fontSize: '17px' }}>{c.t}</span><span style={{ fontSize: '11.5px', fontWeight: '600', padding: '3px 9px', borderRadius: '6px', background: c.obg, color: c.ofg }}>{c.o}</span></div>
          <div style={{ fontSize: '14px', lineHeight: '1.55', color: '#5E6862' }}>{c.d}</div>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', paddingTop: '12px', borderTop: '1px dashed #E1DFD7', fontSize: '13.5px' }}><span style={{ color: '#3E4742' }}>{c.ex}</span><span style={{ fontFamily: 'var(--display)', fontWeight: '700', fontSize: '18px' }}>{c.v}</span></div>
        </div>
      </Fragment>))}
    </div>
  </div>

  <div style={{ maxWidth: '1280px', margin: '0 auto', padding: 'clamp(72px,9cqi,128px) clamp(20px,4cqi,48px) 0' }}>
    <div style={{ background: '#0B120E', color: '#fff', borderRadius: 'clamp(24px,3cqi,36px)', padding: 'clamp(24px,4cqi,48px)', display: 'grid', gridTemplateColumns: 'repeat(auto-fit,minmax(min(100%,420px),1fr))', gap: 'clamp(28px,4cqi,48px)', alignItems: 'start' }}>
      <div>
        <div id="programadores" style={{ scrollMarginTop: '24px', fontSize: '14px', fontWeight: '600', color: '#8EE07F' }}>Programadores</div>
        <h2 style={{ margin: '10px 0 0', fontFamily: 'var(--display)', fontWeight: '700', fontSize: 'clamp(30px,3.8cqi,48px)', lineHeight: '1.02', letterSpacing: '-0.035em', textWrap: 'balance' }}>Uma API. Cada pagamento rastreável.</h2>
        <p style={{ margin: '16px 0 0', fontSize: '16px', lineHeight: '1.6', color: '#B9C3BD', maxWidth: '480px' }}>Crie checkouts e assinaturas com chaves de teste e produção. Chaves de idempotência evitam cobranças em duplicado, e eventos assinados avisam o seu servidor.</p>
        <div style={{ marginTop: '22px', padding: '18px', borderRadius: '16px', background: '#151E19', border: '1px solid rgba(255,255,255,.08)', fontFamily: 'var(--mono)', fontSize: '13px', lineHeight: '1.8', color: '#C9D3CD', whiteSpace: 'pre', overflow: 'auto' }}>{'curl https://rihagateway.com/v1/checkout \\\n  -u '}<span style={{ color: '#FFD21F' }}>sk_live_…</span>{': \\\n  -H "Idempotency-Key: 4471" \\\n  -d amount=890000 \\\n  -d currency=MZN \\\n  -d client_reference_id='}<span style={{ color: '#8EE07F' }}>{'"Pedido #4471"'}</span>{' \\\n  -d success_url='}<span style={{ color: '#8EE07F' }}>https://loja.co.mz/ok</span></div>
        <Link to="/docs" style={{ display: 'inline-block', marginTop: '18px', fontWeight: '600', color: '#fff' }}>Ler a documentação →</Link>
      </div>
      <div style={{ background: '#F4F3EE', color: '#0B120E', borderRadius: '22px', padding: '20px', fontSize: '13.5px' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}><span style={{ fontFamily: 'var(--display)', fontWeight: '700', fontSize: '24px', letterSpacing: '-0.03em' }}>8 900,00 MT</span><span style={{ fontSize: '11.5px', fontWeight: '600', padding: '3px 9px', borderRadius: '999px', background: '#E2F3DE', color: '#0A6B05' }}>Paga</span></div>
        <div style={{ fontSize: '13px', color: '#5E6862', marginTop: '2px' }}>M-Pesa · 84 ··· 4567 · hoje 14:32</div>
        <div style={{ fontSize: '11.5px', fontWeight: '600', color: '#5E6862', textTransform: 'uppercase', letterSpacing: '.06em', margin: '18px 0 8px' }}>Origem</div>
        <div style={{ background: '#fff', borderRadius: '14px', fontFamily: 'var(--mono)', fontSize: '12.5px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', gap: '12px', padding: '10px 14px', borderBottom: '1px solid #F1EFE9' }}><span style={{ color: '#5E6862' }}>canal</span><span>API</span></div>
          <div style={{ display: 'flex', justifyContent: 'space-between', gap: '12px', padding: '10px 14px', borderBottom: '1px solid #F1EFE9' }}><span style={{ color: '#5E6862' }}>sessão</span><span>cs_live_8Hq2…</span></div>
          <div style={{ display: 'flex', justifyContent: 'space-between', gap: '12px', padding: '10px 14px', borderBottom: '1px solid #F1EFE9' }}><span style={{ color: '#5E6862' }}>domínio</span><span>loja.co.mz ✓</span></div>
          <div style={{ display: 'flex', justifyContent: 'space-between', gap: '12px', padding: '10px 14px', borderBottom: '1px solid #F1EFE9' }}><span style={{ color: '#5E6862' }}>chave</span><span>sk_live_…4f2a</span></div>
          <div style={{ display: 'flex', justifyContent: 'space-between', gap: '12px', padding: '10px 14px', borderBottom: '1px solid #F1EFE9' }}><span style={{ color: '#5E6862' }}>referência</span><span>Pedido #4471</span></div>
          <div style={{ display: 'flex', justifyContent: 'space-between', gap: '12px', padding: '10px 14px' }}><span style={{ color: '#5E6862' }}>evento</span><span style={{ color: '#0A6B05' }}>payment.succeeded</span></div>
        </div>
      </div>
    </div>
  </div>

  <div style={{ maxWidth: '1280px', margin: '0 auto', padding: 'clamp(72px,9cqi,128px) clamp(20px,4cqi,48px) 0' }}>
    <div id="seguranca" style={{ scrollMarginTop: '24px', fontSize: '14px', fontWeight: '600', color: '#0A7A05' }}>Segurança</div>
    <h2 style={{ margin: '10px 0 0', fontFamily: 'var(--display)', fontWeight: '700', fontSize: 'clamp(32px,4.4cqi,56px)', lineHeight: '1', letterSpacing: '-0.035em', maxWidth: '720px', textWrap: 'balance' }}>Nada é cobrado sem o cliente confirmar.</h2>
    <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit,minmax(min(100%,260px),1fr))', gap: '12px', marginTop: '32px' }}>
      <div style={{ padding: '22px', borderRadius: '22px', background: '#fff', border: '1px solid #E8E6DF' }}><div style={{ fontWeight: '600', fontSize: '16px' }}>Aprovado no telemóvel</div><div style={{ fontSize: '14px', lineHeight: '1.55', color: '#5E6862', marginTop: '6px' }}>M-Pesa e e-Mola pedem o PIN, e Bim IZI pede confirmação no *181#. Quem aprova é sempre o cliente, no próprio telemóvel.</div></div>
      <div style={{ padding: '22px', borderRadius: '22px', background: '#fff', border: '1px solid #E8E6DF' }}><div style={{ fontWeight: '600', fontSize: '16px' }}>Contas verificadas</div><div style={{ fontSize: '14px', lineHeight: '1.55', color: '#5E6862', marginTop: '6px' }}>Cada negócio é verificado com NUIT, BI e dados bancários antes de levantar.</div></div>
      <div style={{ padding: '22px', borderRadius: '22px', background: '#fff', border: '1px solid #E8E6DF' }}><div style={{ fontWeight: '600', fontSize: '16px' }}>Teste separado de produção</div><div style={{ fontSize: '14px', lineHeight: '1.55', color: '#5E6862', marginTop: '6px' }}>Chaves e dados de teste nunca tocam em dinheiro real.</div></div>
      <div style={{ padding: '22px', borderRadius: '22px', background: '#fff', border: '1px solid #E8E6DF' }}><div style={{ fontWeight: '600', fontSize: '16px' }}>Domínios e eventos assinados</div><div style={{ fontSize: '14px', lineHeight: '1.55', color: '#5E6862', marginTop: '6px' }}>Callbacks só para domínios verificados, cada evento com assinatura.</div></div>
    </div>
  </div>

  <div style={{ maxWidth: '1280px', margin: '0 auto', padding: 'clamp(72px,9cqi,128px) clamp(20px,4cqi,48px) 0', display: 'grid', gridTemplateColumns: 'repeat(auto-fit,minmax(min(100%,420px),1fr))', gap: '16px', alignItems: 'stretch' }}>
    <div style={{ background: '#fff', border: '1px solid #E8E6DF', borderRadius: '28px', padding: 'clamp(24px,3cqi,36px)' }}>
      <div id="preco" style={{ scrollMarginTop: '24px', fontSize: '14px', fontWeight: '600', color: '#0A7A05' }}>Preço</div>
      <div style={{ display: 'flex', alignItems: 'flex-end', gap: '14px', marginTop: '10px', flexWrap: 'wrap' }}><span style={{ fontFamily: 'var(--display)', fontWeight: '800', fontSize: 'clamp(80px,10cqi,128px)', lineHeight: '.85', letterSpacing: '-0.06em' }}>{v.fee}</span><span style={{ fontSize: '15px', color: '#5E6862', paddingBottom: '8px' }}>por pagamento recebido.<br />Sem mensalidade.</span></div>
      <div style={{ marginTop: '28px', paddingTop: '22px', borderTop: '1px solid #EEECE5' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', fontSize: '14px' }}><span style={{ color: '#3E4742' }}>Se receber por mês</span><span style={{ fontFamily: 'var(--display)', fontWeight: '700', fontSize: '22px' }}>{v.volLabel} MT</span></div>
        <input type="range" min="10000" max="2000000" step="10000" value={v.vol} aria-label="Quanto recebe por mês" onChange={v.setVol} style={{ width: '100%', marginTop: '12px', accentColor: '#0A8205' }} />
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '10px', marginTop: '14px' }}>
          <div style={{ padding: '14px', borderRadius: '14px', background: '#F4F3EE' }}><div style={{ fontSize: '12.5px', color: '#5E6862' }}>Taxa RIHA</div><div style={{ fontWeight: '600', fontSize: '18px', marginTop: '2px' }}>{v.feeLabel} MT</div></div>
          <div style={{ padding: '14px', borderRadius: '14px', background: '#E2F3DE' }}><div style={{ fontSize: '12.5px', color: '#0A6B05' }}>Fica consigo</div><div style={{ fontWeight: '600', fontSize: '18px', marginTop: '2px' }}>{v.netLabel} MT</div></div>
        </div>
      </div>
    </div>
    <div style={{ background: '#0A8205', color: '#fff', borderRadius: '28px', padding: 'clamp(24px,3cqi,36px)', display: 'flex', flexDirection: 'column', position: 'relative', overflow: 'hidden' }}>
      <div style={{ position: 'absolute', top: '-60px', right: '70px', width: '46px', height: '500px', background: '#0B120E', transform: 'skewX(22deg)', opacity: '.85' }}></div>
      <div style={{ position: 'absolute', top: '-60px', right: '8px', width: '46px', height: '500px', background: '#FF0F15', transform: 'skewX(22deg)', opacity: '.16', filter: stripeBlur(100) }}></div>
      <div style={{ position: 'relative', fontSize: '14px', fontWeight: '600', color: '#CDEBC7' }}>Volume alto</div>
      <div style={{ position: 'relative', fontFamily: 'var(--display)', fontWeight: '700', fontSize: 'clamp(30px,3.6cqi,44px)', lineHeight: '1.02', letterSpacing: '-0.035em', marginTop: '10px', maxWidth: '400px' }}>Recebe muito? Negociamos a sua taxa.</div>
      <div style={{ position: 'relative', fontSize: '15.5px', lineHeight: '1.55', color: '#E3F4DF', marginTop: '12px', maxWidth: '400px' }}>Diga-nos quanto recebe por mês e preparamos uma proposta para o seu negócio.</div>
      <div style={{ flex: '1', minHeight: '24px' }}></div>
      <a href={SUPPORT_MAILTO} style={{ position: 'relative', alignSelf: 'flex-start', height: '52px', padding: '0 22px', borderRadius: '15px', background: '#fff', color: '#0B120E', fontWeight: '600', fontSize: '15.5px', display: 'flex', alignItems: 'center' }}>Falar com a equipa</a>
    </div>
  </div>

  <div style={{ maxWidth: '1280px', margin: '0 auto', padding: 'clamp(72px,9cqi,128px) clamp(20px,4cqi,48px) 0', textAlign: 'center', display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
    <h2 style={{ margin: '0', fontFamily: 'var(--display)', fontWeight: '700', fontSize: 'clamp(40px,6cqi,80px)', lineHeight: '.95', letterSpacing: '-0.045em', maxWidth: '860px', textWrap: 'balance' }}>Comece a receber hoje.</h2>
    <Link to="/registo" style={{ marginTop: '28px', height: '58px', padding: '0 28px', borderRadius: '16px', background: '#0A8205', color: '#fff', fontWeight: '600', fontSize: '17px', display: 'flex', alignItems: 'center', boxShadow: '0 14px 30px -12px rgba(10,130,5,.7)', textDecoration: 'none', whiteSpace: 'nowrap' }}>Criar conta grátis</Link>
  </div>

  <div style={{ maxWidth: '1280px', margin: '0 auto', padding: 'clamp(56px,7cqi,96px) clamp(20px,4cqi,48px) 40px', display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: '20px', fontSize: '14px', color: '#5E6862' }}>
    <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}><img src="/assets/riha-icon-dark.png" alt="RIHA" style={{ width: '22px', height: '22px', objectFit: 'contain' }} /><span style={{ fontFamily: 'var(--display)', fontWeight: '800', fontSize: '13px', letterSpacing: '.2em', color: '#0B120E' }}>RIHA</span></div>
    <span>© 2026 RIHA · Maputo, Moçambique</span><div className="lp-foot-gap" style={{ flex: '1' }}></div><span>Termos</span><span>Privacidade</span><a href={SUPPORT_MAILTO} style={{ color: 'inherit' }}>{SUPPORT_EMAIL}</a><Link to="/docs" style={{ color: 'inherit' }}>API</Link>
  </div>
</div>
    </div>
  )
}
