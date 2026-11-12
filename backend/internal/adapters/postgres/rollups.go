package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"riha/internal/core/domain"
)

// Rollups keep the sums RIHA reads all the time, so no screen adds up the
// payments or the ledger from the beginning:
//
//   - payment_days: every payment, by the day it was made (Maputo time),
//     method, integration, status and failure, with how many and how much;
//   - settled_days: paid payments, by the day they were paid, with their
//     fees, the partners' part and refunds: what settlement reads;
//   - merchant_balances: each merchant's balance, per mode.
//
// Triggers keep them in the same transaction as the payment or ledger row
// they sum, so they can't be bypassed nor fall behind. A nightly check adds
// everything up again from the rows and rebuilds a rollup that drifted.

type paymentDayRecord struct {
	Day         time.Time `gorm:"type:date;primaryKey"`
	MerchantID  string    `gorm:"size:40;primaryKey"`
	Livemode    bool      `gorm:"primaryKey"`
	Method      string    `gorm:"size:10;primaryKey"`
	Integration string    `gorm:"size:30;primaryKey"`
	Status      string    `gorm:"size:24;primaryKey"`
	FailureCode string    `gorm:"size:40;primaryKey"`
	N           int64     `gorm:"not null;default:0"`
	Amount      int64     `gorm:"not null;default:0"`
}

func (paymentDayRecord) TableName() string { return "payment_days" }

type settledDayRecord struct {
	Day         time.Time `gorm:"type:date;primaryKey"`
	MerchantID  string    `gorm:"size:40;primaryKey"`
	Livemode    bool      `gorm:"primaryKey"`
	Integration string    `gorm:"size:30;primaryKey"`
	Method      string    `gorm:"size:10;primaryKey"`
	N           int64     `gorm:"not null;default:0"`
	Amount      int64     `gorm:"not null;default:0"`
	Net         int64     `gorm:"not null;default:0"`
	Fee         int64     `gorm:"not null;default:0"`
	PlatformFee int64     `gorm:"not null;default:0"`
	PartnerFee  int64     `gorm:"not null;default:0"`
	Refunded    int64     `gorm:"not null;default:0"`
}

func (settledDayRecord) TableName() string { return "settled_days" }

type merchantBalanceRecord struct {
	MerchantID string `gorm:"size:40;primaryKey"`
	Livemode   bool   `gorm:"primaryKey"`
	Balance    int64  `gorm:"not null;default:0"`
}

func (merchantBalanceRecord) TableName() string { return "merchant_balances" }

// schemaRunRecord remembers a one-time change to the data, so it runs once.
type schemaRunRecord struct {
	Name  string `gorm:"size:80;primaryKey"`
	RanAt time.Time
}

func (schemaRunRecord) TableName() string { return "schema_runs" }

func rollupRecords() []any {
	return []any{&paymentDayRecord{}, &settledDayRecord{}, &merchantBalanceRecord{}, &schemaRunRecord{}}
}

func settledList() string {
	return "'" + strings.Join(domain.SettledStatuses, "','") + "'"
}

