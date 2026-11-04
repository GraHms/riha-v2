# Riha: payment gateway for Mozambique

Riha combines M-Pesa, e-Mola and bank cards in one gateway. Merchants sign up as a lead, submit their business details (KYC) and, once an admin approves them, create payment links (WhatsApp, SMS, QR) or integrate through a REST API, see their balance, and request payouts to their bank within 72 hours. A super admin approves accounts, processes payouts and sets per-merchant fees.

This is the first working version, built from the Claude Design handoff in `project/` and `chats/` (see `HANDOFF.md`).

| Layer | Stack |
|---|---|
| API | Go 1.24, gin, GORM, PostgreSQL 16 (`backend/`), ports and adapters |
| Web | React 19, TypeScript, Vite, react-router (`frontend/`), responsive mobile and desktop |
| Payment rails | **Live mode: M-Pesa** (`backend/internal/adapters/rails/mpesa.go`). e-Mola and cards show as coming soon, and live refunds are off because no rail can send money back yet. **Test mode: simulated**, and it never reaches an operator. |

## Backend architecture (ports and adapters)

```
backend/
  cmd/server/            composition root: builds adapters and injects them into the core
  internal/core/
    domain/              entities, rules, money, payout state machine, API resources, webhook signing (no I/O)
    ports/               interfaces the core needs: repositories, Tx, Rail, WebhookSender, FileStore, Renderer, Mailer, PasswordHasher, Reports
    app/                 use cases: signup, KYC, checkout, payments, links, payouts, refunds, keys, webhooks, admin, reports
  internal/adapters/
    httpapi/             driving adapter: gin handlers for /api (dashboard, checkout, admin) and /v1 (public REST API)
    postgres/            GORM records, mappers and repositories; transactions through context
    rails/               mock M-Pesa, e-Mola and card rail (implements ports.Rail)
    webhooks/            HTTP webhook sender (blocks private and loopback targets)
    filestore/           local disk storage for KYC documents and bank receipts
    pdf/                 payout transfer orders and payout statements (fpdf)
    mail/                email with attachments: SMTP, or .eml files in an outbox directory
    crypto/              bcrypt password hasher
  internal/seed/         demo data, written through the application layer
  internal/integration/  end-to-end tests: core + Postgres adapter + mock rail
```

The core imports nothing from `adapters`, gin or GORM. A real M-Pesa or e-Mola integration is a new adapter implementing `ports.Rail`; S3 storage is a new `ports.FileStore`.

## Frontend architecture

```
frontend/src/
  core/model.ts          shared types (Merchant, PaymentLink, CheckoutSession, ApiError)
  core/ports.ts          Gateway interface: auth, merchant, payments, links, payouts, developers, checkout, admin
  core/gateway.tsx       React context that provides the Gateway
  core/useQuery.ts       data hook that calls the Gateway (polling, reload)
  adapters/http/         Transport (fetch, cookie session, X-Riha-Mode, CSRF header) and httpGateway implementing the ports
  lib/                   session state and pure formatting helpers
  pages/, components/    UI; pages talk only to the Gateway, never to URLs
```

`main.tsx` injects `httpGateway()`. A mock gateway for Storybook or tests is a drop-in replacement.

## Running it

```bash
make db            # Postgres in Docker (or use your own: DATABASE_URL=postgres://…)
make api           # API on :8080 (development mode); migrates and seeds demo data on an empty DB
make web           # Vite on :5173, proxies /api and /v1 to :8080
```

Or as a single process serving the built frontend: `make run`, then open http://localhost:8080.

Demo accounts (created by the seed):

| Role | Email | Password |
|---|---|---|
| Merchant (Café Maputo, verified) | `ola@cafemaputo.co.mz` | `cafe-maputo-2026` |
| Super admin | `admin@riha.co.mz` | `riha-admin-2026` |

Configuration (environment variables):

