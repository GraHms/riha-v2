# Riha payment gateway UI

_Started 2026-11-03 19:27 UTC_

---

## User

Make UI mockups for for riha, a mozambican payment gateway, that aggregates mpesa, emola, bank cards into one platform. businesses can create accounts, create payment links via consile and share or create via api. the api will redirect the the ontegrates to finish payments in riha and redirect them back. they can see their balance, ask to whidraw with payments do be processed in 72h, they can add their banking details for withdrays, a section to generate api keys. it should have SaaS paypal, stripe, lemon squeezy vibe.. with dribble moderness. include landing page and it should be for mobile. mozambican use mobiles, make it intuive

## Assistant



## Assistant

_[tool: list_files]_

_[tool: list_files]_

_[tool: ask_user]_

I've put up a short form. Once you answer (language, brand color, which screens to include, and how to present them), I'll start building. You can skip any question and I'll decide.

## User

<artifact-content-authored-by-others/>
Answered — language: Portuguese; logo: Logo-LogIn (2).png; screens: Landing page, Sign up / onboarding (KYC), Dashboard home + balance, Create payment link, Share link (WhatsApp, QR), Hosted checkout (payer: M-Pesa / e-Mola / card), Payment success + redirect back, T…; presentation: Both; variations: One polished direction; fees: Single flat rate; extras: lets have our own colors rather than stripe colors. but the visual direction and philosopy should have that vibe, with dribbble SaaS vibe to it

## Assistant

_[tool: view_image]_

_[tool: image_metadata]_

_[tool: run_script]_

_[tool: run_script]_

_[tool: view_image]_

_[tool: run_script]_

_[tool: run_script]_

_[tool: run_script]_

_[tool: view_image]_

_[tool: update_todos]_

_[tool: dc_write]_

_[tool: dc_write]_

_[tool: dc_write]_

_[tool: show_html]_

_[tool: get_webview_logs]_

_[tool: update_todos]_

_[tool: ready_for_verification]_

I've built the Riha mockups in Portuguese, using colors taken from your logo: green and red diagonal stripes on near-black, with a warm off-white background. The verifier is still reviewing them.

- **Riha Mockups:** a canvas of 15 phone screens. They cover the landing page, sign-up, business verification, the dashboard with balance, creating and sharing payment links, the customer checkout (choose method, confirm with the PIN, success and redirect back to the store), transactions, withdrawals within 72h, bank details, API keys, and webhooks with redirect URLs. Each screen has an id like 1a or 1d that you can mention in chat.
- **Riha Prototype:** a clickable phone that runs the whole flow. Amounts, payment method, the test/live switch, copy buttons and transaction filters all respond. An M-Pesa payment moves from the PIN confirmation to success on its own after a few seconds.

**Placeholders to replace:**
- The 3.5% fee and the riha.co.mz domains are examples.
- The M-Pesa, e-Mola, Visa and Mastercard icons are stand-in letter tiles. Please send the official logos.
- The source logo is only 192px, so it looks a little soft at large sizes. A vector file (SVG) would fix that.

## User

lets have a desktop direction too, and a super admin dashborad for account approval, withdral management etc. api keys can be in settings

## Assistant

_[tool: list_files]_

_[tool: update_todos]_

_[tool: dc_write]_

_[tool: dc_write]_

_[tool: dc_js_str_replace]_

_[tool: dc_html_str_replace]_

_[tool: dc_html_str_replace]_

_[tool: dc_js_str_replace]_

_[tool: dc_set_props]_

_[tool: dc_js_str_replace]_

_[tool: dc_html_str_replace]_

_[tool: update_todos]_

_[tool: ready_for_verification]_

I've added a new section at the top of Riha Mockups with the desktop direction, the super admin dashboard, and a mobile Settings screen. A check of the screens is still running.

