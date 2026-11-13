package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

type Endpoints struct{ *Store }

func (r Endpoints) Create(ctx context.Context, e *domain.WebhookEndpoint) error {
	return create[endpointRecord](r.conn(ctx), e)
}
func (r Endpoints) ByID(ctx context.Context, merchantID, id string) (*domain.WebhookEndpoint, error) {
	return first[endpointRecord, domain.WebhookEndpoint](r.conn(ctx).Where("id = ? AND merchant_id = ?", id, merchantID))
}
func (r Endpoints) Save(ctx context.Context, e *domain.WebhookEndpoint) error {
	return save[endpointRecord](r.conn(ctx), e)
}
func (r Endpoints) Delete(ctx context.Context, merchantID string, live *bool, id string) (bool, error) {
	q := r.conn(ctx).Where("id = ? AND merchant_id = ?", id, merchantID)
	if live != nil {
		q = q.Where("livemode = ?", *live)
	}
	res := q.Delete(&endpointRecord{})
	return res.RowsAffected > 0, res.Error
}
func (r Endpoints) List(ctx context.Context, merchantID string, live bool) ([]domain.WebhookEndpoint, error) {
	return find[endpointRecord, domain.WebhookEndpoint](r.conn(ctx).Where("merchant_id = ? AND livemode = ?", merchantID, live).Order("created_at"))
}

type Domains struct{ *Store }

func (r Domains) Create(ctx context.Context, d *domain.AllowedDomain) error {
	return create[domainRecord](r.conn(ctx), d)
}
func (r Domains) List(ctx context.Context, merchantID string) ([]domain.AllowedDomain, error) {
	return find[domainRecord, domain.AllowedDomain](r.conn(ctx).Where("merchant_id = ?", merchantID).Order("created_at"))
}
func (r Domains) MarkVerified(ctx context.Context, merchantID, id string, at time.Time) error {
	return r.conn(ctx).Model(&domainRecord{}).Where("id = ? AND merchant_id = ?", id, merchantID).Update("verified_at", at).Error
}
func (r Domains) Delete(ctx context.Context, merchantID, id string) error {
	return r.conn(ctx).Where("id = ? AND merchant_id = ?", id, merchantID).Delete(&domainRecord{}).Error
}

type Events struct{ *Store }

func (r Events) Create(ctx context.Context, e *domain.Event) error {
	return create[eventRecord](r.conn(ctx), e)
}
func (r Events) ByID(ctx context.Context, id string) (*domain.Event, error) {
	return first[eventRecord, domain.Event](r.conn(ctx).Where("id = ?", id))
}
func (r Events) ByObjects(ctx context.Context, merchantID string, objectIDs []string) ([]domain.Event, error) {
	return find[eventRecord, domain.Event](r.conn(ctx).Where("merchant_id = ? AND object_id IN ?", merchantID, objectIDs).Order("created_at"))
}

type Deliveries struct{ *Store }

func (r Deliveries) Create(ctx context.Context, d *domain.Delivery) error {
	return create[deliveryRecord](r.conn(ctx), d)
}
func (r Deliveries) ClaimDue(ctx context.Context, now, lease time.Time, limit int) ([]domain.Delivery, error) {
	var out []domain.Delivery
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rs []deliveryRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status = ? AND next_attempt_at <= ?", domain.DeliveryPending, now).Order("next_attempt_at").Limit(limit).Find(&rs).Error; err != nil {
			return err
		}
		ids := make([]string, len(rs))
		for i := range rs {
			ids[i] = rs[i].ID
		}
		if len(ids) > 0 {
			if err := tx.Model(&deliveryRecord{}).Where("id IN ?", ids).Update("next_attempt_at", lease).Error; err != nil {
				return err
			}
		}
		out = many[domain.Delivery](rs)
		return nil
	})
	return out, err
}
func (r Deliveries) Save(ctx context.Context, d *domain.Delivery) error {
	return save[deliveryRecord](r.conn(ctx), d)
}
func (r Deliveries) Requeue(ctx context.Context, merchantID, id string, at time.Time) (bool, error) {
	res := r.conn(ctx).Model(&deliveryRecord{}).Where("id = ? AND merchant_id = ?", id, merchantID).
		Updates(map[string]any{"status": domain.DeliveryPending, "next_attempt_at": at})
	return res.RowsAffected > 0, res.Error
}
func (r Deliveries) Recent(ctx context.Context, merchantID string, live bool, limit int) ([]domain.Delivery, error) {
	return find[deliveryRecord, domain.Delivery](r.conn(ctx).Where("merchant_id = ? AND livemode = ?", merchantID, live).Order("created_at DESC").Limit(limit))
}
func (r Deliveries) ByEvents(ctx context.Context, eventIDs []string) ([]domain.Delivery, error) {
	return find[deliveryRecord, domain.Delivery](r.conn(ctx).Where("event_id IN ?", eventIDs).Order("created_at"))
}