// rollupTriggers are created again on every start, so a fix to them ships
// with the code. Within one trigger the two rows it changes are always taken
// in the same order, so two payments never wait on each other's rollups.
func rollupTriggers() string {
	return fmt.Sprintf(`
CREATE OR REPLACE FUNCTION riha_pd_add(d date, m text, l boolean, me text, i text, s text, f text, dn bigint, da bigint) RETURNS void AS $$
BEGIN
	INSERT INTO payment_days (day, merchant_id, livemode, method, integration, status, failure_code, n, amount)
	VALUES (d, m, l, me, i, s, f, dn, da)
	ON CONFLICT (day, merchant_id, livemode, method, integration, status, failure_code)
	DO UPDATE SET n = payment_days.n + EXCLUDED.n, amount = payment_days.amount + EXCLUDED.amount;
END $$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION riha_sd_add(d date, m text, l boolean, i text, me text, dn bigint, da bigint, dnet bigint, df bigint, dpf bigint, dptf bigint, dr bigint) RETURNS void AS $$
BEGIN
	INSERT INTO settled_days (day, merchant_id, livemode, integration, method, n, amount, net, fee, platform_fee, partner_fee, refunded)
	VALUES (d, m, l, i, me, dn, da, dnet, df, dpf, dptf, dr)
	ON CONFLICT (day, merchant_id, livemode, integration, method)
	DO UPDATE SET n = settled_days.n + EXCLUDED.n, amount = settled_days.amount + EXCLUDED.amount, net = settled_days.net + EXCLUDED.net,
		fee = settled_days.fee + EXCLUDED.fee, platform_fee = settled_days.platform_fee + EXCLUDED.platform_fee,
		partner_fee = settled_days.partner_fee + EXCLUDED.partner_fee, refunded = settled_days.refunded + EXCLUDED.refunded;
END $$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION riha_payment_pd(p payments, sign int) RETURNS void AS $$
BEGIN
	PERFORM riha_pd_add((p.created_at AT TIME ZONE 'Africa/Maputo')::date, p.merchant_id, p.livemode, p.method, p.integration, p.status,
		COALESCE(p.failure_code, ''), sign, sign * p.amount);
END $$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION riha_payment_sd(p payments, sign int) RETURNS void AS $$
BEGIN
	IF p.status IN (%[1]s) AND p.succeeded_at IS NOT NULL THEN
		PERFORM riha_sd_add((p.succeeded_at AT TIME ZONE 'Africa/Maputo')::date, p.merchant_id, p.livemode, p.integration, p.method,
			sign, sign * p.amount, sign * p.net, sign * p.fee, sign * p.platform_fee, sign * p.partner_fee, sign * p.amount_refunded);
	END IF;
END $$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION riha_payment_rollup() RETURNS trigger AS $$
DECLARE
	ok text; nk text;
BEGIN
	IF TG_OP = 'INSERT' THEN
		PERFORM riha_payment_pd(NEW, 1);
		PERFORM riha_payment_sd(NEW, 1);
		RETURN NULL;
	ELSIF TG_OP = 'DELETE' THEN
		PERFORM riha_payment_pd(OLD, -1);
		PERFORM riha_payment_sd(OLD, -1);
		RETURN NULL;
	END IF;
	IF (OLD.created_at, OLD.merchant_id, OLD.livemode, OLD.method, OLD.integration, OLD.status, OLD.failure_code, OLD.amount)
		IS DISTINCT FROM (NEW.created_at, NEW.merchant_id, NEW.livemode, NEW.method, NEW.integration, NEW.status, NEW.failure_code, NEW.amount) THEN
		ok := concat_ws('|', OLD.created_at::date, OLD.merchant_id, OLD.livemode, OLD.method, OLD.integration, OLD.status, OLD.failure_code);
		nk := concat_ws('|', NEW.created_at::date, NEW.merchant_id, NEW.livemode, NEW.method, NEW.integration, NEW.status, NEW.failure_code);
		IF ok <= nk THEN
			PERFORM riha_payment_pd(OLD, -1); PERFORM riha_payment_pd(NEW, 1);
		ELSE
			PERFORM riha_payment_pd(NEW, 1); PERFORM riha_payment_pd(OLD, -1);
		END IF;
	END IF;
	IF (OLD.status, OLD.succeeded_at, OLD.merchant_id, OLD.livemode, OLD.integration, OLD.method, OLD.amount, OLD.net, OLD.fee, OLD.platform_fee, OLD.partner_fee, OLD.amount_refunded)
		IS DISTINCT FROM (NEW.status, NEW.succeeded_at, NEW.merchant_id, NEW.livemode, NEW.integration, NEW.method, NEW.amount, NEW.net, NEW.fee, NEW.platform_fee, NEW.partner_fee, NEW.amount_refunded) THEN
		ok := concat_ws('|', OLD.succeeded_at::date, OLD.merchant_id, OLD.livemode, OLD.integration, OLD.method);
		nk := concat_ws('|', NEW.succeeded_at::date, NEW.merchant_id, NEW.livemode, NEW.integration, NEW.method);
		IF ok <= nk THEN
			PERFORM riha_payment_sd(OLD, -1); PERFORM riha_payment_sd(NEW, 1);
		ELSE
			PERFORM riha_payment_sd(NEW, 1); PERFORM riha_payment_sd(OLD, -1);
		END IF;
	END IF;
	RETURN NULL;
END $$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS payments_rollup ON payments;
CREATE TRIGGER payments_rollup AFTER INSERT OR UPDATE OR DELETE ON payments FOR EACH ROW EXECUTE FUNCTION riha_payment_rollup();

CREATE OR REPLACE FUNCTION riha_ledger_rollup() RETURNS trigger AS $$
BEGIN
	IF TG_OP IN ('UPDATE', 'DELETE') THEN
		INSERT INTO merchant_balances (merchant_id, livemode, balance) VALUES (OLD.merchant_id, OLD.livemode, -OLD.amount)
		ON CONFLICT (merchant_id, livemode) DO UPDATE SET balance = merchant_balances.balance + EXCLUDED.balance;
	END IF;
	IF TG_OP IN ('INSERT', 'UPDATE') THEN
		INSERT INTO merchant_balances (merchant_id, livemode, balance) VALUES (NEW.merchant_id, NEW.livemode, NEW.amount)
		ON CONFLICT (merchant_id, livemode) DO UPDATE SET balance = merchant_balances.balance + EXCLUDED.balance;
	END IF;
	RETURN NULL;
END $$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS ledger_rollup ON ledger_entries;
CREATE TRIGGER ledger_rollup AFTER INSERT OR UPDATE OR DELETE ON ledger_entries FOR EACH ROW EXECUTE FUNCTION riha_ledger_rollup();
`, settledList())
}