- **Desktop merchant app (2a–2i):** landing page, overview with balance, transactions with a detail panel, payment links with a "create link" side panel, balance and withdrawals, and the customer checkout. API keys now sit under Settings, alongside webhooks, bank account and business profile. Navigation, filters and the test/live switch all work when clicked.
- **Super admin (2j–2m):**
  - **Overview:** platform volume, plus live status for M-Pesa, e-Mola and cards.
  - **Account approval:** a queue with automatic checks (NUIT, account holder name, ID document), the uploaded documents, and approve, reject or request-documents buttons. Approving moves you to the next account in the queue.
  - **Withdrawal management:** a countdown bar for each request's 72h deadline, plus bulk approve, mark as paid and export of a bank file.
  - **Merchants list.**
  - The sidebar is dark so the admin area doesn't look like the merchant dashboard.
- **Mobile (2n):** the API tab in the bottom bar is now Settings, which holds API keys, webhooks and bank details. This changes the mobile API screens (1m, 1n) and the clickable prototype too.

In the admin sidebar, Transactions and Risk & alerts are visible but have no screens yet.

## User

<artifact-content-authored-by-others/>
the this is placed feels sloppy: M-Pesa · e-Mola · Visa · Mastercard

I would like to have a section like this. what is left for a production ux. the api should be restful, with external and dynamic webhook. not only for redirect but also where to send back the response of the transaction. what other data can go to riha ? we should know what type of link is. if its from the api, we should tell the origin etc. vs manual links. you are a principal stripe engineer... lets work on this to be stripe level

## User

the this is placed feels sloppy: M-Pesa · e-Mola · Visa · Mastercard

I would like to have a section like this. what is left for a production ux. the api should be restful, with external and dynamic webhook. not only for redirect but also where to send back the response of the transaction. what other data can go to riha ? we should know what type of link is. if its from the api, we should tell the origin etc. vs manual links. you are a principal stripe engineer... lets work on this to be stripe level

## Assistant

_[tool: update_todos]_

_[tool: str_replace_edit]_

_[tool: str_replace_edit]_

_[tool: str_replace_edit]_

_[tool: dc_set_props]_

_[tool: dc_write]_

_[tool: dc_html_str_replace]_

_[tool: ready_for_verification]_

No image came through with your message, so I treated it as a request to remove the hero pill. The payment-methods section under the hero already covers it. Next I'll build the API reference, track where each payment came from in the transaction detail, and write up the production gaps.

I've added a new top section to Riha Mockups. A check of the screens is still running.

- **Pill removed:** "M-Pesa · e-Mola · Visa · Mastercard" is gone from the hero on mobile and desktop. The payment-methods section right below the hero now does that job. You said "a section like this", but no image came through. If you had a reference in mind, send it and I'll match it.
- **API reference (3a):** a Stripe-style docs page in Portuguese. It covers:
  - REST endpoints, auth with public, secret and restricted keys, and an `Idempotency-Key` header so retries never charge twice.
  - Creating a checkout session with all the data a store can send to Riha: `client_reference_id`, customer, line items, metadata (custom key-value data), expiry, payment methods and language.
  - A per-session `callback_url`: a webhook address sent with each transaction, separate from the redirect. It only accepts verified domains.
  - Signed events, a retry schedule and error codes.
- **Where a payment came from (3b):** every payment now carries a `source` field: API, payment link, QR or dashboard.
  - API payments show the session, store domain, API key ID, IP, the store's order reference, metadata and the callback response.
  - Manual links show who created the link and the channel used.
  - The transactions table has a new Origem (origin) column.
- **Webhooks settings (3c):** fixed account-wide endpoints and per-transaction callbacks are now separate, with a list of allowed domains.
- **Production gaps:** a list grouped by what blocks launch. Top items are refunds, settlement reports showing which payments make up each withdrawal, failed withdrawals, 2FA, team roles and an audit log. It ends with product decisions your team needs to make, like the withdrawal fee and Banco de Moçambique rules.

The mobile transaction list doesn't show the origin yet. I'd add that next, along with refunds and the merchant detail page in the admin.

