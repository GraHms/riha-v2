import { Fragment, useEffect, useState, type CSSProperties, type ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { Icon } from '../../components/ui'
import { useGateway } from '../../core/gateway'
import { bpsLabel } from '../../lib/format'

const DISPLAY = "'Plus Jakarta Sans', sans-serif"
const MONO = "'Geist Mono', monospace"

const css = `
.docs { background: #F4F3EE; min-height: 100%; }
.docs-top { position: sticky; top: 0; z-index: 20; height: 64px; display: flex; align-items: center; gap: 20px; padding: 0 28px; background: #fff; border-bottom: 1px solid #E8E6DF; }
.docs-search { display: flex; align-items: center; gap: 10px; width: 360px; height: 38px; padding: 0 12px; border-radius: 10px; background: #F4F3EE; color: #8A938E; font-size: 13.5px; }
.docs-search input { flex: 1; min-width: 0; border: 0; background: transparent; font-size: 13.5px; color: #0B120E; }
.docs-search input::placeholder { color: #8A938E; }
.docs-menu-btn { display: none; }
.docs-layout { display: flex; align-items: flex-start; }
.docs-nav { width: 240px; flex: none; padding: 28px 20px; position: sticky; top: 64px; max-height: calc(100vh - 64px); overflow-y: auto; display: flex; flex-direction: column; gap: 22px; font-size: 14px; }
.docs-nav-group { display: flex; flex-direction: column; gap: 2px; }
.docs-nav-h { font-size: 11.5px; font-weight: 600; color: #5E6862; text-transform: uppercase; letter-spacing: .06em; margin-bottom: 6px; }
.docs-nav-item { display: block; width: 100%; text-align: left; border: 0; background: transparent; color: #3E4742; padding: 6px 10px; border-radius: 8px; font-size: 14px; }
.docs-nav-item:hover { color: #0B120E; }
.docs-nav-item.on { color: #0B120E; font-weight: 600; background: #EAE8E1; }
.docs-main { flex: 1; min-width: 0; border-left: 1px solid #E8E6DF; background: #fff; }
.docs-sec { display: grid; grid-template-columns: minmax(0,1fr) minmax(0,1fr); gap: 48px; padding: 48px; border-bottom: 1px solid #EEECE5; scroll-margin-top: 64px; }
.docs-sec.single { display: block; }
.docs-sec.last { border-bottom: 0; padding-bottom: 64px; }
.docs-anchor { scroll-margin-top: 84px; }
.docs-sticky { position: sticky; top: 84px; align-self: start; }
.docs-flow { display: grid; grid-template-columns: repeat(5, minmax(0,1fr)); gap: 10px; margin-top: 26px; }
.docs-2 { display: grid; grid-template-columns: minmax(0,1fr) minmax(0,1fr); gap: 10px; }
.docs-code { padding: 16px 18px; font-family: ${MONO}; font-size: 12px; line-height: 1.75; color: #C9D3CD; white-space: pre; overflow-x: auto; -webkit-overflow-scrolling: touch; }
.docs-tabs { display: flex; gap: 2px; font-family: ${MONO}; font-size: 11.5px; }
.docs-tabs button { padding: 4px 9px; border-radius: 7px; border: 0; background: transparent; color: #93A09A; font-family: inherit; font-size: inherit; }
.docs-tabs button:hover { color: #fff; }
.docs-tabs button.on { background: rgba(255,255,255,.14); color: #fff; }
.docs-row { display: grid; grid-template-columns: var(--w, 130px) minmax(0,1fr); gap: 12px; padding: 12px 16px; border-bottom: 1px solid #EEECE5; }
.docs-row:last-child { border-bottom: 0; }
.docs-retries { display: flex; gap: 4px; margin-top: 12px; }
.docs-scrim { display: none; }

@media (max-width: 1100px) {
  .docs-search { width: 240px; }
  .docs-sec { grid-template-columns: minmax(0,1fr); gap: 28px; padding: 40px 32px; }
  .docs-sticky { position: static; }
  .docs-flow { grid-template-columns: repeat(3, minmax(0,1fr)); }
}

@media (max-width: 860px) {
  .docs-top { padding: 0 16px; gap: 12px; }
  .docs-search, .docs-version, .docs-label { display: none !important; }
  .docs-menu-btn { display: grid; place-items: center; width: 38px; height: 38px; border-radius: 10px; border: 1px solid #E4E2DA; background: #fff; padding: 0; flex: none; }
  .docs-nav { display: none; position: fixed; top: 64px; left: 0; right: 0; bottom: 0; width: auto; max-height: none; z-index: 19; background: #F4F3EE; padding: 20px 16px 32px; border-top: 1px solid #E8E6DF; }
  .docs-nav.open { display: flex; }
  .docs-main { border-left: 0; }
  .docs-sec { padding: 32px 16px; gap: 24px; }
  .docs-flow { grid-template-columns: minmax(0,1fr); }
  .docs-2 { grid-template-columns: minmax(0,1fr); }
  .docs-row:not(.keep) { grid-template-columns: minmax(0,1fr); gap: 4px; }
  .docs-h1 { font-size: 34px !important; }
  .docs-h2 { font-size: 26px !important; }
  .docs-retries { flex-wrap: wrap; }
  .docs-retries span { flex: 1 0 22% !important; }
}
`

const COL: Record<string, string> = { y: '#FFD21F', g: '#8EE07F', b: '#7FB2FF', m: '#93A09A', r: '#FF8A8D' }
const TOKEN = /("(?:[^"\\\n]|\\.)*")|((?<=^|\s)(?:\/\/|#)[^\n]*)|\b(const|let|await|if|return|import|true|false|null)\b/gm

function highlight(src: string, json: boolean): ReactNode[] {
  const out: ReactNode[] = []
  let k = 0
  src.split(/«(.)([\s\S]*?)»/).forEach((part, i, arr) => {
    if (i % 3 === 1) return
    if (i % 3 === 2) { out.push(<span key={k++} style={{ color: COL[arr[i - 1]] }}>{part}</span>); return }
    let last = 0
    for (const m of part.matchAll(TOKEN)) {
      const at = m.index ?? 0
      if (at > last) out.push(part.slice(last, at))
      const [tok, str, comment, kw] = m
      let color: string | undefined
      if (str) {
        const after = part.slice(at + tok.length).match(/^\s*(:|=>)/)
        color = after ? COL.y : json ? (str.startsWith('"http') ? COL.g : undefined) : COL.g
      } else if (comment) color = COL.m
      else if (kw) color = json ? undefined : COL.b
      out.push(color ? <span key={k++} style={{ color }}>{tok}</span> : tok)
      last = at + tok.length
    }
    if (last < part.length) out.push(part.slice(last))
  })
  return out
}

