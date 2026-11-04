# Riha: payment gateway for Mozambique

Riha combines M-Pesa, e-Mola and bank cards in one gateway. Merchants sign up, verify their business (KYC), create payment links (WhatsApp, SMS, QR) or integrate through a REST API, see their balance, and request payouts to their bank within 72 hours. A super admin approves accounts, processes payouts and sets per-merchant fees.

This is the first working version, built from the Claude Design handoff in `project/` and `chats/` (see `HANDOFF.md`).

| Layer | Stack |
|---|---|
| API | Go 1.24, gin, GORM, PostgreSQL 16 (`backend/`) |
| Web | React 19, TypeScript, Vite, react-router (`frontend/`), responsive mobile and desktop |
| Payment rails | **Mocked** (`backend/internal/providers`). M-Pesa and e-Mola simulate the PIN prompt on the phone; cards authorise synchronously. |

## Running it

```bash
make db            # Postgres in Docker (or use your own: DATABASE_URL=postgres://…)
make api           # API on :8080; migrates and seeds demo data on an empty DB
make web           # Vite on :5173, proxies /api and /v1 to :8080
```

Or as a single process serving the built frontend: `make run`, then open http://localhost:8080.

Demo accounts (created by the seed):

| Role | Email | Password |
|---|---|---|
| Merchant (Café Maputo, verified) | `ola@cafemaputo.co.mz` | `cafe-maputo-2026` |
| Super admin | `admin@riha.co.mz` | `riha-admin-2026` |

Configuration (environment variables): `DATABASE_URL`, `RIHA_ADDR` (`:8080`), `RIHA_PUBLIC_URL` (base URL for checkout links, `http://localhost:5173`), `RIHA_STATIC_DIR` (serve a built frontend), `RIHA_UPLOAD_DIR` (KYC documents, `./data/uploads`), `RIHA_COOKIE_SECURE` (`true` behind HTTPS), `RIHA_SEED` (`true`), `RIHA_SIM_DELAY_MS` (mock PIN confirmation delay, `4000`).

Tests: `make test`. The service integration tests (fees, ledger, payouts, refunds, session validation) need a throwaway database: `RIHA_TEST_DATABASE_URL=postgres://riha:riha@localhost:5432/riha_test?sslmode=disable`.

## What is in v1

**Merchants** (`/painel`)
- Signup (step 1) and business verification (step 2): business type, NUIT, documents (PDF/JPG/PNG, content-sniffed, 8 MB max), bank account (NIB, 21 digits).
- Test and live modes, with separate keys, data and balances. Test mode is always available. Live payouts and live secret keys require an approved account.
- Overview with balance, money on its way to the bank, received volume (7 days, 30 days, 12 months), split by method, and recent activity.
- Transactions: filters, search and CSV export. The detail panel shows the timeline, the **origin** (API session, domain, key, IP, `client_reference_id`, `callback_url` and its response, metadata; or link, creator, channel, device), fee, net amount, and full or partial refunds.
- Payment links: fixed or customer-chosen amount, reusable, accepted methods, expiry, and QR (counter) links. Share by WhatsApp, SMS, copy, or a downloadable QR.
- Balance and payouts: request a payout (debited immediately and due within 72 h), with history and status.
- Settings: business profile, bank account, API keys (secret shown once, rotate), recent API requests, redirect URLs, verified domains for `callback_url`, webhook endpoints (choose events, enable or disable, signing secret), deliveries with resend and test event, and password change.

**Customer checkout** (`/c/:session`, `/p/:merchant/:link`)
- Choose M-Pesa, e-Mola or card. Wallets show the "confirm on your phone" screen and poll until the result arrives. Errors such as wrong PIN or insufficient balance let the customer retry in the same session. On success, the page counts down and redirects to `success_url?session_id=…`.

**Super admin** (`/admin`)
- Overview: volume today, success rate, volume by method over 14 days, operator health (computed from recent attempts), payouts falling due, and the approval queue.
- Approvals: automatic checks (mock NUIT lookup, account holder vs. business name, documents present, high volume), documents, bank account, and approve / reject / request documents with an internal note.
- Payouts: 72 h SLA bars, tabs, bulk approve, mark as paid, reject or mark failed (the amount goes back to the balance), and a bank transfer CSV export.
- Merchants: list and detail page with **negotiated fee**, the platform default fee, suspend or reactivate, recent payments, payouts and audit log.

**Fees.** The platform default is **6%** (`default_fee_bps = 600`, editable in admin). Each merchant can have a negotiated `fee_bps`; `NULL` means the default applies. The fee is snapshotted on each payment at creation, so changing a fee never rewrites history. Fee = round-half-up(amount × bps / 10 000), in centavos. The merchant pays the fee, which is deducted from the credited amount. Refunds debit the full refunded amount and the fee is not returned.

**Money model.** All amounts are integer centavos. The balance is `SUM(ledger_entries.amount)` per merchant and mode. Payments credit the net amount, payouts debit at request time, and rejected or failed payouts and refunds write compensating entries. Balance-changing operations lock the merchant row (`SELECT … FOR UPDATE`) inside a transaction.

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

## Assumptions made for v1 (please confirm)

- **The merchant pays the 6% fee.** The customer pays the listed price. There is no withdrawal fee, and the minimum payout is 100,00 MT.
- An unverified merchant can use test mode and create links. Live payouts and live secret keys need admin approval. Suspended merchants cannot take payments.
- Domain ownership for `callback_url` is not really checked: an added domain counts as verified immediately (mock). A DNS TXT check is the production version.
- The KYC "automatic checks" are simulated. For example, a NUIT starting with `409` is reported as not found.
- The 72 h deadline counts calendar hours (no business days or holidays).
- Each merchant has a single user (the owner). There are no team roles yet.

## Not done yet (production gaps)

- Real M-Pesa (Vodacom OpenAPI), e-Mola and card acquirer integrations, and daily reconciliation against them. The `providers.Provider` interface is where they plug in.
- 2FA, password reset by email/SMS, team members and roles, restricted keys (`rk_`).
- Chargebacks and disputes. Payout holds after a NIB change. Two-admin approval above a threshold. Risk rules and alerts (the admin "Risco e alertas" item is a placeholder).
- Real SMS/email receipts (currently mocked). Object storage for KYC documents (currently the local disk).
- The rate limiter and idempotency are per process and in the database; a multi-instance deployment needs a shared limiter.
- The frontend uses loose `any` types for API payloads (lint warnings). Generating types from an OpenAPI spec would tighten this.
