import { useState, type CSSProperties, type ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { Icon } from '../../components/ui'
import { dl, dly, mt, prest, quote, rateLabel, unit, type Freq } from '../../lib/credit'
import { SUPPORT_EMAIL, SUPPORT_MAILTO } from '../../lib/support'

const MAILTO = SUPPORT_MAILTO + '?subject=' + encodeURIComponent('Microcrédito RIHA')

const PRODUCTS: { name: string; min: number; max: number; freq: Freq; tMin: number; tMax: number; rate: number; clientDay: boolean }[] = [
  { name: 'Crédito Negócio', min: 500000, max: 5000000, freq: 'mensal', tMin: 2, tMax: 12, rate: 600, clientDay: false },
  { name: 'Crédito Salário', min: 300000, max: 3000000, freq: 'mensal', tMin: 1, tMax: 3, rate: 500, clientDay: true },
  { name: 'Crédito Rápido', min: 100000, max: 1000000, freq: 'semanal', tMin: 4, tMax: 8, rate: 200, clientDay: false },
]

const STEPS: [string, string][] = [
  ['Crie o produto', 'Montantes, prazos, taxa, mora e o que perguntar ao cliente.'],
  ['O cliente pede', 'Simula no link, vê o custo total antes de pedir e envia o BI e os dados.'],
  ['A instituição decide', 'Vê o pedido, o esforço e o histórico. Aprova, ajusta o prazo ou recusa.'],
  ['Acordo e dinheiro', 'O cliente aceita com código SMS. O dinheiro sai do saldo RIHA para a carteira dele, ou é entregue fora da RIHA.'],
  ['Prestações', 'Link em cada data, lembretes e reenvios. Em atraso, a instituição propõe um novo acordo.'],
]

const TIMELINE: [string, string, string, string][] = [
  ['3 dias antes', 'Lembrete', 'Valor e data, para ter saldo', '#C9D3CD'],
  ['No dia · 08:00', 'Link de pagamento', 'Valor e carteira já preenchidos. O cliente confirma com o PIN.', '#8EE07F'],
  ['No dia · 18:00', 'Lembrete', 'Só se não abriu o link', '#C9D3CD'],
  ['+2 dias', 'Reenvio', 'Passa a Em atraso. A mora começa a contar.', '#FFD21F'],
  ['+7 dias', 'Reenvio', 'Segundo aviso. A instituição vê o atraso no painel.', '#FFD21F'],
  ['+14 dias', 'Proposta de acordo', 'O cliente pode pedir um novo acordo no link', '#FFD21F'],
  ['+30 dias', 'Ação final', 'Cobrança manual pela instituição', '#FFD21F'],
]

const AUDIENCE: [string, string, string, string][] = [
  ['Instituições de microcrédito', 'Crédito a negócios e a assalariados, com um produto para cada caso.', 'Crédito Negócio', '6% ao mês'],
  ['Cooperativas de poupança e crédito', 'Empréstimos a membros, com o acordo aceite por SMS e as prestações no saldo.', 'Crédito Machamba', '6 a 9 meses'],
  ['Lojas', 'Venda a prestações com acordo assinado. A entrega é feita fora da RIHA.', 'Venda a prestações', 'Dia à escolha'],
  ['Empresas', 'Adiantamentos de salário, com a prestação no dia do salário.', 'Crédito Salário', '1 a 3 meses'],
]

const RULES: [string, string][] = [
  ['A RIHA não empresta', 'O crédito, a taxa e a decisão são da instituição. A RIHA trata do pedido, do acordo e da cobrança.'],
  ['Custo total antes de pedir', 'A prestação, os juros e o total a pagar aparecem no simulador e no acordo.'],
  ['Acordo aceite por código SMS', 'Cada versão do acordo fica guardada, com a data, a hora e o número que aceitou.'],
  ['Nada sai sem confirmação', 'O dinheiro só sai depois de o cliente aceitar. Cada prestação é aprovada com o PIN.'],
]

const css = `
.ml{background:#F4F3EE;color:#0B120E;font-family:var(--sans);overflow:hidden}
.ml a{text-decoration:none}
.ml-w{max-width:1280px;margin:0 auto;padding:0 clamp(20px,4vw,48px)}
.ml-h2{font-family:var(--display);font-weight:700;letter-spacing:-.045em;line-height:.98;font-size:clamp(36px,5.2vw,68px);margin:10px 0 0;text-wrap:balance}
.ml-eyebrow{color:#0A8205;font-weight:600;font-size:15px}
.ml-p{font-size:clamp(16px,1.4vw,18px);line-height:1.55;color:#3E4742;text-wrap:pretty}
.ml-btn{height:56px;padding:0 26px;border-radius:16px;font-weight:600;font-size:16px;display:inline-flex;align-items:center;white-space:nowrap}
.ml-steps{display:grid;grid-template-columns:repeat(5,1fr);gap:12px;margin-top:28px}
.ml-two{display:grid;grid-template-columns:1fr 1fr;gap:48px;align-items:center}
.ml-four{display:grid;grid-template-columns:repeat(4,1fr);gap:12px;margin-top:28px}
.ml-phone{width:300px;border-radius:44px;background:#0B120E;padding:10px;box-shadow:0 50px 90px -30px rgba(0,0,0,.6);flex:none}
.ml-screen{border-radius:36px;background:#F4F3EE;padding:18px 18px 22px;min-height:520px;display:flex;flex-direction:column}
.ml-nav{display:flex;gap:26px;font-size:14.5px;font-weight:500;color:#3E4742}
@media (max-width:1000px){.ml-steps{grid-template-columns:1fr 1fr}.ml-two{grid-template-columns:1fr;gap:28px}.ml-four{grid-template-columns:1fr 1fr}.ml-nav{display:none}.ml-hide{display:none!important}}
@media (max-width:620px){.ml-steps,.ml-four{grid-template-columns:1fr}.ml-phone{width:100%;max-width:330px}}
`

function Phone({ children, sub = 'Crédito Negócio' }: { children: ReactNode; sub?: string }) {
  return (
    <div className="ml-phone">
      <div className="ml-screen">
        <div style={{ alignSelf: 'center', display: 'inline-flex', alignItems: 'center', gap: 6, padding: '5px 12px', borderRadius: 999, background: '#EDECE6', fontSize: 12, color: '#3E4742' }}><Icon name="lock" size={11} stroke={2.4} />pay.rihagateway.com</div>
        <div style={{ display: 'flex', gap: 10, alignItems: 'center', marginTop: 16 }}>
          <span style={{ width: 38, height: 38, borderRadius: 11, background: '#FFD21F', display: 'grid', placeItems: 'center', fontWeight: 700, fontSize: 13 }}>AM</span>
          <span><b style={{ fontSize: 14.5 }}>Avante Microcrédito</b><div style={{ fontSize: 12, color: '#5E6862' }}>{sub}</div></span>
        </div>
        {children}
      </div>
    </div>
  )
}

function Simulator({ pi }: { pi: number }) {
  const p = PRODUCTS[pi]
  const [amt, setAmt] = useState(2000000)
  const [n, setN] = useState(4)
  const a = Math.min(Math.max(amt, p.min), p.max)
  const nn = Math.min(Math.max(n, p.tMin), p.tMax)
  const q = quote({ freq: p.freq, rate_bp: p.rate }, a, nn, 12)
  const step = p.max - p.min >= 2000000 ? 100000 : 50000
  const btn = (on: boolean): CSSProperties => ({ width: 46, height: '100%', border: 0, background: 'none', fontSize: 20, color: on ? '#0B120E' : '#C9CDC6' })
  return (
    <Phone sub={p.name}>
      <div style={{ fontFamily: 'var(--display)', fontWeight: 700, fontSize: 20, marginTop: 16 }}>Quanto precisa?</div>
      <div style={{ fontFamily: 'var(--display)', fontWeight: 700, fontSize: 40, letterSpacing: '-.04em' }}>{mt(a)} <span style={{ fontSize: 16, color: '#5E6862' }}>MT</span></div>
      <input type="range" min={p.min} max={p.max} step={step} value={a} onChange={e => setAmt(+e.target.value)} style={{ width: '100%', accentColor: '#0A8205' }} aria-label="Montante" />
      <div className="mono" style={{ display: 'flex', justifyContent: 'space-between', fontSize: 11, color: '#5E6862' }}><span>{mt(p.min)}</span><span>{mt(p.max)}</span></div>
      <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 12.5, margin: '12px 0 6px' }}><span>Em quanto tempo paga</span><span style={{ color: '#5E6862' }}>de {p.tMin} a {unit(p.freq, p.tMax)}</span></div>
      <div style={{ display: 'flex', alignItems: 'center', height: 46, borderRadius: 14, background: '#fff', border: '1px solid #E4E2DA' }}>
        <button style={btn(nn > p.tMin)} onClick={() => nn > p.tMin && setN(nn - 1)} aria-label="Menos">−</button><b style={{ flex: 1, textAlign: 'center' }}>{unit(p.freq, nn)}</b><button style={btn(nn < p.tMax)} onClick={() => nn < p.tMax && setN(nn + 1)} aria-label="Mais">+</button>
      </div>
      <div style={{ background: '#fff', borderRadius: 16, padding: 14, marginTop: 12, fontSize: 13 }}>
        <div style={{ color: '#5E6862', fontSize: 12 }}>Paga {p.freq === 'mensal' ? 'por mês' : 'por semana'}</div>
        <div><b style={{ fontFamily: 'var(--display)', fontSize: 24 }}>{mt(q.inst)}</b> <span style={{ color: '#5E6862', fontSize: 12 }}>MT × {nn}</span></div>
        <div style={{ borderTop: '1px dashed #E1DFD7', margin: '8px 0 4px' }} />
        {[['Recebe', `${mt(a)} MT`], [`Juros · ${rateLabel(p.rate, p.freq)}`, `${mt(q.interest)} MT`], ['Total a pagar', `${mt(q.total)} MT`], ['Datas', `${dl(q.dates[0])} a ${dly(q.dates[nn - 1])}`]].map(([k, v], i) => (
          <div key={k} style={{ display: 'flex', justifyContent: 'space-between', padding: '3px 0', fontWeight: i === 2 ? 700 : 400, color: i === 2 ? '#0B120E' : undefined }}><span style={{ color: i === 2 ? undefined : '#5E6862' }}>{k}</span><span>{v}</span></div>
        ))}
      </div>
      <div style={{ flex: 1 }} />
      <div style={{ height: 50, borderRadius: 14, background: '#0A8205', color: '#fff', display: 'grid', placeItems: 'center', fontWeight: 600, marginTop: 14 }}>Pedir {mt(a)} MT</div>
      <div style={{ textAlign: 'center', fontSize: 11.5, color: '#5E6862', marginTop: 6 }}>Sem outras taxas. Resposta em 1 dia útil.</div>
    </Phone>
  )
}