function CodeCard({ title, right, rightColor = '#93A09A', code, json, deep, tabs, style }: {
  title: string; right?: string; rightColor?: string; code: string; json?: boolean; deep?: boolean
  tabs?: { list: string[]; on: string; set: (t: string) => void }; style?: CSSProperties
}) {
  return (
    <div style={{ background: deep ? '#151E19' : '#0B120E', border: deep ? '1px solid #23302A' : undefined, borderRadius: 18, overflow: 'hidden', minWidth: 0, ...style }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12, padding: tabs ? '10px 12px 10px 18px' : '10px 18px', minHeight: 41, borderBottom: '1px solid rgba(255,255,255,.08)' }}>
        <span style={{ fontSize: 12.5, fontWeight: 600, color: '#fff' }}>{title}</span>
        {tabs ? (
          <div className="docs-tabs" role="tablist">
            {tabs.list.map(t => <button key={t} role="tab" aria-selected={tabs.on === t} className={tabs.on === t ? 'on' : ''} onClick={() => tabs.set(t)}>{t}</button>)}
          </div>
        ) : right ? <span style={{ fontFamily: MONO, fontSize: 11.5, color: rightColor, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{right}</span> : null}
      </div>
      <div className="docs-code">{highlight(code, !!json)}</div>
    </div>
  )
}

const C = ({ children, s = 13.5 }: { children: ReactNode; s?: number }) => <span style={{ fontFamily: MONO, fontSize: s }}>{children}</span>
const Chip = ({ children }: { children: ReactNode }) => <span style={{ fontFamily: MONO, fontSize: 14, padding: '1px 6px', borderRadius: 5, background: '#F4F3EE' }}>{children}</span>
const H2 = ({ children, mt = 0 }: { children: ReactNode; mt?: number }) => <h2 className="docs-h2" style={{ margin: `${mt}px 0 0`, fontFamily: DISPLAY, fontWeight: 700, fontSize: 30, letterSpacing: '-0.03em' }}>{children}</h2>
const P = ({ children, mt = 10, s = 15.5, style }: { children: ReactNode; mt?: number; s?: number; style?: CSSProperties }) => <p style={{ margin: `${mt}px 0 0`, fontSize: s, lineHeight: 1.6, color: '#3E4742', ...style }}>{children}</p>
const Eyebrow = ({ children, id, m = '26px 0 10px' }: { children: ReactNode; id?: string; m?: string }) => <div id={id} className={id ? 'docs-anchor' : undefined} style={{ fontSize: 12, fontWeight: 600, color: '#5E6862', textTransform: 'uppercase', letterSpacing: '.06em', margin: m }}>{children}</div>
const Table = ({ children }: { children: ReactNode }) => <div style={{ border: '1px solid #E8E6DF', borderRadius: 14, overflow: 'hidden', fontSize: 14 }}>{children}</div>

function Row({ k, children, w }: { k: ReactNode; children: ReactNode; w?: number }) {
  return (
    <div className="docs-row" style={w ? ({ '--w': `${w}px` } as CSSProperties) : undefined}>
      <span style={{ fontFamily: MONO, fontSize: 13 }}>{k}</span>
      <span style={{ color: '#3E4742' }}>{children}</span>
    </div>
  )
}

function Param({ name, type, req, badge, hl, last, children }: { name: string; type: string; req?: boolean; badge?: string; hl?: boolean; last?: boolean; children: ReactNode }) {
  const st: CSSProperties = hl
    ? { padding: '14px 16px', margin: '0 -16px', borderTop: '1px solid #EEECE5', background: '#F3FAF1', borderRadius: 12 }
    : { padding: '14px 0', borderTop: '1px solid #EEECE5', borderBottom: last ? '1px solid #EEECE5' : undefined }
  return (
    <div style={st}>
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8, alignItems: 'baseline' }}>
        <span style={{ fontFamily: MONO, fontSize: 13.5, fontWeight: 500 }}>{name}</span>
        <span style={{ fontSize: 12.5, color: '#5E6862' }}>{type}</span>
        {req && <span style={{ fontSize: 11.5, fontWeight: 600, color: '#C2080D' }}>obrigatório</span>}
        {badge && <span style={{ fontSize: 11, fontWeight: 700, letterSpacing: '.03em', padding: '2px 6px', borderRadius: 5, background: '#0B120E', color: '#fff' }}>{badge}</span>}
      </div>
      <div style={{ fontSize: 14, lineHeight: 1.55, color: '#3E4742', marginTop: 4 }}>{children}</div>
    </div>
  )
}

function Endpoint({ method, path }: { method: string; path: string }) {
  const c: Record<string, [string, string]> = { POST: ['#E2F3DE', '#0A6B05'], GET: ['#E6EEFB', '#1F4FA0'], PATCH: ['#FFF3C4', '#7A5A00'], DELETE: ['#FFE3E1', '#C2080D'] }
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap' }}>
      <span style={{ fontFamily: MONO, fontSize: 12, fontWeight: 500, padding: '3px 8px', borderRadius: 6, background: c[method][0], color: c[method][1] }}>{method}</span>
      <span style={{ fontFamily: MONO, fontSize: 14, wordBreak: 'break-all' }}>{path}</span>
    </div>
  )
}

const MCOL: Record<string, string> = { POST: '#8EE07F', GET: '#7FB2FF', PATCH: '#FFD21F', DELETE: '#FF8A8D' }
const ENDPOINTS: [string, string][][] = [
  [['POST', '/v1/checkout/sessions'], ['GET', '/v1/checkout/sessions'], ['GET', '/v1/checkout/sessions/:id'], ['POST', '/v1/checkout/sessions/:id/expire']],
  [['GET', '/v1/payments'], ['GET', '/v1/payments/:id'], ['POST', '/v1/refunds']],
  [['GET', '/v1/payment_links'], ['POST', '/v1/payment_links'], ['PATCH', '/v1/payment_links/:id']],
  [['GET', '/v1/balance'], ['GET', '/v1/payouts'], ['POST', '/v1/payouts']],
  [['GET', '/v1/webhook_endpoints'], ['POST', '/v1/webhook_endpoints'], ['DELETE', '/v1/webhook_endpoints/:id'], ['GET', '/v1/events/:id']],
]