| Variable | Default | Purpose |
|---|---|---|
| `DATABASE_URL` | local `riha` database | Postgres DSN |
| `RIHA_ENV` | `production` | `development` relaxes private network checks; seeding is refused in production |
| `RIHA_ADDR` | `:8080` | listen address |
| `RIHA_PAY_URL` | `RIHA_PUBLIC_URL` | address used in checkout and payment links |
| `RIHA_PUBLIC_URL` | `http://localhost:5173` | base URL for checkout links; `https` turns on secure cookies and HSTS |
| `RIHA_STATIC_DIR` | empty | serve a built frontend |
| `RIHA_UPLOAD_DIR` | `./data/uploads` | KYC documents and bank receipts |
| `RIHA_OUTBOX_DIR` | `./data/outbox` | emails are written here as `.eml` when SMTP is not configured |
| `RIHA_SMTP_ADDR`, `RIHA_SMTP_USER`, `RIHA_SMTP_PASSWORD`, `RIHA_MAIL_FROM` | empty | SMTP delivery for every email |
| `RIHA_MPESA_API_KEY`, `RIHA_MPESA_PUBLIC_KEY`, `RIHA_MPESA_SERVICE_PROVIDER_CODE` | empty | M-Pesa for live mode. Production refuses to start without them. `RIHA_MPESA_BASE_URL`, `RIHA_MPESA_C2B_PORT` and `RIHA_MPESA_QUERY_PORT` default to `https://api.vm.co.mz`, `18352` and `18353` |
| `RIHA_ADMIN_NOTIFY_EMAIL` | empty | where new accounts to review and new payouts are announced; empty sends nothing |
| `RIHA_ADMIN_EMAIL`, `RIHA_ADMIN_PASSWORD` | empty | create or reset the super admin on start (password of 12 characters or more) |
| `RIHA_TRUSTED_PROXIES` | none | comma list of proxy IPs or CIDRs allowed to set `X-Forwarded-For` |
| `RIHA_ALLOW_PRIVATE_NETWORKS` | `true` in development only | allow webhooks and `callback_url` to localhost and private addresses |
| `RIHA_COOKIE_SECURE` | from `RIHA_PUBLIC_URL` | force secure cookies |
| `RIHA_SEED` | `false` (`make api` sets `true`) | seed demo data into an empty database |
| `RIHA_SIM_DELAY_MS` | `4000` | mock PIN confirmation delay |


Tests: `make test`. Domain tests are pure. The integration tests (fees, ledger, payouts, refunds, session validation, webhook delivery) need a throwaway database: `RIHA_TEST_DATABASE_URL=postgres://riha:riha@localhost:5432/riha_test?sslmode=disable`.

## What is in v1

**Merchants** (`/painel`)
- Signup creates a **lead**. Step 2 (business verification) asks for business type, NUIT and the bank account (NIB, 21 digits). Documents (PDF/JPG/PNG, content-sniffed, 8 MB max) are optional and can be sent later or when the admin asks for them. Submitting moves the lead to `in_review`.
- **No operation is possible until the account is approved** (`active`), in test or live mode: payment links, checkout sessions, payments, refunds, payouts, API key generation and API key use all return `403 account_not_approved`. The dashboard shows a banner and disables the actions.
- Test and live modes, with separate keys, data and balances.
- Overview with balance, money on its way to the bank, received volume (7 days, 30 days, 12 months), split by method, and recent activity.
- Transactions: filters, search and CSV export. The detail panel shows the timeline, the **origin** (API session, domain, key, IP, `client_reference_id`, `callback_url` and its response, metadata; or link, creator, channel, device), fee, net amount, and full or partial refunds.
- Payment links: fixed or customer-chosen amount, reusable, accepted methods, expiry, and QR (counter) links. Share by WhatsApp, SMS, copy, or a downloadable QR.
- Balance and payouts: request a payout (debited immediately and due within 72 h), with history and status. A paid payout offers the statement PDF and the bank receipt for download.
- Settings: business profile, bank account, API keys (secret shown once, rotate), recent API requests, redirect URLs, verified domains for `callback_url`, webhook endpoints (choose events, enable or disable, signing secret), deliveries with resend and test event, and password change.

**Customer checkout** (`/c/:session`, `/p/:merchant/:link`)
- Choose M-Pesa, e-Mola or card. Wallets show the "confirm on your phone" screen and poll until the result arrives. Errors such as wrong PIN or insufficient balance let the customer retry in the same session. On success, the page counts down and redirects to `success_url?session_id=…`.

**Super admin** (`/admin`)
- Overview: volume today, success rate, volume by method over 14 days, operator health (computed from recent attempts), payouts falling due, and the approval queue.
- Approvals: tabs **Por rever** (submitted) and **Leads** (signed up, not submitted yet). Automatic checks (mock NUIT lookup, account holder vs. business name, documents present, high volume), documents, bank account, and approve / reject / request documents with an internal note.
- Payouts: 72 h SLA bars, tabs, bulk approve, reject or mark failed (the amount goes back to the balance), and a bank transfer CSV export.
  - **Transfer order PDF** (one payout or a selection): beneficiary, NUIT, bank, holder, NIB, transfer description, deadline and signature lines, with a summary page for a batch.
  - **Complete**: a live payout can only be marked paid with a **bank receipt** (PDF, JPG or PNG, 8 MB max, content-sniffed). Its SHA-256 is stored as evidence. One receipt can cover a bulk completion.
  - After completion: download the **payout statement PDF** (amounts, bank details, timeline, receipt name and hash, the receipt image attached on page 2 when it is an image), download the original receipt, or **email** the statement and receipt to the merchant.