type RequestLogs struct{ *Store }

func (r RequestLogs) Create(ctx context.Context, l *domain.RequestLog) error {
	return create[requestLogRecord](r.conn(ctx), l)
}
func (r RequestLogs) Recent(ctx context.Context, merchantID string, live bool, limit int) ([]domain.RequestLog, error) {
	return find[requestLogRecord, domain.RequestLog](r.conn(ctx).Where("merchant_id = ? AND livemode = ?", merchantID, live).Order("created_at DESC").Limit(limit))
}

type Idempotency struct{ *Store }

func (r Idempotency) Get(ctx context.Context, merchantID string, live bool, key string) (*domain.IdempotencyRecord, error) {
	return first[idempotencyRecord, domain.IdempotencyRecord](r.conn(ctx).Where("merchant_id = ? AND livemode = ? AND key = ?", merchantID, live, key))
}
func (r Idempotency) Save(ctx context.Context, x *domain.IdempotencyRecord) error {
	return create[idempotencyRecord](r.conn(ctx), x)
}

// Claim takes a key before its request runs: the first of two requests at
// once wins, and the other finds the claim.
func (r Idempotency) Claim(ctx context.Context, x *domain.IdempotencyRecord) (bool, error) {
	res := r.conn(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(toRecord[idempotencyRecord](x))
	return res.RowsAffected == 1, res.Error
}

func (r Idempotency) Finish(ctx context.Context, id string, status int, body string) error {
	return r.conn(ctx).Model(&idempotencyRecord{}).Where("id = ?", id).Updates(map[string]any{"status_code": status, "response_body": body}).Error
}

func (r Idempotency) Release(ctx context.Context, id string) error {
	return r.conn(ctx).Where("id = ? AND status_code = 0", id).Delete(&idempotencyRecord{}).Error
}
func (r Idempotency) Purge(ctx context.Context, before time.Time) error {
	return r.conn(ctx).Where("created_at < ?", before).Delete(&idempotencyRecord{}).Error
}

type Audit struct{ *Store }

func (r Audit) Record(ctx context.Context, e *domain.AuditEntry) error {
	return create[auditRecord](r.conn(ctx), e)
}
func (r Audit) ByMerchant(ctx context.Context, merchantID string, limit int) ([]domain.AuditEntry, error) {
	return find[auditRecord, domain.AuditEntry](r.conn(ctx).Where("merchant_id = ?", merchantID).Order("created_at DESC").Limit(limit))
}

type Settings struct{ *Store }

func (r Settings) Get(ctx context.Context, key string) (string, error) {
	var s settingRecord
	if err := r.conn(ctx).First(&s, "key = ?", key).Error; err != nil {
		return "", wrap(err)
	}
	return s.Value, nil
}
func (r Settings) Set(ctx context.Context, key, value string) error {
	return r.conn(ctx).Save(&settingRecord{Key: key, Value: value, UpdatedAt: time.Now().UTC()}).Error
}

type Reports struct{ *Store }

func (r Reports) scope(ctx context.Context, s ports.Scope) *gorm.DB {
	q := r.conn(ctx).Model(&paymentRecord{}).Where("livemode = ?", s.Live)
	if s.MerchantID != "" {
		q = q.Where("merchant_id = ?", s.MerchantID)
	}
	if s.From != nil {
		q = q.Where("created_at >= ?", *s.From)
	}
	if s.To != nil {
		q = q.Where("created_at < ?", *s.To)
	}
	return q
}

// days answers from the payment_days rollup when the scope's bounds start
// Maputo days, as the console's and the reports' do; any other bounds read
// the payments themselves, within those bounds.
func (r Reports) days(ctx context.Context, s ports.Scope) (*gorm.DB, bool) {
	if !maputoMidnight(s.From) || !maputoMidnight(s.To) {
		return nil, false
	}
	q := r.conn(ctx).Model(&paymentDayRecord{}).Where("livemode = ?", s.Live)
	if s.MerchantID != "" {
		q = q.Where("merchant_id = ?", s.MerchantID)
	}
	if s.From != nil {
		q = q.Where("day >= ?", maputoDay(*s.From))
	}
	if s.To != nil {
		q = q.Where("day < ?", maputoDay(*s.To))
	}
	return q, true
}

func (r Reports) SettledVolume(ctx context.Context, s ports.Scope) (int64, error) {
	var v int64
	if q, ok := r.days(ctx, s); ok {
		err := q.Where("status IN ?", domain.SettledStatuses).Select("COALESCE(SUM(amount),0)").Scan(&v).Error
		return v, err
	}
	err := r.scope(ctx, s).Where("status IN ?", domain.SettledStatuses).Select("COALESCE(SUM(amount),0)").Scan(&v).Error
	return v, err
}
func (r Reports) SettledCount(ctx context.Context, s ports.Scope) (int64, error) {
	var n int64
	if q, ok := r.days(ctx, s); ok {
		err := q.Where("status IN ?", domain.SettledStatuses).Select("COALESCE(SUM(n),0)").Scan(&n).Error
		return n, err
	}
	err := r.scope(ctx, s).Where("status IN ?", domain.SettledStatuses).Count(&n).Error
	return n, err
}
func (r Reports) Attempts(ctx context.Context, s ports.Scope) (int64, error) {
	var n int64
	if q, ok := r.days(ctx, s); ok {
		err := q.Select("COALESCE(SUM(n),0)").Scan(&n).Error
		return n, err
	}
	err := r.scope(ctx, s).Count(&n).Error
	return n, err
}
func (r Reports) VolumeBuckets(ctx context.Context, s ports.Scope, unit string) ([]ports.Bucket, error) {
	if unit != "month" {
		unit = "day"
	}
	var out []ports.Bucket
	if q, ok := r.days(ctx, s); ok {
		err := q.Where("status IN ?", domain.SettledStatuses).
			Select(fmt.Sprintf("date_trunc('%s', day)::date AS date, SUM(amount) AS amount", unit)).Group("date").Order("date").Scan(&out).Error
		return out, err
	}
	err := r.scope(ctx, s).Where("status IN ?", domain.SettledStatuses).
		Select(fmt.Sprintf("date_trunc('%s', created_at) AS date, SUM(amount) AS amount", unit)).Group("date").Order("date").Scan(&out).Error
	return out, err
}
func (r Reports) VolumeByMethod(ctx context.Context, s ports.Scope) (map[string]int64, error) {
	var rows []ports.Bucket
	q, ok := r.days(ctx, s)
	if !ok {
		q = r.scope(ctx, s)
	}
	err := q.Where("status IN ?", domain.SettledStatuses).Select("method, SUM(amount) AS amount").Group("method").Scan(&rows).Error
	out := map[string]int64{}
	for _, x := range rows {
		out[x.Method] = x.Amount
	}
	return out, err
}
func (r Reports) VolumeByDayAndMethod(ctx context.Context, s ports.Scope) ([]ports.Bucket, error) {
	var out []ports.Bucket
	if q, ok := r.days(ctx, s); ok {
		err := q.Where("status IN ?", domain.SettledStatuses).Select("day AS date, method, SUM(amount) AS amount").Group("1, 2").Scan(&out).Error
		return out, err
	}
	err := r.scope(ctx, s).Where("status IN ?", domain.SettledStatuses).
		Select("date_trunc('day', created_at) AS date, method, SUM(amount) AS amount").Group("1, 2").Scan(&out).Error
	return out, err
}
func (r Reports) Health(ctx context.Context, since time.Time) ([]ports.MethodHealth, error) {
	var out []ports.MethodHealth
	err := r.conn(ctx).Raw(`SELECT method, COUNT(*) AS total,
		SUM(CASE WHEN status IN ? THEN 1 ELSE 0 END) AS succeeded,
		SUM(CASE WHEN failure_code = ? THEN 1 ELSE 0 END) AS operator_off
		FROM payments WHERE created_at >= ? AND status <> ? GROUP BY method`,
		domain.SettledStatuses, domain.FailOperatorUnavailable, since, domain.PaymentProcessing).Scan(&out).Error
	return out, err
}
func (r Reports) MainMethod(ctx context.Context, merchantID string) (string, error) {
	var row struct{ Method string }
	err := r.conn(ctx).Raw(`SELECT method FROM settled_days WHERE merchant_id = ? AND livemode GROUP BY method HAVING SUM(n) > 0 ORDER BY SUM(amount) DESC LIMIT 1`,
		merchantID).Scan(&row).Error
	return row.Method, err
}

func (r Reports) PaymentTotal(ctx context.Context, live bool, merchantID, method string, statuses []string) (int64, error) {
	q := r.conn(ctx).Model(&paymentDayRecord{}).Where("livemode = ?", live)
	if merchantID != "" {
		q = q.Where("merchant_id = ?", merchantID)
	}
	if method != "" {
		q = q.Where("method = ?", method)
	}
	if len(statuses) > 0 {
		q = q.Where("status IN ?", statuses)
	}
	var n int64
	err := q.Select("COALESCE(SUM(n),0)").Scan(&n).Error
	return n, err
}

func (r Reports) MerchantSummaries(ctx context.Context, ids []string, since time.Time) (map[string]ports.MerchantSummary, error) {
	out := map[string]ports.MerchantSummary{}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []ports.MerchantSummary
	err := r.conn(ctx).Raw(`SELECT m.id AS merchant_id,
		COALESCE((SELECT SUM(amount) FROM settled_days s WHERE s.merchant_id = m.id AND s.livemode AND s.day >= ?), 0) AS volume,
		COALESCE((SELECT method FROM settled_days s WHERE s.merchant_id = m.id AND s.livemode GROUP BY method HAVING SUM(n) > 0 ORDER BY SUM(amount) DESC LIMIT 1), '') AS main_method,
		COALESCE((SELECT balance FROM merchant_balances b WHERE b.merchant_id = m.id AND b.livemode), 0) AS balance
		FROM unnest(ARRAY[?]::text[]) AS m(id)`, maputoDay(since), ids).Scan(&rows).Error
	for _, x := range rows {
		out[x.MerchantID] = x
	}
	return out, err
}

func (r Reports) Prune(ctx context.Context, before time.Time) (int64, error) {
	var total int64
	for _, q := range []string{
		`DELETE FROM api_request_logs WHERE created_at < ?`,
		`DELETE FROM webhook_deliveries WHERE created_at < ? AND status <> 'pending'`,
		`DELETE FROM events e WHERE e.created_at < ? AND NOT EXISTS (SELECT 1 FROM webhook_deliveries d WHERE d.event_id = e.id)`,
	} {
		res := r.conn(ctx).Exec(q, before)
		if res.Error != nil {
			return total, res.Error
		}
		total += res.RowsAffected
	}
	return total, nil
}

type Emails struct{ *Store }

func (r Emails) Create(ctx context.Context, e *domain.QueuedEmail) error {
	return create[emailRecord](r.conn(ctx), e)
}
func (r Emails) ClaimDue(ctx context.Context, now, lease time.Time, limit int) ([]domain.QueuedEmail, error) {
	var out []domain.QueuedEmail
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rs []emailRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status = ? AND next_attempt_at <= ?", domain.EmailPending, now).Order("next_attempt_at").Limit(limit).Find(&rs).Error; err != nil {
			return err
		}
		ids := make([]string, len(rs))
		for i := range rs {
			ids[i] = rs[i].ID
		}
		if len(ids) > 0 {
			if err := tx.Model(&emailRecord{}).Where("id IN ?", ids).Update("next_attempt_at", lease).Error; err != nil {
				return err
			}
		}
		out = many[domain.QueuedEmail](rs)
		return nil
	})
	return out, err
}
func (r Emails) Save(ctx context.Context, e *domain.QueuedEmail) error {
	return save[emailRecord](r.conn(ctx), e)
}
func (r Emails) Purge(ctx context.Context, before time.Time) error {
	return r.conn(ctx).Where("status <> ? AND created_at < ?", domain.EmailPending, before).Delete(&emailRecord{}).Error
}