// Expected rollups, added up from the rows themselves.
func expectedPaymentDays() string {
	return `SELECT (created_at AT TIME ZONE 'Africa/Maputo')::date AS day, merchant_id, livemode, method, integration, status, COALESCE(failure_code, '') AS failure_code,
		COUNT(*) AS n, COALESCE(SUM(amount), 0) AS amount FROM payments GROUP BY 1, 2, 3, 4, 5, 6, 7`
}

func expectedSettledDays() string {
	return `SELECT (succeeded_at AT TIME ZONE 'Africa/Maputo')::date AS day, merchant_id, livemode, integration, method,
		COUNT(*) AS n, SUM(amount) AS amount, SUM(net) AS net, SUM(fee) AS fee, SUM(platform_fee) AS platform_fee, SUM(partner_fee) AS partner_fee,
		SUM(amount_refunded) AS refunded FROM payments WHERE status IN (` + settledList() + `) AND succeeded_at IS NOT NULL GROUP BY 1, 2, 3, 4, 5`
}

const expectedBalances = `SELECT merchant_id, livemode, SUM(amount) AS balance FROM ledger_entries GROUP BY 1, 2`

// rebuildRollups adds every rollup up again from the rows, with writers held
// off the rollups while it does.
func rebuildRollups(tx *gorm.DB) error {
	for _, q := range []string{
		`LOCK TABLE payment_days, settled_days, merchant_balances IN EXCLUSIVE MODE`,
		`DELETE FROM payment_days`,
		`INSERT INTO payment_days (day, merchant_id, livemode, method, integration, status, failure_code, n, amount) ` + expectedPaymentDays(),
		`DELETE FROM settled_days`,
		`INSERT INTO settled_days (day, merchant_id, livemode, integration, method, n, amount, net, fee, platform_fee, partner_fee, refunded) ` + expectedSettledDays(),
		`DELETE FROM merchant_balances`,
		`INSERT INTO merchant_balances (merchant_id, livemode, balance) ` + expectedBalances,
	} {
		if err := tx.Exec(q).Error; err != nil {
			return err
		}
	}
	return nil
}