function HeroPanel() {
  const card: CSSProperties = { background: '#fff', borderRadius: 14, padding: 14, border: '1px solid #ECEAE3' }
  return (
    <div style={{ position: 'relative', borderRadius: 'clamp(22px,3vw,36px)', background: '#0B120E', overflow: 'hidden', padding: 'clamp(16px,4vw,56px)', minHeight: 420 }}>
      <div style={{ position: 'absolute', right: 140, top: -80, width: 70, height: 700, background: '#0A8205', opacity: .35, transform: 'rotate(-22deg)', filter: 'blur(4px)' }} />
      <div style={{ position: 'absolute', right: 40, top: -80, width: 70, height: 700, background: '#C2080D', opacity: .35, transform: 'rotate(-22deg)', filter: 'blur(4px)' }} />
      <div style={{ position: 'relative', display: 'flex', gap: 24, alignItems: 'flex-start' }}>
        <div className="ml-hide" style={{ flex: 1, minWidth: 0, background: '#F4F3EE', borderRadius: 18, padding: 20, fontSize: 12.5 }}>
          <div style={{ color: '#5E6862', fontSize: 11 }}>Serviços</div>
          <div style={{ fontFamily: 'var(--display)', fontWeight: 700, fontSize: 22 }}>Microcrédito</div>
          <div style={{ display: 'flex', gap: 16, margin: '10px 0 14px', borderBottom: '1px solid #E4E2DA', paddingBottom: 6, color: '#8A938E', fontWeight: 600 }}><span style={{ color: '#0B120E' }}>Resumo</span><span>Pedidos <span style={{ background: '#FFD21F', borderRadius: 99, padding: '0 6px', color: '#3F3200' }}>5</span></span><span>Empréstimos</span><span>Produtos</span></div>
          <div style={{ display: 'grid', gridTemplateColumns: '1.3fr 1fr 1fr', gap: 10 }}>
            <div style={{ ...card, background: '#0B120E', color: '#fff', border: 0 }}><div style={{ color: '#B9C3BD' }}>Carteira ativa</div><b style={{ fontFamily: 'var(--display)', fontSize: 24 }}>1 284 600</b> MT<div style={{ color: '#B9C3BD', fontSize: 11 }}>86 empréstimos em 3 produtos</div></div>
            <div style={card}><div style={{ color: '#5E6862' }}>Pedidos por decidir</div><b style={{ fontFamily: 'var(--display)', fontSize: 24 }}>5</b><div style={{ color: '#6B5200', fontSize: 11, fontWeight: 600 }}>O mais antigo espera há 26 h</div></div>
            <div style={card}><div style={{ color: '#5E6862' }}>Em atraso</div><b style={{ fontFamily: 'var(--display)', fontSize: 24 }}>61 400</b> MT<div style={{ color: '#5E6862', fontSize: 11 }}>7 empréstimos</div></div>
          </div>
          <div style={{ ...card, marginTop: 10 }}>
            <b>Novembro</b><span style={{ float: 'right', color: '#5E6862' }}>318 400 MT em prestações</span>
            <div style={{ display: 'flex', height: 10, borderRadius: 5, overflow: 'hidden', gap: 2, margin: '12px 0' }}><span style={{ width: '62%', background: '#0A8205' }} /><span style={{ width: '5%', background: 'repeating-linear-gradient(135deg,#9AA39E 0 2px,#D7D5CD 2px 4px)' }} /><span style={{ width: '10%', background: '#FFD21F' }} /><span style={{ flex: 1, background: '#E6E4DC' }} /></div>
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4,1fr)', gap: 8 }}>{[['Recebido pela RIHA', '196 200'], ['Pago fora da RIHA', '16 400'], ['Em atraso', '31 000'], ['Por vencer', '74 800']].map(([k, v]) => <div key={k}><div style={{ color: '#5E6862', fontSize: 11 }}>{k}</div><b style={{ fontSize: 15 }}>{v}</b></div>)}</div>
          </div>
        </div>
        <div style={{ margin: '0 auto' }}><Simulator pi={0} /></div>
      </div>
    </div>
  )
}

