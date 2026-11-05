# RIHA: payment gateway for Mozambique

RIHA combines M-Pesa, e-Mola and bank cards in one gateway. Merchants sign up as a lead, submit their business details (KYC) and, once an admin approves them, create payment links (WhatsApp, SMS, QR) or integrate through a REST API, see their balance, and request payouts to their bank within 72 hours. A super admin approves accounts, processes payouts and sets per-merchant fees.

This is the first working version, built from the Claude Design handoff in `project/` and `chats/` (see `HANDOFF.md`).

| Layer | Stack |
|---|---|
| API | Go 1.24, gin, GORM, PostgreSQL 16 (`backend/`), ports and adapters |
| Web | React 19, TypeScript, Vite, react-router (`frontend/`), responsive mobile and desktop |
| Payment rails | **Live M-Pesa goes direct** (`backend/internal/adapters/rails/mpesa.go`). **e-Mola and bim go through mtoca** (`backend/internal/adapters/rails/mtoca`) once `RIHA_MTOCA_ENABLED` is on, and show as coming soon until then. Cards show as coming soon, and live refunds are off because no rail can send money back yet. **Test mode** uses mtoca's dev API for the wallets on mtoca and is simulated for everything else. |

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
    rails/               method router, simulator, and the mtoca adapter (implement ports.Rail and ports.RailQuery)
    webhooks/            HTTP webhook sender (blocks private and loopback targets)
    filestore/           Cloudflare R2 (S3 API) or local disk for KYC documents, bank receipts and payment proofs
    pdf/                 payout transfer orders and payout statements (fpdf)
    mail/                email with attachments: SMTP, or .eml files in an outbox directory
    messaging/           SMS and WhatsApp (mock: appends to RIHA_OUTBOX_DIR/messages/messages.jsonl)
    sheets/              CSV and XLSX reader for member imports
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
  adapters/http/         Transport (fetch, cookie session, X-RIHA-Mode, CSRF header) and httpGateway implementing the ports
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
| Merchant (Escola Primavera, subscriptions demo) | `escola@primavera.co.mz` | `primavera-2026` |
| Super admin | `admin@riha.co.mz` | `riha-admin-2026` |

Configuration (environment variables):

| Variable | Default | Purpose |
|---|---|---|
| `DATABASE_URL` | local `riha` database | Postgres DSN |
| `RIHA_ENV` | `development` | set `production` in deployments: it blocks private network targets, refuses seeding and requires R2 |
| `RIHA_ADDR` | `:8080` | listen address |
| `RIHA_PAY_URL` | `RIHA_PUBLIC_URL` | address used in checkout and payment links |
| `RIHA_PUBLIC_URL` | `http://localhost:5173` | base URL for checkout links; `https` turns on secure cookies and HSTS |
| `RIHA_STATIC_DIR` | empty | serve a built frontend |
| `RIHA_STORAGE` | `r2` in production or when `RIHA_R2_BUCKET` is set, otherwise `local` | where uploaded files go |
| `RIHA_R2_ACCOUNT_ID`, `RIHA_R2_BUCKET`, `RIHA_R2_ACCESS_KEY_ID`, `RIHA_R2_SECRET_ACCESS_KEY` | empty | Cloudflare R2 bucket and an API token with Object Read and Write on that bucket only |
| `RIHA_R2_PREFIX` | empty | optional key prefix, for example `prod`, to share one bucket between environments |
| `RIHA_R2_ENDPOINT` | `https://<account>.r2.cloudflarestorage.com` | override, for example the EU jurisdiction endpoint `https://<account>.eu.r2.cloudflarestorage.com` |
| `RIHA_UPLOAD_DIR` | `./data/uploads` | local storage, and where files uploaded before R2 are still read from |
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
| `RIHA_MTOCA_ENABLED` | `false` | turn on the wallets that go through mtoca |
| `RIHA_MTOCA_LIVE_URL` | `https://api.payments.mtoca.net` | mtoca's production API, used for live payments |
| `RIHA_MTOCA_TEST_URL` | `https://api.payments.dev.mtoca.net` | mtoca's dev API, used for test payments. Production refuses to start if the two are mixed up |
| `RIHA_MTOCA_CHANNELS` | `mpesa=MPESA,emola=EMOLA,bim=BIM` | channel code for each wallet. Leave a wallet out to keep it off |
| `RIHA_MTOCA_LIVE_TOKEN`, `RIHA_MTOCA_TEST_TOKEN` | empty | sent as `Authorization: Bearer`, only if mtoca requires it |
| `RIHA_MPESA_VIA_MTOCA` | `false` | send M-Pesa through mtoca instead of the direct connection. It also needs `RIHA_MTOCA_ENABLED` |
| `RIHA_MTOCA_DEBUG` | `false` | log every mtoca response (phone numbers masked) |



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

