package integration

// Scale test: fills a database with a year of a busy RIHA and times the pages
// and jobs that read it, with the number of SQL statements each runs.
//
//	RIHA_PERF_DATABASE_URL='postgres://r:r@127.0.0.1:55463/perf?sslmode=disable' \
//	RIHA_PERF_PAYMENTS=1000000 go test ./internal/integration -run TestPerfScale -v -timeout 60m
//
// The database needs pg_stat_statements (shared_preload_libraries) to count statements.

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"riha/internal/adapters/crypto"
	"riha/internal/adapters/filestore"
	"riha/internal/adapters/messaging"
	"riha/internal/adapters/pdf"
	"riha/internal/adapters/postgres"
	"riha/internal/adapters/rails"
	"riha/internal/adapters/sheets"
	"riha/internal/core/app"
	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

func perfEnvInt(k string, def int) int {
	if n, err := strconv.Atoi(os.Getenv(k)); err == nil && n > 0 {
		return n
	}
	return def
}

func perfExec(t *testing.T, db *sql.DB, label, q string, args ...any) {
	t.Helper()
	start := time.Now()
	if _, err := db.Exec(q, args...); err != nil {
		t.Fatalf("%s: %v", label, err)
	}
	t.Logf("seed %-22s %v", label, time.Since(start).Round(time.Millisecond))
}