export default function MicroLanding() {
  const [pi, setPi] = useState(0)
  return (
    <div className="ml">
      <style>{css}</style>
      <div className="ml-w" style={{ display: 'flex', alignItems: 'center', gap: 28, height: 76 }}>
        <Link to="/" style={{ display: 'flex', alignItems: 'center', gap: 10, color: '#0B120E' }}><img src="/assets/riha-icon-dark.png" alt="RIHA" style={{ width: 30, height: 30, objectFit: 'contain' }} /><span style={{ fontFamily: 'var(--display)', fontWeight: 800, fontSize: 17, letterSpacing: '.2em' }}>RIHA</span></Link>
        <span style={{ borderLeft: '1px solid #D5D3CB', paddingLeft: 16, fontWeight: 600, fontSize: 15 }}>Microcrédito</span>
        <nav className="ml-nav" style={{ flex: 1 }}>{[['#como', 'Como funciona'], ['#cliente', 'O cliente'], ['#cobranca', 'Cobrança'], ['#para-quem', 'Para quem'], ['#regras', 'Regras']].map(([h, l]) => <a key={h} href={h} style={{ color: 'inherit' }}>{l}</a>)}</nav>
        <div style={{ flex: 1 }} className="ml-hide-spacer" />
        <Link to="/entrar" style={{ color: '#0B120E', fontWeight: 600, fontSize: 14.5 }}>Entrar</Link>
        <a href={MAILTO} className="ml-btn" style={{ height: 40, padding: '0 16px', borderRadius: 12, background: '#0B120E', color: '#fff', fontSize: 14 }}>Falar com a equipa</a>
      </div>

      <div className="ml-w" style={{ textAlign: 'center', padding: 'clamp(40px,6vw,80px) clamp(20px,4vw,48px) 0' }}>
        <div style={{ display: 'inline-flex', alignItems: 'center', gap: 8, height: 32, padding: '0 12px 0 6px', borderRadius: 999, background: '#fff', border: '1px solid #E4E2DA', fontSize: 13, fontWeight: 600 }}><span style={{ fontSize: 11, padding: '3px 8px', borderRadius: 999, background: '#0A8205', color: '#fff' }}>Novo serviço</span>Para instituições de crédito</div>
        <h1 style={{ margin: '22px auto 0', fontFamily: 'var(--display)', fontWeight: 700, fontSize: 'clamp(44px,7.4vw,100px)', lineHeight: .94, letterSpacing: '-0.045em', maxWidth: 1000, textWrap: 'balance' }}>Crédito pedido por link. Prestações cobradas na data.</h1>
        <p className="ml-p" style={{ margin: '24px auto 0', maxWidth: 660 }}>O cliente pede e simula no telemóvel. A instituição decide e define as condições. O cliente aceita o acordo com um código SMS e recebe o dinheiro na carteira. A RIHA envia o link de cada prestação, com lembretes e reenvios.</p>
        <div style={{ display: 'flex', flexWrap: 'wrap', justifyContent: 'center', gap: 12, marginTop: 30 }}>
          <a href={MAILTO} className="ml-btn" style={{ background: '#0A8205', color: '#fff', boxShadow: '0 14px 30px -12px rgba(10,130,5,.7)' }}>Falar com a equipa</a>
          <Link to="/entrar" className="ml-btn" style={{ background: '#fff', border: '1px solid #E4E2DA', color: '#0B120E' }}>Ver o painel</Link>
        </div>
      </div>
      <div className="ml-w" style={{ marginTop: 56 }}><HeroPanel /></div>

      <section id="como" className="ml-w" style={{ paddingTop: 110 }}>
        <div className="ml-eyebrow">Como funciona</div>
        <h2 className="ml-h2">Do pedido à última prestação.</h2>
        <div className="ml-steps">
          {STEPS.map(([t, d], i) => (
            <div key={t} style={{ borderRadius: 24, padding: 22, minHeight: 210, background: i === 4 ? '#0A8205' : '#fff', color: i === 4 ? '#fff' : undefined, border: i === 4 ? 0 : '1px solid #E8E6DF' }}>
              <div style={{ fontFamily: 'var(--display)', fontWeight: 700, fontSize: 48, color: i === 4 ? 'rgba(255,255,255,.55)' : '#D5D3CB', lineHeight: 1 }}>{i + 1}</div>
              <div style={{ fontWeight: 600, fontSize: 18, marginTop: 26 }}>{t}</div>
              <div style={{ fontSize: 14, lineHeight: 1.5, marginTop: 6, color: i === 4 ? 'rgba(255,255,255,.9)' : '#5E6862' }}>{d}</div>
            </div>
          ))}
        </div>
      </section>

      <section id="cliente" className="ml-w" style={{ paddingTop: 110 }}>
        <div className="ml-two" style={{ background: '#fff', borderRadius: 32, padding: 'clamp(24px,4vw,48px)', border: '1px solid #E8E6DF' }}>
          <div>
            <div className="ml-eyebrow">O pedido</div>
            <h2 className="ml-h2">O cliente vê o custo total antes de pedir.</h2>
            <p className="ml-p" style={{ marginTop: 18 }}>O link abre no telemóvel, sem aplicação. O cliente escolhe o montante e o prazo dentro dos limites do produto e vê a prestação, os juros e o total. Se a prestação passar o limite de esforço sobre o rendimento declarado, o pedido não segue.</p>
            <div style={{ fontSize: 13.5, color: '#5E6862', margin: '22px 0 8px' }}>Experimente um produto</div>
            <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>{PRODUCTS.map((p, i) => <button key={p.name} onClick={() => setPi(i)} style={{ height: 42, padding: '0 16px', borderRadius: 999, fontWeight: 600, fontSize: 14.5, background: i === pi ? '#0B120E' : '#fff', color: i === pi ? '#fff' : '#0B120E', border: '1px solid ' + (i === pi ? '#0B120E' : '#E4E2DA') }}>{p.name}</button>)}</div>
            <div className="mono" style={{ fontSize: 12.5, color: '#3E4742', marginTop: 12 }}>{mt(PRODUCTS[pi].min)} a {mt(PRODUCTS[pi].max)} MT · {PRODUCTS[pi].tMin} a {unit(PRODUCTS[pi].freq, PRODUCTS[pi].tMax)} · {rateLabel(PRODUCTS[pi].rate, PRODUCTS[pi].freq)}{PRODUCTS[pi].clientDay ? ' · dia à escolha' : ''}</div>
            <ul style={{ listStyle: 'none', padding: 0, margin: '20px 0 0', display: 'flex', flexDirection: 'column', gap: 10, fontSize: 15 }}>
              {['Qualquer montante e prazo dentro dos limites', 'Prestações semanais, quinzenais ou mensais', 'Dia do salário à escolha, se o produto permitir', 'BI e dados enviados no mesmo link'].map(x => <li key={x} style={{ display: 'flex', gap: 10 }}><Icon name="check" size={18} stroke={2.4} color="#0A8205" />{x}</li>)}
            </ul>
          </div>
          <div style={{ display: 'flex', justifyContent: 'center' }}><Simulator key={pi} pi={pi} /></div>
        </div>
      </section>

      <section className="ml-w" style={{ paddingTop: 110 }}>
        <div className="ml-two">
          <div>
            <div className="ml-eyebrow">A decisão</div>
            <h2 className="ml-h2">Aprove, ajuste ou recuse. A taxa e o risco são seus.</h2>
            <p className="ml-p" style={{ marginTop: 18 }}>Cada pedido chega com o esforço, os documentos, as verificações e o histórico do cliente na sua instituição. Ajuste o montante, o prazo e o dia antes de enviar o acordo. Acima do valor que escolher, pode exigir duas pessoas a aprovar.</p>
            <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', marginTop: 18 }}>{['No computador', 'No telemóvel', 'Duas pessoas acima de um valor'].map(x => <span key={x} style={{ padding: '8px 14px', borderRadius: 999, background: '#fff', border: '1px solid #E4E2DA', fontSize: 14, fontWeight: 600 }}>{x}</span>)}</div>
          </div>
          <div style={{ background: '#fff', borderRadius: 24, border: '1px solid #E8E6DF', padding: 22, boxShadow: '0 40px 80px -40px rgba(0,0,0,.35)' }}>
            <div style={{ fontSize: 12, color: '#5E6862' }}>Celeste Machava · Crédito Negócio</div>
            <div style={{ fontFamily: 'var(--display)', fontWeight: 700, fontSize: 22 }}>Aprovar e enviar acordo</div>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 10, marginTop: 14, fontSize: 14 }}>
              {[['Montante · pediu 20 000', '20 000 MT'], ['Prazo · pediu 4', '4 meses'], ['Dia da prestação', 'Dia 12'], ['1ª prestação', '12 dez 2026']].map(([k, v]) => <div key={k}><div style={{ fontSize: 12, color: '#5E6862', marginBottom: 4 }}>{k}</div><div style={{ height: 44, borderRadius: 12, border: '1px solid #E4E2DA', display: 'flex', alignItems: 'center', padding: '0 12px', fontWeight: 600 }}>{v}</div></div>)}
            </div>
            <div style={{ border: '1px solid #ECEAE3', borderRadius: 14, marginTop: 14, padding: '10px 14px', fontSize: 13.5 }}>
              <div style={{ display: 'flex', justifyContent: 'space-between' }}><b>{prest(4)} de 6 200 MT</b><span style={{ color: '#5E6862' }}>Total 24 800 · juros 4 800</span></div>
              {['12 dez 2026', '12 jan 2027', '12 fev 2027', '12 mar 2027'].map((d, i) => <div key={d} style={{ display: 'flex', justifyContent: 'space-between', padding: '4px 0', color: '#3E4742' }}><span>{i + 1}  {d}</span><span>6 200 MT</span></div>)}
              <div style={{ color: '#0A6B05', fontWeight: 600, marginTop: 4 }}>Esforço de 25% do rendimento declarado</div>
            </div>
            <div style={{ height: 50, borderRadius: 14, background: '#0A8205', color: '#fff', display: 'grid', placeItems: 'center', fontWeight: 600, marginTop: 14 }}>Enviar acordo · 20 000 MT</div>
          </div>
        </div>
      </section>

      <section id="cobranca" className="ml-w" style={{ paddingTop: 110 }}>
        <div style={{ position: 'relative', overflow: 'hidden', background: '#0B120E', color: '#fff', borderRadius: 36, padding: 'clamp(24px,4vw,56px)' }}>
          <div className="ml-two">
            <div>
              <div className="ml-eyebrow" style={{ color: '#8EE07F' }}>Cobrança</div>
              <h2 className="ml-h2" style={{ color: '#fff' }}>Cada prestação cobrada como uma assinatura.</h2>
              <p className="ml-p" style={{ color: '#B9C3BD', marginTop: 18 }}>M-Pesa e e-Mola não permitem débitos automáticos. Na data, a RIHA envia o link com o valor e a carteira já preenchidos. O cliente confirma com o PIN e o pagamento entra no seu saldo.</p>
              <div style={{ border: '1px solid rgba(255,255,255,.1)', borderRadius: 18, marginTop: 24, overflow: 'hidden' }}>
                {TIMELINE.map(([w, t, n, c], i) => <div key={i} style={{ display: 'grid', gridTemplateColumns: '130px 1fr', gap: 14, padding: '13px 18px', borderTop: i ? '1px solid rgba(255,255,255,.07)' : 0 }}><span className="mono" style={{ color: c, fontSize: 12.5 }}>{w}</span><span><b style={{ fontWeight: 600 }}>{t}</b><div style={{ color: '#93A09A', fontSize: 13 }}>{n}</div></span></div>)}
              </div>
              <div style={{ color: '#93A09A', fontSize: 13, marginTop: 10 }}>Os dias de cada passo ajustam-se nas definições.</div>
            </div>
            <div style={{ display: 'flex', justifyContent: 'center' }}>
              <Phone sub="Crédito de Celeste Machava">
                <div style={{ background: '#fff', borderRadius: 16, padding: 14, marginTop: 16 }}><div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 13 }}><span>Prestação 2 de 4</span><span style={{ background: '#EEEDE8', borderRadius: 99, padding: '1px 8px', fontSize: 11 }}>vence 12 jan</span></div><div style={{ borderTop: '1px dashed #E1DFD7', margin: '10px 0' }} /><div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline' }}><span style={{ color: '#5E6862', fontSize: 12 }}>Total</span><b style={{ fontFamily: 'var(--display)', fontSize: 26 }}>6 200,00 <span style={{ fontSize: 12, color: '#5E6862' }}>MT</span></b></div></div>
                <div style={{ fontWeight: 700, marginTop: 18, color: '#0B120E' }}>Pagar com</div>
                <div style={{ background: '#fff', border: '2px solid #0B120E', borderRadius: 14, padding: 12, marginTop: 8, display: 'flex', gap: 10, alignItems: 'center', color: '#0B120E' }}><span style={{ width: 32, height: 32, borderRadius: 9, background: '#FDE7E6', color: '#C2080D', display: 'grid', placeItems: 'center', fontWeight: 700 }}>M</span><span style={{ flex: 1 }}><b>M-Pesa</b><div style={{ fontSize: 11, color: '#5E6862' }}>84 555 0142 · o número do acordo</div></span></div>
                <div style={{ flex: 1 }} />
                <div style={{ height: 50, borderRadius: 14, background: '#0A8205', color: '#fff', display: 'grid', placeItems: 'center', fontWeight: 600 }}>Pagar 6 200,00 MT</div>
              </Phone>
            </div>
          </div>
        </div>
      </section>

      <section className="ml-w" style={{ paddingTop: 110 }}>
        <div className="ml-two">
          <div>
            <div className="ml-eyebrow">Em atraso</div>
            <h2 className="ml-h2">Um novo acordo, sem refazer o crédito.</h2>
            <p className="ml-p" style={{ marginTop: 18 }}>Quando o cliente não consegue pagar, junte o atraso às prestações que faltam ou alargue o prazo, e perdoe a mora se quiser. O cliente recebe o novo plano por SMS e aceita com um código. O acordo antigo fica no histórico.</p>
          </div>
          <div style={{ display: 'flex', justifyContent: 'center' }}>
            <Phone sub="Crédito de Zacarias Muianga">
              <div style={{ fontFamily: 'var(--display)', fontWeight: 700, fontSize: 21, marginTop: 16, color: '#0B120E' }}>Novo acordo de pagamento</div>
              <div style={{ border: '1px dashed #D6D4CB', borderRadius: 14, padding: 12, marginTop: 12, fontSize: 12.5, color: '#8A938E' }}>Plano atual<div style={{ textDecoration: 'line-through' }}>6 868 MT em atraso, mais 2 prestações de 6 800 MT</div></div>
              <div style={{ background: '#fff', border: '2px solid #0B120E', borderRadius: 16, padding: 14, marginTop: 8, fontSize: 13.5, color: '#0B120E' }}><div style={{ color: '#5E6862', fontSize: 12 }}>Novo plano</div><b>4 prestações de 5 100 MT</b>{['20 nov', '20 dez', '20 jan', '20 fev'].map((d, i) => <div key={d} style={{ display: 'flex', justifyContent: 'space-between', padding: '3px 0' }}><span>{i + 1}  {d}</span><span>5 100</span></div>)}<div style={{ color: '#0A6B05', fontWeight: 600, fontSize: 12, marginTop: 6 }}>Mora de 68 MT perdoada. O total não aumenta.</div></div>
              <div style={{ flex: 1 }} />
              <div style={{ height: 50, borderRadius: 14, background: '#0A8205', color: '#fff', display: 'grid', placeItems: 'center', fontWeight: 600, marginTop: 14 }}>Aceitar com código SMS</div>
            </Phone>
          </div>
        </div>
      </section>

      <section id="para-quem" className="ml-w" style={{ paddingTop: 110 }}>
        <div className="ml-eyebrow">Para quem</div>
        <h2 className="ml-h2">Para quem empresta, vende a prestações ou adianta salários.</h2>
        <div className="ml-four">
          {AUDIENCE.map(([t, d, ex, v]) => <div key={t} style={{ background: '#fff', borderRadius: 24, border: '1px solid #E8E6DF', padding: 22, display: 'flex', flexDirection: 'column' }}><b style={{ fontSize: 17 }}>{t}</b><div style={{ color: '#5E6862', fontSize: 14, lineHeight: 1.5, marginTop: 8, flex: 1 }}>{d}</div><div style={{ borderTop: '1px dashed #E1DFD7', marginTop: 22, paddingTop: 14, display: 'flex', justifyContent: 'space-between', fontSize: 14 }}><span style={{ color: '#5E6862' }}>{ex}</span><b>{v}</b></div></div>)}
        </div>
      </section>

      <section id="regras" className="ml-w" style={{ paddingTop: 110 }}>
        <div className="ml-eyebrow">Regras</div>
        <h2 className="ml-h2">Claras para a instituição e para o cliente.</h2>
        <div className="ml-four">
          {RULES.map(([t, d], i) => <div key={t} style={{ background: i === 0 ? '#E2F3DE' : '#fff', borderRadius: 24, border: i === 0 ? 0 : '1px solid #E8E6DF', padding: 22 }}><b style={{ fontSize: 17 }}>{t}</b><div style={{ color: i === 0 ? '#0A4F06' : '#5E6862', fontSize: 14, lineHeight: 1.5, marginTop: 8 }}>{d}</div></div>)}
        </div>
      </section>

      <section className="ml-w" style={{ padding: '120px clamp(20px,4vw,48px) 80px', textAlign: 'center' }}>
        <h2 className="ml-h2" style={{ fontSize: 'clamp(40px,6.2vw,84px)', maxWidth: 820, margin: '0 auto' }}>Pedidos, acordos e prestações num só painel.</h2>
        <p className="ml-p" style={{ marginTop: 18 }}>Já tem conta RIHA? A equipa RIHA ativa o Microcrédito na sua conta depois de conhecer a sua instituição.</p>
        <div style={{ display: 'flex', flexWrap: 'wrap', justifyContent: 'center', gap: 12, marginTop: 26 }}>
          <a href={MAILTO} className="ml-btn" style={{ background: '#0A8205', color: '#fff', boxShadow: '0 14px 30px -12px rgba(10,130,5,.7)' }}>Falar com a equipa</a>
          <Link to="/registo" className="ml-btn" style={{ background: '#fff', border: '1px solid #E4E2DA', color: '#0B120E' }}>Criar conta RIHA</Link>
        </div>
      </section>

      <footer className="ml-w" style={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: '12px 22px', padding: '28px clamp(20px,4vw,48px) 40px', borderTop: '1px solid #E4E2DA', fontSize: 14, color: '#5E6862' }}>
        <span style={{ display: 'flex', alignItems: 'center', gap: 8, color: '#0B120E' }}><img src="/assets/riha-icon-dark.png" alt="" style={{ width: 22, height: 22 }} /><b style={{ letterSpacing: '.2em' }}>RIHA</b></span>
        <span>Maputo, Moçambique</span>
        <span style={{ flex: 1 }} />
        <Link to="/" style={{ color: 'inherit' }}>Pagamentos</Link>
        <Link to="/#assinaturas" style={{ color: 'inherit' }}>Assinaturas</Link>
        <a href={SUPPORT_MAILTO} style={{ color: 'inherit' }}>{SUPPORT_EMAIL}</a>
      </footer>
    </div>
  )
}