- Merchants: list and detail page with **negotiated fee**, the platform default fee, suspend or reactivate, recent payments, payouts and audit log.

**Fees.** The platform default is **6%** (`default_fee_bps = 600`, editable in admin). Each merchant can have a negotiated `fee_bps`; `NULL` means the default applies. The fee is snapshotted on each payment at creation, so changing a fee never rewrites history. Fee = round-half-up(amount × bps / 10 000), in centavos. The merchant pays the fee, which is deducted from the credited amount. Refunds debit the full refunded amount and the fee is not returned.

**Money model.** All amounts are integer centavos. The balance is `SUM(ledger_entries.amount)` per merchant and mode. Payments credit the net amount, payouts debit at request time, and rejected or failed payouts and refunds write compensating entries. Balance-changing operations lock the merchant row (`SELECT … FOR UPDATE`) inside a transaction.

## M-Pesa

The request blocks until the customer answers the prompt, so it runs on a goroutine. Each payment stores a `RIHA…` reference before the request goes out. A payment fails only when M-Pesa says so: a timeout, a dropped connection, `INS-1`, `INS-9` or a restart leave it processing, and the sweep asks the query API after 3 minutes. With no record at M-Pesa it ends after 5 minutes, and with no answer at all after 30, logged for a manual check.

## Link previews and search

Apps that preview a link, and most crawlers, read the HTML without running the site, so the server writes each page's title, description and image into it (`adapters/httpapi/seo.go`). Payment links get their own preview image from `/og/`, showing the merchant, product and amount, and stay out of search results. An unknown address gets a real 404. `/robots.txt` and `/sitemap.xml` are served by the API.

## Emails

Every email goes through `email_outbox`: it is written in the same transaction as the change it reports, sent by a worker, and retried on the webhook schedule, so a mail server problem never fails a payment. The body is cleared once sent, because a reset email holds a live link. Payment, refund and payout emails are live mode only.

| Email | Sent to | When |
|---|---|---|
| Welcome | Merchant | Sign up |
| Data received, approved, rejected, more information, suspended, reactivated | Merchant | Verification and admin decisions. Only "more information" quotes the admin's note. |
| Password reset, password changed | User | Reset links last 1 hour, work once, carry the token in the URL fragment, and at most 3 are sent per account per hour. A reset signs out every device. |
| Bank account changed, new live API key | Merchant | Always, these are security notices |
| Payment receipt, refund | Customer | When the session or the checkout form has their email. The dashboard can send a receipt again. |
| Payment received | Merchant | Can be turned off in Settings |
| Refund, payout requested, sent, paid, not completed | Merchant | Always |
| New account to review, new payout | `RIHA_ADMIN_NOTIFY_EMAIL` | Only when set |

## Public REST API (`/v1`)

Authenticate with `Authorization: Bearer sk_test_…` (or `curl -u sk_test_…:`). Bodies can be JSON or form-encoded, including nested fields such as `customer[phone]` and `line_items[0][name]`. Full reference: `/docs` in the app.

```
POST   /v1/checkout/sessions          GET /v1/checkout/sessions[/:id]   POST /v1/checkout/sessions/:id/expire
GET    /v1/payments[/:id]             POST /v1/refunds
GET    /v1/payment_links              POST /v1/payment_links            PATCH /v1/payment_links/:id
GET    /v1/balance                    GET  /v1/payouts                  POST  /v1/payouts
GET    /v1/webhook_endpoints          POST /v1/webhook_endpoints        DELETE /v1/webhook_endpoints/:id
GET    /v1/events/:id
```

- `Idempotency-Key` on POST: the same key and body replay the stored response (header `Idempotent-Replayed: true`). The same key with a different body returns `409 idempotency_key_reused`. Keys are kept for 24 h.
- Stripe-style errors: `{"error":{"type","code","param","message","doc_url","request_id"}}`.
- Rate limit: 100 requests per second per account (in memory, per process).
- `callback_url` is a dynamic per-session webhook. Its domain must be verified in Settings, otherwise the request fails with `callback_url_not_allowed`. In test mode `localhost` is also accepted.
- Webhooks go to account endpoints and the session's `callback_url` (session and payment events only). They are signed `Riha-Signature: t=<unix>,v1=hex(HMAC-SHA256(secret, t + "." + body))`. Delivery uses a transactional outbox: the first attempt is immediate, then retries after 5 min, 30 min, 2 h, 5 h, 10 h, 24 h and 72 h.