const SESSION_CODE: Record<string, string> = {
  cURL: String.raw`curl https://rihagateway.com/v1/checkout/sessions \
  -H "Authorization: Bearer sk_test_…" \
  -H "Idempotency-Key: pedido-2094-a1" \
  -H "Content-Type: application/json" \
  -d '{
    "amount": 125000,
    "currency": "MZN",
    "client_reference_id": "pedido_2094",
    "success_url": "https://loja.co.mz/ok?s={CHECKOUT_SESSION_ID}",
    "cancel_url": "https://loja.co.mz/carrinho",
    "callback_url": "https://loja.co.mz/webhooks/riha",
    "customer": { "name": "Ana Machava", "phone": "+258841234231" },
    "line_items": [
      { "name": "Bolo de chocolate 1 kg", "quantity": 1, "unit_amount": 125000 }
    ],
    "metadata": { "canal": "loja_online" }
  }'`,
  Node: String.raw`const res = await fetch("https://rihagateway.com/v1/checkout/sessions", {
  method: "POST",
  headers: {
    "Authorization": "Bearer " + process.env.RIHA_SECRET_KEY,
    "Idempotency-Key": "pedido-2094-a1",
    "Content-Type": "application/json",
  },
  body: JSON.stringify({
    amount: 125000,
    currency: "MZN",
    client_reference_id: "pedido_2094",
    success_url: "https://loja.co.mz/ok?s={CHECKOUT_SESSION_ID}",
    cancel_url: "https://loja.co.mz/carrinho",
    callback_url: "https://loja.co.mz/webhooks/riha",
    customer: { name: "Ana Machava", phone: "+258841234231" },
    line_items: [
      { name: "Bolo de chocolate 1 kg", quantity: 1, unit_amount: 125000 },
    ],
    metadata: { canal: "loja_online" },
  }),
});
const session = await res.json();
// Envie o cliente para session.url (303)
res.redirect(303, session.url);`,
  PHP: String.raw`<?php
$ch = curl_init("https://rihagateway.com/v1/checkout/sessions");
curl_setopt_array($ch, [
  CURLOPT_POST => true,
  CURLOPT_RETURNTRANSFER => true,
  CURLOPT_HTTPHEADER => [
    "Authorization: Bearer " . getenv("RIHA_SECRET_KEY"),
    "Idempotency-Key: pedido-2094-a1",
    "Content-Type: application/json",
  ],
  CURLOPT_POSTFIELDS => json_encode([
    "amount" => 125000,
    "currency" => "MZN",
    "client_reference_id" => "pedido_2094",
    "success_url" => "https://loja.co.mz/ok?s={CHECKOUT_SESSION_ID}",
    "cancel_url" => "https://loja.co.mz/carrinho",
    "callback_url" => "https://loja.co.mz/webhooks/riha",
    "customer" => ["name" => "Ana Machava", "phone" => "+258841234231"],
    "line_items" => [
      ["name" => "Bolo de chocolate 1 kg", "quantity" => 1, "unit_amount" => 125000],
    ],
    "metadata" => ["canal" => "loja_online"],
  ]),
]);
$session = json_decode(curl_exec($ch), true);
// Envie o cliente para o checkout
header("Location: " . $session["url"], true, 303);`,
  Python: String.raw`import os
import requests

r = requests.post(
    "https://rihagateway.com/v1/checkout/sessions",
    headers={
        "Authorization": "Bearer " + os.environ["RIHA_SECRET_KEY"],
        "Idempotency-Key": "pedido-2094-a1",
    },
    json={
        "amount": 125000,
        "currency": "MZN",
        "client_reference_id": "pedido_2094",
        "success_url": "https://loja.co.mz/ok?s={CHECKOUT_SESSION_ID}",
        "cancel_url": "https://loja.co.mz/carrinho",
        "callback_url": "https://loja.co.mz/webhooks/riha",
        "customer": {"name": "Ana Machava", "phone": "+258841234231"},
        "line_items": [
            {"name": "Bolo de chocolate 1 kg", "quantity": 1, "unit_amount": 125000},
        ],
        "metadata": {"canal": "loja_online"},
    },
    timeout=30,
)
r.raise_for_status()
session = r.json()
# Envie o cliente para session["url"] (303)`,
}

const SESSION_RES = `{
  "id": "cs_test_8K2D4410",
  "object": "checkout.session",
  "status": "open",
  "amount": 125000,
  "currency": "MZN",
  "url": "https://pay.rihagateway.com/c/cs_test_8K2D4410",
  "client_reference_id": "pedido_2094",
  "callback_url": "https://loja.co.mz/webhooks/riha",
  "payment": null,
  "expires_at": 1793741400,
  "livemode": false
}`

const AUTH_CODE = String.raw`# Bearer
curl https://rihagateway.com/v1/balance \
  -H "Authorization: Bearer sk_test_51RhA…V71c"

# Basic auth: a chave é o utilizador, sem palavra-passe
curl https://rihagateway.com/v1/balance \
  -u «ysk_test_51RhA…V71c»:

# Corpo form-encoded (o mesmo que JSON)
curl https://rihagateway.com/v1/checkout \
  -u «ysk_test_…»: \
  -d amount=125000 \
  -d currency=MZN \
  -d success_url=«ghttps://loja.co.mz/ok»`

const paymentJson = (fee: number) => `{
  "id": "pay_7Q1A2093",
  "object": "payment",
  "status": "succeeded",
  "amount": 348000,
  "fee": ${fee},
  "net": ${348000 - fee},
  "payment_method": {
    "type": "mpesa",
    "mpesa": { "phone_last4": "4231", "operator_reference": "QK72H1LZ9P" }
  },
  «g"source"»: {
    "type": "api_checkout",
    "checkout_session": "cs_live_7Q1A2093",
    "api_key": "key_3f9a",
    "origin": {
      "domain": "loja.cafemaputo.co.mz",
      "ip": "41.220.12.88",
      "user_agent": "WooCommerce/8.9 riha-woo/1.2.0"
    }
  },
  "client_reference_id": "pedido_2093",
  "metadata": { "canal": "loja_online" },
  "livemode": true
}`

const SOURCE_LINK = `«g"source"»: {
  "type": "payment_link",
  "payment_link": "plink_bolo_1kg",
  "created_by": "usr_ana_mabota",
  "created_via": "dashboard",
  "channel": "whatsapp"
}`

const LINK_CODE = String.raw`curl https://rihagateway.com/v1/payment_links \
  -u «ysk_test_…»: \
  -d name="Bolo de chocolate 1 kg" \
  -d amount=125000 \
  -d details="Feito no dia. Entrega na Matola." \
  -d redirect_url="https://cafemaputo.co.mz/obrigado"`

const LINK_RES = `{
  "id": "plink_bolo_1kg",
  "object": "payment_link",
  "name": "Bolo de chocolate 1 kg",
  "slug": "bolo",
  "amount": 125000,
  "custom_amount": false,
  "reusable": true,
  "payment_method_types": ["mpesa"],
  "active": true,
  "expires_at": null,
  "details": "Feito no dia. Entrega na Matola.",
  "suggested_amount": null,
  "min_amount": null,
  "max_amount": null,
  "after_message": "",
  "redirect_url": "https://cafemaputo.co.mz/obrigado",
  "url": "https://pay.rihagateway.com/p/cafe-maputo/bolo",
  "livemode": false
}`

const LINK_CUSTOM_CODE = String.raw`curl https://rihagateway.com/v1/payment_links \
  -u «ysk_test_…»: \
  -d name="Donativo para a biblioteca" \
  -d custom_amount=true \
  -d suggested_amount=50000 \
  -d min_amount=10000 \
  -d max_amount=500000 \
  -d after_message="Obrigado pelo seu donativo!"`

const REFUND_CODE = String.raw`curl https://rihagateway.com/v1/refunds \
  -u «ysk_test_…»: \
  -H "Idempotency-Key: reemb-2093-1" \
  -d payment=pay_7Q1A2093 \
  -d amount=50000 \
  -d reason="Produto em falta"`

const MONEY_CODE = String.raw`curl https://rihagateway.com/v1/balance -u «ysk_test_…»:

{
  "object": "balance",
  "available": 4832050,
  "in_transit": 1500000,
  "currency": "MZN",
  "livemode": false
}`

const PAYOUT_RES = `{
  "id": "po_3M8K1C",
  "object": "payout",
  "status": "new",
  "amount": 2000000,
  "currency": "MZN",
  "bank_account": { "bank": "BCI", "holder": "Café Maputo, Lda", "nib": "•••• 01135" },
  "requested_at": 1793738412,
  "due_at": 1793997612,
  "paid_at": null,
  "livemode": false
}`