type PasswordResets struct{ *Store }

func (r PasswordResets) Create(ctx context.Context, p *domain.PasswordReset) error {
	return create[passwordResetRecord](r.conn(ctx), p)
}
func (r PasswordResets) CountSince(ctx context.Context, userID string, since time.Time) (int64, error) {
	var n int64
	err := r.conn(ctx).Model(&passwordResetRecord{}).Where("user_id = ? AND created_at >= ?", userID, since).Count(&n).Error
	return n, err
}
func (r PasswordResets) Take(ctx context.Context, tokenHash string, now time.Time) (string, error) {
	var rec passwordResetRecord
	err := r.conn(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("token_hash = ? AND used_at IS NULL AND expires_at > ?", tokenHash, now).First(&rec).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", domain.ErrNotFound
	}
	return rec.UserID, err
}
func (r PasswordResets) SpendAll(ctx context.Context, userID string, now time.Time) error {
	return r.conn(ctx).Model(&passwordResetRecord{}).Where("user_id = ? AND used_at IS NULL", userID).Update("used_at", now).Error
}
func (r PasswordResets) Purge(ctx context.Context, before time.Time) error {
	if err := r.conn(ctx).Where("expires_at < ?", before).Delete(&trustedDeviceRecord{}).Error; err != nil {
		return err
	}
	if err := r.conn(ctx).Where("expires_at < ?", before).Delete(&loginCodeRecord{}).Error; err != nil {
		return err
	}
	return r.conn(ctx).Where("expires_at < ?", before).Delete(&passwordResetRecord{}).Error
}