```bash
curl http://localhost:8080/v1/checkout/sessions -u sk_test_…: \
  -H "Idempotency-Key: pedido-2094" \
  -d amount=125000 -d currency=MZN \
  -d success_url=https://loja.co.mz/ok -d client_reference_id=pedido_2094
# → { "id": "cs_test_…", "url": "http://localhost:5173/c/cs_test_…", … }
```

### Test numbers (mock rails, both modes)

| Input | Result |
|---|---|
| Wallet number ending `0000` | `pin_incorrect` |
| ending `1111` | `insufficient_funds` |
| ending `2222` | `customer_timeout` |
| ending `3333` | `customer_cancelled` |
| ending `4444` | `operator_unavailable` (immediate) |
| any other valid number (M-Pesa 84/85, e-Mola 86/87) | success after `RIHA_SIM_DELAY_MS` |
| Card `4242 4242 4242 4242` (any Luhn-valid number) | success |
| Card `4000 0000 0000 0002` / `4000 0000 0000 9995` | `card_declined` / `insufficient_funds` |

Background workers time out wallet prompts stuck for more than 3 minutes, expire open sessions, and settle **test-mode** payouts by themselves after about 20 s, so the whole payout lifecycle can be seen without an admin.

## Confirmed decisions

- A lead or an account under review cannot operate at all, not even in test mode.
- Admins may approve an account that has no documents. The automatic checks show what is missing.
- One bank receipt can cover a bulk payout completion, matching a single bank batch transfer.
- Email delivery goes to the outbox directory until `RIHA_SMTP_ADDR` is set.

## Assumptions made for v1 (please confirm)

- **The merchant pays the 6% fee.** The customer pays the listed price. There is no withdrawal fee, and the minimum payout is 100,00 MT.
- Domain ownership for `callback_url` is not really checked: an added domain counts as verified immediately (mock). A DNS TXT check is the production version.
- The KYC "automatic checks" are simulated. For example, a NUIT starting with `409` is reported as not found.
- The 72 h deadline counts calendar hours (no business days or holidays).
- Each merchant has a single user (the owner). There are no team roles yet.

## Not done yet (production gaps)

- Real M-Pesa (Vodacom OpenAPI), e-Mola and card acquirer integrations, and daily reconciliation against them. They plug in as adapters implementing `ports.Rail`.
- 2FA, password reset by SMS, team members and roles, restricted keys (`rk_`).
- Chargebacks and disputes. Payout holds after a NIB change. Two-admin approval above a threshold. Risk rules and alerts (the admin "Risco e alertas" item is a placeholder).
- Real SMS receipts. Object storage for KYC documents (currently the local disk).
- The rate limiter and idempotency are per process and in the database; a multi-instance deployment needs a shared limiter.
- The frontend uses loose `any` types for API payloads (lint warnings). Generating types from an OpenAPI spec would tighten this.

## Security notes

- Dashboard sessions: random 48 character token (crypto/rand) in an `HttpOnly`, `SameSite=Lax` cookie (`Secure` behind HTTPS). The database stores only its SHA-256. Changing the password revokes every other session, and expired sessions are purged.
- CSRF: state-changing `/api` requests need the `X-Requested-With: riha` header, which a cross-site form cannot send.
- API keys are stored as SHA-256 hashes and shown once. Keys of accounts that are not approved are refused.
- Every merchant query is scoped by the merchant of the session or key. Another merchant's payout, document or receipt returns 404. Admin routes check the role.
- Responses are built by presenters, never by serializing domain structs, so password hashes, key hashes and file paths never leave the server. The public receipt endpoint was removed.
- Rate limits: login and signup 10 per minute per IP, checkout pay 20 per minute per IP, `/v1` 100 per second per account. Checkout sessions accept at most 10 failed attempts.
- Request bodies are capped at 1 MB (9 MB for uploads). Uploads are content-sniffed and stored under random names; reads cannot escape the upload directory.
- Webhook delivery refuses loopback, private, link-local, CGNAT and multicast addresses at connection time (DNS rebinding included) and does not follow redirects. Endpoint creation rejects private hosts too.
- Headers: CSP, `X-Frame-Options: DENY`, `nosniff`, `Referrer-Policy`, `Permissions-Policy`, COOP, HSTS behind HTTPS, and `Cache-Control: no-store` on API responses.
- `X-Forwarded-For` is ignored unless `RIHA_TRUSTED_PROXIES` lists the proxy. SQL logging uses parameterized output, so values do not reach the logs.