const EVENT_CODE = `«mRiha-Signature: t=1793738412,v1=5f2b…c91e»

{
  "id": "evt_1PZ8Rk2093",
  "type": "checkout.session.completed",
  "created": 1793738412,
  "livemode": true,
  "delivery": { "target": "callback_url", "attempt": 1 },
  "data": {
    "object": {
      "id": "cs_live_7Q1A2093",
      "status": "complete",
      "client_reference_id": "pedido_2093",
      "payment": "pay_7Q1A2093",
      "amount": 348000,
      "metadata": { "canal": "loja_online" }
    }
  }
}`

const VERIFY_CODE = String.raw`import crypto from "node:crypto";

// rawBody: o corpo exatamente como chegou (Buffer)
const header = req.headers["riha-signature"];
const { t, v1 } = Object.fromEntries(header.split(",").map(p => p.split("=")));
const expected = crypto
  .createHmac("sha256", process.env.RIHA_WEBHOOK_SECRET)
  .update(t + "." + rawBody)
  .digest("hex");
const ok = v1.length === expected.length &&
  crypto.timingSafeEqual(Buffer.from(v1), Buffer.from(expected));
if (!ok || Date.now() / 1000 - Number(t) > 300) return res.sendStatus(400);

const event = JSON.parse(rawBody);
if (event.type === "checkout.session.completed") {
  await marcarComoPago(event.data.object.client_reference_id);
}
res.sendStatus(200);`

const ERROR_RES = `{
  "error": {
    "type": "invalid_request_error",
    "code": "callback_url_not_allowed",
    "param": "callback_url",
    "message": "O domínio loja-teste.com não está verificado.",
    "doc_url": "https://rihagateway.com/docs#erros",
    "request_id": "req_9fK2mA81"
  }
}`

const NAV: { h: string; items: [string, string][] }[] = [
  { h: 'Começar', items: [['Introdução', 'intro'], ['Modo de teste', 'teste'], ['Fluxo de pagamento', 'fluxo'], ['Autenticação', 'auth'], ['Idempotência', 'idempotencia'], ['Erros', 'erros']] },
  { h: 'Pagamentos', items: [['Checkout Sessions', 'sessao'], ['Payments', 'payment'], ['Payment Links', 'links'], ['Refunds', 'refunds']] },
  { h: 'Dinheiro', items: [['Balance', 'balance'], ['Payouts', 'payouts']] },
  { h: 'Webhooks', items: [['Endpoints e callback_url', 'webhooks'], ['Eventos', 'eventos'], ['Assinatura e tentativas', 'assinatura']] },
]
const ALL_IDS = NAV.flatMap(g => g.items.map(i => i[1]))

function useScrollSpy() {
  const [active, setActive] = useState('intro')
  useEffect(() => {
    let raf = 0
    const run = () => {
      raf = 0
      const els = ALL_IDS.map(id => document.getElementById(id)).filter((e): e is HTMLElement => !!e)
        .sort((a, b) => a.getBoundingClientRect().top - b.getBoundingClientRect().top)
      let cur = els[0]?.id || 'intro'
      for (const e of els) if (e.getBoundingClientRect().top <= 120) cur = e.id
      if (window.innerHeight + window.scrollY >= document.documentElement.scrollHeight - 4) cur = els[els.length - 1]?.id || cur
      setActive(cur)
    }
    const on = () => { if (!raf) raf = requestAnimationFrame(run) }
    run()
    window.addEventListener('scroll', on, { passive: true })
    window.addEventListener('resize', on)
    return () => { window.removeEventListener('scroll', on); window.removeEventListener('resize', on); if (raf) cancelAnimationFrame(raf) }
  }, [])
  return [active, setActive] as const
}

const FAILS: [string, string, string][] = [
  ['0000', 'pin_incorrect', 'O cliente errou o PIN'],
  ['1111', 'insufficient_funds', 'Saldo insuficiente na carteira'],
  ['2222', 'customer_timeout', 'O cliente não respondeu a tempo'],
  ['3333', 'customer_cancelled', 'O cliente cancelou no telemóvel'],
  ['4444', 'operator_unavailable', 'Operador indisponível (falha imediata)'],
]

const FLOW = [
  { n: '01 · Servidor', t: 'Cria a sessão', d: 'POST /v1/checkout/sessions → url', mono: true },
  { n: '02 · Browser', t: 'Redireciona o cliente', d: '303 → pay.rihagateway.com/c/cs_…', mono: true },
  { n: '03 · RIHA', t: 'O cliente paga', d: 'Confirma com o PIN no telemóvel', mono: false, green: true },
  { n: '04 · Servidor a servidor', t: 'Evento assinado', d: 'POST callback_url + endpoints', mono: true, dark: true },
  { n: '05 · Browser', t: 'Volta à loja', d: 'success_url?session_id=cs_…', mono: true },
]

const EVENTS = ['checkout.session.completed', 'checkout.session.expired', 'payment.processing', 'payment.succeeded', 'payment.failed', 'refund.succeeded', 'payout.created', 'payout.paid', 'payout.failed', 'account.updated']
const RETRIES = ['0', '5 min', '30 min', '2 h', '5 h', '10 h', '24 h', '72 h']