## Subscriptions (Assinaturas)

A service for anything paid by period: school fees, gyms, tutors, condominiums. It lives under **Serviços › Assinaturas** in the dashboard (`/painel/assinaturas`) and on mobile behind the **+** button.

M-Pesa and e-Mola do not allow automatic debits, so **RIHA never charges on its own**. Each period the customer receives a link by SMS or WhatsApp with the amount and their last wallet number filled in, and confirms with the PIN.

**Model.** A *plan* (amount, frequency, fixed day or signup anniversary, months with charges, what happens to a missed month, questions for the customer, optional end date) has price *versions*. A *member* is one person being paid for, identified by payer phone and name, and can hold several *subscriptions* (for example school fee and transport). Each subscription produces one *charge* per period, unique per subscription and period.

**Merchant screens.** Summary (expected per month, members, debt, the month split into received by RIHA, paid outside RIHA and in debt, what needs attention, upcoming charges), plans, new plan with a live preview of what the customer sees, plan detail with price versions and the subscribe link and QR, change price, members with filters, add one member or import a sheet, member detail with the charge history and a per-charge timeline, mark as paid outside RIHA, catalogue, and service settings.

**Customer pages.** `/s/:merchant/:plan` subscribe from a plan link; `/a/:merchant` catalogue (one or several plans, paid together); `/i/:token` pay a period, with months in debt added and the option to pay only the oldest; then the paid screen with the next date, a receipt and the subscription summary.

**Lifecycle.** A background worker issues each charge 4 days ahead and walks the reminder schedule: reminder 3 days before, link on the day at 09:00, evening reminder if the link was not opened, past due and a resend at +2 days, a resend on the second channel at +5, a last warning 3 days before the end of the tolerance, and the final action on its last day. Messages are only sent between 08:00 and 20:00 and never on Sunday (Maputo time); a step that falls on a Sunday moves to Monday. If several steps are overdue at once (for example after downtime), their state changes all apply but only the latest one sends a message. Messages go through an outbox table, so a message is never lost or sent twice when a transaction rolls back.

**Paid outside RIHA.** Cash, transfer, deposit or other, with date, reference and optional or required proof (PDF, JPG or PNG, SHA-256 stored). It closes the debt and stops the reminders, but writes **no ledger entry**: it never reaches the balance, payouts or fees. Who marked it, when and how is recorded on the charge and in the audit log.

**API and events.** `GET/POST /v1/plans`, `GET /v1/plans/:id`, `GET/POST /v1/subscriptions` (POST invites a customer), `GET /v1/subscriptions/:id`, `POST /v1/subscriptions/:id/cancel`, `GET /v1/subscription_charges[/:id]`. Events: `plan.created`, `plan.updated`, `subscription.created`, `subscription.updated`, `subscription.canceled`, `subscription_charge.paid`, `subscription_charge.paid_outside`, `subscription_charge.past_due`.

**Test numbers.** In the mock messenger, a number ending in `9999` fails delivery, which shows up under "Precisa de atenção". In test mode the member page has "Enviar próximo link agora (teste)" to open the next period without waiting.

### How the design inconsistencies were resolved