func perfSeed(t *testing.T, db *sql.DB, n, merchants int) {
	perfExec(t, db, "setseed", `SELECT setseed(0.42)`)
	perfExec(t, db, "merchants", `INSERT INTO merchants (id, slug, name, status, email, phone, created_at, updated_at, sms_notifications, email_notifications)
		SELECT 'mch_'||i, 'shop-'||i, 'Loja '||i, 'active', 'loja'||i||'@example.com', '84'||lpad(i::text,7,'0'), now() - (i||' hours')::interval, now(), true, false
		FROM generate_series(1,$1) i`, merchants)
	perfExec(t, db, "banks", `INSERT INTO bank_accounts (id, merchant_id, bank_name, holder, nib, verified, created_at, updated_at)
		SELECT 'ba_'||i, 'mch_'||i, 'BCI', 'Loja '||i, lpad(i::text,21,'0'), true, now(), now() FROM generate_series(1,$1) i`, merchants)
	// Payments: a year of them, a few merchants making most, 90% live.
	perfExec(t, db, "payments", `INSERT INTO payments (id, reference, merchant_id, livemode, session_id, status, amount, fee_bps, fee, net, amount_refunded,
			method, phone_prefix, phone_last4, rail_reference, attempt_id, failure_code, customer_name, created_at, updated_at, succeeded_at, failed_at,
			integration, platform_fee, partner_fee, priced, paid_late, duplicate_of, contact_phone, rechecks, source_type)
		SELECT 'pay_'||lpad(i::text,9,'0'), 'R'||lpad(i::text,10,'0'), 'mch_'||(1 + floor($2 * power(random(),3)))::int, i % 10 <> 0, 'cs_'||lpad(i::text,9,'0'),
			s.status, a.amount, 300, a.amount*3/100, a.amount - a.amount*3/100, CASE WHEN s.status='refunded' THEN a.amount ELSE 0 END,
			(ARRAY['mpesa','emola','bim'])[1 + i % 3], '84', lpad((i % 10000)::text,4,'0'), lpad(i::text,12,'0'), '',
			CASE WHEN s.status='failed' THEN 'customer_timeout' ELSE '' END, 'Cliente '||i, c.at, c.at,
			CASE WHEN s.status IN ('succeeded','refunded') THEN c.at + interval '25 seconds' END,
			CASE WHEN s.status='failed' THEN c.at + interval '60 seconds' END,
			CASE WHEN i % 10 = 0 THEN 'simulator' WHEN i % 3 = 0 AND i % 2 = 0 THEN 'mpesa_direct' ELSE 'mtoca' END,
			CASE WHEN i % 3 = 0 AND i % 2 = 0 THEN a.amount*3/100 ELSE 0 END, a.amount*9/1000, true, i % 997 = 0, '', '', 0, 'link'
		FROM generate_series(1,$1) i
		CROSS JOIN LATERAL (SELECT 10000 + (i % 500) * 100 AS amount) a
		CROSS JOIN LATERAL (SELECT now() - (random() + i*0) * interval '365 days' AS at) c  -- i keeps it per row
		CROSS JOIN LATERAL (SELECT CASE WHEN i % 100 < 80 THEN 'succeeded' WHEN i % 100 < 95 THEN 'failed' WHEN i % 100 < 97 THEN 'refunded' ELSE 'processing' END AS status) s`,
		n, merchants)
	perfExec(t, db, "sessions", `INSERT INTO checkout_sessions (id, merchant_id, livemode, status, amount, currency, expires_at, payment_id, created_at, updated_at, source_type)
		SELECT session_id, merchant_id, livemode, CASE WHEN status IN ('succeeded','refunded') THEN 'complete' ELSE 'open' END, amount, 'MZN',
			created_at + interval '1 hour', id, created_at, created_at, 'link' FROM payments`)
	perfExec(t, db, "ledger", `INSERT INTO ledger_entries (id, merchant_id, livemode, type, amount, reference_id, description, created_at)
		SELECT 'le_'||substr(id,5), merchant_id, livemode, 'payment', net, id, 'Pagamento '||reference, succeeded_at FROM payments WHERE status IN ('succeeded','refunded')`)
	perfExec(t, db, "payouts", `INSERT INTO payouts (id, number, merchant_id, livemode, amount, fee, status, bank_name, holder, nib, requested_by, requested_at, due_at, paid_at, created_at, updated_at)
		SELECT 'po_'||i, i, 'mch_'||(1 + i % $1), true, 500000, 0, CASE WHEN i % 50 = 0 THEN 'new' WHEN i % 50 = 1 THEN 'sent' ELSE 'paid' END, 'BCI', 'x', lpad(i::text,21,'0'),
			'usr', now() - (i||' minutes')::interval, now() - (i||' minutes')::interval + interval '24 hours',
			CASE WHEN i % 50 > 1 THEN now() - (i||' minutes')::interval END, now(), now()
		FROM generate_series(1,$2) i`, merchants, merchants*20)
	perfExec(t, db, "payout seq", `SELECT setval('payout_number_seq', (SELECT max(number) FROM payouts))`)
	perfExec(t, db, "funding", `INSERT INTO payout_funding (id, payout_id, merchant_id, source, amount, send, status, created_at)
		SELECT 'fnd_'||p.id||'_'||k, p.id, p.merchant_id, CASE WHEN k=1 THEN 'mtoca' ELSE 'mpesa_direct' END, p.amount/2, p.amount/2,
			CASE WHEN p.status='paid' THEN 'paid' ELSE 'open' END, p.requested_at FROM payouts p CROSS JOIN generate_series(1,2) k`)
	// The trail: three events a payment, with bodies on the rail ones.
	perfExec(t, db, "trail", `INSERT INTO trail_events (id, merchant_id, livemode, session_id, payment_id, request_id, kind, source, level, title, status, duration_ms, network, request, response, data, created_at)
		SELECT 'trl_'||substr(p.id,5)||'_'||k, p.merchant_id, p.livemode, p.session_id, p.id, 'req_'||substr(p.id,5),
			(ARRAY['payment.submitted','rail.response','payment.status'])[k], 'riha', CASE WHEN k=2 AND p.status='failed' THEN 'warn' ELSE 'info' END,
			(ARRAY['O cliente pediu para pagar','O bim respondeu ao pedido','Pagamento concluído'])[k]||' '||p.reference, 200, 900 + (random()*4000)::int, 'Vodacom',
			CASE WHEN k=2 THEN '{"operationNumber":"'||p.rail_reference||'","amount":"100.00"}' ELSE '' END,
			CASE WHEN k=2 THEN '{"paymentStatus":"P","transactionID_NEP":"'||p.rail_reference||'"}' ELSE '' END,
			'{}'::jsonb, p.created_at + (k||' seconds')::interval
		FROM payments p CROSS JOIN generate_series(1,3) k`)
	perfExec(t, db, "deliveries", `INSERT INTO webhook_deliveries (id, event_id, merchant_id, livemode, event_type, target, url, status, attempts, last_status_code, last_duration_ms, created_at, updated_at)
		SELECT 'whd_'||substr(id,5), 'evt_'||substr(id,5), merchant_id, livemode, 'payment.succeeded', 'endpoint', 'https://example.com/hook', 'succeeded', 1, 200, 120, created_at, created_at
		FROM payments WHERE status = 'succeeded' AND random() < 0.5`)
	perfExec(t, db, "api logs", `INSERT INTO api_request_logs (id, merchant_id, livemode, method, path, status, ip, duration_ms, request_id, created_at)
		SELECT 'log_'||substr(id,5), merchant_id, livemode, 'POST', '/v1/checkout/sessions', 200, '1.2.3.4', 40, 'req_'||substr(id,5), created_at FROM payments WHERE random() < 0.5`)
	perfExec(t, db, "analyze", `ANALYZE`)
}