export default function Docs() {
  const g = useGateway()
  const [lang, setLang] = useState('cURL')
  const [menu, setMenu] = useState(false)
  const [active, setActive] = useScrollSpy()
  const [bps, setBps] = useState(600)

  useEffect(() => {
    let alive = true
    g.checkout.pricing()
      .then(d => { if (alive && typeof d?.default_fee_bps === 'number') setBps(d.default_fee_bps) })
      .catch(() => {})
    return () => { alive = false }
  }, [])

  useEffect(() => {
    document.body.style.overflow = menu ? 'hidden' : ''
    return () => { document.body.style.overflow = '' }
  }, [menu])

  const go = (id: string) => {
    setMenu(false)
    setActive(id)
    requestAnimationFrame(() => document.getElementById(id)?.scrollIntoView({ behavior: 'smooth', block: 'start' }))
    history.replaceState(history.state, '', '#' + id)
  }

  useEffect(() => {
    const id = decodeURIComponent(location.hash.slice(1))
    if (id) requestAnimationFrame(() => document.getElementById(id)?.scrollIntoView({ block: 'start' }))
  }, [])

  const fee = Math.floor((348000 * bps + 5000) / 10000)

  return (
    <div className="docs">
      <style>{css}</style>

      <header className="docs-top">
        <button className="docs-menu-btn" aria-label={menu ? 'Fechar menu' : 'Abrir menu'} aria-expanded={menu} onClick={() => setMenu(m => !m)}>
          {menu ? <Icon name="close" size={17} stroke={2} /> : (
            <svg width="17" height="17" viewBox="0 0 24 24" aria-hidden="true" style={{ fill: 'none', stroke: 'currentColor', strokeWidth: 2, strokeLinecap: 'round' }}><path d="M4 7h16M4 12h16M4 17h16" /></svg>
          )}
        </button>
        <Link to="/" style={{ display: 'flex', alignItems: 'center', gap: 9, color: '#0B120E', minWidth: 0 }}>
          <img src="/assets/riha-icon-dark.png" alt="RIHA" style={{ width: 28, height: 28, objectFit: 'contain' }} />
          <span style={{ fontFamily: DISPLAY, fontWeight: 800, fontSize: 16, letterSpacing: '.2em' }}>RIHA</span>
          <span className="docs-label" style={{ fontSize: 14, color: '#5E6862', paddingLeft: 10, borderLeft: '1px solid #E4E2DA' }}>Documentação</span>
        </Link>
        <label className="docs-search">
          <Icon name="search" size={15} stroke={2} />
          <input type="search" placeholder="Pesquisar na documentação" aria-label="Pesquisar na documentação" />
          <span style={{ fontFamily: MONO, fontSize: 11, padding: '2px 6px', borderRadius: 5, background: '#fff' }}>/</span>
        </label>
        <div style={{ flex: 1 }} />
        <span className="docs-version" style={{ fontFamily: MONO, fontSize: 12, padding: '5px 10px', borderRadius: 8, background: '#F4F3EE', whiteSpace: 'nowrap' }}>API v1</span>
        <Link to="/painel" style={{ height: 38, padding: '0 14px', borderRadius: 10, background: '#0B120E', color: '#fff', fontWeight: 600, fontSize: 14, display: 'flex', alignItems: 'center', whiteSpace: 'nowrap', flex: 'none' }}>Ir para o painel</Link>
      </header>

      <div className="docs-layout">
        <nav className={'docs-nav' + (menu ? ' open' : '')} aria-label="Secções">
          {NAV.map(g => (
            <div key={g.h} className="docs-nav-group">
              <div className="docs-nav-h">{g.h}</div>
              {g.items.map(([label, id]) => (
                <button key={label} className={'docs-nav-item' + (active === id ? ' on' : '')} onClick={() => go(id)}>{label}</button>
              ))}
            </div>
          ))}
        </nav>

        <main className="docs-main">
          <section id="intro" className="docs-sec">
            <div>
              <h1 className="docs-h1" style={{ margin: 0, fontFamily: DISPLAY, fontWeight: 700, fontSize: 44, letterSpacing: '-0.035em', lineHeight: 1.02 }}>Referência da API</h1>
              <P mt={16} s={16} style={{ lineHeight: 1.65, textWrap: 'pretty' }}>A API da RIHA é REST. Tem URLs organizados por recurso, recebe JSON ou dados form-encoded, devolve JSON, usa códigos HTTP padrão e autentica com a sua chave secreta. Todos os valores são inteiros em centavos de metical: 125000 corresponde a 1 250,00 MT.</P>
              <P mt={12} s={16} style={{ lineHeight: 1.65, textWrap: 'pretty' }}>O modo de teste usa as chaves <Chip>sk_test_</Chip>. Nenhum dinheiro real é movimentado e os <a href="#teste" onClick={e => { e.preventDefault(); go('teste') }}>números de teste</a> simulam cada resultado (sucesso, PIN errado, saldo insuficiente, tempo esgotado).</P>
              <div className="docs-2" style={{ marginTop: 24 }}>
                <div style={{ padding: '14px 16px', borderRadius: 14, border: '1px solid #E8E6DF', minWidth: 0 }}><div style={{ fontSize: 12, color: '#5E6862' }}>URL base</div><div style={{ fontFamily: MONO, fontSize: 13.5, marginTop: 4, wordBreak: 'break-all' }}>https://rihagateway.com/v1</div></div>
                <div style={{ padding: '14px 16px', borderRadius: 14, border: '1px solid #E8E6DF', minWidth: 0 }}><div style={{ fontSize: 12, color: '#5E6862' }}>Formato</div><div style={{ fontFamily: MONO, fontSize: 13.5, marginTop: 4 }}>application/json · UTF-8</div></div>
              </div>
            </div>
            <div style={{ background: '#0B120E', color: '#C9D3CD', borderRadius: 18, overflow: 'hidden', alignSelf: 'start', minWidth: 0 }}>
              <div style={{ padding: '12px 18px', borderBottom: '1px solid rgba(255,255,255,.08)', fontSize: 12.5, fontWeight: 600, color: '#fff' }}>Endpoints</div>
              <div style={{ padding: '10px 18px 16px', fontFamily: MONO, fontSize: 12.5, lineHeight: 2, overflowX: 'auto' }}>
                {ENDPOINTS.map((grp, gi) => (
                  <Fragment key={gi}>
                    {gi > 0 && <div style={{ height: 8 }} />}
                    {grp.map(([m, p]) => (
                      <div key={m + p} style={{ display: 'flex', gap: 12, whiteSpace: 'nowrap' }}><span style={{ width: 52, flex: 'none', color: MCOL[m] }}>{m}</span><span>{p}</span></div>
                    ))}
                  </Fragment>
                ))}
              </div>
            </div>
          </section>

          <section id="teste" className="docs-sec">
            <div>
              <H2>Modo de teste</H2>
              <P>Com uma chave <C>sk_test_</C> (ou o painel em modo de teste) os pagamentos são simulados. Em produção, os pagamentos são reais. No M-Pesa, o resultado depende dos últimos quatro dígitos do telefone (84, 85) usado no checkout.</P>
              <Eyebrow>M-Pesa</Eyebrow>
              <Table>
                {FAILS.map(([d, code, t]) => <Row key={d} k={<>84 123 <b style={{ fontWeight: 600 }}>{d}</b></>} w={150}><C s={13}>{code}</C> · {t}</Row>)}
                <Row k="84 123 4567" w={150}><C s={13}>succeeded</C> · Qualquer outro número é pago com sucesso</Row>
              </Table>
              <P mt={14} s={14.5}>Os pedidos de PIN em teste respondem em poucos segundos, como numa carteira real. Os eventos e o <C s={13}>callback_url</C> são enviados da mesma forma que em produção.</P>
            </div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 14, alignSelf: 'start', minWidth: 0 }}>
              <div style={{ background: '#0B120E', color: '#C9D3CD', borderRadius: 18, overflow: 'hidden' }}>
                <div style={{ padding: '12px 18px', borderBottom: '1px solid rgba(255,255,255,.08)', fontSize: 12.5, fontWeight: 600, color: '#fff' }}>Cartões de teste</div>
                <div style={{ padding: '6px 18px 12px', fontSize: 13.5 }}>
                  {[['4242 4242 4242 4242', 'succeeded', '#8EE07F'], ['4000 0000 0000 0002', 'card_declined', '#FF8A8D'], ['4000 0000 0000 9995', 'insufficient_funds', '#FF8A8D']].map(([n, r, c]) => (
                    <div key={n} style={{ display: 'flex', flexWrap: 'wrap', justifyContent: 'space-between', gap: '4px 16px', padding: '10px 0', borderBottom: '1px solid rgba(255,255,255,.08)' }}>
                      <span style={{ fontFamily: MONO, color: '#fff', whiteSpace: 'nowrap' }}>{n}</span>
                      <span style={{ fontFamily: MONO, fontSize: 12.5, color: c }}>{r}</span>
                    </div>
                  ))}
                  <div style={{ paddingTop: 10, fontSize: 12.5, color: '#93A09A', lineHeight: 1.55 }}>Use qualquer validade futura (MM / AA) e qualquer CVV de 3 dígitos. Outros números válidos também são aprovados.</div>
                </div>
              </div>
              <div style={{ display: 'flex', gap: 12, padding: '14px 16px', borderRadius: 14, background: '#FFF6D1', fontSize: 14, lineHeight: 1.55, color: '#3B3410' }}>
                <Icon name="alert" size={18} stroke={2} style={{ marginTop: 2 }} />
                <span>Os dados de teste e de produção estão separados. Um objeto criado com <C s={13}>sk_test_</C> não existe em produção.</span>
              </div>
            </div>
          </section>

          <section id="fluxo" className="docs-sec single">
            <H2>Fluxo de pagamento</H2>
            <P style={{ maxWidth: 720 }}>O servidor da loja cria uma sessão e envia o cliente para o checkout da RIHA. Depois do pagamento, a RIHA avisa o servidor e devolve o cliente à loja. Os dois passos são independentes.</P>
            <div className="docs-flow">
              {FLOW.map(f => (
                <div key={f.n} style={{ padding: 16, borderRadius: 16, minWidth: 0, ...(f.green ? { background: '#0A8205', color: '#fff' } : f.dark ? { background: '#0B120E', color: '#fff' } : { border: '1px solid #E8E6DF', background: '#FAFAF7' }) }}>
                  <div style={{ fontFamily: MONO, fontSize: 12, color: f.green ? '#CDEBC7' : f.dark ? '#93A09A' : '#5E6862' }}>{f.n}</div>
                  <div style={{ fontWeight: 600, fontSize: 14.5, marginTop: 10 }}>{f.t}</div>
                  <div style={{ fontFamily: f.mono ? MONO : undefined, fontSize: f.mono ? 11.5 : 12.5, color: f.green ? '#E3F4DF' : f.dark ? '#C9D3CD' : '#3E4742', marginTop: 6, lineHeight: 1.5, overflowWrap: 'anywhere' }}>{f.d}</div>
                </div>
              ))}
            </div>
            <div style={{ display: 'flex', gap: 12, marginTop: 16, padding: '14px 16px', borderRadius: 14, background: '#FFF6D1', fontSize: 14, lineHeight: 1.55, color: '#3B3410', maxWidth: 860 }}>
              <Icon name="alert" size={18} stroke={2} style={{ marginTop: 2 }} />
              <span>Não confirme uma encomenda só porque o cliente voltou ao success_url. O cliente pode fechar o browser ou perder a rede depois de pagar. Confirme pelo evento <b style={{ fontFamily: MONO, fontWeight: 500 }}>checkout.session.completed</b> ou com <b style={{ fontFamily: MONO, fontWeight: 500 }}>GET /v1/checkout/sessions/:id</b>.</span>
            </div>
          </section>

          <section id="auth" className="docs-sec">
            <div>
              <H2>Autenticação e idempotência</H2>
              <P>Envie a chave no cabeçalho <C>Authorization: Bearer sk_…</C>, ou por basic auth com a chave como utilizador e a palavra-passe vazia (<C>-u sk_…:</C> no curl). As chaves secretas ficam só no servidor.</P>
              <div style={{ marginTop: 18 }}>
                <Table>
                  <Row k="pk_live_…">Pública. Só identifica a conta no browser e não cria pagamentos.</Row>
                  <Row k="sk_live_…">Secreta. Acesso total à API.</Row>
                </Table>
              </div>
              <P mt={18}>O corpo dos pedidos POST e PATCH pode ser JSON (<C>application/json</C>) ou form-encoded (<C>application/x-www-form-urlencoded</C>, como <C>curl -d amount=125000</C>). As respostas são sempre JSON.</P>
              <P mt={18}><span id="idempotencia" className="docs-anchor" />Envie <C>Idempotency-Key</C> em todos os POST. Se a rede falhar e repetir o pedido com a mesma chave durante 24 horas, recebe a mesma resposta e não cria uma segunda cobrança.</P>
            </div>
            <CodeCard title="Pedido autenticado" code={AUTH_CODE} style={{ alignSelf: 'start' }} />
          </section>

          <section id="sessao" className="docs-sec">
            <div>
              <Endpoint method="POST" path="/v1/checkout/sessions" />
              <H2 mt={12}>Criar uma sessão de checkout</H2>
              <P>Devolve um <C>url</C> para onde envia o cliente. A sessão expira se não for paga.</P>
              <Eyebrow m="26px 0 4px">Parâmetros</Eyebrow>
              <Param name="amount" type="integer" req>Valor em centavos. Mínimo 1000 (10,00 MT).</Param>
              <Param name="currency" type="string" req>Só <C s={13}>MZN</C>.</Param>
              <Param name="success_url" type="string" req>Para onde o cliente volta depois de pagar. Use <C s={13}>{'{CHECKOUT_SESSION_ID}'}</C> para receber o ID da sessão no URL.</Param>
              <Param name="cancel_url" type="string">Para onde o cliente volta se desistir ou se a sessão expirar.</Param>
              <Param name="callback_url" type="string" badge="WEBHOOK DINÂMICO" hl>Endereço do seu servidor que recebe o resultado desta sessão: <C s={12.5}>checkout.session.completed</C>, <C s={12.5}>payment.succeeded</C>, <C s={12.5}>payment.failed</C> e <C s={12.5}>checkout.session.expired</C>. É enviado além dos endpoints fixos da conta. O domínio tem de estar verificado em Definições.</Param>
              <Param name="client_reference_id" type="string">O seu identificador interno, como o número da encomenda. Volta em todos os eventos e aparece no painel.</Param>
              <Param name="customer" type="object"><C s={13}>name</C>, <C s={13}>phone</C> (formato E.164, +258…) e <C s={13}>email</C>. O telefone pré-preenche o número M-Pesa no checkout.</Param>
              <Param name="line_items" type="array">Lista de <C s={13}>name</C>, <C s={13}>quantity</C> e <C s={13}>unit_amount</C>, mostrada no checkout e no recibo. A soma tem de ser igual a amount.</Param>
              <Param name="payment_method_types" type="array"><C s={13}>mpesa</C>, <C s={13}>emola</C> ou <C s={13}>bim</C>. Métodos ainda não ativos são ignorados, e <C s={13}>card</C> chega em breve.</Param>
              <Param name="metadata" type="object">Até 20 pares chave-valor (chave até 40 caracteres, valor até 500). Não é mostrado ao cliente. Copiado para o Payment.</Param>
              <Param name="expires_at" type="timestamp">Entre 15 minutos e 24 horas. Por omissão, 1 hora.</Param>
              <Param name="description · locale" type="string · pt-MZ | en" last>Texto mostrado no checkout e no recibo. Idioma do checkout.</Param>
              <P mt={18} s={14.5}>Para consultar, use <C s={13}>GET /v1/checkout/sessions/:id</C>; para listar, <C s={13}>GET /v1/checkout/sessions</C>; para cancelar uma sessão aberta, <C s={13}>POST /v1/checkout/sessions/:id/expire</C>.</P>
            </div>
            <div className="docs-sticky" style={{ display: 'flex', flexDirection: 'column', gap: 14, minWidth: 0 }}>
              <CodeCard title="Pedido" code={SESSION_CODE[lang]} tabs={{ list: Object.keys(SESSION_CODE), on: lang, set: setLang }} />
              <CodeCard title="Resposta" right="200 OK" rightColor="#8EE07F" code={SESSION_RES} json deep />
            </div>
          </section>

          <section id="payment" className="docs-sec">
            <div>
              <H2>O objeto Payment</H2>
              <P>Cada tentativa de pagamento cria um Payment. O campo <C>source</C> diz de onde veio, para separar vendas da loja online, links manuais e QR no balcão.</P>
              <Eyebrow>source.type</Eyebrow>
              <Table>
                <Row k="api_checkout" w={150}>Sessão criada pela API. Inclui a chave usada (só o ID), o domínio, o IP do pedido e a sessão.</Row>
                <Row k="payment_link" w={150}>Link criado no painel ou pela API. Inclui quem o criou e o canal (whatsapp, sms, qr, outro) se o link foi partilhado pelo painel.</Row>
                <Row k="qr" w={150}>QR estático impresso ou mostrado no balcão.</Row>
                <Row k="dashboard" w={150}>Cobrança enviada a partir do painel para o telemóvel do cliente.</Row>
              </Table>
              <Eyebrow>status</Eyebrow>
              <div style={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 6, fontFamily: MONO, fontSize: 12.5 }}>
                <span style={{ padding: '4px 9px', borderRadius: 7, background: '#FFF3C4', color: '#7A5A00' }}>processing</span><Icon name="arrowRight" size={14} stroke={2.2} color="#8A938E" />
                <span style={{ padding: '4px 9px', borderRadius: 7, background: '#E2F3DE', color: '#0A6B05' }}>succeeded</span><span style={{ color: '#8A938E' }}>ou</span>
                <span style={{ padding: '4px 9px', borderRadius: 7, background: '#FFE3E1', color: '#C2080D' }}>failed</span><Icon name="arrowRight" size={14} stroke={2.2} color="#8A938E" />
                <span style={{ padding: '4px 9px', borderRadius: 7, background: '#EDECE6', color: '#3E4742' }}>refunded</span><span style={{ padding: '4px 9px', borderRadius: 7, background: '#EDECE6', color: '#3E4742' }}>partially_refunded</span>
              </div>
              <div style={{ fontSize: 14, lineHeight: 1.55, color: '#3E4742', marginTop: 12 }}>Em <C s={13}>failed</C>, o campo <C s={13}>failure_code</C> indica o motivo: <C s={12.5}>pin_incorrect</C>, <C s={12.5}>insufficient_funds</C>, <C s={12.5}>customer_timeout</C>, <C s={12.5}>customer_cancelled</C>, <C s={12.5}>card_declined</C>, <C s={12.5}>operator_unavailable</C>.</div>
              <P mt={14} s={14}><C s={13}>fee</C> é a taxa RIHA ({bpsLabel(bps)} do <C s={13}>amount</C>, arredondada ao centavo) e <C s={13}>net</C> o valor creditado no saldo.</P>
              <P mt={10} s={14}>Liste com <C s={13}>GET /v1/payments</C> (os mais recentes primeiro; <C s={13}>?limit=</C> de 1 a 100, por omissão 10). As listas devolvem <C s={13}>{'{ "object": "list", "data": [...], "has_more": false }'}</C>.</P>
            </div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 14, alignSelf: 'start', minWidth: 0 }}>
              <CodeCard title="Payment · via API" right="GET /v1/payments/pay_7Q1A" code={paymentJson(fee)} json />
              <CodeCard title="source · link manual" code={SOURCE_LINK} json deep />
            </div>
          </section>

          <section id="links" className="docs-sec">
            <div>
              <Endpoint method="POST" path="/v1/payment_links" />
              <H2 mt={12}>Payment Links</H2>
              <P>Um link de pagamento é uma página pronta em <C>pay.rihagateway.com/p/…</C> que pode partilhar por WhatsApp, SMS ou QR, sem escrever código. Os links criados pela API aparecem no painel.</P>
              <Eyebrow m="26px 0 4px">Parâmetros</Eyebrow>
              <Param name="name" type="string" req>Nome mostrado ao cliente e no painel.</Param>
              <Param name="amount" type="integer">Valor fixo em centavos. Omita e envie <C s={13}>custom_amount=true</C> para o cliente escolher o valor.</Param>
              <Param name="reusable" type="boolean">Se o link aceita vários pagamentos. Por omissão, sim.</Param>
              <Param name="payment_method_types" type="array">Métodos aceites. Por omissão, todos os ativos na conta.</Param>
              <Param name="slug · expires_at" type="string · timestamp">Fim do URL e data a partir da qual o link deixa de aceitar pagamentos.</Param>
              <Param name="details" type="string">Descrição mais longa, até 500 caracteres, mostrada na página do link e no checkout. As quebras de linha mantêm-se.</Param>
              <Param name="suggested_amount · min_amount · max_amount" type="integer">Só com <C s={13}>custom_amount=true</C>, em centavos. O sugerido aparece já escrito; o checkout recusa valores fora do mínimo e do máximo. Sem eles, vale o intervalo da conta, de 10 a 1 000 000 MT. Num link de valor fixo são ignorados.</Param>
              <Param name="after_message" type="string">Mensagem mostrada ao cliente na confirmação, depois de pagar. Até 300 caracteres.</Param>
              <Param name="redirect_url" type="string" last>Para onde o cliente vai depois de pagar, em vez do endereço de retorno da conta. Em produção tem de usar https. Use <C s={13}>after_message</C> ou <C s={13}>redirect_url</C>, não os dois.</Param>
              <P mt={18} s={14.5}>Liste com <C s={13}>GET /v1/payment_links</C>. Altere ou desative com <C s={13}>PATCH /v1/payment_links/:id</C> (por exemplo <C s={13}>active=false</C>). Para limpar um texto, envie-o vazio; para limpar um valor, envie <C s={13}>0</C>. A imagem do link escolhe-se no painel.</P>
            </div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 14, alignSelf: 'start', minWidth: 0 }}>
              <CodeCard title="Criar um link" code={LINK_CODE} />
              <CodeCard title="Resposta" right="200 OK" rightColor="#8EE07F" code={LINK_RES} json deep />
              <CodeCard title="O cliente escolhe o valor, dentro de limites" code={LINK_CUSTOM_CODE} />
            </div>
          </section>

          <section id="refunds" className="docs-sec">
            <div>
              <Endpoint method="POST" path="/v1/refunds" />
              <H2 mt={12}>Refunds</H2>
              <P>Devolve o dinheiro de um Payment com <C>succeeded</C>, no todo ou em parte. O valor sai do seu saldo RIHA e o Payment passa a <C>refunded</C> ou <C>partially_refunded</C>.</P>
              <P><b>Só em modo de teste, por agora.</b> Em produção devolve <C>refunds_unavailable</C>: os reembolsos reais chegam em breve.</P>
              <Eyebrow m="26px 0 4px">Parâmetros</Eyebrow>
              <Param name="payment" type="string" req>ID do Payment, por exemplo <C s={13}>pay_7Q1A2093</C>.</Param>
              <Param name="amount" type="integer">Valor em centavos. Por omissão, o que falta reembolsar.</Param>
              <Param name="reason" type="string" last>Motivo interno, visível no painel.</Param>
            </div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 14, alignSelf: 'start', minWidth: 0 }}>
              <CodeCard title="Reembolso parcial" code={REFUND_CODE} />
              <CodeCard title="Evento" right="refund.succeeded" code={`{\n  "id": "re_5H2K9A",\n  "object": "refund",\n  "payment": "pay_7Q1A2093",\n  "amount": 50000,\n  "currency": "MZN",\n  "reason": "Produto em falta",\n  "status": "succeeded",\n  "livemode": false\n}`} json deep />
            </div>
          </section>

          <section id="balance" className="docs-sec">
            <div>
              <Endpoint method="GET" path="/v1/balance" />
              <H2 mt={12}>Saldo e levantamentos</H2>
              <P><C>available</C> é o que pode levantar agora; <C>in_transit</C> são levantamentos pedidos que ainda não chegaram ao banco. Valores em centavos.</P>
              <Eyebrow id="payouts">Payouts</Eyebrow>
              <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                <Endpoint method="POST" path="/v1/payouts" />
                <Endpoint method="GET" path="/v1/payouts" />
              </div>
              <P mt={14} s={14.5}>Envie <C s={13}>amount</C> em centavos para pedir um levantamento para a conta bancária verificada. O dinheiro chega em até 72 horas (<C s={13}>due_at</C>). Estados: <C s={12.5}>new</C>, <C s={12.5}>review</C>, <C s={12.5}>sent</C>, <C s={12.5}>paid</C>, <C s={12.5}>rejected</C> e <C s={12.5}>failed</C>. Os eventos <C s={12.5}>payout.*</C> chegam aos endpoints da conta.</P>
            </div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 14, alignSelf: 'start', minWidth: 0 }}>
              <CodeCard title="Saldo" code={MONEY_CODE} json />
              <CodeCard title="Payout" right="POST /v1/payouts" code={PAYOUT_RES} json deep />
            </div>
          </section>

          <section id="webhooks" className="docs-sec">
            <div>
              <H2>Webhooks</H2>
              <P>Há duas formas de receber eventos. Usam o mesmo formato e a mesma assinatura.</P>
              <div className="docs-2" style={{ marginTop: 20 }}>
                <div style={{ padding: 18, borderRadius: 16, border: '1px solid #E8E6DF' }}>
                  <div style={{ fontWeight: 600, fontSize: 15 }}>Endpoints da conta</div>
                  <div style={{ fontSize: 12, fontWeight: 700, color: '#5E6862', marginTop: 2, letterSpacing: '.03em' }}>FIXO</div>
                  <div style={{ fontSize: 13.5, lineHeight: 1.55, color: '#3E4742', marginTop: 10 }}>Registados em Definições ou com <C s={12.5}>POST /v1/webhook_endpoints</C> (<C s={12.5}>url</C>, <C s={12.5}>enabled_events</C>). Liste com <C s={12.5}>GET /v1/webhook_endpoints</C>. Recebem os eventos escolhidos de toda a conta, incluindo levantamentos.</div>
                </div>
                <div style={{ padding: 18, borderRadius: 16, background: '#F3FAF1', border: '1px solid #CFE6C9' }}>
                  <div style={{ fontWeight: 600, fontSize: 15 }}>callback_url</div>
                  <div style={{ fontSize: 12, fontWeight: 700, color: '#0A6B05', marginTop: 2, letterSpacing: '.03em' }}>DINÂMICO, POR SESSÃO</div>
                  <div style={{ fontSize: 13.5, lineHeight: 1.55, color: '#3E4742', marginTop: 10 }}>Enviado ao criar a sessão. Recebe só os eventos dessa sessão. Útil quando cada loja, filial ou encomenda tem o seu próprio endereço.</div>
                </div>
              </div>
              <Eyebrow id="eventos">Eventos</Eyebrow>
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6, fontFamily: MONO, fontSize: 12 }}>
                {EVENTS.map(e => <span key={e} style={{ padding: '5px 9px', borderRadius: 7, background: '#F4F3EE' }}>{e}</span>)}
              </div>
              <P mt={12} s={14}>Qualquer evento pode ser consultado com <C s={13}>GET /v1/events/:id</C>.</P>
              <Eyebrow id="assinatura" m="28px 0 10px">Assinatura</Eyebrow>
              <P mt={0} s={14.5}>Cada pedido traz <C s={13}>Riha-Signature: t=…,v1=…</C>. Calcule HMAC-SHA256 de <C s={13}>t + "." + corpo</C> com o segredo <C s={13}>whsec_</C> e compare com v1. Rejeite se t tiver mais de 5 minutos.</P>
              <Eyebrow m="24px 0 10px">Tentativas</Eyebrow>
              <P mt={0} s={14.5}>Responda 2xx em até 10 segundos. Caso contrário, a RIHA volta a tentar 8 vezes ao longo de 3 dias:</P>
              <div className="docs-retries">
                {RETRIES.map((r, i) => <span key={r} style={{ flex: 1, textAlign: 'center', padding: '8px 0', borderRadius: 8, background: i === 0 ? '#0B120E' : '#F4F3EE', color: i === 0 ? '#fff' : undefined, fontFamily: MONO, fontSize: 11.5, whiteSpace: 'nowrap' }}>{r}</span>)}
              </div>
              <P mt={14} s={14.5}>Um evento pode chegar mais de uma vez ou fora de ordem. Guarde o <C s={13}>event.id</C> e ignore repetidos. Pode reenviar qualquer evento pelo painel.</P>
            </div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 14, alignSelf: 'start', minWidth: 0 }}>
              <CodeCard title="POST para callback_url" right="evento" code={EVENT_CODE} json />
              <CodeCard title="Verificar assinatura" right="Node" code={VERIFY_CODE} deep />
            </div>
          </section>

          <section id="erros" className="docs-sec last">
            <div>
              <H2>Erros</H2>
              <P>Respostas 4xx trazem um objeto <C>error</C> com um código estável que pode tratar no seu código. Inclua o <C>request_id</C> quando contactar o suporte.</P>
              <div style={{ marginTop: 18 }}>
                <Table>
                  {[['400', 'Parâmetro em falta ou inválido'], ['401', 'Chave API inválida ou revogada'], ['403', 'Chave restrita sem permissão, ou conta por verificar'], ['404', 'Recurso não existe neste modo (teste ou produção)'], ['409', 'Idempotency-Key reutilizada com outro corpo'], ['429', 'Demasiados pedidos (100 por segundo por conta)'], ['5xx', 'Erro da RIHA ou do operador. Repita com a mesma Idempotency-Key.']].map(([k, v]) => (
                    <div key={k} className="docs-row keep" style={{ '--w': '70px', padding: '11px 16px' } as CSSProperties}><span style={{ fontFamily: MONO }}>{k}</span><span style={{ color: '#3E4742' }}>{v}</span></div>
                  ))}
                </Table>
              </div>
            </div>
            <CodeCard title="Resposta de erro" right="400" rightColor="#FF8A8D" code={ERROR_RES} json style={{ alignSelf: 'start' }} />
          </section>
        </main>
      </div>
    </div>
  )
}