- **Tolerance and the final action.** The design showed "tolerância 7 dias" next to a final action at "+10 dias" and "suspende a 15 out" for a charge due on 5 Oct. There is now one setting: the tolerance (default 10 days, between 6 and 30). The final action runs on its last day, and the last warning 3 days before. The +2, +5 and last-warning offsets are fixed.
- **Who can mark as paid outside.** "Admin e Financeiro" assumed team roles, which do not exist yet. It shows "Dono da conta" until roles exist.
- **Months with charges on non-monthly plans.** For weekly, quarterly and annual plans, a charge whose due date falls in a switched-off month is skipped.
- **Prefilled payment method.** Only wallets are prefilled (the number used last time). Cards are typed again, because storing them needs a card tokenisation provider.
- **Demo figures.** The design used 7 500, 10 000 and 11 000 MT for the same debt. The seed is built so the screens agree (11 000 MT across 3 members, one with 2 months).
- **SMS sender.** "EscPrimavera" has 12 characters; alphanumeric SMS senders allow 11, so the field enforces 11 and the demo uses "EPrimavera".
- **Link paths.** `/c/` is already the checkout path, so the catalogue is `/a/:merchant` and plan links are `/s/:merchant/:plan` instead of `/c/primavera` and `/s/primavera-1a4`.
- **The + button.** It now opens a sheet with "Novo link de pagamento" and the Serviços list, as in design section 2, which replaces screens 1a and 1d.

### Security and privacy

- Charge links use 16 random characters and are rate limited; the page sends `Cache-Control: no-store` and `Referrer-Policy: no-referrer`. It shows the student's name and amounts, but only a masked phone number (`84 ••• 4567`), and paying with "the number used last time" never sends the full number to the browser.
- The agreement text is generated by the server from the plan, amount, phone and channel. The exact text the customer accepted is stored with the time and IP on each subscription.
- Payments go oldest month first: a later month cannot be paid while an older one is open on the same subscription.
- Every merchant route checks the merchant of the session; other merchants' members, charges and proofs return 404. Creating plans, inviting, importing, changing prices, editing the catalogue or settings, and marking payments all require an approved account.
- Imports accept `.csv` and `.xlsx` up to 4 MB and 2 000 rows; the XLSX reader caps the unzipped size. CSV exports neutralise formula injection.

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
- Webhooks go to account endpoints and the session's `callback_url` (session and payment events only). They are signed `RIHA-Signature: t=<unix>,v1=hex(HMAC-SHA256(secret, t + "." + body))`. Delivery uses a transactional outbox: the first attempt is immediate, then retries after 5 min, 30 min, 2 h, 5 h, 10 h, 24 h and 72 h.

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

- Confirm the open points with mtoca (see Payment rails), the card acquirer integration, and daily reconciliation against every rail.
- 2FA, password reset by SMS, team members and roles, restricted keys (`rk_`).
- Chargebacks and disputes. Payout holds after a NIB change. Two-admin approval above a threshold. Risk rules and alerts (the admin "Risco e alertas" item is a placeholder).
- Real SMS and WhatsApp providers (both mocked: messages are written to `RIHA_OUTBOX_DIR/messages/messages.jsonl`), a registered SMS sender ID, and delivery receipts.
- Subscriptions: team roles for marking payments, customer self-service pause or cancel, card tokenisation for prefilled cards, and the `/docs` page does not yet document the subscription endpoints.
- The rate limiter and idempotency are per process and in the database; a multi-instance deployment needs a shared limiter.
- The frontend uses loose `any` types for API payloads (lint warnings). Generating types from an OpenAPI spec would tighten this.

## Payment rails (mtoca)

`rails.Modes` picks by mode and `rails.Router` by method. Live M-Pesa goes direct unless `RIHA_MPESA_VIA_MTOCA` is on. The wallets on mtoca use the production API for live payments and the dev API for test payments. Other test payments are simulated, and a live method with no rail shows as coming soon.