type perfCounter struct {
	db *sql.DB
}

func (c perfCounter) reset(t *testing.T) {
	if _, err := c.db.Exec(`SELECT pg_stat_statements_reset()`); err != nil {
		t.Logf("pg_stat_statements unavailable: %v", err)
	}
}

func (c perfCounter) calls() int64 {
	var n sql.NullInt64
	_ = c.db.QueryRow(`SELECT sum(calls) FROM pg_stat_statements WHERE dbid = (SELECT oid FROM pg_database WHERE datname = current_database())
		AND query NOT ILIKE '%pg_stat_statements%'`).Scan(&n)
	return n.Int64
}

func TestPerfScale(t *testing.T) {
	dsn := os.Getenv("RIHA_PERF_DATABASE_URL")
	if dsn == "" {
		t.Skip("RIHA_PERF_DATABASE_URL not set")
	}
	n := perfEnvInt("RIHA_PERF_PAYMENTS", 200000)
	merchants := perfEnvInt("RIHA_PERF_MERCHANTS", 500)
	store, err := postgres.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE EXTENSION IF NOT EXISTS pg_stat_statements`); err != nil {
		t.Logf("pg_stat_statements: %v", err)
	}
	ctx := context.Background()
	if os.Getenv("RIHA_PERF_KEEP") == "" {
		if err := store.DropAll(); err != nil {
			t.Fatal(err)
		}
		if err := store.Migrate(); err != nil {
			t.Fatal(err)
		}
		perfSeed(t, db, n, merchants)
	}
	var count int64
	_ = db.QueryRow(`SELECT count(*) FROM payments`).Scan(&count)
	var trailN int64
	_ = db.QueryRow(`SELECT count(*) FROM trail_events`).Scan(&trailN)
	t.Logf("data: %d payments, %d trail events, %d merchants", count, trailN, merchants)

	mock := rails.NewMock(0)
	mock.After = func(_ time.Duration, f func()) { f() }
	r := store.Repositories()
	a := app.New(app.Deps{
		Tx: r.Tx, Users: r.Users, AuthSessions: r.AuthSessions, Merchants: r.Merchants, Documents: r.Documents, Banks: r.Banks,
		Keys: r.Keys, Links: r.Links, Sessions: r.Sessions, Payments: r.Payments, Refunds: r.Refunds, Ledger: r.Ledger,
		Payouts: r.Payouts, Endpoints: r.Endpoints, Domains: r.Domains, Events: r.Events, Deliveries: r.Deliveries,
		RequestLogs: r.RequestLogs, Idempotency: r.Idempotency, Audit: r.Audit, Settings: r.Settings, Reports: r.Reports,
		Plans: r.Plans, Members: r.Members, Subs: r.Subs, Charges: r.Charges, Messages: r.Messages, SubSettings: r.SubSettings, Integrations: r.Integrations,
		Messenger: &messaging.Outbox{}, Sheets: sheets.Reader{},
		Rail: mock, Sender: nullSender{}, Files: filestore.Local{Root: t.TempDir()}, Hasher: crypto.Bcrypt{},
		Renderer: pdf.New(), Mailer: mails, Emails: r.Emails, Resets: r.Resets,
	}, "http://localhost:5173")
	trail := postgres.NewTrail(store)
	a.Trail, a.Insights = trail, postgres.Insights{Store: store}
	pc := perfCounter{db}

	measure := func(name string, f func() error) time.Duration {
		pc.reset(t)
		start := time.Now()
		err := f()
		d := time.Since(start)
		calls := pc.calls()
		if err != nil {
			t.Logf("%-44s %10v  %6d stmts  ERROR %v", name, d.Round(time.Millisecond), calls, err)
		} else {
			t.Logf("%-44s %10v  %6d stmts", name, d.Round(time.Millisecond), calls)
		}
		return d
	}

	// Boot: what every start of the server does on a full database.
	measure("boot: Migrate (AutoMigrate + backfills)", store.Migrate)
	measure("boot: SetupIntegrations (0 unpriced)", func() error { return a.SetupIntegrations(ctx) })
	if _, err := db.Exec(`UPDATE payments SET priced = false WHERE id IN (SELECT id FROM payments WHERE livemode ORDER BY id LIMIT 20000)`); err != nil {
		t.Fatal(err)
	}
	measure("boot: SetupIntegrations (20k unpriced)", func() error { return a.SetupIntegrations(ctx) })

	now := time.Now()
	from30 := now.AddDate(0, 0, -30)
	big, _ := a.Merchants.ByID(ctx, "mch_1")

	measure("admin settlement (all time)", func() error { _, err := a.Settlement(ctx, nil, nil); return err })
	measure("admin settlement (30 days)", func() error { _, err := a.Settlement(ctx, &from30, &now); return err })
	measure("admin integrations", func() error { _, err := a.IntegrationViews(ctx); return err })
	measure("admin overview", func() error { a.PlatformOverview(ctx); a.ActiveAlerts(ctx); return nil })
	measure("admin merchants directory", func() error { a.MerchantDirectory(ctx, "", ""); return nil })
	measure("admin payments page 1", func() error { a.PlatformPayments(ctx, app.PlatformPaymentsQuery{}); return nil })
	measure("admin payments page 2000 (offset 100k)", func() error { a.PlatformPayments(ctx, app.PlatformPaymentsQuery{Page: 2000}); return nil })
	measure("admin payments search 'cliente 4242'", func() error { a.PlatformPayments(ctx, app.PlatformPaymentsQuery{Query: "cliente 4242"}); return nil })
	measure("admin payments view=late", func() error { a.PlatformPayments(ctx, app.PlatformPaymentsQuery{View: "late"}); return nil })
	measure("admin payouts queue pending", func() error { a.PayoutQueue(ctx, "pending"); return nil })
	measure("admin payouts queue paid", func() error { a.PayoutQueue(ctx, "paid"); return nil })
	measure("admin metrics 7 days", func() error {
		_, err := a.AdminMetrics(ctx, ports.MetricsQuery{From: now.AddDate(0, 0, -7), To: now, Live: true})
		return err
	})
	measure("admin metrics 90 days", func() error {
		_, err := a.AdminMetrics(ctx, ports.MetricsQuery{From: now.AddDate(0, 0, -90), To: now, Live: true})
		return err
	})
	measure("admin trail search '4242'", func() error { _, err := a.FindThreads(ctx, "4242"); return err })
	measure("admin trail search 'nothing-matches'", func() error { _, err := a.FindThreads(ctx, "nothing-matches"); return err })
	since := now.Add(-48 * time.Hour)
	measure("admin trail feed 48h", func() error { _, err := a.TrailEvents(ctx, ports.TrailQuery{Since: &since}); return err })
	measure("admin trail feed 48h level=warn", func() error {
		_, err := a.TrailEvents(ctx, ports.TrailQuery{Since: &since, Level: domain.TrailWarn})
		return err
	})
	measure("admin trail feed search 'bim'", func() error { _, err := a.TrailEvents(ctx, ports.TrailQuery{Search: "zzz-none"}); return err })
	measure("partner home mtoca (30 days)", func() error { _, err := a.PartnerHome(ctx, "mtoca", from30, now); return err })
	measure("partner payouts to_pay", func() error {
		_, err := a.PartnerFunding(ctx, app.PartnerFundingQuery{Partner: "mtoca", Tab: "to_pay"})
		return err
	})
	measure("partner payouts paid", func() error {
		_, err := a.PartnerFunding(ctx, app.PartnerFundingQuery{Partner: "mtoca", Tab: "paid"})
		return err
	})
	measure("merchant overview 30d (biggest)", func() error { _, err := a.Overview(ctx, big, true, 30); return err })
	measure("merchant overview 365d (biggest)", func() error { _, err := a.Overview(ctx, big, true, 365); return err })
	measure("merchant transactions page (biggest)", func() error {
		_, err := a.SearchPayments(ctx, big.ID, true, app.PaymentFilter{Limit: 50})
		return err
	})
	measure("merchant transactions search (biggest)", func() error {
		_, err := a.SearchPayments(ctx, big.ID, true, app.PaymentFilter{Limit: 50, Search: "cliente 77"})
		return err
	})
	measure("merchant export csv (biggest, 10k)", func() error { _, err := a.ExportPayments(ctx, big.ID, true, app.PaymentFilter{}); return err })
	measure("merchant balance summary", func() error { a.BalanceSummary(ctx, big, true); return nil })
	measure("watchdog health", func() error { a.ActiveAlerts(ctx); return nil })

	// A withdrawal holds the merchant's row lock while it recomputes every
	// partner's all-time position; a payment completing for the same merchant waits.
	var pid string
	_ = db.QueryRow(`SELECT id FROM payments WHERE merchant_id = 'mch_1' AND livemode AND status = 'processing' LIMIT 1`).Scan(&pid)
	measure("request payout (alone)", func() error { _, err := a.RequestPayout(ctx, big.ID, true, 100000, "usr"); return err })
	var wg sync.WaitGroup
	var payoutTook, completeTook time.Duration
	wg.Add(2)
	go func() {
		defer wg.Done()
		s := time.Now()
		if _, err := a.RequestPayout(ctx, big.ID, true, 100000, "usr"); err != nil {
			t.Logf("payout: %v", err)
		}
		payoutTook = time.Since(s)
	}()
	go func() {
		defer wg.Done()
		time.Sleep(30 * time.Millisecond)
		s := time.Now()
		if err := a.CompletePayment(ctx, pid, domain.Outcome{Success: true, OperatorReference: "X"}); err != nil {
			t.Logf("complete: %v", err)
		}
		completeTook = time.Since(s)
	}()
	wg.Wait()
	t.Logf("contention: payout %v, payment completion for the same merchant started 30ms later took %v", payoutTook.Round(time.Millisecond), completeTook.Round(time.Millisecond))
	var pid2 string
	_ = db.QueryRow(`SELECT id FROM payments WHERE merchant_id = 'mch_1' AND livemode AND status = 'processing' LIMIT 1`).Scan(&pid2)
	measure("complete payment (alone)", func() error {
		return a.CompletePayment(ctx, pid2, domain.Outcome{Success: true, OperatorReference: "Y"})
	})

	// The plans of the queries that grow with the data.
	explain := func(name, q string, args ...any) {
		rows, err := db.Query("EXPLAIN (ANALYZE, BUFFERS) "+q, args...)
		if err != nil {
			t.Logf("explain %s: %v", name, err)
			return
		}
		defer rows.Close()
		var b strings.Builder
		for rows.Next() {
			var line string
			_ = rows.Scan(&line)
			b.WriteString("    " + line + "\n")
		}
		t.Logf("EXPLAIN %s\n%s", name, b.String())
	}
	explain("Collected (all time)", `SELECT integration, merchant_id, COUNT(*) AS count, COALESCE(SUM(amount),0), COALESCE(SUM(fee),0), COALESCE(SUM(platform_fee),0),
		COALESCE(SUM(partner_fee),0), COALESCE(SUM(amount_refunded),0) FROM payments WHERE livemode AND status IN ('succeeded','refunded','partially_refunded')
		GROUP BY integration, merchant_id ORDER BY integration, 4 DESC`)
	explain("HeldFor(mch_1)", `SELECT source, SUM(total) AS total FROM (
		SELECT integration AS source, COALESCE(SUM(net - amount_refunded),0) AS total FROM payments
			WHERE merchant_id = 'mch_1' AND livemode AND status IN ('succeeded','refunded','partially_refunded') AND integration <> '' GROUP BY integration
		UNION ALL SELECT source, -COALESCE(SUM(amount),0) FROM payout_funding WHERE merchant_id = 'mch_1' AND status <> 'cancelled' GROUP BY source) t GROUP BY source`)
	explain("Ledger balance(mch_1)", `SELECT COALESCE(SUM(amount),0) FROM ledger_entries WHERE merchant_id = 'mch_1' AND livemode`)
	explain("MainMethod(mch_1)", `SELECT method FROM payments WHERE merchant_id = 'mch_1' AND livemode AND status IN ('succeeded','refunded','partially_refunded') GROUP BY method ORDER BY SUM(amount) DESC LIMIT 1`)
	explain("admin payments count (live)", `SELECT count(*) FROM payments WHERE livemode = true`)
	explain("admin payments paid_late count", `SELECT count(*) FROM payments WHERE livemode = true AND paid_late`)
	explain("admin payments duplicates count", `SELECT count(*) FROM payments WHERE livemode = true AND duplicate_of <> '' AND status IN ('succeeded')`)
	explain("admin payments offset 100k", `SELECT * FROM payments WHERE livemode = true ORDER BY created_at DESC, id DESC LIMIT 50 OFFSET 99950`)
	explain("admin payments search", `SELECT * FROM payments WHERE livemode = true AND (LOWER(customer_name) LIKE '%cliente 4242%' OR LOWER(reference) LIKE '%cliente 4242%' OR LOWER(client_reference_id) LIKE '%cliente 4242%'
		OR id = 'cliente 4242' OR phone_last4 = 'cliente 4242' OR attempt_id = 'CLIENTE 4242' OR rail_reference = 'cliente 4242' OR rail_id = 'cliente 4242' OR operator_reference = 'cliente 4242')
		ORDER BY created_at DESC, id DESC LIMIT 50`)
	explain("FindThreads('nothing-matches')", `select p.id from payments p join merchants m on m.id = p.merchant_id
		where p.id = $1 or upper(p.reference) = upper($1) or p.rail_reference = $1 or p.session_id = $1 or upper(p.attempt_id) = upper($1)
		   or p.rail_id = $1 or (length($1) = 4 and p.phone_last4 = $1)
		   or p.id in (select payment_id from trail_events where request_id = $1 and payment_id <> '')
		   or p.session_id in (select session_id from trail_events where request_id = $1 and session_id <> '')
		order by p.created_at desc limit 50`, "nothing-matches")
	explain("trail feed search", `SELECT * FROM trail_events WHERE (title ILIKE '%zzz%' OR path ILIKE '%zzz%' OR request_id = 'zzz' OR payment_id = 'zzz' OR session_id = 'zzz') ORDER BY created_at DESC LIMIT 100`)
	explain("trail feed level=warn 48h", `SELECT * FROM trail_events WHERE level = 'warn' AND created_at >= now() - interval '48 hours' ORDER BY created_at DESC LIMIT 100`)
	explain("metrics webhooks 7d", `select count(*) from webhook_deliveries d where d.livemode = true and d.created_at >= now() - interval '7 days'`)
	explain("health stuck payments", `select count(*) from payments where status = 'processing' and created_at < now() - interval '15 minutes'`)
	explain("health failed webhooks", `select count(*) from webhook_deliveries where status = 'failed' and updated_at >= now() - interval '15 minutes'`)
	explain("health trail errors", `select count(*) from trail_events where kind = 'log' and level = 'error' and created_at >= now() - interval '15 minutes'`)
	explain("trail prune bodies", `SELECT count(*) FROM trail_events WHERE created_at < now() - interval '90 days' AND (request <> '' OR response <> '')`)
	explain("PartnerHome CollectedDaily 30d", `SELECT to_char(succeeded_at AT TIME ZONE 'Africa/Maputo', 'YYYY-MM-DD') AS day, COUNT(*), COALESCE(SUM(amount),0), COALESCE(SUM(partner_fee),0)
		FROM payments WHERE livemode AND status IN ('succeeded','refunded','partially_refunded') AND integration IN ('mtoca') AND succeeded_at >= now() - interval '30 days' AND succeeded_at < now() GROUP BY day ORDER BY day`)
	explain("tag payments by integration (boot)", `SELECT count(*) FROM payments WHERE integration = ''`)
	explain("unpriced (boot)", `SELECT * FROM payments WHERE NOT priced AND integration <> '' ORDER BY created_at LIMIT 500`)

	var sizes []string
	rows, err := db.Query(`SELECT relname, pg_size_pretty(pg_total_relation_size(relid)) FROM pg_stat_user_tables ORDER BY pg_total_relation_size(relid) DESC LIMIT 8`)
	if err == nil {
		for rows.Next() {
			var name, size string
			_ = rows.Scan(&name, &size)
			sizes = append(sizes, fmt.Sprintf("%s=%s", name, size))
		}
		rows.Close()
	}
	t.Logf("table sizes: %s", strings.Join(sizes, ", "))
}

// TestPerfExplainMore explains, on the database TestPerfScale left, the
// metrics page's queries one by one and the merchant list's counts.
func TestPerfExplainMore(t *testing.T) {
	dsn := os.Getenv("RIHA_PERF_DATABASE_URL")
	if dsn == "" || os.Getenv("RIHA_PERF_KEEP") == "" {
		t.Skip("needs RIHA_PERF_DATABASE_URL and RIHA_PERF_KEEP on a filled database")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	run := func(name, q string) {
		start := time.Now()
		rows, err := db.Query("EXPLAIN (ANALYZE) " + q)
		if err != nil {
			t.Logf("%s: %v", name, err)
			return
		}
		var plan []string
		for rows.Next() {
			var l string
			_ = rows.Scan(&l)
			if strings.Contains(l, "Scan") || strings.Contains(l, "Execution Time") || strings.Contains(l, "Removed") {
				plan = append(plan, strings.TrimSpace(l))
			}
		}
		rows.Close()
		t.Logf("%-34s %v\n    %s", name, time.Since(start).Round(time.Millisecond), strings.Join(plan, "\n    "))
	}
	w := `p.livemode and p.created_at >= now() - interval '7 days' and p.created_at < now()`
	run("metrics daily 7d", `select to_char(date_trunc('day', p.created_at at time zone 'Africa/Maputo'), 'YYYY-MM-DD'), p.method, count(*) from payments p where `+w+` group by 1,2`)
	run("metrics methods p50 7d", `select p.method, percentile_cont(0.5) within group (order by extract(epoch from p.succeeded_at - p.created_at)) from payments p where `+w+` group by 1`)
	run("metrics operator 7d", `select p.method, count(*), percentile_cont(0.5) within group (order by t.duration_ms) from trail_events t join payments p on p.id = t.payment_id
		where t.kind = 'rail.response' and p.livemode and t.created_at >= now() - interval '7 days' and t.created_at < now() group by 1`)
	run("metrics networks 7d", `select t.network, count(*) from trail_events t join payments p on p.id = t.payment_id
		where t.kind = 'payment.submitted' and p.livemode and t.created_at >= now() - interval '7 days' and t.created_at < now() group by 1 order by 2 desc limit 12`)
	run("metrics funnel 7d", `select count(*), count(*) filter (where exists (select 1 from payments x where x.session_id = cs.id)) from checkout_sessions cs
		where cs.livemode and cs.created_at >= now() - interval '7 days' and cs.created_at < now()`)
	run("metrics api 7d", `select count(*), percentile_cont(0.9) within group (order by r.duration_ms) from api_request_logs r where r.livemode and r.created_at >= now() - interval '7 days'`)
	run("merchant count paid (biggest)", `select count(*) from payments where merchant_id = 'mch_1' and livemode and status in ('succeeded','refunded','partially_refunded')`)
	run("merchant count all (biggest)", `select count(*) from payments where merchant_id = 'mch_1' and livemode`)
	run("merchant page (biggest)", `select * from payments where merchant_id = 'mch_1' and livemode order by created_at desc, id desc limit 50`)
	run("merchant page (small)", `select * from payments where merchant_id = 'mch_900' and livemode order by created_at desc, id desc limit 50`)
	run("merchant page paid (small)", `select * from payments where merchant_id = 'mch_900' and livemode and status in ('succeeded','refunded','partially_refunded') order by created_at desc, id desc limit 50`)
	run("settled volume 30d (biggest)", `select coalesce(sum(amount),0) from payments where livemode and merchant_id = 'mch_1' and created_at >= now() - interval '30 days' and status in ('succeeded','refunded','partially_refunded')`)
}