// performanceIndexes are what the busy screens and the workers filter on.
var performanceIndexes = []string{
	`CREATE INDEX IF NOT EXISTS payments_merchant_created ON payments (merchant_id, livemode, created_at DESC)`,
	`CREATE INDEX IF NOT EXISTS payments_live_created ON payments (livemode, created_at DESC)`,
	`CREATE INDEX IF NOT EXISTS payments_unpriced ON payments (created_at) WHERE NOT priced`,
	`CREATE INDEX IF NOT EXISTS payments_paid_late ON payments (created_at DESC) WHERE paid_late`,
	`CREATE INDEX IF NOT EXISTS payments_duplicates ON payments (created_at DESC) WHERE duplicate_of <> ''`,
	`CREATE INDEX IF NOT EXISTS payments_unconfirmed ON payments (livemode, created_at DESC) WHERE status = 'unconfirmed'`,
	`CREATE INDEX IF NOT EXISTS payments_operator_ref ON payments (operator_reference) WHERE operator_reference <> ''`,
	`CREATE INDEX IF NOT EXISTS payments_rail_id ON payments (rail_id) WHERE rail_id <> ''`,
	`CREATE INDEX IF NOT EXISTS payments_phone_last4 ON payments (phone_last4, created_at DESC)`,
	`CREATE INDEX IF NOT EXISTS payout_funding_source_status ON payout_funding (source, status, created_at DESC)`,
	`CREATE INDEX IF NOT EXISTS webhook_deliveries_due ON webhook_deliveries (next_attempt_at) WHERE status = 'pending'`,
	`CREATE INDEX IF NOT EXISTS trail_events_level ON trail_events (created_at DESC) WHERE level <> 'info'`,
	`CREATE INDEX IF NOT EXISTS events_created ON events (created_at)`,
	`CREATE INDEX IF NOT EXISTS webhook_deliveries_created ON webhook_deliveries (created_at)`,
}

// migrateRollups creates the triggers and indexes, and fills the rollups the
// first time.
func (s *Store) migrateRollups() error {
	if err := s.db.Exec(rollupTriggers()).Error; err != nil {
		return fmt.Errorf("rollup triggers: %w", err)
	}
	for _, q := range performanceIndexes {
		if err := s.db.Exec(q).Error; err != nil {
			return fmt.Errorf("index: %w", err)
		}
	}
	return s.once("rollups_v1", rebuildRollups)
}

// once runs a change to the data a single time, in its own transaction,
// recording that it ran.
func (s *Store) once(name string, fn func(tx *gorm.DB) error) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		res := tx.Exec(`INSERT INTO schema_runs (name, ran_at) VALUES (?, ?) ON CONFLICT DO NOTHING`, name, time.Now().UTC())
		if res.Error != nil || res.RowsAffected == 0 {
			return res.Error
		}
		return fn(tx)
	})
}

// RollupDrift is how many rollup rows differ from adding the rows up again.
func (r Reports) RollupDrift(ctx context.Context) (int64, error) {
	var n int64
	err := r.conn(ctx).Raw(`SELECT
		(SELECT COUNT(*) FROM (` + expectedPaymentDays() + `) e FULL JOIN (SELECT * FROM payment_days WHERE n <> 0 OR amount <> 0) a
			USING (day, merchant_id, livemode, method, integration, status, failure_code)
			WHERE e.n IS DISTINCT FROM a.n OR e.amount IS DISTINCT FROM a.amount)
		+ (SELECT COUNT(*) FROM (` + expectedSettledDays() + `) e FULL JOIN (SELECT * FROM settled_days WHERE n <> 0) a
			USING (day, merchant_id, livemode, integration, method)
			WHERE (e.n, e.amount, e.net, e.fee, e.platform_fee, e.partner_fee, e.refunded)
				IS DISTINCT FROM (a.n, a.amount, a.net, a.fee, a.platform_fee, a.partner_fee, a.refunded))
		+ (SELECT COUNT(*) FROM (` + expectedBalances + `) e FULL JOIN (SELECT * FROM merchant_balances WHERE balance <> 0) a
			USING (merchant_id, livemode) WHERE COALESCE(e.balance, 0) <> COALESCE(a.balance, 0))`).Scan(&n).Error
	return n, err
}

func (r Reports) RebuildRollups(ctx context.Context) error {
	return r.Do(ctx, func(ctx context.Context) error { return rebuildRollups(r.conn(ctx)) })
}

// maputoDay is the Maputo calendar day of t, as midnight UTC: how dates come
// back from the rollups.
func maputoDay(t time.Time) time.Time {
	m := t.In(maputo)
	return time.Date(m.Year(), m.Month(), m.Day(), 0, 0, 0, 0, time.UTC)
}

// maputoMidnight tells whether t starts a day in Maputo: a bound the daily
// rollups can answer exactly.
func maputoMidnight(t *time.Time) bool {
	if t == nil {
		return true
	}
	m := t.In(maputo)
	return m.Hour() == 0 && m.Minute() == 0 && m.Second() == 0 && m.Nanosecond() == 0
}

var maputo = func() *time.Location {
	if l, err := time.LoadLocation("Africa/Maputo"); err == nil {
		return l
	}
	return time.FixedZone("CAT", 2*3600)
}()