**Flow.** Before calling mtoca, RIHA stores the payment as `processing` with a new 12 digit `operationNumber` (`payments.rail_reference`) and the payer's whole number, which mtoca's status call needs and which is erased when the payment ends. It then calls `POST /api/payments` and polls `POST /api/payments/status` every 3 seconds for up to 4 minutes. `paymentStatus` `P` credits the payment; `R` is a rejection, with `rejectionReason` mapped in `rejectionFailures` (`01` is a timeout); `C` is a cancellation. If the server restarts or polling ends while the payment is still pending, the sweep worker (every 10 s) asks mtoca again after 3 minutes, at most every 30 seconds per payment. After 30 minutes without a final status the payment is closed as timed out and logged for manual reconciliation.

**Uncertain requests.** A 400 or 422 to the create call fails the payment as rejected, and any other 4xx shows "operador indisponível". A 5xx, a network error or a timeout is *not* treated as a failure, because mtoca may have accepted the payment: RIHA keeps asking for the status, so a customer who confirms the PIN is never left paid but uncredited. While that lasts, the session refuses a second attempt, for up to 30 minutes if mtoca never answers.

**Testing from your computer** (no RIHA server or database needed):

```bash
cd backend
go run ./cmd/mtoca-check -phone 258848255237 -amount 10.00
go run ./cmd/mtoca-check -phone 258848255237 -status-only -op 123456789012
```

It prints every request and the raw response, and how Riha reads it. It uses dev by default and refuses the production URL unless `-i-know-this-is-production` is passed.

**Open points to confirm with mtoca.** The adapter follows their message exactly, but these were not specified:
- Authentication: none was mentioned. `RIHA_MTOCA_LIVE_TOKEN` and `RIHA_MTOCA_TEST_TOKEN` send a bearer token if one is needed.
- The response body of `POST /api/payments`: the adapter accepts anything and relies on the status call. If the create call can already return `paymentStatus`, that is honoured.
- The phone format differs between their two examples (`+258…` to create, `258…` for status). RIHA follows the examples; if both accept the same format, simplify.
- `operationNumber` length and character rules (RIHA sends 12 digits) and whether it must be unique per account forever.
- The full list of `rejectionReason` codes and error responses, and whether the status call answers differently for a payment mtoca never received. Only `01` is mapped so far.
- The exact channel codes for M-Pesa and e-Mola. Only `BIM` appears in their message; `MPESA` and `EMOLA` are guesses. Test them with `mtoca-check -channel` and correct `RIHA_MTOCA_CHANNELS` if needed.
- Which numbers bim accepts. RIHA accepts any Mozambican mobile number for bim, and only 84/85 for M-Pesa and 86/87 for e-Mola.
- Whether mtoca can call a webhook instead of being polled, and its rate limits.
- What `cancelationStatus` means after a payment was already paid (`S` estorno): RIHA logs nothing special yet, so a reversal after success needs a refund recorded by hand.

## File storage (Cloudflare R2)

KYC documents, payout bank receipts and proofs of payments made outside RIHA go to a **private** R2 bucket through the S3 API (`internal/adapters/filestore/r2.go`). Files are never served from R2 directly: every download goes through the API, which checks the session and the merchant first, so the bucket needs no public access, custom domain or presigned URLs.

- Keys are `<prefix>/<folder>/<random id>.<ext>` (folders: merchant id, `payout-receipts`, `outside-proofs`). Original file names stay only in the database. Objects are stored with `Cache-Control: private, no-store` and `Content-Disposition: attachment`. R2 encrypts data at rest.
- The database stores `r2:<key>`. Files uploaded before the switch keep their local path and are still read from `RIHA_UPLOAD_DIR`, so nothing breaks on migration; copy that folder to the bucket later if the disk goes away.
- At start the server checks the bucket (`HeadBucket`) and refuses to run if it cannot reach it, and in production it refuses to run without R2 unless `RIHA_STORAGE=local` is set on purpose.
- Create the token in Cloudflare (R2, Manage API tokens) with **Object Read & Write** scoped to this one bucket. Keep the secret in the environment, never in the repository.
- `aws-sdk-go-v2/service/s3` is pinned to v1.72.0. Later releases send default CRC checksums on uploads, which R2 did not fully support; upgrade only after testing against the bucket.

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