// SaveLoginCode keeps one code per user: a new one replaces the last.
func (r PasswordResets) SaveLoginCode(ctx context.Context, c *domain.LoginCode) error {
	return r.conn(ctx).Save(toRecord[loginCodeRecord](c)).Error
}

func (r PasswordResets) LoginCode(ctx context.Context, userID string) (*domain.LoginCode, error) {
	return first[loginCodeRecord, domain.LoginCode](r.conn(ctx).Where("user_id = ?", userID))
}

// TryLoginCode counts one try at a user's code.
func (r PasswordResets) TryLoginCode(ctx context.Context, userID string) error {
	return r.conn(ctx).Model(&loginCodeRecord{}).Where("user_id = ?", userID).Update("attempts", gorm.Expr("attempts + 1")).Error
}

func (r PasswordResets) SpendLoginCode(ctx context.Context, userID string) error {
	return r.conn(ctx).Where("user_id = ?", userID).Delete(&loginCodeRecord{}).Error
}

func (r PasswordResets) TrustDevice(ctx context.Context, d *domain.TrustedDevice) error {
	return create[trustedDeviceRecord](r.conn(ctx), d)
}

func (r PasswordResets) DeviceTrusted(ctx context.Context, userID, tokenHash string, now time.Time) (bool, error) {
	return exists(r.conn(ctx).Model(&trustedDeviceRecord{}).Where("token_hash = ? AND user_id = ? AND expires_at > ?", tokenHash, userID, now))
}
